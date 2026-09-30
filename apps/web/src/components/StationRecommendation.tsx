import { useMutation } from '@tanstack/react-query'
import { useI18n } from '../i18n/I18nProvider'
import { api, errorMessageKey } from '../services/api'
import type { Site, StationRecommendation as StationProposal, StationRecommendationText as StationText } from '../types/domain'

export type { StationProposal }

function RecommendationLanguage({ heading, text, assumptionLabel, missingLabel }: { heading: string; text: StationText; assumptionLabel: string; missingLabel: string }) {
  return <article className="rounded-xl border border-line bg-slate-50 p-4">
    <h3 className="font-bold text-brand">{heading}</h3>
    <p className="mt-3 text-sm leading-6">{text.reason}</p>
    <div className="mt-4 space-y-4">
      <div>
        <h4 className="font-semibold">{assumptionLabel}</h4>
        <ul className="mt-2 list-disc space-y-2 pl-5 text-sm">{text.assumptions.map((item, index) => <li key={index}>{item}</li>)}</ul>
      </div>
      <div>
        <h4 className="font-semibold">{missingLabel}</h4>
        <ul className="mt-2 list-disc space-y-2 pl-5 text-sm">{text.missingData.map((item, index) => <li key={index}>{item}</li>)}</ul>
      </div>
    </div>
  </article>
}

