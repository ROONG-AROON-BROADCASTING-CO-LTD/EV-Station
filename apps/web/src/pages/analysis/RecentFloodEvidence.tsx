import { useI18n } from '../../i18n/I18nProvider'
import { recentFloodWindows } from './flood'

export function RecentFloodEvidence({ value }: { value: unknown }) {
  const { language } = useI18n()
  const windows = recentFloodWindows(value)
  if (!windows.length) return null
  const th = language === 'th'
  const reasons: Record<string, string> = th ? {
    not_configured: 'ยังไม่ได้ตั้งค่าการเชื่อมต่อ GISTDA Disaster', unauthorized: 'แหล่งข้อมูลไม่อนุญาตให้เข้าถึง', rate_limited: 'แหล่งข้อมูลจำกัดการเรียกใช้งาน', invalid_response: 'ข้อมูลที่ได้รับไม่สมบูรณ์', invalid_coordinates: 'พิกัดไม่ถูกต้อง',
  } : { not_configured: 'GISTDA Disaster connection not configured', unauthorized: 'Source access denied', rate_limited: 'Source rate limit reached', invalid_response: 'Invalid source response', invalid_coordinates: 'Invalid coordinates' }
  return <section className="mt-4 rounded-xl border border-slate-200 bg-white p-4" aria-label={th ? 'ข้อมูลน้ำท่วมช่วงล่าสุด' : 'Recent flood observations'}>
    <h4 className="text-sm font-semibold text-ink">{th ? 'ข้อมูลน้ำท่วมช่วงล่าสุด · GISTDA' : 'Recent flood observations · GISTDA'}</h4>
    <p className="mt-1 text-xs leading-5 text-muted">{th ? 'ข้อมูลประกอบ ไม่รวมในคะแนนประวัติน้ำท่วม และยังไม่ยืนยันสภาพถนนเข้า–ออก' : 'Context only; excluded from historical scoring. Access-road conditions remain unverified.'}</p>
    <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">{windows.map(w => <div key={w.days} className="rounded-lg border border-slate-200 p-3 text-xs leading-5">
      <p className="font-semibold text-ink">{th ? `ย้อนหลัง ${w.days} วัน` : `Past ${w.days} days`}</p>
      {w.status === 'missing' ? <p className="mt-1 text-amber-700">{reasons[w.reason ?? ''] ?? (th ? 'เรียกแหล่งข้อมูลไม่ได้' : 'Source unavailable')}</p> : <>
        <p className="mt-1 text-ink">{th ? `พื้นที่น้ำท่วมที่พบ ${w.featureCount} รายการ · ทับพิกัด ${w.siteOverlapCount} รายการ` : `${w.featureCount} mapped areas · ${w.siteOverlapCount} overlap the pin`}</p>
        {!w.complete && <p className="text-amber-700">{th ? 'ผลลัพธ์บางส่วน อาจมีข้อมูลเพิ่มเติม' : 'Partial results; more records may exist'}</p>}
        <p className="mt-1 text-muted">{th ? 'ดึงข้อมูล: ' : 'Retrieved: '}{new Date(w.retrievedAt!).toLocaleString(th ? 'th-TH' : 'en-GB')}</p>
      </>}
    </div>)}</div>
    <p className="mt-3 text-xs leading-5 text-muted">{th ? 'ไม่พบข้อมูลในชั้นนี้ไม่ได้แปลว่าปลอดน้ำท่วม เวลาดึงข้อมูลไม่ใช่เวลาถ่ายภาพดาวเทียม' : 'No mapped areas does not establish flood-free conditions. Retrieval time is not satellite observation time.'}</p>
    <a href="https://data.go.th/dataset/flood-disaster-data" target="_blank" rel="noreferrer" className="mt-2 inline-block text-xs font-medium text-teal-700 underline">{th ? 'แหล่งข้อมูลและขอบเขตการใช้งาน' : 'Source and coverage'}</a>
  </section>
}
