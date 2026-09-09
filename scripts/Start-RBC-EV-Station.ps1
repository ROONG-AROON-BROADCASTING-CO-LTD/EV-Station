[CmdletBinding()]
param(
    [switch]$OpenEdge
)

$ErrorActionPreference = 'Stop'

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$RuntimeDirectory = Join-Path $ProjectRoot '.runtime'
$TunnelLog = Join-Path $RuntimeDirectory 'cloudflared.err.log'
$TunnelOutputLog = Join-Path $RuntimeDirectory 'cloudflared.out.log'
$TunnelUrlFile = Join-Path $RuntimeDirectory 'tunnel-url.txt'
$DockerDesktopCandidates = @(
    "$env:ProgramFiles\Docker\Docker\Docker Desktop.exe",
    "${env:ProgramFiles(x86)}\Docker\Docker\Docker Desktop.exe"
)

New-Item -ItemType Directory -Force -Path $RuntimeDirectory | Out-Null

function Write-RbcStatus([string]$Message) {
    Write-Host "[RBC EV Station] $Message" -ForegroundColor Green
}

function Wait-ForDocker {
    try {
        docker info *> $null
        return
    } catch {
        $dockerDesktop = $DockerDesktopCandidates | Where-Object { $_ -and (Test-Path -LiteralPath $_) } | Select-Object -First 1
        if (-not $dockerDesktop) {
            throw 'Docker Desktop was not found. Install or start Docker Desktop first.'
        }

        Write-RbcStatus 'Starting Docker Desktop...'
        Start-Process -FilePath $dockerDesktop | Out-Null
        $deadline = (Get-Date).AddMinutes(2)
        do {
            Start-Sleep -Seconds 3
            try {
                docker info *> $null
                return
            } catch {
                # Docker Desktop is still starting.
            }
        } while ((Get-Date) -lt $deadline)

        throw 'Docker Desktop was not ready within 2 minutes.'
    }
}

function Get-QuickTunnelUrl {
    if (-not (Test-Path -LiteralPath $TunnelLog)) {
        return $null
    }

    $content = Get-Content -LiteralPath $TunnelLog -Raw -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($content)) {
        return $null
    }
    $matches = [regex]::Matches($content, 'https://[-a-z0-9]+\.trycloudflare\.com')
    if ($matches.Count -gt 0) {
        return $matches[$matches.Count - 1].Value
    }
    return $null
}

function Test-QuickTunnel([string]$Url) {
    if (-not $Url) {
        return $false
    }

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri "$Url/health" -TimeoutSec 12
        return $response.StatusCode -eq 200
    } catch {
        return $false
    }
}

function Get-RbcQuickTunnelProcesses {
    Get-CimInstance Win32_Process -Filter "Name='cloudflared.exe'" |
        Where-Object { $_.CommandLine -match 'tunnel\s+--url\s+http://127\.0\.0\.1:8081' }
}

function Start-QuickTunnel {
    $existingUrl = Get-QuickTunnelUrl
    if ($existingUrl -and (Test-QuickTunnel $existingUrl)) {
        Set-Content -LiteralPath $TunnelUrlFile -Value $existingUrl -NoNewline
        return $existingUrl
    }

    # A Quick Tunnel URL expires when its process stops. Only stop processes that
    # this launcher started for the local web app; leave any named tunnels intact.
    Get-RbcQuickTunnelProcesses | ForEach-Object {
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }

    Write-RbcStatus 'Starting Cloudflare Quick Tunnel...'
    Clear-Content -LiteralPath $TunnelLog -ErrorAction SilentlyContinue
    Clear-Content -LiteralPath $TunnelOutputLog -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $TunnelUrlFile -Force -ErrorAction SilentlyContinue
    Start-Process -FilePath 'cloudflared.exe' `
        -ArgumentList @('tunnel', '--url', 'http://127.0.0.1:8081', '--no-autoupdate') `
        -WindowStyle Hidden `
        -RedirectStandardOutput $TunnelOutputLog `
        -RedirectStandardError $TunnelLog | Out-Null

    $deadline = (Get-Date).AddSeconds(45)
    do {
        $url = Get-QuickTunnelUrl
        if ($url -and (Test-QuickTunnel $url)) {
            Set-Content -LiteralPath $TunnelUrlFile -Value $url -NoNewline
            return $url
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw 'Cloudflare Tunnel did not become reachable within 45 seconds. Check .runtime\\cloudflared.err.log.'
}

Wait-ForDocker
Write-RbcStatus 'Starting database, API and web...'
Push-Location $ProjectRoot
try {
    docker compose up -d --wait
} finally {
    Pop-Location
}

$localHealth = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:8080/health' -TimeoutSec 15
if ($localHealth.StatusCode -ne 200) {
    throw "API was not ready (HTTP $($localHealth.StatusCode))."
}

$tunnelUrl = Start-QuickTunnel
Write-RbcStatus "Ready: $tunnelUrl"
Write-RbcStatus "LIFF endpoint: $tunnelUrl/liff/site-submission"

if ($OpenEdge) {
    $edge = "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
    if (-not (Test-Path -LiteralPath $edge)) {
        $edge = "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe"
    }
    if (Test-Path -LiteralPath $edge) {
        Start-Process -FilePath $edge -ArgumentList $tunnelUrl | Out-Null
    }
}
