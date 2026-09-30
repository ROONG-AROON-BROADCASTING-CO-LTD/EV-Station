type Translator = (key: string) => string

export function floodAccessWarning(value: unknown, t: Translator): string {
  const record = value && typeof value === 'object' ? value as Record<string, unknown> : undefined
  const reported = record?.assessmentType === 'administrative_district_historical_reports' &&
    typeof record.reportedFloodYearCount === 'number' && record.reportedFloodYearCount > 0
  return t(reported
    ? 'Flood reports exist in this district. Even with a strong location score, check flood history on the frontage road and access routes. District reports do not confirm that this road flooded.'
    : 'Flood history on the frontage road and access routes has not been verified. A strong location score does not confirm uninterrupted access during floods.')
}

export function provinceFloodNote(value: unknown, t: Translator): string {
  if (!value || typeof value !== 'object') return ''
  const report = (value as Record<string, unknown>).latestProvinceReport
  if (!report || typeof report !== 'object') return ''
  const item = report as Record<string, unknown>
  if (item.geographicScope !== 'province' || typeof item.province !== 'string' ||
    typeof item.year !== 'number' || !Number.isInteger(item.year) ||
    typeof item.reportedOccurrences !== 'number' || !Number.isInteger(item.reportedOccurrences) || item.reportedOccurrences < 0 ||
    typeof item.affectedHouseholds !== 'number' || !Number.isInteger(item.affectedHouseholds) || item.affectedHouseholds < 0) return ''
  const year = t('BE') === 'พ.ศ.' ? item.year : item.year - 543
  return `${t('Latest provincial flood summary')} (${year}) · ${item.province}: ${item.reportedOccurrences.toLocaleString()} ${t('province-wide reported occurrences')}, ${item.affectedHouseholds.toLocaleString()} ${t('affected households')}. ${t('Provincial context only; excluded from district-history scoring.')}`
}

export function floodHistoryPeriod(start: number, end: number, t: Translator): string {
  const length = end - start + 1
  return Number.isInteger(length) && length > 0 && length <= 50 ? `${t('of')} ${length} ${t('years')}` : t('period unavailable')
}
