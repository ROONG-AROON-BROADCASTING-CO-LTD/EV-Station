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
  if (internetControl) return <tr className="hover:bg-slate-50"><td className="px-5 py-4 text-sm font-semibold">{label}</td><td className="px-4 py-4">{internetControl}</td><td className="px-4 py-4"><DataStatus status={statusOverride ?? metric.status} compact /></td></tr>
  return <tr className="hover:bg-slate-50"><td className="px-5 py-4 text-sm font-semibold">{label}</td><td className="px-4 py-4 text-sm text-muted"><span className={result.available ? 'font-semibold text-ink' : undefined}>{result.text}</span>{typeof metric.normalizedScore === 'number' && Number.isFinite(metric.normalizedScore) ? <span className="mt-1 block text-xs font-semibold text-blue-700">{scoreLabel}: {metric.normalizedScore.toFixed(1)}/100</span> : null}{result.note ? <span className="mt-1 block text-xs text-muted">{result.note}</span> : null}{unavailableReason ? <span className="mt-1 block text-xs leading-5 text-amber-700">{unavailableReason}</span> : null}{detail}</td><td className="px-4 py-4"><DataStatus status={metric.status} compact /></td></tr>
}
