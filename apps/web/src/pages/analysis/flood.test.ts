import { describe, expect, it } from 'vitest'
import { translations } from '../../i18n/translations'
import { floodAccessWarning, floodHistoryPeriod, provinceFloodNote, recentFloodWindows, gatewayFloodEvidence } from './flood'

const en = (key: string) => key
const th = (key: string) => translations.th[key as keyof typeof translations.th] || key

describe('Flood-history context', () => {
  it('preserves all recent windows and partial-year administrative archives through 2026', () => {
    const historical = { status: 'missing', complete: false, license: 'Open Data Common', sourceUri: 'https://data.go.th/dataset/disasters-01' }
    const empty = { status: 'missing', complete: false, license: 'Disaster Platform catalogue licence unspecified; situational context only', sourceUri: 'https://disaster.gistda.or.th/v2/services/open-api', result: 'unknown', reason: 'no_observations_in_window', featureCount: 0 }
    const windows = [1,3,7,30].map(days => ({days,evidence:empty}))
    const annual = [{year:2026,status:'available',scope:'district_records_in_search_bbox',recordCount:2,reportedAreaRai:10,complete:true,partialYear:true,sourceUri:'https://disaster.gistda.or.th/services/stac/flood/index'}]
    const value = {gatewayFlood:{assessmentType:'gistda_gateway_flood',historical,latest:empty,windows,annual}}
    expect(gatewayFloodEvidence(value)?.windows?.map(w=>w.days)).toEqual([1,3,7,30])
    expect(gatewayFloodEvidence(value)?.annual?.[0].partialYear).toBe(true)
    if (new Date().getFullYear()===2026) expect(floodAccessWarning(value,en)).toContain('administrative totals do not confirm flooding at this plot')
    expect(gatewayFloodEvidence({gatewayFlood:{...value.gatewayFlood,windows:[windows[0],windows[0],windows[2],windows[3]]}})).toBeUndefined()
  })
  it('keeps the current FeatureCollection source visible without claiming an observation date', () => {
    const historical = { status: 'missing', complete: false, license: 'Open Data Common', sourceUri: 'https://data.go.th/dataset/disasters-01' }
    const latest = { status: 'available', complete: true, license: 'Disaster Platform catalogue licence unspecified; situational context only', sourceUri: 'https://disaster.gistda.or.th/v2/services/open-api', retrievedAt: '2026-10-12T07:00:00Z', result: 'not found', reason: 'observation_date_unknown' }
    const value = { gatewayFlood: { assessmentType: 'gistda_gateway_flood', historical, latest } }
    expect(gatewayFloodEvidence(value)?.latest.reason).toBe('observation_date_unknown')
    expect(floodAccessWarning(value, en)).toContain('has not been verified')
    expect(gatewayFloodEvidence({ gatewayFlood: { ...value.gatewayFlood, latest: { ...latest, sourceUri: 'javascript:alert(1)' } } })).toBeUndefined()
  })
  it('validates point history and keeps stale observations out of current-road warnings', () => {
    const historical = { status: 'available', complete: true, license: 'Open Data Common', sourceUri: 'https://data.go.th/dataset/disasters-01', retrievedAt: '2026-10-06T02:00:00Z', periodStartYear: 2011, periodEndYear: 2023, yearsObserved: [2011] }
    const latest = { status: 'available', complete: true, license: 'Open Data Common', sourceUri: 'https://data.go.th/dataset/disasters-03', retrievedAt: '2026-10-06T02:00:00Z', result: 'found', observationDates: ['2026-10-06'] }
    const wrap = (h = historical, l = latest) => ({ gatewayFlood: { assessmentType: 'gistda_gateway_flood', historical: h, latest: l } })
    expect(gatewayFloodEvidence(wrap())?.historical.yearsObserved).toEqual([2011])
    expect(gatewayFloodEvidence(wrap({ ...historical, yearsObserved: [2024] }))).toBeUndefined()
    expect(floodAccessWarning(wrap(), en)).toContain('overlap the site pin')
    expect(floodAccessWarning({ gatewayFlood: { assessmentType: 'gistda_gateway_flood', historical, latest: { ...latest, reason: 'stale_data' } } }, en)).not.toContain('Recent mapped')
  })
  it('keeps unavailable recent windows visible and rejects malformed evidence', () => {
    const windows = [30, 1, 7, 3].map(days => ({ days, status: 'missing', reason: 'not_configured', complete: false }))
    const wrap = (items: unknown[]) => ({ recentFlood: { assessmentType: 'gistda_recent_flood', windows: items } })
    expect(recentFloodWindows(wrap(windows)).map(w => w.days)).toEqual([1, 3, 7, 30])
    expect(recentFloodWindows(wrap([...windows.slice(1), windows[1]]))).toEqual([])
    expect(recentFloodWindows(wrap(windows.map(w => ({ ...w, status: 'available', featureCount: 0 }))))).toEqual([])
    expect(recentFloodWindows({})).toEqual([])
    const positive = windows.map(w => ({ ...w, status: 'available', featureCount: 1, siteOverlapCount: 1, retrievedAt: '2026-10-05T02:00:00Z' }))
    expect(floodAccessWarning(wrap(positive), en)).toContain('overlap the site pin')
    expect(floodAccessWarning({ floodEvidence: [{ rawValue: { assessmentType: 'administrative_district_historical_reports', reportedFloodYearCount: 2 } }] }, en)).toContain('reports exist')
  })
  it('warns about access disruption without claiming a particular road flooded', () => {
    expect(floodAccessWarning({ assessmentType: 'administrative_district_historical_reports', reportedFloodYearCount: 4 }, en)).toContain('do not confirm that this road flooded')
    expect(floodAccessWarning({}, en)).toContain('has not been verified')
    expect(floodAccessWarning({}, th)).toContain('ถนนหน้าแปลง')
  })
  it('derives the number of years from the record period, including old reports', () => {
    expect(floodHistoryPeriod(2562, 2567, th)).toBe('จาก 6 ปี')
    expect(floodHistoryPeriod(2562, 2568, en)).toBe('of 7 years')
    expect(floodHistoryPeriod(2568, 2562, en)).toBe('period unavailable')
  })
  it('labels province-wide totals separately in Thai and English', () => {
    const value = { latestProvinceReport: { geographicScope: 'province', province: 'พระนครศรีอยุธยา', year: 2568, reportedOccurrences: 11, affectedHouseholds: 70999 } }
    expect(provinceFloodNote(value, en)).toContain('(2025)')
    expect(provinceFloodNote(value, en)).toContain('excluded from district-history scoring')
    expect(provinceFloodNote(value, th)).toContain('(2568)')
    expect(provinceFloodNote(value, th)).toContain('ไม่นำไปเพิ่มปีที่ท่วม')
  })
  it('leaves absent or invalid province context absent', () => {
    expect(provinceFloodNote({}, en)).toBe('')
    expect(provinceFloodNote({ latestProvinceReport: { geographicScope: 'district' } }, en)).toBe('')
  })
})