export function StationRecommendation({ id, site, initialProposal }: { id: string; site?: Site; initialProposal?: StationProposal }) {
  const { language, t } = useI18n()
  const th = language === 'th'
  const initialCurrent = initialProposal?.recommendationStage ? initialProposal : undefined
  const hasLegacyProposal = Boolean(initialProposal && !initialCurrent)
  const proposal = useMutation({ mutationFn: (refresh: boolean) => api.recommendStation(id, refresh) })

	const currentBlocker = site?.electricalSupplyType === 'underground' ? 'underground_electricity' : undefined
	const result = currentBlocker ? undefined : proposal.data ?? initialCurrent
	const text = result?.[language]
	const recommendationAvailable = result?.recommendationAvailable === true
	const displayText: StationText | undefined = text
  return <section className="mt-5 rounded-2xl border border-line bg-white p-6 shadow-panel">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2 className="section-title">{th ? 'คำแนะนำ: ขนาดและจำนวนตู้ชาร์จ' : 'Charger power and quantity recommendation'}</h2>
      <button className="button-secondary print-hide" disabled={proposal.isPending} onClick={() => proposal.mutate(Boolean(initialCurrent))}>{proposal.isPending ? (th ? 'AI กำลังวิเคราะห์…' : 'Analyzing…') : initialCurrent ? (th ? 'ให้ AI วิเคราะห์ใหม่' : 'Regenerate AI recommendation') : (th ? 'สร้างคำแนะนำแบบใหม่' : 'Generate updated recommendation')}</button>
    </div>
    <p className="mt-2 text-sm text-muted">{th ? 'ระบบแนะนำขนาดและจำนวนตู้ชาร์จ DC จากข้อมูลทำเลภายในขอบเขตผัง S / M / L โดยแนะนำทางเข้า–ออกอย่างน้อย 7 ม. กำลังติดตั้งจริงและขนาดหม้อแปลงต้องตรวจสอบจากผลสำรวจพื้นที่ แบบวิศวกรรม และการไฟฟ้า' : 'The system recommends DC charger power and cabinet count from site evidence within the S / M / L layout. An entrance/exit of at least 7 m is recommended. Final installed power and transformer size require a site survey, engineering design, and utility review.'}</p>
    {result?.recommendationStage === 'screening_evidence' ? <p className="mt-3 rounded-xl border border-sky-200 bg-sky-50 px-4 py-3 text-sm leading-6 text-sky-950">{th ? 'AI วิเคราะห์ข้อมูลทำเลที่มีร่วมกันเพื่อแนะนำจำนวนตู้ภายในขอบเขตผังแพ็กเกจ S / M / L โดยพิจารณาการจราจร แนวโน้ม EV ประชากร สถานที่รอบพื้นที่ คู่แข่ง และข้อมูลโครงข่ายไฟฟ้าที่เผยแพร่ ข้อมูลไฟฟ้าในแผนที่ยังไม่ใช่การยืนยันกำลังไฟ' : 'AI combines available site evidence to recommend a cabinet quantity within the S / M / L layout ceiling, considering traffic, EV trend, population, nearby places, competitors, and published grid context. Published grid data does not confirm electrical capacity.'}</p> : null}
    {hasLegacyProposal && !proposal.data ? <p className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-950">{th ? 'คำแนะนำเดิมใช้สูตรพื้นที่รุ่นเก่า จึงไม่แสดงจำนวนตู้เดิม กรุณากด “สร้างคำแนะนำแบบใหม่” เพื่อใช้หลักเกณฑ์เฟสเริ่มต้นปัจจุบัน' : 'The saved recommendation used the previous land-based formula, so its cabinet quantity is hidden. Generate an updated recommendation to use the current initial-phase rule.'}</p> : null}
    {proposal.isError && <p role="alert" className="mt-3 text-red-700">{t(errorMessageKey(proposal.error))}</p>}
    {currentBlocker ? <div className="mt-4 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm leading-6 text-rose-950"><p className="font-bold">{th ? 'ยังไม่แนะนำจำนวนตู้ชาร์จ' : 'Charger quantity is not recommended yet'}</p><p className="mt-1">{th ? 'จุดเชื่อมต่อระบุว่าเป็นสายไฟฟ้าใต้ดิน ซึ่งนโยบายปัจจุบันยังไม่ลงทุนเพราะต้นทุนงานโยธาและเชื่อมต่อสูง' : 'The connection point is recorded as underground electricity; the current policy does not invest because civil-work and connection costs are high.'}</p></div> : null}
    {result && displayText && <div className="mt-4 space-y-4">
      {recommendationAvailable ? <><p className="text-xl font-bold text-brand">{`${result.powerKw} kW × ${result.chargerCount} ${th ? 'ตู้' : 'cabinet'} = ${result.totalPowerKw} kW`}</p><div className={`rounded-xl p-4 text-sm leading-6 ${result.capacityConfirmed ? 'bg-emerald-50 text-emerald-950' : 'border border-amber-200 bg-amber-50 text-amber-950'}`}><p className="font-bold">{result.capacityConfirmed ? (th ? 'PEA/MEA ยืนยันกำลังไฟแล้ว — ยังต้องออกแบบไฟฟ้าขั้นสุดท้าย' : 'Utility capacity confirmed — final electrical design is still required') : (th ? 'ผังตามคำแนะนำ — ต้องยืนยันกำลังไฟก่อนติดตั้งจริง' : 'Recommended layout — utility capacity must be confirmed before installation')}</p><p className="mt-1">{displayText.reason}</p></div>{!result.capacityConfirmed ? <div className="space-y-1 text-sm text-muted"><p>{th ? `รูปแบบ ${result.franchisePackage}: พื้นที่รวมแนะนำ ${result.recommendedAreaSqWah.toLocaleString()} ตร.วา รวมร้านกาแฟ บริการ ทางเข้า และสถานีชาร์จแล้ว` : `${result.franchisePackage} format: ${result.recommendedAreaSqWah.toLocaleString()} sq wah total site area, including café, services, access, and charging.`}</p><p>{th ? `จำนวนตามรูปแบบ ${result.layoutCabinetLimit} ตู้ · แนะนำให้จัดทางเข้า–ออกอย่างน้อย 7 ม. และตรวจ PEA/MEA เพิ่มก่อนติดตั้งจริง` : `Layout quantity: ${result.layoutCabinetLimit} cabinets · an entrance/exit at least 7 m is recommended; verify with PEA/MEA before installation.`}</p></div> : null}<p className="text-sm">{th ? 'จำนวนหมายถึงตู้ชาร์จ ไม่ใช่จำนวนหัวชาร์จ และกำลังไฟ จำนวนติดตั้งจริง และขนาดหม้อแปลงต้องยืนยันกับวิศวกรและ PEA/MEA' : 'Quantity means cabinets, not connectors; actual power, installed count, and transformer size require engineering and PEA/MEA confirmation.'}</p></> : null}
      {!recommendationAvailable ? <div className="rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm leading-6 text-rose-950"><p className="font-bold">{th ? 'ยังไม่แนะนำจำนวนตู้ชาร์จ' : 'Charger quantity is not recommended yet'}</p><p className="mt-1">{displayText.reason}</p></div> : null}
      <RecommendationLanguage heading={th ? 'ภาษาไทย' : 'English'} text={displayText} assumptionLabel={th ? 'สมมติฐานที่ใช้' : 'Assumptions'} missingLabel={th ? 'ข้อมูลที่ยังต้องยืนยัน' : 'Outstanding information'} />
    </div>}
  </section>
}
