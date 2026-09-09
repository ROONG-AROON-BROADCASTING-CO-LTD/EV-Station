import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Calculator, CheckCircle2, Circle, ClipboardList, Download, RefreshCw } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { DataStatus, DataStatusLegend } from '../components/DataStatus'
import { StationRecommendation } from '../components/StationRecommendation'
import { MapPanel, PrintableLocationMap, RadiusLabel, RadiusSelector } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { useI18n } from '../i18n/I18nProvider'
import { api, errorMessageKey } from '../services/api'
import type { AIAssessment, FranchisePlan, Metric, Site } from '../types/domain'

const metricKeys: Record<string, string> = { traffic: 'Traffic volume', road_accessibility: 'Road access', ev_demand: 'Registered EVs', population: 'Population', poi: 'Points of interest', competition: 'Competition', flood: 'Flood risk', electrical: 'Electrical readiness', site_requirements: 'Internet signal access', site_readiness: 'Real site condition analysis' }

export function AnalysisPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { t, language } = useI18n()
  const [rerunRadiusMeters, setRerunRadiusMeters] = useState<number>()
  const analysis = useQuery({ queryKey: ['analysis', id], queryFn: () => api.getAnalysis(id), enabled: Boolean(id) })
  const plans = useQuery({ queryKey: ['franchise-plans'], queryFn: api.getFranchisePlans })
  const site = useQuery({ queryKey: ['site', analysis.data?.siteId], queryFn: () => api.getSite(analysis.data!.siteId), enabled: Boolean(analysis.data?.siteId) })
  const rerun = useMutation({ mutationFn: () => api.runAnalysis(analysis.data!.siteId, rerunRadiusMeters ?? analysis.data!.analysisRadiusMeters), onSuccess: run => navigate(`/analysis/${run.id}`) })
  const recalculate = useMutation({ mutationFn: () => api.recalculatePreliminary(id), onSuccess: run => { queryClient.setQueryData(['analysis', id], run) } })
  const updateInternet = useMutation({ mutationFn: (internetAvailable: boolean | undefined) => api.updateSite(analysis.data!.siteId, { ...site.data!, internetAvailable }), onSuccess: updated => { queryClient.setQueryData(['site', updated.id], updated) } })
  const aiNotes = useMutation({ mutationFn: (refresh: boolean) => api.generateBilingualAssessment(id, refresh) })
	const { mutate: generateAINotes, reset: resetAINotes } = aiNotes
  const generatedForRun = useRef<string | undefined>(undefined)
  const currentRole = (() => { try { return JSON.parse(localStorage.getItem('rbc-session') || '{}').user?.role as string | undefined } catch { return undefined } })()
  const canManageAnalysis = currentRole === 'super_admin' || currentRole === 'admin' || currentRole === 'sales'
  useEffect(() => {
    if (!canManageAnalysis || !analysis.data?.id || generatedForRun.current === analysis.data.id) return
    generatedForRun.current = analysis.data.id
    resetAINotes()
    generateAINotes(false)
  }, [analysis.data?.id, canManageAnalysis, generateAINotes, resetAINotes])
  if (analysis.isLoading) return <LoadingState label="Loading analysis…" />
  if (analysis.isError || !analysis.data) return <ErrorState error={analysis.error} message="error.ANALYSIS_NOT_FOUND" />
  const run = analysis.data
	const availableAssessments = { ...(run.aiAssessments || {}), ...(aiNotes.data?.assessments || {}) }
	const displayedAssessment = availableAssessments[language] || availableAssessments.en
	const displayedAssessmentError = displayedAssessment ? undefined : aiNotes.data?.errors[language] || aiNotes.error
  const canUpdateInternet = canManageAnalysis
  const dateLocale = language === 'th' ? 'th-TH' : 'en-US'
  const downloadPdf = () => {
    const previousTitle = document.title
    document.title = `${site.data?.name || t('Analysis result')} - ${t('Analysis result')}`
    window.print()
    window.setTimeout(() => { document.title = previousTitle }, 1000)
  }
  return <div className="analysis-print-report">
    <header className="print-company-header">
      <img className="print-company-logo" src="/rbc-group-logo.jpg" alt="RBC Group" />
	  <div><p className="print-company-name">{language === 'th' ? 'บริษัท รุ่งอรุณบรอดคาสติ้ง จำกัด' : 'Rung Arun Broadcasting Co., Ltd.'}</p><p>{language === 'th' ? 'เลขที่ 9 ซ.รามอินทรา 21 แยก 1 ถนนรามอินทรา แขวงท่าแร้ง เขตบางเขน กรุงเทพมหานคร 10220' : '9 Soi Ramintra 21 Yaek 1, Ramintra Road, Tha Raeng, Bang Khen, Bangkok 10220'}</p><p>{language === 'th' ? 'โทร. 02-970-7552' : 'Tel. 02-970-7552'}</p></div>
    </header>
	<div className="flex flex-wrap items-start justify-between gap-5"><div><h1 className="page-title">{site.data?.name || t('Analysis result')}</h1><p className="mt-2 text-sm text-muted">{t('Analysis result')} · {new Date(run.createdAt).toLocaleString(dateLocale)}</p></div><div className="print-hide flex flex-wrap items-center gap-3">{canManageAnalysis ? <><RadiusSelector value={rerunRadiusMeters ?? run.analysisRadiusMeters} onChange={setRerunRadiusMeters} /><button className="button-secondary" onClick={() => rerun.mutate()} disabled={rerun.isPending}><RefreshCw size={17} />{t(rerun.isPending ? 'Running…' : 'Run new analysis')}</button></> : null}<button type="button" className="button-secondary" onClick={downloadPdf} title={t('Save this analysis as a PDF')}><Download size={17} />{t('Download PDF')}</button></div></div>
	<div className="mt-8 grid gap-5 xl:grid-cols-2"><ScoreCard run={run} canRecalculate={canManageAnalysis} isRecalculating={recalculate.isPending} recalculationError={recalculate.error?.message} onRecalculate={() => recalculate.mutate()} /><section className="flex flex-col"><div className="mb-3 flex items-start justify-between gap-3"><div><h2 className="section-title">{t('Location context')}</h2>{site.data?.latitude !== undefined && site.data?.longitude !== undefined ? <p className="mt-1 text-xs leading-5 text-muted">{t('Customer site pin:')} {site.data.latitude.toFixed(6)}, {site.data.longitude.toFixed(6)} · <a className="font-semibold text-brand hover:underline print-hide" href={`https://www.google.com/maps?q=${site.data.latitude},${site.data.longitude}`} target="_blank" rel="noreferrer">{t('Open in Google Maps')}</a></p> : null}</div><RadiusLabel meters={run.analysisRadiusMeters} /></div>{site.data?.latitude !== undefined && site.data?.longitude !== undefined ? <div className="print-site-location"><PrintableLocationMap latitude={site.data.latitude} longitude={site.data.longitude} /></div> : null}<div className="screen-map"><MapPanel latitude={site.data?.latitude} longitude={site.data?.longitude} radiusMeters={run.analysisRadiusMeters} className="min-h-[350px] flex-1" /></div></section></div>
    <InvestmentRecommendation run={run} assessment={displayedAssessment} isGenerating={aiNotes.isPending} />
    {canManageAnalysis ? <StationRecommendation key={run.id} id={run.id} /> : null}
	 <section className="mt-5 overflow-hidden rounded-2xl border border-line bg-white shadow-panel"><div className="border-b border-line bg-slate-50/70 px-5 py-4"><h2 className="section-title">{t('Analysis metrics')}</h2><p className="mt-1 text-sm text-muted">{t('Review the key results and data status for this location.')}</p></div><div className="overflow-x-auto"><table className="w-full min-w-[640px] text-left"><thead className="border-b border-line bg-slate-50/60 text-xs uppercase tracking-wide text-muted"><tr><th className="px-5 py-3">{t('Metric')}</th><th className="px-4 py-3">{t('Result')}</th><th className="px-4 py-3">{t('Data status')}</th></tr></thead><tbody className="divide-y divide-line">{run.metrics.map(metric => <MetricRow key={metric.id} metric={metric} site={site.data} canUpdateInternet={canUpdateInternet} onInternetChange={updateInternet.mutate} savingInternet={updateInternet.isPending} />)}</tbody></table></div>{updateInternet.isError ? <p className="border-t border-line px-5 py-3 text-sm text-rose-700">{t('Internet status could not be saved. Please try again.')}</p> : null}</section>
    <ProjectRecommendation plans={plans.data} isLoading={plans.isLoading} site={site.data} score={run.overallScore} assessment={displayedAssessment} />
    <AnalysisNotes assessment={displayedAssessment} isGenerating={aiNotes.isPending} error={displayedAssessmentError} canRegenerate={canManageAnalysis} onRegenerate={() => aiNotes.mutate(true)} fallback={run.recommendation} />
  </div>
}

