import { useI18n } from '../../i18n/I18nProvider'
import { gatewayFloodEvidence, type GatewayEvidence, type GatewayWindow, type AnnualFloodEvidence } from './flood'

export function GatewayFloodEvidence({ value }: { value: unknown }) {
  const { language } = useI18n()
  const evidence = gatewayFloodEvidence(value)
  if (!evidence) return null
  const th = language === 'th'
  const reasons: Record<string, string> = th ? {
    not_configured: 'กำลังรอการเชื่อมต่อข้อมูลจาก GISTDA', unauthorized: 'ยังเข้าถึงแหล่งข้อมูลไม่ได้', rate_limited: 'แหล่งข้อมูลจำกัดจำนวนการเรียก กรุณาวิเคราะห์ใหม่ภายหลัง', invalid_response: 'ข้อมูลต้นทางยังไม่ครบหรือมีรูปแบบที่ตรวจสอบไม่ได้', no_record: 'ไม่พบระเบียนประวัติสำหรับพิกัดนี้', invalid_coordinates: 'ต้องมีพิกัดที่ถูกต้อง',
  } : {
    not_configured: 'GISTDA data connection pending', unauthorized: 'Source access unavailable', rate_limited: 'Source rate limit reached; retry the analysis later', invalid_response: 'Source data is incomplete or could not be validated', no_record: 'No historical record returned for this pin', invalid_coordinates: 'Valid coordinates required',
  }
  reasons.incomplete_response = th ? 'ข้อมูลต้นทางมีหลายหน้าและยังตรวจไม่ครบ จึงสรุปผลที่พิกัดนี้ไม่ได้' : 'Source results are paginated and incomplete; this pin cannot be assessed yet'
  reasons.no_observations_in_window = th ? 'ต้นทางไม่ส่งระเบียนสำหรับกรอบค้นหาในช่วงนี้ จึงยังยืนยันสถานการณ์ไม่ได้' : 'No records supplied for this search area and window; conditions remain unverified'
  const h = evidence.historical, l = evidence.latest
  const year = (y: number) => th ? y + 543 : y
  const date = (s: string) => new Date(s).toLocaleString(th ? 'th-TH' : 'en-GB')
  return <section className="mt-4 rounded-xl border border-slate-200 bg-white p-4" aria-label={th ? 'ประวัติและสถานการณ์น้ำท่วมจาก GISTDA' : 'GISTDA flood history and observations'}>
    <h4 className="text-sm font-semibold text-ink">{th ? 'หลักฐานน้ำท่วม · GISTDA' : 'Flood evidence · GISTDA'}</h4>
    <div className="mt-3 grid gap-3 sm:grid-cols-2">
      <div className="rounded-lg border border-slate-200 p-3 text-xs leading-5">
        <p className="font-semibold text-ink">{th ? 'ประวัติที่พิกัด' : 'History at the pin'} · {year(2011)}–{year(2023)}</p>
        {h.status !== 'available' || !h.complete ? <p className="mt-1 text-amber-700">{reasons[h.reason ?? ''] ?? (th ? 'ยังดึงข้อมูลไม่ได้' : 'Source unavailable')}</p> : <>
          <p className="mt-1 text-ink">{th ? `ตรวจพบ ${h.yearsObserved!.length} ปี จากช่วงข้อมูล 13 ปี` : `Observations in ${h.yearsObserved!.length} of 13 years`}</p>
          {!!h.yearsObserved!.length && <p>{h.yearsObserved!.map(year).join(', ')}</p>}
          <p className="mt-1 text-muted">{th ? 'หลักฐานจากดาวเทียม ใช้เป็นตัวชี้วัดคัดกรองเบื้องต้น ไม่ใช่โอกาสเกิดน้ำท่วม' : 'Satellite evidence supports preliminary screening; the score is not flood probability.'}</p>
        </>}
        {h.retrievedAt && <p className="mt-1 text-muted">{th ? 'ดึงข้อมูล: ' : 'Retrieved: '}{date(h.retrievedAt)}</p>}
        <a className="mt-2 inline-block text-teal-700 underline" href="https://data.go.th/dataset/disasters-01" target="_blank" rel="noreferrer">{th ? 'แหล่งข้อมูลประวัติ' : 'Historical source'}</a>
        {evidence.mappedHistory && <MappedHistory value={evidence.mappedHistory} th={th} />}
      </div>
      <div className="rounded-lg border border-slate-200 p-3 text-xs leading-5">
        {evidence.windows ? <CurrentWindows windows={evidence.windows} th={th} reasons={reasons} /> : <>
        <p className="font-semibold text-ink">{th ? 'ข้อมูลล่าสุดจากบริการย้อนหลัง 1 วัน' : 'Latest response from the one-day service'}</p>
        {l.status !== 'available' ? <p className="mt-1 text-amber-700">{reasons[l.reason ?? ''] ?? (th ? 'ยังดึงข้อมูลไม่ได้' : 'Source unavailable')}</p> : <>
          <p className="mt-1 text-ink">{l.result === 'found' ? (th ? 'พบระเบียนพื้นที่น้ำท่วมตรงพิกัด' : 'Mapped flood record returned at the pin') : (th ? 'ไม่พบระเบียนในผลตอบกลับล่าสุด' : 'No record returned in the latest response')}</p>
          <p>{th ? 'วันที่ข้อมูล: ' : 'Observation dates: '}{l.observationDates?.length ? l.observationDates.join(', ') : (th ? 'ต้นทางไม่ระบุ' : 'Not supplied')}</p>
          {l.reason === 'stale_data' && <p className="text-amber-700">{th ? 'วันที่ข้อมูลเก่ากว่า 3 วัน จึงยังใช้ยืนยันสถานการณ์ปัจจุบันไม่ได้' : 'Observation is older than 3 days; current conditions remain unverified.'}</p>}
          {l.reason === 'observation_date_unknown' && <p className="text-amber-700">{th ? 'ต้นทางไม่ระบุวันที่ภาพดาวเทียม จึงยังยืนยันสถานการณ์ปัจจุบันไม่ได้' : 'Satellite observation dates are not supplied; current conditions remain unverified.'}</p>}
          {!l.complete && <p className="text-amber-700">{th ? 'พบข้อมูลตรงพิกัด แต่ยังตรวจรายการทั้งหมดไม่ครบ' : 'A match was found at the pin, but the full result set is incomplete.'}</p>}
          <p className="mt-1 text-muted">{th ? 'แสดงประกอบการตรวจสอบ ไม่รวมในคะแนนประวัติ' : 'Situational context; excluded from historical scoring.'}</p>
        </>}
        {l.retrievedAt && <p className="mt-1 text-muted">{th ? 'ดึงข้อมูล: ' : 'Retrieved: '}{date(l.retrievedAt)}</p>}
        <a className="mt-2 inline-block text-teal-700 underline" href={l.sourceUri ?? 'https://data.go.th/dataset/disasters-03'} target="_blank" rel="noreferrer">{th ? 'แหล่งข้อมูลล่าสุด' : 'Latest source'}</a>
        </>}
      </div>
    </div>
    {evidence.annual && <AnnualEvidence annual={evidence.annual} th={th} />}
    <p className="mt-3 text-xs leading-5 text-muted">{th ? 'การไม่พบระเบียนไม่ได้ยืนยันว่าปลอดน้ำท่วม ถนนหน้าแปลงและเส้นทางเข้า–ออกยังต้องตรวจสอบ แต่ละแหล่งมีขอบเขตปีและระดับพื้นที่ต่างกัน โปรดอ่านช่วงข้อมูลของแต่ละส่วน' : 'Missing or zero records do not establish flood-free conditions. Frontage roads and access routes require checks. Sources have different year ranges and geographic scopes; review each section.'}</p>
    {evidence.windows && <p className="mt-2 text-xs leading-5 text-muted">{th ? 'ขอบเขตปีข้อมูล: ประวัติจุด 2554–2566 · ชั้นแผนที่ย้อนหลังตามปีที่ต้นทางเผยแพร่ · ปภ. ระดับอำเภอ 2562–2567 และรายจังหวัด 2568 · ตรวจคลังรายปีที่เผยแพร่ถึง 2569 พร้อม 1/3/7/30 วันล่าสุด ข้อมูลปีปัจจุบันยังไม่ครบทั้งปี และข้อมูลระดับอำเภอไม่ใช่ผลน้ำท่วมเฉพาะแปลง คะแนนด้านบนเป็นคะแนนประวัติ ไม่ใช่ระดับความปลอดภัยปัจจุบัน' : 'Coverage: point history 2011–2023; published mapped-history years; DDPM district history 2019–2024 and provincial context 2025; published annual archives through 2026 plus 1/3/7/30-day observations. The current year is partial, and administrative totals are not plot-level inundation. The score above concerns history, not current safety.'}</p>}
  </section>
}

