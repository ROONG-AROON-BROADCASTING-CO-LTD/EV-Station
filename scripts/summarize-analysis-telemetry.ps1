param(
  [string]$Path,
  [string[]]$AnalysisID
)

$ErrorActionPreference = 'Stop'

function Get-Percentile([long[]]$Values, [double]$Percentile) {
  if ($Values.Count -eq 0) { return 0 }
  $ordered = @($Values | Sort-Object)
  $index = [Math]::Ceiling($Percentile * $ordered.Count) - 1
  return $ordered[[Math]::Max(0, [Math]::Min($index, $ordered.Count - 1))]
}

function Get-Summary([object[]]$Events, [string]$Name, [object[]]$CacheEvents = @()) {
  $durations = @($Events | ForEach-Object { [long]$_.duration_ms })
  $cacheHits = @($CacheEvents | Where-Object { $_.cache_hit -eq $true }).Count

  [PSCustomObject]@{
    Name      = $Name
    Calls     = $Events.Count
    AvgMs     = if ($durations.Count) { [Math]::Round(($durations | Measure-Object -Average).Average) } else { 0 }
    P50Ms     = Get-Percentile $durations 0.50
    P95Ms     = Get-Percentile $durations 0.95
    MaxMs     = if ($durations.Count) { ($durations | Measure-Object -Maximum).Maximum } else { 0 }
    CacheHit  = if ($CacheEvents.Count) { "$cacheHits/$($CacheEvents.Count)" } else { '-' }
    Timeout   = @($Events | Where-Object { $_.error_type -eq 'timeout' }).Count
    Error     = @($Events | Where-Object { $_.status -eq 'error' }).Count
  }
}

$lines = if ($Path) {
  Get-Content -LiteralPath $Path
} else {
  docker compose logs --no-log-prefix api
}

$events = foreach ($line in $lines) {
  $jsonStart = $line.IndexOf('{')
  if ($jsonStart -lt 0) { continue }
  try {
    $entry = $line.Substring($jsonStart) | ConvertFrom-Json
    if ($entry.msg -eq 'analysis_provider_telemetry') { $entry }
  } catch {
    continue
  }
}

$events = @($events)
$analysisIDs = @($AnalysisID | ForEach-Object { $_ -split ',' } | Where-Object { $_ })
if ($analysisIDs.Count -gt 0) {
  $events = @($events | Where-Object { $analysisIDs -contains $_.analysis_id })
}

if ($events.Count -eq 0) {
  Write-Output 'No analysis_provider_telemetry events found.'
  exit 0
}

Write-Output 'Provider summary'
$providerEvents = @($events | Where-Object { $_.operation -eq 'provider_collect' })
@($providerEvents | Group-Object provider | ForEach-Object {
  $providerName = $_.Name
  $cacheEvents = @($events | Where-Object { $_.provider -eq $providerName -and $_.operation -eq 'cache_get' })
  Get-Summary $_.Group $providerName $cacheEvents
}) |
  Sort-Object -Property AvgMs -Descending |
  Format-Table -AutoSize

$googleCategories = @($events | Where-Object { $_.provider -eq 'google_places' -and $_.operation -eq 'cache_get' -and $_.category })
if ($googleCategories.Count -gt 0) {
  Write-Output 'Google Places category summary'
  @($googleCategories | Group-Object category | ForEach-Object {
    $category = $_.Name
    $logicalCalls = $_.Group
    $httpCalls = @($events | Where-Object { $_.provider -eq 'google_places' -and $_.operation -eq 'google_places_search' -and $_.category -eq $category })
    $summary = Get-Summary $httpCalls $category $logicalCalls
    [PSCustomObject]@{
      Name = $summary.Name; Calls = $logicalCalls.Count; HttpCalls = $httpCalls.Count; AvgMs = $summary.AvgMs; P50Ms = $summary.P50Ms; P95Ms = $summary.P95Ms; MaxMs = $summary.MaxMs; CacheHit = $summary.CacheHit; Timeout = $summary.Timeout; Error = $summary.Error
    }
  }) |
    Sort-Object -Property AvgMs -Descending |
    Format-Table -AutoSize
}

Write-Output 'Operation and endpoint summary'
@($events | Where-Object { $_.operation -ne 'provider_collect' -and $_.operation -ne 'cache_get' } | Group-Object { "$($_.provider) | $($_.operation) | $($_.endpoint)" } | ForEach-Object {
  Get-Summary $_.Group $_.Name
}) |
  Sort-Object -Property AvgMs -Descending |
  Format-Table -AutoSize
