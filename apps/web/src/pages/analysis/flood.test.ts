import { describe, expect, it } from 'vitest'
import { translations } from '../../i18n/translations'
import { floodAccessWarning, floodHistoryPeriod, provinceFloodNote } from './flood'

const en = (key: string) => key
const th = (key: string) => translations.th[key as keyof typeof translations.th] || key

describe('Flood-history context', () => {
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