function ScoreCard({ run, canRecalculate, isRecalculating, recalculationError, onRecalculate }: { run: import('../types/domain').AnalysisRun; canRecalculate: boolean; isRecalculating: boolean; recalculationError?: string; onRecalculate: () => void }) {
  const { t } = useI18n()
  const hasScore = run.overallScore !== undefined
  const grade = getScoreGrade(run.overallScore)
	return <section className="rounded-2xl border border-line bg-white p-6 shadow-panel"><div className="flex items-center justify-between gap-3"><div><p className="text-xs font-bold uppercase tracking-[0.14em] text-brand">{t('Screening result')}</p><h2 className="mt-1 section-title">{t(hasScore ? 'Preliminary assessment' : 'Assessment unavailable')}</h2></div><DataStatus status={run.assessmentStatus} compact /></div><div className="mt-6 text-center"><div className="flex items-end justify-center gap-2"><span className="text-6xl font-extrabold tracking-tight text-ink">{run.overallScore?.toFixed(1) ?? '—'}</span>{hasScore ? <span className="pb-2 text-lg font-semibold text-muted">/100</span> : null}</div><p className="mt-2 font-semibold">{t('Overall location screening score')}</p>{grade ? <p className="mt-2 text-lg font-extrabold text-brand">{t('Grade')} {grade}</p> : null}</div>{canRecalculate && !run.scoring ? <button type="button" className="button-secondary mt-4 w-full" onClick={onRecalculate} disabled={isRecalculating}><Calculator size={16} />{t(isRecalculating ? 'Calculating preliminary score…' : 'Calculate preliminary score')}</button> : null}{recalculationError ? <p role="alert" className="mt-3 text-xs text-red-700">{t(errorMessageKey(recalculationError))}</p> : null}<div className="mt-5 border-t border-line pt-5"><DataStatusLegend /></div></section>
}

type InvestmentDecision = 'invest' | 'not_recommended'

