type Translator = (key: string) => string

export type GatewayEvidence = { status: 'available' | 'missing'; reason?: string; license: string; sourceUri: string; complete: boolean; retrievedAt?: string; periodStartYear?: number; periodEndYear?: number; yearsObserved?: number[]; nearbyYearsObserved?: number[]; result?: 'found' | 'not found' | 'unknown'; observationDates?: string[]; dateBasis?: string; featureCount?: number; siteOverlapCount?: number; searchRadiusMeters?: number; geoScope?: string }
export type GatewayWindow = { days: number; evidence: GatewayEvidence }
export type AnnualFloodEvidence = {year: number; status: 'available' | 'missing'; reason?: string; scope: string; province?: string; district?: string; recordCount: number; reportedAreaRai?: number; complete: boolean; partialYear: boolean; retrievedAt?: string; sourceUri: string}
export function gatewayFloodEvidence(value: unknown): { historical: GatewayEvidence; latest: GatewayEvidence; windows?: GatewayWindow[]; mappedHistory?: GatewayEvidence; annual?: AnnualFloodEvidence[] } | undefined {
  if (!value || typeof value !== 'object') return undefined
  const gateway = (value as Record<string, unknown>).gatewayFlood
  if (!gateway || typeof gateway !== 'object') return undefined
  const v = gateway as Record<string, unknown>
  if (v.assessmentType !== 'gistda_gateway_flood') return undefined
  const valid = (x: unknown): x is GatewayEvidence => {
    if (!x || typeof x !== 'object') return false
    const e = x as GatewayEvidence
    const sourceValid = e.license === 'Open Data Common' && ['https://data.go.th/dataset/disasters-01', 'https://data.go.th/dataset/disasters-03'].includes(e.sourceUri) || e.sourceUri === 'https://disaster.gistda.or.th/v2/services/open-api' && e.license === 'Disaster Platform catalogue licence unspecified; situational context only'
    return ['available', 'missing'].includes(e.status) && typeof e.complete === 'boolean' && sourceValid && (e.status === 'missing' || (typeof e.retrievedAt === 'string' && Number.isFinite(Date.parse(e.retrievedAt))))
  }
  if (!valid(v.historical) || !valid(v.latest)) return undefined
  const h = v.historical, l = v.latest
  if (h.status === 'available' && h.complete && (h.periodStartYear !== 2011 || h.periodEndYear !== 2023 || !Array.isArray(h.yearsObserved) || new Set(h.yearsObserved).size !== h.yearsObserved.length || h.yearsObserved.some(y => !Number.isInteger(y) || y < 2011 || y > 2023))) return undefined
  if (l.status === 'available' && (!['found', 'not found'].includes(l.result ?? '') || (l.observationDates !== undefined && (!Array.isArray(l.observationDates) || l.observationDates.some(d => !Number.isFinite(Date.parse(d))))))) return undefined
  let windows: GatewayWindow[] | undefined
  if (v.windows !== undefined) {
    if (!Array.isArray(v.windows) || v.windows.length !== 4 || new Set(v.windows.map(w => w?.days)).size !== 4 || v.windows.some(w => ![1,3,7,30].includes(w?.days) || !valid(w?.evidence) || (w.evidence.featureCount !== undefined && (!Number.isInteger(w.evidence.featureCount) || w.evidence.featureCount < 0)))) return undefined
    windows = v.windows as GatewayWindow[]
  }
  if (v.mappedHistory !== undefined && !valid(v.mappedHistory)) return undefined
  if (v.annual !== undefined && (!Array.isArray(v.annual) || v.annual.some(e => !Number.isInteger(e?.year) || e.year < 2011 || e.year > 2100 || !['available','missing'].includes(e.status) || !Number.isInteger(e.recordCount) || e.recordCount < 0 || e.sourceUri !== 'https://disaster.gistda.or.th/services/stac/flood/index'))) return undefined
  return { historical: h, latest: l, windows, mappedHistory: v.mappedHistory as GatewayEvidence | undefined, annual: v.annual as AnnualFloodEvidence[] | undefined }
}

export type RecentFloodWindow = { days: number; status: 'available' | 'missing'; reason?: string; featureCount?: number; siteOverlapCount?: number; complete: boolean; retrievedAt?: string }
export function recentFloodWindows(value: unknown): RecentFloodWindow[] {
  if (!value || typeof value !== 'object') return []
  const recent = (value as Record<string, unknown>).recentFlood
  if (!recent || typeof recent !== 'object') return []
  const data = recent as Record<string, unknown>
  if (data.assessmentType !== 'gistda_recent_flood' || !Array.isArray(data.windows) || data.windows.length !== 4) return []
  const windows = data.windows as RecentFloodWindow[]
  if (new Set(windows.map(w => w?.days)).size !== 4 || windows.some(w => !w || ![1, 3, 7, 30].includes(w.days) || !['available', 'missing'].includes(w.status) || typeof w.complete !== 'boolean' || (w.status === 'available' && (!Number.isInteger(w.featureCount) || (w.featureCount ?? -1) < 0 || !Number.isInteger(w.siteOverlapCount) || (w.siteOverlapCount ?? -1) < 0 || (w.siteOverlapCount ?? 0) > (w.featureCount ?? 0) || !w.retrievedAt || !Number.isFinite(Date.parse(w.retrievedAt)))))) return []
  return [...windows].sort((a, b) => a.days - b.days)
}

export function floodAccessWarning(value: unknown, t: Translator): string {
  const latest = gatewayFloodEvidence(value)?.latest
  if (gatewayFloodEvidence(value)?.windows?.some(w => w.evidence.status === 'available' && w.evidence.result === 'found' && !w.evidence.reason)) return t('Recent mapped flood areas overlap the site pin. Check the frontage road and access routes before investing; polygon overlap alone does not verify road conditions.')
  if (gatewayFloodEvidence(value)?.annual?.some(e => e.year === new Date().getFullYear() && e.status === 'available' && e.recordCount > 0)) return t('Flood reports exist in the latest published year in the search area. Check the frontage road and access routes; administrative totals do not confirm flooding at this plot.')
  if (latest?.status === 'available' && latest.result === 'found' && !latest.reason) return t('Recent mapped flood areas overlap the site pin. Check the frontage road and access routes before investing; polygon overlap alone does not verify road conditions.')
  const record = value && typeof value === 'object' ? value as Record<string, unknown> : undefined
  if (recentFloodWindows(value).some(w => w.status === 'available' && (w.siteOverlapCount ?? 0) > 0)) return t('Recent mapped flood areas overlap the site pin. Check the frontage road and access routes before investing; polygon overlap alone does not verify road conditions.')
  const histories = [record, ...(Array.isArray(record?.floodEvidence) ? record.floodEvidence.map(item => item?.rawValue) : [])]
  const reported = histories.some(item => item?.assessmentType === 'administrative_district_historical_reports' && typeof item.reportedFloodYearCount === 'number' && item.reportedFloodYearCount > 0)
  const recurrence = Array.isArray(record?.recurrenceYears) && record.recurrenceYears.length > 0
  return t(reported || recurrence
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