function CurrentWindows({ windows, th, reasons }: { windows: GatewayWindow[]; th: boolean; reasons: Record<string,string> }) {
  return <><p className="font-semibold text-ink">{th ? 'สถานการณ์ล่าสุด · ตรวจทั้ง 4 ช่วงเวลา' : 'Recent observations · all four windows'}</p><p className="mt-1 text-muted">{th ? 'ค้นตามกรอบพื้นที่ของรัศมีที่เลือก และตรวจรูปทรงกับพิกัดจริง' : 'Searches the selected radius bounding box and checks geometry at the actual pin.'}</p><div className="mt-3 space-y-3">{windows.map(({days,evidence:e}) => <div key={days} className="border-t border-slate-100 pt-2"><p className="font-semibold">{th ? `ย้อนหลัง ${days} วัน` : `Past ${days} days`}</p>{e.status==='missing' ? <p className="text-amber-700">{reasons[e.reason ?? ''] ?? (th ? 'ยังตรวจไม่ได้' : 'Source unavailable')}</p> : <><p>{th ? `ตรวจ ${e.featureCount ?? 0} ระเบียนในกรอบค้นหา · ${e.result==='found' ? 'พบรูปทรงทับพิกัด' : 'ไม่พบรูปทรงทับพิกัดในระเบียนที่ตรวจ'}` : `${e.featureCount ?? 0} records in the search box · ${e.result==='found' ? 'geometry overlaps the pin' : 'no overlap in checked records'}`}</p>{!e.complete && <p className="text-amber-700">{th ? 'ยังดึงไม่ครบทุกหน้า จึงยังสรุปการไม่พบน้ำท่วมไม่ได้' : 'Pagination is incomplete; absence of flooding cannot be concluded.'}</p>}{!!e.observationDates?.length && <p className="text-muted">{th ? 'วันที่อ้างอิงจากชื่อชุดภาพ: ' : 'Dates inferred from product names: '}{e.observationDates.join(', ')}</p>}{e.reason==='observation_date_unknown' && <p className="text-amber-700">{th ? 'ต้นทางไม่ระบุวันที่สังเกตการณ์' : 'Observation dates not supplied'}</p>}{e.reason==='stale_data' && <p className="text-amber-700">{th ? 'วันที่ภาพเก่ากว่าช่วงค้นหา ยังยืนยันปัจจุบันไม่ได้' : 'Product dates are older than the window; current conditions remain unverified.'}</p>}</>}{e.retrievedAt && <p className="text-muted">{th ? 'ดึงข้อมูล: ' : 'Retrieved: '}{new Date(e.retrievedAt).toLocaleString(th?'th-TH':'en-GB')}</p>}</div>)}</div><a className="mt-3 inline-block text-teal-700 underline" href="https://disaster.gistda.or.th/v2/services/open-api" target="_blank" rel="noreferrer">{th ? 'แหล่งข้อมูลล่าสุด' : 'Recent-data source'}</a></>
}

