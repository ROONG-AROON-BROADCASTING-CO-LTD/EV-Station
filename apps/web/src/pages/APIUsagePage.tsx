import { useQuery } from '@tanstack/react-query'
import { BarChart3, TriangleAlert } from 'lucide-react'
import { ErrorState, LoadingState } from '../components/PageState'
import { useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'
import type { APIUsageSummary } from '../types/domain'

export function APIUsagePage() {
  const { language, t } = useI18n()
  const usage = useQuery({ queryKey: ['api-usage'], queryFn: api.getAPIUsage, refetchInterval: 60_000 })
  if (usage.isLoading) return <LoadingState label={t('Loading API usage…')} />
  if (usage.isError || !usage.data) return <ErrorState error={usage.error} />
  return <div>
    <div className="flex items-start gap-3"><span className="grid h-11 w-11 place-items-center rounded-xl bg-emerald-50 text-brand"><BarChart3 size={23}/></span><div><h1 className="page-title">{t('API usage and costs')}</h1><p className="mt-1 text-sm text-muted">{t('Tracks only APIs with quotas or possible charges for the current month.')}</p></div></div>
    <div className="mt-6 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900"><TriangleAlert className="mr-2 inline-block" size={17}/><strong>{t('RBC system total')}</strong> {t('This is the request count recorded by RBC, not a provider invoice. Google cost is an estimate; use Cloud Billing as the source of truth.')}</div>
    <div className="mt-6 grid gap-5 xl:grid-cols-2">{usage.data.map(item => <UsageCard key={item.providerId} item={item} language={language} t={t}/>)}</div>
  </div>
}

function UsageCard({ item, language, t }: { item: APIUsageSummary; language: 'th' | 'en'; t: (key: string) => string }) {
  const number = new Intl.NumberFormat(language === 'th' ? 'th-TH' : 'en-US')
  const money = new Intl.NumberFormat(language === 'th' ? 'th-TH' : 'en-US', { style: 'currency', currency: 'THB', maximumFractionDigits: 2 })
  const included = item.includedUnits
  const hasQuota = included !== undefined
  const hasPrice = item.overagePriceThb !== undefined
  const percent = hasQuota && included > 0 ? Math.min(100, (item.usedUnits / included) * 100) : 0
  const overQuota = item.overageUnits > 0
  return <section className="rounded-2xl border border-line bg-white p-5 shadow-panel"><div className="flex items-start justify-between gap-3"><div><h2 className="text-lg font-extrabold text-ink">{item.displayName}</h2><p className="mt-1 text-sm text-muted">{t('Unit:')} {item.unitLabel}</p></div>{hasQuota ? <span className={`rounded-full px-3 py-1 text-xs font-bold ${overQuota ? 'bg-rose-100 text-rose-700' : 'bg-emerald-100 text-emerald-700'}`}>{t(overQuota ? 'Over quota' : 'Within quota')}</span> : <span className="rounded-full bg-slate-100 px-3 py-1 text-xs font-bold text-slate-600">{t('No quota')}</span>}</div>
    <div className="mt-5 grid grid-cols-2 gap-3"><Metric label={hasQuota ? `${t('Used')} / ${number.format(included!)}` : t('Used')} value={number.format(item.usedUnits)} /><Metric label={t(overQuota ? 'Over quota' : 'Remaining')} value={hasQuota ? number.format(overQuota ? item.overageUnits : item.remainingUnits ?? 0) : '—'} tone={overQuota ? 'text-rose-700' : ''}/></div>
    {hasQuota ? <><div className="mt-4 h-2 overflow-hidden rounded-full bg-slate-100"><div className={overQuota ? 'h-full bg-rose-500' : 'h-full bg-emerald-500'} style={{ width: `${percent}%` }}/></div><div className="mt-3 flex justify-between gap-3 text-sm"><span className="text-muted">{t('Allowance')} {number.format(included!)} {t('units/month')}</span><span className={overQuota ? 'font-bold text-rose-700 text-right' : 'font-bold text-ink text-right'}>{overQuota ? hasPrice ? `${t('Over quota')} ${number.format(item.overageUnits)} ${t('requests')} · ${t('Estimated')} ${money.format(item.estimatedCostThb ?? 0)}` : t('No over-quota rate is available.') : t('No over-quota cost.')}</span></div>{hasPrice && <p className="mt-3 text-xs leading-5 text-muted">{t('Over-quota rate:')} {money.format(item.overagePriceThb ?? 0)}/{t('requests')}</p>}{item.pricingNote && <p className="mt-2 text-xs leading-5 text-muted">{item.pricingNote}</p>}</> : <p className="mt-4 text-sm text-muted">{t('This provider has no quota data that RBC can compare.')}</p>}
  </section>
}
function Metric({ label, value, tone = '' }: { label: string; value: string; tone?: string }) { return <div className="rounded-xl bg-slate-50 p-3"><p className="text-xs font-semibold text-muted">{label}</p><p className={`mt-1 text-xl font-extrabold ${tone || 'text-ink'}`}>{value}</p></div> }
