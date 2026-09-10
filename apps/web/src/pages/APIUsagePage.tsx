import { useQuery } from '@tanstack/react-query'
import { BarChart3, TriangleAlert } from 'lucide-react'
import { ErrorState, LoadingState } from '../components/PageState'
import { api } from '../services/api'
import type { APIUsageSummary } from '../types/domain'

const number = new Intl.NumberFormat('th-TH')
const money = new Intl.NumberFormat('th-TH', { style: 'currency', currency: 'THB', maximumFractionDigits: 2 })

export function APIUsagePage() {
  const usage = useQuery({ queryKey: ['api-usage'], queryFn: api.getAPIUsage, refetchInterval: 60_000 })
  if (usage.isLoading) return <LoadingState label="กำลังโหลดการใช้งาน API…" />
  if (usage.isError || !usage.data) return <ErrorState error={usage.error} />
  return <div>
    <div className="flex items-start gap-3"><span className="grid h-11 w-11 place-items-center rounded-xl bg-emerald-50 text-brand"><BarChart3 size={23}/></span><div><h1 className="page-title">การใช้งาน API และค่าใช้จ่าย</h1><p className="mt-1 text-sm text-muted">นับเฉพาะ API ที่มีโควตาหรืออาจคิดค่าบริการ รอบเดือนปัจจุบัน</p></div></div>
    <div className="mt-6 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900"><TriangleAlert className="mr-2 inline-block" size={17}/><strong>ยอดจากระบบ RBC</strong> เป็นจำนวนคำขอที่ระบบบันทึก ไม่ใช่ใบแจ้งหนี้จากผู้ให้บริการ ค่าใช้จ่าย Google เป็นเพียงยอดประมาณการ; ยอดจริงให้ยึด Cloud Billing</div>
    <div className="mt-6 grid gap-5 xl:grid-cols-2">{usage.data.map(item => <UsageCard key={item.providerId} item={item}/>)}</div>
  </div>
}

function UsageCard({ item }: { item: APIUsageSummary }) {
  const included = item.includedUnits
  const hasQuota = included !== undefined
  const hasPrice = item.overagePriceThb !== undefined
  const percent = hasQuota && included > 0 ? Math.min(100, (item.usedUnits / included) * 100) : 0
  const overQuota = item.overageUnits > 0
  return <section className="rounded-2xl border border-line bg-white p-5 shadow-panel"><div className="flex items-start justify-between gap-3"><div><h2 className="text-lg font-extrabold text-ink">{item.displayName}</h2><p className="mt-1 text-sm text-muted">หน่วย: {item.unitLabel}</p></div>{hasQuota ? <span className={`rounded-full px-3 py-1 text-xs font-bold ${overQuota ? 'bg-rose-100 text-rose-700' : 'bg-emerald-100 text-emerald-700'}`}>{overQuota ? 'เกินโควตา' : 'อยู่ในโควตา'}</span> : <span className="rounded-full bg-slate-100 px-3 py-1 text-xs font-bold text-slate-600">ยังไม่มีโควตา</span>}</div>
    <div className="mt-5 grid grid-cols-2 gap-3"><Metric label={hasQuota ? `ใช้ไป จาก ${number.format(included!)}` : 'ใช้ไป'} value={number.format(item.usedUnits)} /><Metric label={overQuota ? 'เกินโควตา' : 'เหลือใช้'} value={hasQuota ? number.format(overQuota ? item.overageUnits : item.remainingUnits ?? 0) : '—'} tone={overQuota ? 'text-rose-700' : ''}/></div>
    {hasQuota ? <><div className="mt-4 h-2 overflow-hidden rounded-full bg-slate-100"><div className={overQuota ? 'h-full bg-rose-500' : 'h-full bg-emerald-500'} style={{ width: `${percent}%` }}/></div><div className="mt-3 flex justify-between gap-3 text-sm"><span className="text-muted">สิทธิ์ {number.format(included!)} หน่วย/เดือน</span><span className={overQuota ? 'font-bold text-rose-700 text-right' : 'font-bold text-ink text-right'}>{overQuota ? hasPrice ? `เกิน ${number.format(item.overageUnits)} ครั้ง · ประมาณ ${money.format(item.estimatedCostThb ?? 0)}` : 'ยังไม่มีอัตราค่าใช้จ่ายหลังเกินโควตา' : 'ยังไม่มีค่าใช้จ่ายเกินโควตา'}</span></div>{hasPrice && <p className="mt-3 text-xs leading-5 text-muted">อัตราหลังเกินโควตา: {money.format(item.overagePriceThb ?? 0)}/ครั้ง</p>}{item.pricingNote && <p className="mt-2 text-xs leading-5 text-muted">{item.pricingNote}</p>}</> : <p className="mt-4 text-sm text-muted">ผู้ให้บริการนี้ยังไม่มีข้อมูลโควตาที่ระบบใช้เปรียบเทียบได้</p>}
  </section>
}
function Metric({ label, value, tone = '' }: { label: string; value: string; tone?: string }) { return <div className="rounded-xl bg-slate-50 p-3"><p className="text-xs font-semibold text-muted">{label}</p><p className={`mt-1 text-xl font-extrabold ${tone || 'text-ink'}`}>{value}</p></div> }
