# Offline regression checks. Load only the helper's AST; never execute the
# operations script, contact Docker, or change production services.
$ErrorActionPreference = 'Stop'
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'Invoke-RBC-Operations.ps1'), [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw 'Operations script contains syntax errors' }
$helper = $ast.Find({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'GetRoadUpdaterState' }, $true)
if (-not $helper) { throw 'Road worker recovery helper is missing' }
. ([scriptblock]::Create($helper.Extent.Text))

function RunDocker([string[]]$Arguments) {
    $script:Calls.Add($Arguments -join ' ') | Out-Null
    switch ($Arguments[0]) {
        'ps' { if ($script:Fixture.Exists) { return 'ev-station-road-updater-1' }; return }
        'inspect' { $script:Inspections++; return $script:Fixture.States[[Math]::Min($script:Inspections - 1, $script:Fixture.States.Count - 1)] }
        'start' { return 'ev-station-road-updater-1' }
        default { throw "Unexpected Docker operation: $($Arguments[0])" }
    }
}

$cases = @(
    @{Name='optional profile absent'; Exists=$false; States=@(); Expected='not-configured'; Starts=0},
    @{Name='healthy worker preserved'; Exists=$true; States=@('true|healthy'); Expected='healthy'; Starts=0},
    @{Name='stopped worker recovered'; Exists=$true; States=@('false|unhealthy','true|starting'); Expected='starting'; Starts=1},
    @{Name='stale data remains distinct'; Exists=$true; States=@('true|unhealthy'); Expected='unhealthy'; Starts=0},
    @{Name='worker immediately exits'; Exists=$true; States=@('false|unhealthy'); Expected='stopped'; Starts=1}
)
foreach ($case in $cases) {
    $script:Fixture = $case
    $script:Calls = [Collections.Generic.List[string]]::new()
    $script:Inspections = 0
    $actual = GetRoadUpdaterState
    if ($actual -ne $case.Expected) { throw "$($case.Name): expected $($case.Expected), received $actual" }
    $starts = @($script:Calls | Where-Object { $_ -eq 'start ev-station-road-updater-1' }).Count
    if ($starts -ne $case.Starts) { throw "$($case.Name): unexpected start count $starts" }
    if (@($script:Calls | Where-Object { $_ -match '^(restart|compose|build|update) ' }).Count) { throw 'Recovery tried to rebuild or restart another service' }
    Write-Output "PASS $($case.Name)"
}
