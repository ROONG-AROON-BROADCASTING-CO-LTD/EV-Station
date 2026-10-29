[CmdletBinding()]
param([ValidateSet('Recover','Monitor','Backup','RestoreTest')][string]$Action = 'Monitor', [string]$BackupFile)
$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$StateDir = Join-Path $ProjectRoot '.runtime/operations'
New-Item -ItemType Directory -Force -Path $StateDir | Out-Null
$ConfigPath = Join-Path $StateDir 'config.json'
$Config = if (Test-Path $ConfigPath) { Get-Content $ConfigPath -Raw | ConvertFrom-Json } else { $null }
$Mutex = [Threading.Mutex]::new($false, 'Local\RBC-Operations-' + $Action)
if (-not $Mutex.WaitOne(0)) { exit 0 }
function RunDocker([string[]]$Arguments) {
    $result = & docker @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Docker operation failed: $($Arguments[0]) $($result | Select-Object -Last 2)" }
    return $result
}
function Healthy([string]$Url) {
    try { return (Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 12).StatusCode -eq 200 } catch { return $false }
}
function SendAlert([string]$Status) {
    if ($Config.AlertWebhook) {
        Invoke-RestMethod -Method Post -Uri $Config.AlertWebhook -ContentType 'application/json' -Body (@{service='RBC EV Station';status=$Status;timestamp=(Get-Date).ToUniversalTime().ToString('o')} | ConvertTo-Json) -TimeoutSec 15 | Out-Null
    }
}
function Recover {
    & docker info --format '{{.ServerVersion}}' *> $null
    if ($LASTEXITCODE -ne 0) {
        Start-Process -FilePath "$env:ProgramFiles/Docker/Docker/Docker Desktop.exe" -WindowStyle Hidden
        $deadline = (Get-Date).AddSeconds(90)
        do { Start-Sleep -Seconds 3; & docker info --format '{{.ServerVersion}}' *> $null } while ($LASTEXITCODE -ne 0 -and (Get-Date) -lt $deadline)
        if ($LASTEXITCODE -ne 0) { throw 'Docker engine unavailable; inspect Docker Desktop. Never factory-reset customer storage.' }
    }
    # Start only existing deployed containers; recovery never builds or migrates.
    RunDocker @('start','ev-station-db-1','ev-station-redis-1') | Out-Null
    $deadline = (Get-Date).AddSeconds(45)
    do {
        $db = (& docker inspect --format '{{.State.Health.Status}}' ev-station-db-1)
        $redis = (& docker inspect --format '{{.State.Health.Status}}' ev-station-redis-1)
        if ($db -eq 'healthy' -and $redis -eq 'healthy') { break }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    if ($db -ne 'healthy' -or $redis -ne 'healthy') { throw 'Database or Redis not healthy' }
    RunDocker @('start','ev-station-api-1') | Out-Null
    $deadline = (Get-Date).AddSeconds(40)
    while (-not (Healthy 'http://127.0.0.1:8080/health')) { if ((Get-Date) -gt $deadline) { throw 'API recovery failed' }; Start-Sleep -Seconds 2 }
    # Nginx may have started before API DNS existed, so explicitly restart it.
    RunDocker @('restart','ev-station-web-1') | Out-Null
    $service = Get-Service cloudflared
    if ($service.Status -ne 'Running') { Start-Service cloudflared }
}
try {
    switch ($Action) {
        Recover { Recover }
        Monitor {
            $healthy = (Healthy 'http://127.0.0.1:8080/health') -and (Healthy 'https://www.rbcevstation.com/')
            if (-not $healthy) { Recover; $healthy = (Healthy 'http://127.0.0.1:8080/health') -and (Healthy 'https://www.rbcevstation.com/') }
            $state = if ($healthy) { 'healthy' } else { 'down' }
            $previousPath = Join-Path $StateDir 'monitor-state.txt'
            $previous = if (Test-Path $previousPath) { (Get-Content $previousPath -Raw).Trim() } else { 'unknown' }
            if ($previous -ne $state -and $Config.AlertWebhook) {
                # Configure only a user-approved recipient. No customer data is sent.
                SendAlert $state
            }
            Set-Content $previousPath $state
            if (-not $healthy) { throw 'Public service remains unavailable after recovery' }
        }
        Backup {
            $destination = Join-Path $ProjectRoot 'backups/automated'
            New-Item -ItemType Directory -Force $destination | Out-Null
            # Customer backups are restricted to this operator and SYSTEM.
            & icacls $destination /inheritance:r /grant:r "$([Security.Principal.WindowsIdentity]::GetCurrent().Name):(OI)(CI)F" 'SYSTEM:(OI)(CI)F' | Out-Null
            if ($LASTEXITCODE -ne 0) { throw 'Could not protect backup directory' }
            $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
            $BackupFile = Join-Path $destination "rbc-$stamp.dump"
            RunDocker @('exec','ev-station-db-1','pg_dump','-U','rbc','-d','rbc_ev_station','-Fc','-f','/tmp/rbc-operations.dump') | Out-Null
            RunDocker @('cp','ev-station-db-1:/tmp/rbc-operations.dump',$BackupFile) | Out-Null
            (Get-FileHash $BackupFile -Algorithm SHA256).Hash | Set-Content "$BackupFile.sha256"
            # Bound local disk growth; only matching generated backups inside
            # this exact directory are pruned after a successful fresh dump.
            $backupRoot = [IO.Path]::GetFullPath($destination) + [IO.Path]::DirectorySeparatorChar
            Get-ChildItem -LiteralPath $destination -File | Where-Object Name -match '^rbc-\d{8}-\d{6}\.dump$' | Sort-Object Name -Descending | Select-Object -Skip 14 | ForEach-Object {
                if (-not $_.FullName.StartsWith($backupRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'Backup retention path escaped storage directory' }
                Remove-Item -LiteralPath $_.FullName -Force
                if (Test-Path -LiteralPath ($_.FullName + '.sha256')) { Remove-Item -LiteralPath ($_.FullName + '.sha256') -Force }
            }
            if ($Config.OffHostDirectory) {
                if (-not $Config.OffHostDirectory.StartsWith('\\')) { throw 'OffHostDirectory must be an approved network share, not another local disk' }
                Copy-Item -LiteralPath $BackupFile -Destination $Config.OffHostDirectory
                Copy-Item -LiteralPath "$BackupFile.sha256" -Destination $Config.OffHostDirectory
                $remote = Join-Path $Config.OffHostDirectory (Split-Path $BackupFile -Leaf)
                if ((Get-FileHash $remote).Hash -ne (Get-FileHash $BackupFile).Hash) { throw 'Off-host copy checksum mismatch' }
            }
            @{file=$BackupFile;sha256=(Get-FileHash $BackupFile).Hash;offHostVerified=[bool]$Config.OffHostDirectory;created=(Get-Date).ToUniversalTime().ToString('o')} | ConvertTo-Json | Set-Content (Join-Path $StateDir 'last-backup.json')
        }
        RestoreTest {
            if (-not $BackupFile) { $BackupFile = (Get-Content (Join-Path $StateDir 'last-backup.json') -Raw | ConvertFrom-Json).file }
            if ((Get-FileHash $BackupFile).Hash -ne (Get-Content "$BackupFile.sha256" -Raw).Trim()) { throw 'Backup checksum mismatch' }
            $name = 'rbc-restore-audit-' + (Get-Date -Format 'yyyyMMddHHmmss')
            try {
                RunDocker @('run','-d','--name',$name,'--network','none','--tmpfs','/var/lib/postgresql/data:rw,size=4g','-e','POSTGRES_HOST_AUTH_METHOD=trust','postgis/postgis:16-3.4-alpine') | Out-Null
                for ($i=0;$i -lt 30;$i++) { & docker exec $name pg_isready -U postgres *> $null; if ($LASTEXITCODE -eq 0) { break }; Start-Sleep -Seconds 2 }
                RunDocker @('exec',$name,'createdb','-U','postgres','rbc_restore') | Out-Null
                RunDocker @('cp',$BackupFile,"${name}:/tmp/backup.dump") | Out-Null
                RunDocker @('exec',$name,'pg_restore','-U','postgres','-d','rbc_restore','--no-owner','--no-privileges','--exit-on-error','/tmp/backup.dump') | Out-Null
                $counts = RunDocker @('exec',$name,'psql','-U','postgres','-d','rbc_restore','-Atc',"SELECT 'users='||count(*) FROM users UNION ALL SELECT 'sites='||count(*) FROM sites UNION ALL SELECT 'reports='||count(*) FROM analysis_runs UNION ALL SELECT 'metrics='||count(*) FROM analysis_metrics;")
                @{backup=$BackupFile;result='restored successfully';counts="$counts";tested=(Get-Date).ToUniversalTime().ToString('o')} | ConvertTo-Json | Set-Content (Join-Path $StateDir 'last-restore-test.json')
            } finally { & docker rm -f $name *> $null }
        }
    }
    "$(Get-Date -Format o) $Action OK" | Add-Content (Join-Path $StateDir 'operations.log')
} catch {
    "$(Get-Date -Format o) $Action FAILED: $_" | Add-Content (Join-Path $StateDir 'operations.log')
    if ($Config.AlertWebhook) {
        try { SendAlert "$Action failed" } catch { 'Alert delivery failed; inspect approved monitoring destination' | Add-Content (Join-Path $StateDir 'operations.log') }
    }
    throw
} finally { $Mutex.ReleaseMutex(); $Mutex.Dispose() }
