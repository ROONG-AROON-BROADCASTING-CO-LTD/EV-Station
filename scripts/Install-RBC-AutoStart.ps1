[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$StartScript = Join-Path $PSScriptRoot 'Start-RBC-EV-Station.ps1'
$StartupFolder = [Environment]::GetFolderPath('Startup')
$StartupCommand = Join-Path $StartupFolder 'Start RBC EV Station.cmd'

$commandLines = @(
    '@echo off',
    ('start "" powershell.exe -NoProfile -ExecutionPolicy Bypass -File "{0}" -OpenEdge' -f $StartScript)
)

Set-Content -LiteralPath $StartupCommand -Value $commandLines -Encoding ASCII
Write-Host "Auto-start installed: $StartupCommand" -ForegroundColor Green
Write-Host 'After sign-in, Docker, web, the Cloudflare Named Tunnel and Edge will start automatically.' -ForegroundColor Green
