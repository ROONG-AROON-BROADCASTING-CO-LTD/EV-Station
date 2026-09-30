[CmdletBinding()]
param(
    [switch]$OpenEdge
)

$ErrorActionPreference = 'Stop'

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$PublicUrl = 'https://www.rbcevstation.com'
$CloudflaredServiceName = 'cloudflared'
$DockerDesktopCandidates = @(
    "$env:ProgramFiles\Docker\Docker\Docker Desktop.exe",
    "${env:ProgramFiles(x86)}\Docker\Docker\Docker Desktop.exe"
)

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

function Get-RbcQuickTunnelProcesses {
    Get-CimInstance Win32_Process -Filter "Name='cloudflared.exe'" |
        Where-Object { $_.CommandLine -match 'tunnel\s+--url\s+http://127\.0\.0\.1:8081' }
}

function Stop-RbcQuickTunnels {
    # Remove only the old, temporary tunnel command created by this launcher.
    # The named Cloudflare service is left untouched.
    Get-RbcQuickTunnelProcesses | ForEach-Object {
        Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

function Ensure-NamedTunnel {
    $service = Get-Service -Name $CloudflaredServiceName -ErrorAction SilentlyContinue
    if (-not $service) {
        throw "Cloudflare Named Tunnel service '$CloudflaredServiceName' was not found. Install the named tunnel before running this launcher."
    }
    if ($service.Status -ne 'Running') {
        Write-RbcStatus 'Starting Cloudflare Named Tunnel...'
        Start-Service -Name $CloudflaredServiceName
    }

    $deadline = (Get-Date).AddSeconds(30)
    do {
        $service = Get-Service -Name $CloudflaredServiceName
        if ($service.Status -eq 'Running') {
            try {
                $response = Invoke-WebRequest -UseBasicParsing -Uri $PublicUrl -TimeoutSec 12
                if ($response.StatusCode -eq 200) {
                    return
                }
            } catch {
                # The named tunnel may still be reconnecting after Docker starts.
            }
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw "Cloudflare Named Tunnel did not make $PublicUrl reachable within 30 seconds."
}

Wait-ForDocker
Write-RbcStatus 'Starting database, API and web...'
Push-Location $ProjectRoot
try {
    docker compose up -d --build --wait
    if ($LASTEXITCODE -ne 0) {
        throw 'Docker build/start failed. The updated application was not started.'
    }
} finally {
    Pop-Location
}

$localHealth = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:8080/health' -TimeoutSec 15
if ($localHealth.StatusCode -ne 200) {
    throw "API was not ready (HTTP $($localHealth.StatusCode))."
}

Stop-RbcQuickTunnels
Ensure-NamedTunnel
Write-RbcStatus "Ready: $PublicUrl"
Write-RbcStatus "LIFF endpoint: $PublicUrl/liff/site-submission"

if ($OpenEdge) {
    $edge = "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
    if (-not (Test-Path -LiteralPath $edge)) {
        $edge = "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe"
    }
    if (Test-Path -LiteralPath $edge) {
        Start-Process -FilePath $edge -ArgumentList $PublicUrl | Out-Null
    }
}