function MappedHistory({value:e,th}:{value:GatewayEvidence;th:boolean}) {
  const year=(n:number)=>th?n+543:n
  return <div className="mt-3 border-t border-slate-100 pt-3"><p className="font-semibold">{th ? 'ชั้นแผนที่ประวัติเพิ่มเติม' : 'Additional mapped history'}{e.periodStartYear && e.periodEndYear ? ` · ${year(e.periodStartYear)}–${year(e.periodEndYear)}` : ''}</p>{e.status==='available' ? <><p>{th ? `${e.featureCount ?? 0} ระเบียน${e.geoScope === 'district_map' ? 'ในเขต/อำเภอ' : 'ในกรอบค้นหา'} · ${e.siteOverlapCount ?? 0} รูปทรงทับพิกัด` : `${e.featureCount ?? 0} mapped records ${e.geoScope === 'district_map' ? 'in the district' : 'in the search box'} · ${e.siteOverlapCount ?? 0} overlap the pin`}</p><p>{th ? 'ปีที่พบในบริเวณค้นหา: ' : 'Observed years in the search box: '}{e.nearbyYearsObserved?.length ? e.nearbyYearsObserved.map(year).join(', ') : '—'}</p><p className="text-muted">{th ? 'ระเบียนใกล้เคียงไม่ยืนยันว่าแปลงนี้หรือถนนหน้าแปลงท่วม ไม่รวมเพิ่มในคะแนนประวัติจุด' : 'Nearby records do not establish flooding at the plot or frontage road; not added to the point-history score.'}</p></> : <p className="text-amber-700">{th ? 'ยังไม่พบข้อมูลที่ใช้สรุปประวัติชั้นแผนที่นี้ได้' : 'No usable conclusion from this mapped-history layer yet'}</p>}</div>
}