function InvestmentRecommendation({ run, assessment, isGenerating }: { run: import('../types/domain').AnalysisRun; assessment?: AIAssessment; isGenerating: boolean }) {
  const { t } = useI18n()
  const score = run.overallScore
  const electrical = run.metrics.find(metric => metric.type === 'electrical')
  const weakMetrics = run.metrics.filter(metric => metric.normalizedScore !== undefined && metric.normalizedScore < 50)
  const strongMetrics = run.metrics.filter(metric => metric.normalizedScore !== undefined && metric.normalizedScore >= 70)
  const electricalNeedsCheck = !electrical || electrical.status === 'missing'
	const decision: InvestmentDecision = score !== undefined && score >= 60 ? 'invest' : 'not_recommended'
	const labels: Record<InvestmentDecision, string> = { invest: t('Recommended to invest'), not_recommended: t('Not yet recommended to invest') }
  const descriptions: Record<InvestmentDecision, string | undefined> = {
    invest: assessment?.recommendation,
    not_recommended: assessment?.recommendation,
  }
  const reasons = [
	  assessment?.recommendation ?? (score === undefined ? t('The system has no screening score available for a conclusion.') : `${t('Overall location screening score')} ${score.toFixed(1)}/100`),
	  strongMetrics.length ? `${t('Strengths:')} ${strongMetrics.slice(0, 3).map(metric => t(metricKeys[metric.type] || metric.type)).join(', ')}` : undefined,
	  weakMetrics.length ? `${t('Factors to watch:')} ${weakMetrics.slice(0, 3).map(metric => t(metricKeys[metric.type] || metric.type)).join(', ')}` : undefined,
	  electricalNeedsCheck ? t('Utility readiness must still be confirmed with MEA or PEA at the actual site.') : undefined,
  ].filter((reason): reason is string => Boolean(reason))
  const optionStyles: Record<InvestmentDecision, { selected: string; icon: string }> = {
    invest: { selected: 'border-emerald-300 bg-emerald-50/70', icon: 'text-emerald-600' },
    not_recommended: { selected: 'border-rose-300 bg-rose-50/70', icon: 'text-rose-600' },
  }
	return <section className="mt-5 rounded-2xl border border-line bg-white p-5 shadow-panel sm:p-6"><div className="flex items-start gap-3"><span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-emerald-50 text-brand"><ClipboardList size={21} /></span><div><h2 className="section-title">{t('Investment recommendation summary')}</h2><p className="mt-1 text-sm text-muted">{t('The system determines the status from the screening score; AI explains the evidence.')}</p></div></div>{isGenerating ? <p className="mt-4 text-sm text-muted">{t('AI is analysing the recommendation…')}</p> : null}<div className="mt-5 grid gap-3 md:grid-cols-2">{(Object.keys(labels) as InvestmentDecision[]).map(option => { const selected = decision === option; const styles = optionStyles[option]; const description = descriptions[option]; return <div key={option} className={`rounded-xl border p-4 ${selected ? styles.selected : 'border-line bg-white opacity-60'}`}><div className="flex items-center gap-2"><span className={selected ? styles.icon : 'text-slate-400'}>{selected ? <CheckCircle2 size={20} /> : <Circle size={20} />}</span><p className="font-bold text-ink">{labels[option]}</p></div>{selected && description ? <p className="mt-2 text-sm leading-5 text-muted">{description}</p> : null}</div> })}</div><div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4"><p className="text-sm font-bold text-ink">{t('Reasons from AI and system data')}</p><ul className="mt-3 space-y-2 text-sm leading-6 text-muted">{reasons.map(reason => <li key={reason} className="flex gap-2"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-brand" />{reason}</li>)}</ul></div></section>
}

function AnalysisNotes({ assessment, isGenerating, error, canRegenerate, onRegenerate, fallback }: { assessment?: AIAssessment; isGenerating: boolean; error: unknown; canRegenerate: boolean; onRegenerate: () => void; fallback: string }) {
  const { t } = useI18n()
	return <section className="mt-6 rounded-xl border border-line bg-white p-6"><div className="flex flex-wrap items-start justify-between gap-4"><div><h2 className="section-title">{t('Notes')}</h2><p className="mt-1 text-sm text-muted">{t('Analysis based on the evidence in this analysis run.')}</p></div>{canRegenerate ? <button type="button" className="button-secondary min-h-10" onClick={onRegenerate} disabled={isGenerating}><RefreshCw size={16} className={isGenerating ? 'animate-spin' : undefined} />{isGenerating ? t('AI is analysing the recommendation…') : t('Regenerate AI analysis')}</button> : null}</div>{isGenerating ? <div className="mt-5 rounded-xl border border-emerald-100 bg-emerald-50/60 p-4 text-sm text-muted">{t('AI is summarising site potential, electrical readiness, and required checks…')}</div> : assessment ? <div className="mt-5 space-y-4"><div className="rounded-xl border border-emerald-100 bg-emerald-50/60 p-4"><p className="text-sm font-bold text-ink">{t('Overview')}</p><p className="mt-2 text-sm leading-6 text-muted">{assessment.summary}</p></div><div className="grid gap-4 lg:grid-cols-3"><NoteList title={t('Location strengths')} values={assessment.strengths} tone="emerald" /><NoteList title={t('Risk factors')} values={assessment.risks} tone="rose" /><NoteList title={t('Further checks required')} values={assessment.requiredChecks} tone="amber" /></div></div> : <div className="mt-5 rounded-xl border border-amber-100 bg-amber-50/60 p-4"><p className="text-sm text-amber-800">{error ? t(errorMessageKey(error)) : fallback}</p>{error && canRegenerate ? <p className="mt-2 text-xs leading-5 text-muted">{t('Try “Regenerate AI analysis” again.')}</p> : null}</div>}</section>
}

function NoteList({ title, values, tone }: { title: string; values: string[]; tone: 'emerald' | 'rose' | 'amber' }) {
  const colors = tone === 'emerald' ? 'border-emerald-100 bg-emerald-50/60 marker:text-emerald-600' : tone === 'rose' ? 'border-rose-100 bg-rose-50/60 marker:text-rose-600' : 'border-amber-100 bg-amber-50/60 marker:text-amber-600'
  return <section className={`rounded-xl border p-4 ${colors}`}><h3 className="text-sm font-bold text-ink">{title}</h3><ul className="mt-3 list-disc space-y-2 pl-5 text-sm leading-6 text-muted">{values.length ? values.map(item => <li key={item}>{item}</li>) : <li>—</li>}</ul></section>
}

function getScoreGrade(score?: number) {
  if (score === undefined) return undefined
  if (score >= 80) return 'A'
  if (score >= 70) return 'B'
  if (score >= 60) return 'C'
  return undefined
}

function MetricRow({ metric, site, canUpdateInternet, onInternetChange, savingInternet }: { metric: Metric; site?: Site; canUpdateInternet: boolean; onInternetChange: (value: boolean | undefined) => void; savingInternet: boolean }) {
  const { t } = useI18n(); const result = formatMetricResult(metric, t)
	if (metric.type === 'site_requirements') return <tr className="hover:bg-slate-50"><td className="px-5 py-4 text-sm font-semibold">{t('Internet signal access')}</td><td className="px-4 py-4"><InternetAccessChecklist available={site?.internetAvailable} canEdit={canUpdateInternet} onChange={onInternetChange} saving={savingInternet} /></td><td className="px-4 py-4"><DataStatus status={site?.internetAvailable === undefined ? 'missing' : 'preliminary'} compact /></td></tr>
  const aiAssisted = metric.assumptions.some(item => item.startsWith('Gemini assisted scoring:'))
  const poiObservation = metric.type === 'poi' ? getRawObservation(metric.rawValue) : undefined
  const siteSurface = metric.type === 'site_readiness' ? getSiteSurface(metric.rawValue) : undefined
  const unavailableReason = !result.available && metric.assumptions.length > 0 ? metric.assumptions[0] : undefined
  return <tr className="hover:bg-slate-50"><td className="px-5 py-4 text-sm font-semibold">{metricLabel(metric, t)}</td><td className="px-4 py-4 text-sm text-muted"><span className={result.available ? 'font-semibold text-ink' : undefined}>{result.text}</span>{metric.normalizedScore !== undefined ? <span className="mt-1 block text-xs font-semibold text-blue-700">{t(aiAssisted ? 'AI-assisted metric score' : 'Preliminary metric score')}: {metric.normalizedScore.toFixed(1)}/100</span> : null}{result.note ? <span className="mt-1 block text-xs text-muted">{result.note}</span> : null}{unavailableReason ? <span className="mt-1 block text-xs leading-5 text-amber-700">{t(unavailableReason)}</span> : null}{siteSurface ? <SiteSurfaceBreakdown assessment={siteSurface} /> : null}{poiObservation ? <PoiCategoryBreakdown observation={poiObservation} /> : null}</td><td className="px-4 py-4"><DataStatus status={metric.status} compact /></td></tr>
}

function InternetAccessChecklist({ available, canEdit, onChange, saving }: { available?: boolean; canEdit: boolean; onChange: (value: boolean | undefined) => void; saving: boolean }) {
	const { t } = useI18n()
  const choices = [
	  { label: t('Available'), value: true, selected: available === true, tone: 'border-emerald-300 bg-emerald-50 text-emerald-800' },
	  { label: t('Unavailable'), value: false, selected: available === false, tone: 'border-rose-300 bg-rose-50 text-rose-800' },
	  { label: t('Pending verification'), value: undefined, selected: available === undefined, tone: 'border-amber-300 bg-amber-50 text-amber-800' },
  ]
  return <div className="flex flex-wrap gap-2">{choices.map(choice => <button type="button" disabled={!canEdit || saving} onClick={() => onChange(choice.value)} key={choice.label} className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-bold transition ${choice.selected ? choice.tone : 'border-line bg-white text-slate-400'} ${canEdit ? 'hover:border-brand hover:text-brand' : 'cursor-default'}`}>{choice.selected ? <CheckCircle2 size={15} /> : <Circle size={15} />}{choice.label}</button>)}</div>
}

function metricLabel(metric: Metric, t: (key: string) => string) {
  if (metric.type === 'site_readiness') return t('Real site condition analysis')
  if (metric.type === 'ev_demand') {
    const value = getEVRegistration(metric.rawValue)
    return t(value?.previousRegisteredBev !== undefined ? 'Registered EVs and provincial trend' : 'Registered EVs')
  }
  return t(metricKeys[metric.type] || metric.type)
}

function ProjectRecommendation({ plans, isLoading, site, score, assessment }: { plans?: FranchisePlan[]; isLoading: boolean; site?: Site; score?: number; assessment?: AIAssessment }) {
	const { t, language } = useI18n()
  const area = toSquareWah(site?.landSize, site?.landSizeUnit)
  const frontage = site?.frontageMeters
  const minimumFrontageMeters = 6
  const widthMissing = frontage === undefined
  const widthInsufficient = frontage !== undefined && frontage < minimumFrontageMeters
  const eligible = (plans ?? []).filter(plan => area !== undefined && area >= plan.minimumAreaSqWah)
  const selected = eligible.sort((a, b) => b.minimumAreaSqWah - a.minimumAreaSqWah)[0]
	const investmentNotRecommended = score === undefined || score < 60
  const canRecommend = !!selected && !widthInsufficient && !investmentNotRecommended
  const isPreliminary = canRecommend && (widthMissing || !assessment)
	const reason = widthInsufficient ? (language === 'th' ? `หน้ากว้าง ${frontage.toFixed(1)} ม. ยังต่ำกว่าเกณฑ์เบื้องต้น ${minimumFrontageMeters} ม. สำหรับทางรถเข้า–ออกสวนกันได้` : `The ${frontage.toFixed(1)} m entrance frontage is below the preliminary ${minimumFrontageMeters} m requirement for two-way vehicle access.`) : !selected ? (language === 'th' ? `พื้นที่ ${area?.toFixed(1) ?? '—'} ตร.วา ยังต่ำกว่าขนาดขั้นต่ำของแพ็กเกจ S (${plans?.find(plan => plan.code === 'S')?.minimumAreaSqWah ?? 50} ตร.วา)` : `The ${area?.toFixed(1) ?? '—'} sq wah land area is below the minimum for package S (${plans?.find(plan => plan.code === 'S')?.minimumAreaSqWah ?? 50} sq wah).`) : investmentNotRecommended ? assessment?.recommendation ?? t('Not yet recommended to invest') : widthMissing ? (language === 'th' ? `พื้นที่มีขนาดเพียงพอสำหรับแพ็กเกจ ${selected.code}; ยังต้องวัดหน้ากว้างทางเข้า–ออกก่อนออกแบบและติดตั้งจริง` : `The land area is sufficient for package ${selected.code}; measure entrance frontage before the final design and installation.`) : assessment?.recommendation ?? (language === 'th' ? `พื้นที่และหน้ากว้างผ่านเกณฑ์เบื้องต้นสำหรับแพ็กเกจ ${selected.code}` : `The land area and frontage meet the preliminary criteria for package ${selected.code}.`)
	return <section className="mt-6 rounded-2xl border border-line bg-white p-6 shadow-panel"><div className="flex items-start gap-3"><span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-emerald-50 text-brand"><Calculator size={20} /></span><div><h2 className="section-title">{t('AI project-format recommendation')}</h2><p className="mt-1 text-sm text-muted">{t('Uses the latest analysis, land size, and entrance frontage to select S / M / L.')}</p></div></div>{isLoading ? <p className="mt-5 text-sm text-muted">{t('Loading project formats…')}</p> : <div className="mt-5 grid gap-4 lg:grid-cols-3">{plans?.map(plan => { const isSelected = canRecommend && plan.code === selected.code; return <article key={plan.code} className={`rounded-2xl border p-5 ${isSelected ? 'border-emerald-400 bg-emerald-50/70 shadow-sm' : 'border-line bg-white opacity-65'}`}><div className="flex items-center justify-between"><span className="text-4xl font-extrabold text-brand">{plan.code}</span>{isSelected ? <span className="rounded-full bg-emerald-600 px-2.5 py-1 text-xs font-bold text-white">{t(isPreliminary ? 'AI preliminary recommendation' : 'AI recommendation')}</span> : null}</div><h3 className="mt-3 font-bold text-ink">{plan.name}</h3><dl className="mt-4 space-y-2 text-sm"><div className="flex justify-between gap-3"><dt className="text-muted">{t('Minimum land area')}</dt><dd className="font-bold">{plan.minimumAreaSqWah.toLocaleString()} {t('sq wah')}</dd></div><div className="flex justify-between gap-3"><dt className="text-muted">{t('EV charging stations')}</dt><dd className="font-bold">{plan.evChargingStations}</dd></div></dl></article> })}</div>}<div className={`mt-5 rounded-xl border p-4 ${canRecommend ? 'border-emerald-200 bg-emerald-50/60' : 'border-amber-200 bg-amber-50/70'}`}><p className="font-bold text-ink">{canRecommend ? `${t(isPreliminary ? 'AI preliminary recommendation' : 'AI recommendation')}: ${t('Plan')} ${selected.code}` : t('No package recommended yet')}</p><p className="mt-2 text-sm leading-6 text-muted">{reason}</p><ul className="mt-3 space-y-1.5 text-sm text-muted"><li>• {t('Land area used for calculation:')} {area !== undefined ? `${area.toFixed(1)} ${t('sq wah')}` : t('No data')}</li><li>• {t('Entrance frontage:')} {frontage !== undefined ? `${frontage.toFixed(1)} ${t('m')}` : t('No data')}</li><li>• {t('Preliminary entrance requirement: at least')} {minimumFrontageMeters} {t('m for two cars to pass.')}</li></ul></div></section>
}

function PoiCategoryBreakdown({ observation }: { observation: RawObservation }) {
  const { t } = useI18n(); const groups = summarizePOICategoryGroups(observation.categoryCounts)
	if (!groups.length) return <p className="mt-2 text-xs text-amber-700">{t('No map records were found in this run. This does not mean there are no important places, so the zero result is excluded from scoring.')}</p>
  return <details className="mt-3 rounded-xl border border-emerald-100 bg-emerald-50/50 p-3"><summary className="cursor-pointer text-xs font-bold text-brand">{t('Category breakdown')}</summary><p className="mt-2 text-xs leading-5 text-muted">{t('The categories below total the observed places. EV charging stations are excluded and counted under competitors.')}</p><div className="mt-3 grid gap-2 sm:grid-cols-2">{groups.map(([category, count]) => <div key={category} className="flex items-center justify-between rounded-lg bg-white px-3 py-2 text-xs shadow-sm"><span className="font-medium text-ink">{t(category)}</span><span className="font-bold text-brand">{count.toLocaleString()}</span></div>)}</div></details>
}

function SiteSurfaceBreakdown({ assessment }: { assessment: SiteSurfaceValue }) {
  const { t } = useI18n()
  return <details className="mt-3 rounded-xl border border-amber-100 bg-amber-50/60 p-3"><summary className="cursor-pointer text-xs font-bold text-amber-800">{t('Ground-surface details')}</summary><p className="mt-2 text-xs leading-5 text-muted">{assessment.summary}</p><div className="mt-3 grid gap-3 sm:grid-cols-3"><SurfaceList title="Surface types" values={assessment.surfaceTypes} /><SurfaceList title="Observed risks" values={assessment.observedRisks} /><SurfaceList title="Recommended improvements" values={assessment.recommendedImprovements} /></div><p className="mt-3 text-xs leading-5 text-muted">{assessment.disclaimer}</p></details>
}
function SurfaceList({ title, values }: { title: string; values: string[] }) { const { t } = useI18n(); return <div><p className="text-xs font-bold text-ink">{t(title)}</p><ul className="mt-1.5 space-y-1 text-xs leading-5 text-muted">{values.length ? values.map(value => <li key={value}>• {value}</li>) : <li>—</li>}</ul></div> }

// This evidence view is retained for the next detail-panel iteration.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function PoiSample({ places, residentialBuildingCount = 0, communityAreaCount = 0, categoryCounts }: { places: RawPlace[]; residentialBuildingCount?: number; communityAreaCount?: number; categoryCounts?: Record<string, number> }) {
  const { t } = useI18n(); const categories = summarizeCategories(places); const namedPlaces: RawPlace[] = []; for (const place of places) if (place.name && namedPlaces.length < 8) namedPlaces.push(place)
  const completeCategoryGroups = summarizePOICategoryGroups(categoryCounts)
  return <div className="mt-5 border-t border-line pt-4"><h3 className="text-sm font-semibold">{t('POI sample')}</h3><p className="mt-2 text-xs leading-5 text-muted">{t('Stored examples only (up to 100 records). The total observed count includes every returned record.')}</p>{completeCategoryGroups.length ? <><h4 className="mt-4 text-xs font-bold uppercase tracking-wide text-muted">{t('Category breakdown')}</h4><div className="mt-2 grid gap-2 sm:grid-cols-2">{completeCategoryGroups.map(([category, count]) => <div key={category} className="flex justify-between rounded-lg bg-slate-50 px-3 py-2 text-xs"><span>{t(category)}</span><span className="font-bold text-ink">{count.toLocaleString()}</span></div>)}</div></> : null}<h4 className="mt-4 text-xs font-bold uppercase tracking-wide text-muted">{t('Community context')}</h4><div className="mt-2 grid gap-2 sm:grid-cols-2"><div className="rounded-xl bg-slate-50 px-3 py-3"><p className="text-xs text-muted">{t('Residential buildings mapped')}</p><p className="mt-1 text-lg font-bold text-ink">{residentialBuildingCount.toLocaleString()}</p></div><div className="rounded-xl bg-slate-50 px-3 py-3"><p className="text-xs text-muted">{t('Community areas mapped')}</p><p className="mt-1 text-lg font-bold text-ink">{communityAreaCount.toLocaleString()}</p></div></div><h4 className="mt-4 text-xs font-bold uppercase tracking-wide text-muted">{t('Sample categories')}</h4><div className="mt-2 flex flex-wrap gap-2">{categories.map(([category, categoryCount]) => <span key={category} className="rounded-full bg-slate-100 px-2.5 py-1 text-xs text-ink">{formatCategory(category)} · {categoryCount}</span>)}</div>{namedPlaces.length ? <><h4 className="mt-4 text-xs font-bold uppercase tracking-wide text-muted">{t('Example places')}</h4><ul className="mt-2 space-y-1.5 text-sm text-muted">{namedPlaces.map(place => <li key={`${place.osmType}-${place.osmId}`}><span className="font-medium text-ink">{place.name}</span><span> · {formatCategory(place.category)}</span></li>)}</ul></> : null}</div>
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
function CompetitionEvidence({ value, dateLocale }: { value: CompetitionObservation; dateLocale: string }) {
  const { t } = useI18n()
  return <div className="mt-5 border-t border-line pt-4"><h3 className="text-sm font-semibold">{t('Connected source coverage')}</h3><div className="mt-3 grid gap-3">{value.sources.map(source => <div key={`${source.name}-${source.referenceUri}`} className="rounded-xl border border-line bg-slate-50/70 p-4"><div className="flex items-start justify-between gap-3"><div><p className="text-sm font-semibold text-ink">{t(source.name)}</p><p className="mt-1 text-xs text-muted">{t(source.coverage)}</p></div><span className="rounded-full bg-white px-2.5 py-1 text-xs font-bold text-brand shadow-sm">{source.count.toLocaleString()} {t('records')}</span></div><p className="mt-2 text-xs text-muted">{t('Retrieved')} {new Date(source.retrievedAt).toLocaleString(dateLocale)}</p></div>)}</div>{value.places.length ? <><h4 className="mt-5 text-xs font-bold uppercase tracking-wide text-muted">{t('Example charging stations')}</h4><ul className="mt-2 space-y-2 text-sm text-muted">{value.places.slice(0, 8).map(place => <li key={`${place.recordType}-${place.recordId}`} className="rounded-lg bg-slate-50 px-3 py-2"><span className="font-medium text-ink">{place.name || t('Unnamed charging station')}</span>{place.operator ? <span> · {place.operator}</span> : null}<span className="mt-1 block text-xs">{place.sourceNames.map(t).join(' + ')}</span></li>)}</ul></> : null}</div>
}

type RawCount = { count: number; radiusMeters: number }
type RawPlace = { osmType: string; osmId: number; name?: string; category: string }
type RawObservation = RawCount & { places: RawPlace[]; categoryCounts?: Record<string, number>; excludedChargingStationCount?: number; residentialBuildingCount?: number; communityAreaCount?: number }
type PopulationValue = { population: number; populationDensityPerKm2: number; areaKm2: number; radiusMeters: number; dataYear: number; resolution: string }
type RoadValue = { radiusMeters: number; mappedMajorRoadCount: number; nearestMajorRoadMeters?: number; roadClassCounts: Record<string, number> }
type AADTValue = { aadt: number; dataYear: number; roadAuthority?: 'DOH' | 'DRR'; roadCode: string; roadName?: string; controlSection: string; controlSectionName: string; surveyKm: string; distanceToSectionMeters?: number; distanceToRoadMeters?: number; matchRadiusMeters: number }
type EVRegistrationValue = { province: string; registeredBev: number; datasetDate: string; previousRegisteredBev?: number; previousDatasetDate?: string; changePercent?: number; trend?: 'increased' | 'decreased' | 'unchanged'; provinceResolution: string }
type ElectricalPreliminaryValue = { assessmentType: string; matchingMethod?: 'point_in_published_area' | 'nearest_published_area'; distanceToAreaMeters?: number; mapYear?: number; voltageKv?: number; stationCode?: string; stationName?: string; publishedCapacityMw?: number; mapUrl?: string; serviceArea?: string; dataAvailable?: boolean }
type ElectricalGridValue = { assessmentType: 'pea_public_grid_evidence'; searchRadiusMeters: number; highVoltageLineCount: number; nearestHighVoltageLineMeters?: number; voltageCode?: string; conductorSize?: string; phaseCode?: string; feederId?: string; nearestStationMeters?: number; stationName?: string; stationSecondaryVoltageKv?: number }
type TerrainSamplingValue = { assessmentType: 'gistda_elevation_sampling'; sampleSpanMeters: number; sampleCount: number; minimumElevationMeters: number; maximumElevationMeters: number; elevationRangeMeters: number }
type SiteRequirementsValue = { assessmentType: 'automatic_site_requirements'; nearestPublishedElectricalKm?: number; terrain?: TerrainSamplingValue }
type FloodValue = { mappedFloodRiskAreaCount: number; radiusMeters: number; layerName: string }
type CompetitionSource = { name: string; referenceUri: string; coverage: string; count: number; retrievedAt: string }
type CompetitionPlace = { recordType: string; recordId: string; name?: string; operator?: string; sourceNames: string[] }
type CompetitionObservation = RawCount & { coverageMatched: boolean; places: CompetitionPlace[]; sources: CompetitionSource[] }
type SiteSurfaceValue = { imageCount: number; summary: string; suitability: 'low' | 'moderate' | 'high'; score: number; surfaceTypes: string[]; observedRisks: string[]; recommendedImprovements: string[]; disclaimer: string }
function getRecord(value: unknown): Record<string, unknown> | undefined { return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : undefined }
function finiteNumber(value: unknown): value is number { return typeof value === 'number' && Number.isFinite(value) }
function getRawCount(value: unknown): RawCount | undefined { const item = getRecord(value); return item && finiteNumber(item.count) && item.count >= 0 && finiteNumber(item.radiusMeters) && item.radiusMeters > 0 ? { count: item.count, radiusMeters: item.radiusMeters } : undefined }
function getPopulation(value: unknown): PopulationValue | undefined { const item = getRecord(value); return item && finiteNumber(item.population) && finiteNumber(item.populationDensityPerKm2) && finiteNumber(item.areaKm2) && finiteNumber(item.radiusMeters) && finiteNumber(item.dataYear) && typeof item.resolution === 'string' ? item as unknown as PopulationValue : undefined }
function getRoad(value: unknown): RoadValue | undefined { const item = getRecord(value); return item && finiteNumber(item.mappedMajorRoadCount) && finiteNumber(item.radiusMeters) && getRecord(item.roadClassCounts) ? item as unknown as RoadValue : undefined }
function getAADT(value: unknown): AADTValue | undefined { const item = getRecord(value); return item && finiteNumber(item.aadt) && item.aadt >= 0 && finiteNumber(item.dataYear) && (item.roadAuthority === undefined || item.roadAuthority === 'DOH' || item.roadAuthority === 'DRR') && typeof item.roadCode === 'string' && typeof item.controlSection === 'string' && typeof item.controlSectionName === 'string' && typeof item.surveyKm === 'string' && (finiteNumber(item.distanceToSectionMeters) || finiteNumber(item.distanceToRoadMeters)) && finiteNumber(item.matchRadiusMeters) ? item as unknown as AADTValue : undefined }
function getEVRegistration(value: unknown): EVRegistrationValue | undefined { const item = getRecord(value); return item && typeof item.province === 'string' && finiteNumber(item.registeredBev) && item.registeredBev >= 0 && typeof item.datasetDate === 'string' ? item as unknown as EVRegistrationValue : undefined }
function getElectricalPreliminary(value: unknown): ElectricalPreliminaryValue | undefined { const item = getRecord(value); return item && (item.assessmentType === 'published_station_area_guideline' || item.assessmentType === 'official_public_map_guideline') ? item as unknown as ElectricalPreliminaryValue : undefined }
function getElectricalGrid(value: unknown): ElectricalGridValue | undefined { const item = getRecord(value); return item && item.assessmentType === 'pea_public_grid_evidence' && finiteNumber(item.searchRadiusMeters) && finiteNumber(item.highVoltageLineCount) ? item as unknown as ElectricalGridValue : undefined }
function getSiteRequirements(value: unknown): SiteRequirementsValue | undefined { const item = getRecord(value); if (!item || item.assessmentType !== 'automatic_site_requirements') return undefined; const terrain = getRecord(item.terrain); const validTerrain = terrain && terrain.assessmentType === 'gistda_elevation_sampling' && finiteNumber(terrain.sampleSpanMeters) && finiteNumber(terrain.sampleCount) && finiteNumber(terrain.minimumElevationMeters) && finiteNumber(terrain.maximumElevationMeters) && finiteNumber(terrain.elevationRangeMeters) ? terrain as unknown as TerrainSamplingValue : undefined; return { ...(item as unknown as SiteRequirementsValue), ...(validTerrain ? { terrain: validTerrain } : {}) } }
function getFlood(value: unknown): FloodValue | undefined { const item = getRecord(value); return item && finiteNumber(item.mappedFloodRiskAreaCount) && item.mappedFloodRiskAreaCount >= 0 && finiteNumber(item.radiusMeters) && item.radiusMeters > 0 && typeof item.layerName === 'string' ? item as unknown as FloodValue : undefined }
function getSiteSurface(value: unknown): SiteSurfaceValue | undefined { const item = getRecord(value); if (!item || item.analysisScope !== 'ground_surface_only' || !finiteNumber(item.imageCount) || !finiteNumber(item.score) || typeof item.summary !== 'string' || typeof item.suitability !== 'string' || typeof item.disclaimer !== 'string') return undefined; const strings = (key: string) => Array.isArray(item[key]) ? item[key].filter((entry): entry is string => typeof entry === 'string') : []; return { imageCount: item.imageCount, summary: item.summary, suitability: item.suitability as SiteSurfaceValue['suitability'], score: item.score, surfaceTypes: strings('surfaceTypes'), observedRisks: strings('observedRisks'), recommendedImprovements: strings('recommendedImprovements'), disclaimer: item.disclaimer } }
function formatKilometers(meters: number): string { return (meters / 1000).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function getRawObservation(value: unknown): RawObservation | undefined { const count = getRawCount(value); const candidate = getRecord(value); if (!count || !candidate) return undefined; const places: RawPlace[] = []; if (Array.isArray(candidate.places)) for (const place of candidate.places) { const item = getRecord(place); if (!item || typeof item.osmType !== 'string' || !finiteNumber(item.osmId) || typeof item.category !== 'string') continue; places.push({ osmType: item.osmType, osmId: item.osmId, category: item.category, ...(typeof item.name === 'string' && item.name ? { name: item.name } : {}) }) } const categoryCounts: Record<string, number> = {}; const rawCategoryCounts = getRecord(candidate.categoryCounts); if (rawCategoryCounts) for (const [category, categoryCount] of Object.entries(rawCategoryCounts)) if (finiteNumber(categoryCount) && categoryCount >= 0) categoryCounts[category] = categoryCount; return { ...count, places, ...(Object.keys(categoryCounts).length ? { categoryCounts } : {}), ...(finiteNumber(candidate.excludedChargingStationCount) ? { excludedChargingStationCount: candidate.excludedChargingStationCount } : {}), ...(finiteNumber(candidate.residentialBuildingCount) ? { residentialBuildingCount: candidate.residentialBuildingCount } : {}), ...(finiteNumber(candidate.communityAreaCount) ? { communityAreaCount: candidate.communityAreaCount } : {}) } }
function getCompetitionObservation(value: unknown): CompetitionObservation | undefined { const count = getRawCount(value); const candidate = getRecord(value); if (!count || !candidate || !Array.isArray(candidate.sources) || !Array.isArray(candidate.places)) return undefined; const sources: CompetitionSource[] = []; for (const source of candidate.sources) { const item = getRecord(source); if (!item || typeof item.name !== 'string' || typeof item.referenceUri !== 'string' || typeof item.coverage !== 'string' || !finiteNumber(item.count) || typeof item.retrievedAt !== 'string') continue; sources.push(item as unknown as CompetitionSource) } const places: CompetitionPlace[] = []; for (const place of candidate.places) { const item = getRecord(place); if (!item || typeof item.recordType !== 'string' || typeof item.recordId !== 'string' || !Array.isArray(item.sourceNames)) continue; places.push({ recordType: item.recordType, recordId: item.recordId, sourceNames: item.sourceNames.filter((source): source is string => typeof source === 'string'), ...(typeof item.name === 'string' ? { name: item.name } : {}), ...(typeof item.operator === 'string' ? { operator: item.operator } : {}) }) } return { ...count, coverageMatched: candidate.coverageMatched === true, sources, places } }

function formatMetricResult(metric: Metric, t: (key: string) => string) {
	const siteSurface = getSiteSurface(metric.rawValue); if (siteSurface) return { text: `${t('Ground-surface suitability')}: ${t(siteSurface.suitability)} · ${siteSurface.score.toFixed(0)}/100`, note: `${siteSurface.imageCount} ${t('site photos analyzed')} · ${t('This score is separate from the location score.')}`, available: true }
	const siteRequirements = getSiteRequirements(metric.rawValue); if (siteRequirements) { const electricalNote = siteRequirements.nearestPublishedElectricalKm === undefined ? t('No published electrical-distance reference was returned for this location.') : `${t('Nearest published electrical reference')} ${siteRequirements.nearestPublishedElectricalKm.toFixed(2)} ${t('km')}. ${t('This is not the actual cable-routing distance.')}`; const terrainText = siteRequirements.terrain ? `${t('Automatic terrain check: elevation range')} ${siteRequirements.terrain.elevationRangeMeters.toFixed(2)} ${t('m across')} ${siteRequirements.terrain.sampleCount} ${t('samples over')} ${siteRequirements.terrain.sampleSpanMeters} ${t('m')}.` : t('Internet, Wi-Fi 2.4 GHz, and land frontage still need a connected evidence source for confirmation.'); const terrainNote = siteRequirements.terrain ? t('This is a sample elevation reading at the pin, not a land survey or earthwork estimate.') : ''; return { text: terrainText, note: [electricalNote, terrainNote].filter(Boolean).join(' · '), available: true } }
	const electricalGrid = getElectricalGrid(metric.rawValue); if (electricalGrid) { const line = electricalGrid.nearestHighVoltageLineMeters === undefined ? t('No published PEA high-voltage line found within the search radius') : `${t('Nearest published PEA high-voltage line')} ${formatKilometers(electricalGrid.nearestHighVoltageLineMeters)} ${t('km away')}`; const station = electricalGrid.nearestStationMeters === undefined ? '' : ` · ${t('Nearest PEA station')} ${electricalGrid.stationName || '—'} ${formatKilometers(electricalGrid.nearestStationMeters)} ${t('km away')}${electricalGrid.stationSecondaryVoltageKv !== undefined ? ` · ${electricalGrid.stationSecondaryVoltageKv} kV` : ''}`; const codes = [electricalGrid.voltageCode && `${t('Voltage code')} ${electricalGrid.voltageCode}`, electricalGrid.phaseCode && `${t('Phase code')} ${electricalGrid.phaseCode}`, electricalGrid.conductorSize && `${t('Conductor size')} ${electricalGrid.conductorSize}`].filter(Boolean).join(' · '); return { text: `${line}${station}`, note: [codes, t('Public PEA GIS evidence only — it does not confirm a pole at the plot, remaining capacity, or three-phase connection.')].filter(Boolean).join(' · '), available: true } }
  const electrical = getElectricalPreliminary(metric.rawValue); if (electrical) { if (electrical.assessmentType === 'official_public_map_guideline') return { text: t('PEA Power Map available for this area'), note: t('Open the official PEA Power Map to review the preliminary area information; capacity and three-phase supply still require PEA confirmation.'), available: true }; const nearest = electrical.matchingMethod === 'nearest_published_area'; const distance = nearest && electrical.distanceToAreaMeters !== undefined ? ` · ${t('nearest published area')} ${formatKilometers(electrical.distanceToAreaMeters)} ${t('km from the published boundary')}` : ''; return { text: `${electrical.publishedCapacityMw?.toLocaleString()} MW · ${electrical.voltageKv} kV · ${electrical.mapYear}${electrical.stationCode ? ` · ${electrical.stationCode}` : ''}`, note: nearest ? `${t('Nearest MEA published-area reference only — not capacity for this plot.')}${distance}` : t('Preliminary MEA Power Map guideline — not verified remaining capacity for this plot.'), available: true } }
  if (metric.type === 'electrical' && metric.status !== 'verified') return { text: t('Utility capacity not verified'), available: false }
  const population = getPopulation(metric.rawValue); if (population) return { text: `${Math.round(population.population).toLocaleString()} ${t('estimated people')} · ${population.populationDensityPerKm2.toLocaleString(undefined, { maximumFractionDigits: 0 })} ${t('people per sq. km')}`, note: `${t('Population density within the selected radius')} · ${population.areaKm2.toFixed(2)} ${t('sq. km')} · ${t('Modelled estimate — not an official census count.')}`, available: true }
  const aadt = getAADT(metric.rawValue); if (aadt) { const authority = aadt.roadAuthority === 'DRR' ? t('Rural road') : t('Highway'); const distance = aadt.distanceToSectionMeters ?? aadt.distanceToRoadMeters; return { text: `${aadt.aadt.toLocaleString()} ${t('vehicles/day (AADT)')} · ${authority} ${aadt.roadCode} · ${aadt.roadName || aadt.controlSectionName}`, note: `${t(aadt.roadAuthority === 'DRR' ? 'Matched official rural-road route' : 'Matched official control section')} · ${distance === undefined ? '—' : `${Math.round(distance).toLocaleString()} ${t('m away')}`} · ${aadt.dataYear}`, available: true } }
  const evRegistration = getEVRegistration(metric.rawValue); if (evRegistration) {
    const change = formatEVRegistrationTrend(evRegistration, t)
    return { text: `${evRegistration.registeredBev.toLocaleString()} ${t('registered BEVs')} · ${evRegistration.province}`, note: [change, t('Provincial indicator — not EV demand at this exact location.')].filter(Boolean).join(' · '), available: true }
  }
  const road = getRoad(metric.rawValue); if (road) return { text: `${road.mappedMajorRoadCount.toLocaleString()} ${t('mapped major-road segments')}${road.nearestMajorRoadMeters !== undefined ? ` · ${Math.round(road.nearestMajorRoadMeters).toLocaleString()} ${t('m to nearest mapped major road')}` : ''}`, note: t('Road-map assessment only — it does not confirm the actual entrance, turning movements, road width, or on-site obstructions.'), available: true }
  const flood = getFlood(metric.rawValue); if (flood) return { text: flood.mappedFloodRiskAreaCount ? `${flood.mappedFloodRiskAreaCount.toLocaleString()} ${t('mapped flood-risk areas')} · ${flood.radiusMeters / 1000} ${t('km radius')}` : t('No mapped flood-risk area within the requested radius'), note: t('Published-layer overlap only — not a flood forecast or a no-risk conclusion.'), available: true }
	const count = getRawCount(metric.rawValue); if (count) { const competition = metric.type === 'competition' ? getCompetitionObservation(metric.rawValue) : undefined; const label = metric.type === 'competition' ? t('charging-station records found') : t('points of interest'); const note = metric.type === 'competition' ? competition && !competition.coverageMatched ? t('The search completed, but the connected sources do not yet establish complete coverage for this area. This result is excluded from scoring.') : t('Deduplicated across connected sources — coverage is not nationwide.') : t('Observed count only — not a suitability score.'); return { text: `${count.count.toLocaleString()} ${label} · ${count.radiusMeters / 1000} ${t('km radius')}`, note, available: true } }
  if (metric.normalizedScore !== undefined) return { text: metric.normalizedScore.toFixed(1), available: true }
  return { text: t('Not available'), available: false }
}

function formatEVRegistrationTrend(value: EVRegistrationValue, t: (key: string) => string): string | undefined {
  if (value.previousRegisteredBev === undefined || !value.previousDatasetDate) return undefined
  const previous = `${value.previousRegisteredBev.toLocaleString()} ${t('registered BEVs')} · ${value.previousDatasetDate}`
  if (value.changePercent === undefined) return `${t('Previous official count')}: ${previous} · ${t('EV registrations started from zero in the comparison period.')}`
  const direction = value.trend === 'increased' ? t('increased') : value.trend === 'decreased' ? t('decreased') : t('unchanged')
  return `${t('Compared with previous official count')} (${previous}): ${direction} ${Math.abs(value.changePercent).toFixed(1)}%`
}

function summarizeCategories(places: RawPlace[]) { const counts = new Map<string, number>(); for (const place of places) counts.set(place.category, (counts.get(place.category) || 0) + 1); return Array.from(counts.entries()).sort(([firstCategory, firstCount], [secondCategory, secondCount]) => secondCount - firstCount || firstCategory.localeCompare(secondCategory)).slice(0, 6) }
function poiCategoryGroup(category: string) {
  if (category === 'amenity:hospital') return 'Hospitals'
  if (category === 'building:commercial') return 'Commercial buildings'
  if (category === 'tourism:hotel' || category === 'building:hotel') return 'Hotels'
  if (category === 'tourism:apartment') return 'Apartments'
  if (category === 'building:dormitory') return 'Dormitories'
  if (category === 'building:condominium') return 'Condominiums'
  if (category === 'building:apartments') return 'Apartments / condominiums (OSM combined)'
  if (/^tourism:(attraction|museum|zoo|theme_park)$/.test(category)) return 'Tourist attractions'
  if (category === 'building:office' || category.startsWith('office:')) return 'Office buildings'
  if (/^amenity:(restaurant|cafe|fast_food)$/.test(category)) return 'Food / cafe'
  if (category.startsWith('shop:') || category === 'amenity:marketplace') return 'Retail / shopping'
  if (/^amenity:(clinic|pharmacy)$/.test(category)) return 'Health services'
  if (/^amenity:(university|college|school|kindergarten)$/.test(category)) return 'Education'
  if (category.startsWith('leisure:') || /^tourism:(guest_house|hostel|motel)$/.test(category)) return 'Other places'
  if (/^amenity:(fuel|bus_station|parking)$/.test(category) || /^shop:(car|car_repair|car_parts|motorcycle)$/.test(category)) return 'Transport / automotive'
  return 'Other places'
}
function summarizePOICategoryGroups(categoryCounts?: Record<string, number>) { if (!categoryCounts) return []; const grouped = new Map<string, number>(['Hospitals', 'Commercial buildings', 'Hotels', 'Apartments', 'Dormitories', 'Condominiums', 'Tourist attractions', 'Office buildings'].map(category => [category, 0])); for (const [category, count] of Object.entries(categoryCounts)) { if (category === 'amenity:charging_station') continue; const group = poiCategoryGroup(category); grouped.set(group, (grouped.get(group) || 0) + count) } return Array.from(grouped.entries()).filter(([category, count]) => count > 0 || ['Hospitals', 'Commercial buildings', 'Hotels', 'Apartments', 'Dormitories', 'Condominiums', 'Tourist attractions', 'Office buildings'].includes(category)).sort(([firstCategory, firstCount], [secondCategory, secondCount]) => secondCount - firstCount || firstCategory.localeCompare(secondCategory)) }
function formatCategory(category: string) { return category.replace(/^[^:]+:/, '').replaceAll('_', ' ') }
function formatTHB(value: number, language: string) { return new Intl.NumberFormat(language === 'th' ? 'th-TH' : 'en-US', { style: 'currency', currency: 'THB', maximumFractionDigits: 0 }).format(value) }
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function formatPlanArea(plan: FranchisePlan, language: string, t: (key: string) => string) { const locale = language === 'th' ? 'th-TH' : 'en-US'; const minimum = plan.minimumAreaSqWah.toLocaleString(locale); if (plan.maximumAreaSqWah !== undefined) return `${minimum}–${plan.maximumAreaSqWah.toLocaleString(locale)} ${t('sq.wah')}`; return `${minimum}+ ${t('sq.wah')}` }
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function formatInvestment(plan: FranchisePlan, language: string) { const minimum = formatTHB(plan.investmentMinThb, language); if (plan.investmentMaxThb) return `${minimum} – ${formatTHB(plan.investmentMaxThb, language)}`; if (plan.investmentUpperReferenceThb) return `${minimum} – ${formatTHB(plan.investmentUpperReferenceThb, language)}+`; return `${minimum}+` }
function toSquareWah(value?: number, unit?: string) { if (value === undefined) return undefined; return unit === 'sqm' ? value / 4 : unit === 'rai' ? value * 400 : unit === 'ngan' ? value * 100 : unit === 'sqwah' ? value : undefined }
