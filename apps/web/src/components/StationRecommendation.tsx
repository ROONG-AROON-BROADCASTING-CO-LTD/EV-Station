import { useMutation } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { useI18n } from '../i18n/I18nProvider'
import { api, errorMessageKey } from '../services/api'

type StationText = { reason: string; assumptions: string[]; missingData: string[] }

export interface StationProposal {
  powerKw: number
  chargerCount: number
  totalPowerKw: number
  installationConfirmed: boolean
  landAreaSqWah: number
  preliminaryCabinetLimit: number
  th: StationText
  en: StationText
}

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

export function StationRecommendation({ id }: { id: string }) {
  const { language, t } = useI18n()
  const th = language === 'th'
  const proposal = useMutation({ mutationFn: (refresh: boolean) => api.recommendStation(id, refresh) })
  const generatedForRun = useRef<string | undefined>(undefined)

  useEffect(() => {
    if (generatedForRun.current === id) return
    generatedForRun.current = id
    proposal.mutate(false)
  }, [id, proposal])

  const result = proposal.data
	const text = result?.[language]
  return <section className="mt-5 rounded-2xl border border-line bg-white p-6 shadow-panel">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2 className="section-title">{th ? 'คำแนะนำเบื้องต้น: ขนาดและจำนวนตู้ชาร์จ' : 'Preliminary recommendation: charger power and quantity'}</h2>
      <button className="button-secondary print-hide" disabled={proposal.isPending} onClick={() => proposal.mutate(true)}>{proposal.isPending ? (th ? 'AI กำลังวิเคราะห์…' : 'Analyzing…') : (th ? 'ให้ AI วิเคราะห์ใหม่' : 'Regenerate AI recommendation')}</button>
    </div>
    <p className="mt-2 text-sm text-muted">{th ? '120 / 180 / 240 kW ต่อตู้ · ระบบบันทึกคำแนะนำไว้กับผลวิเคราะห์นี้ และจะสร้างใหม่เมื่อกดปุ่มเท่านั้น' : '120 / 180 / 240 kW per cabinet · This recommendation is saved with this analysis and regenerated only when requested.'}</p>
    {proposal.isError && <p role="alert" className="mt-3 text-red-700">{t(errorMessageKey(proposal.error))}</p>}
    {result && text && <div className="mt-4 space-y-4">
      <p className="text-xl font-bold text-brand">{`${result.powerKw} kW × ${result.chargerCount} ${th ? 'ตู้' : 'cabinets'} = ${result.totalPowerKw} kW`}</p>
      <div className="rounded-xl bg-emerald-50 p-4 text-sm leading-6">
        <p>{th ? <><strong>เพดานจำนวนตู้เบื้องต้นจากพื้นที่:</strong> สูงสุด {result.preliminaryCabinetLimit} ตู้ จากพื้นที่ {result.landAreaSqWah.toLocaleString(undefined, { maximumFractionDigits: 2 })} ตร.วา</> : <><strong>Preliminary land-based limit:</strong> up to {result.preliminaryCabinetLimit} cabinets from {result.landAreaSqWah.toLocaleString(undefined, { maximumFractionDigits: 2 })} sq wah.</>}</p>
      </div>
      <p className="text-sm">{th ? 'เป็นคำแนะนำเบื้องต้นเพื่อคัดกรองและวางแผนลงทุน จำนวนหมายถึงตู้ชาร์จ ไม่ใช่จำนวนหัวชาร์จ ช่างและการไฟฟ้าต้องยืนยันกำลังไฟ จุดเชื่อมต่อ และผังติดตั้งก่อนดำเนินการจริง' : 'This is a preliminary screening and investment-planning recommendation. Quantity means cabinets, not connectors. An electrician and utility must confirm supply, connection point, and layout before installation.'}</p>
      <RecommendationLanguage heading={th ? 'ภาษาไทย' : 'English'} text={text} assumptionLabel={th ? 'สมมติฐานที่ใช้' : 'Assumptions'} missingLabel={th ? 'ข้อมูลที่ยังต้องยืนยัน' : 'Outstanding information'} />
    </div>}
  </section>
}
