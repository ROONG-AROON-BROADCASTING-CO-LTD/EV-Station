import type { ReactNode } from 'react'
import type { Metric } from '../../types/domain'
import { useI18n } from '../../i18n/I18nProvider'

type MetricResultsTableProps = {
  metrics: Metric[]
  renderMetric: (metric: Metric) => ReactNode
  saveError?: boolean
}

export function MetricResultsTable({ metrics, renderMetric, saveError }: MetricResultsTableProps) {
  const { t } = useI18n()
  return <section className="print-breakable-table mt-5 overflow-hidden rounded-2xl border border-line bg-white shadow-panel"><div className="border-b border-line bg-slate-50/70 px-5 py-4"><h2 className="section-title">{t('Analysis metrics')}</h2><p className="mt-1 text-sm text-muted">{t('Review the key results and data status for this location.')}</p></div><div className="overflow-x-auto"><table className="w-full min-w-[640px] text-left"><thead className="border-b border-line bg-slate-50/60 text-xs uppercase tracking-wide text-muted"><tr><th className="px-5 py-3">{t('Metric')}</th><th className="px-4 py-3">{t('Result')}</th><th className="px-4 py-3">{t('Data status')}</th></tr></thead><tbody className="divide-y divide-line">{metrics.map(renderMetric)}</tbody></table></div>{saveError ? <p className="border-t border-line px-5 py-3 text-sm text-rose-700">{t('Internet status could not be saved. Please try again.')}</p> : null}</section>
}
