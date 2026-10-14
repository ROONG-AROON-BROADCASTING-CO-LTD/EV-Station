import type { ReactNode } from 'react'
import { DataStatus } from '../../components/DataStatus'
import type { Metric } from '../../types/domain'

type MetricResult = { text: string; note?: string; available: boolean }

type MetricRowProps = {
  metric: Metric
  label: string
  result: MetricResult
  scoreLabel: string
  unavailableReason?: string
  detail?: ReactNode
  internetControl?: ReactNode
  statusOverride?: 'missing' | 'preliminary'
}

export function MetricRow({ metric, label, result, scoreLabel, unavailableReason, detail, internetControl, statusOverride }: MetricRowProps) {
  if (internetControl) return <tr className="block hover:bg-slate-50 sm:table-row print:table-row"><td className="block px-5 pb-1 pt-4 text-sm font-semibold sm:table-cell sm:py-4 print:table-cell">{label}</td><td className="block px-4 py-3 sm:table-cell sm:py-4 print:table-cell">{internetControl}</td><td className="block px-4 py-3 sm:table-cell sm:py-4 print:table-cell"><DataStatus status={statusOverride ?? metric.status} compact /></td></tr>
  return <tr className="block hover:bg-slate-50 sm:table-row print:table-row"><td className="block px-5 pb-1 pt-4 text-sm font-semibold sm:table-cell sm:py-4 print:table-cell">{label}</td><td className="block px-4 py-3 sm:table-cell sm:py-4 print:table-cell text-sm text-muted"><span className={result.available ? 'font-semibold text-ink' : undefined}>{result.text}</span>{typeof metric.normalizedScore === 'number' && Number.isFinite(metric.normalizedScore) ? <span className="mt-1 block text-xs font-semibold text-blue-700">{scoreLabel}: {metric.normalizedScore.toFixed(1)}/100</span> : null}{result.note ? <span className="mt-1 block text-xs text-muted">{result.note}</span> : null}{unavailableReason ? <span className="mt-1 block text-xs leading-5 text-amber-700">{unavailableReason}</span> : null}{detail}</td><td className="block px-4 py-3 sm:table-cell sm:py-4 print:table-cell"><DataStatus status={metric.status} compact /></td></tr>
}