function AnnualEvidence({annual,th}:{annual:AnnualFloodEvidence[];th:boolean}) {
  const years=[...annual].sort((a,b)=>b.year-a.year)
  const latest=years[0]
  const summary=(e:AnnualFloodEvidence)=>e.status==='missing' ? (th ? 'ยังดึงข้อมูลปีนี้ไม่ได้' : 'Year data unavailable') : e.recordCount>0 ? (th ? `พบ ${e.recordCount} พื้นที่ระดับตำบล/แขวงที่มีรายงาน (ไม่ใช่จำนวนครั้ง)` : `${e.recordCount} administrative areas with reports (not event count)`) : (th ? 'ไม่พบรายงานในข้อมูลที่ตรวจ ยังไม่ยืนยันว่าพื้นที่ไม่เคยท่วม' : 'No report in checked data; this does not establish no flooding')
  return <section className="mt-4 rounded-lg border border-slate-200 p-3 text-xs leading-5" aria-label={th?'คลังน้ำท่วมรายปี':'Annual flood archive'}><p className="font-semibold text-ink">{th ? 'คลังข้อมูลรายปี · ตรวจถึงปีล่าสุดที่เผยแพร่' : 'Annual archive · through the latest published year'}</p>{latest && <p className="mt-2 text-amber-800">{th ? `ปี ${latest.year+543}` : latest.year}{latest.partialYear ? (th?' · ข้อมูลระหว่างปี':' · partial year') : ''} · {summary(latest)}</p>}<p className="mt-2 text-muted">{th?'ค้นทั้งอำเภอ/เขตเมื่อยืนยันรหัสได้ มิฉะนั้นค้นตามกรอบพื้นที่ของรัศมีที่เลือก ยอดน้ำท่วมเป็นข้อมูลระดับตำบล/อำเภอ ไม่ใช่พื้นที่ท่วมเฉพาะแปลง ไม่รวมซ้ำกับคะแนนประวัติจุด':'Searches the full district when its code is verified, otherwise uses the selected radius bounding box. Totals describe administrative areas, not plot inundation, and are not double-counted in point-history scoring.'}</p><details className="mt-3"><summary className="cursor-pointer font-semibold text-teal-700">{th?`ดูผลทั้ง ${years.length} ปี`:`View all ${years.length} years`}</summary><div className="mt-2 space-y-3">{years.map(e=><div key={e.year} className="border-t border-slate-100 pt-2"><p className="font-semibold">{th?e.year+543:e.year}{e.partialYear?(th?' · ระหว่างปี':' · partial year'):''}</p><p>{summary(e)}</p>{e.district&&<p className="text-muted">{e.district}, {e.province}</p>}{!e.complete&&e.status==='available'&&<p className="text-amber-700">{th?'ดึงข้อมูลยังไม่ครบทุกหน้า':'Not all pages were retrieved'}</p>}{e.retrievedAt&&<p className="text-muted">{th?'ดึงข้อมูล: ':'Retrieved: '}{new Date(e.retrievedAt).toLocaleString(th?'th-TH':'en-GB')}</p>}</div>)}</div></details><a className="mt-3 inline-block text-teal-700 underline" href="https://disaster.gistda.or.th/services/stac/flood/index" target="_blank" rel="noreferrer">{th?'คลังข้อมูลต้นทาง':'Source archive'}</a></section>
}
