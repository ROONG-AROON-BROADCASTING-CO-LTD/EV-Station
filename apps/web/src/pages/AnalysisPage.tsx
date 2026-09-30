import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Calculator, CheckCircle2, Circle, ClipboardList, Download, Expand, RefreshCw, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { DataStatus, DataStatusLegend } from '../components/DataStatus'
import { StationRecommendation } from '../components/StationRecommendation'
import { MapPanel, PrintableLocationMap, RadiusLabel, RadiusSelector } from '../components/MapPanel'
import { ErrorState, LoadingState } from '../components/PageState'
import { MetricResultsTable } from './analysis/MetricResultsTable'
import { MetricRow as MetricRowView } from './analysis/MetricRow'
import { selectProjectFormat } from './analysis/projectRecommendation'
import { getScoreGrade } from './analysis/score'
import { floodAccessWarning, floodHistoryPeriod, provinceFloodNote } from './analysis/flood'
import { useI18n } from '../i18n/I18nProvider'
import { api, errorMessageKey } from '../services/api'
import type { AIAssessment, FranchisePlan, Metric, Site, UserRole } from '../types/domain'

const metricKeys: Record<string, string> = { traffic: 'Traffic volume', road_accessibility: 'Road access', ev_demand: 'Registered EVs', population: 'Population', poi: 'Points of interest', competition: 'Competition', flood: 'Flood risk', electrical: 'Electrical readiness', site_requirements: 'Internet signal access', site_readiness: 'Real site condition analysis' }
const planPosterUrl = (code: 'S' | 'M' | 'L') => code === 'S' ? '/franchise-formats/smart-cafe-s.png' : code === 'M' ? '/franchise-formats/lifestyle-cafe-m.png' : '/franchise-formats/lifestyle-hub-l.png'
const franchisePlanProfiles = {
  S: { dimensions: '20 × 20 ม.', idealSites: 'ชุมชน · หน้าอาคาร · ปั๊มน้ำมัน · จุด Takeaway', services: 'กาแฟ + EV Charging 1 สถานี', investment: '1.5–2.5 ล้านบาท', franchiseFee: '300,000 บาท', idealSitesEn: 'Community · building frontage · petrol station · takeaway point', servicesEn: 'Coffee + 1 EV charging station', investmentEn: 'THB 1.5–2.5M', franchiseFeeEn: 'THB 300,000' },
  M: { dimensions: '28 × 28 ม.', idealSites: 'ถนนใหญ่ · Community Mall · ออฟฟิศ · ปั๊มขนาดใหญ่', services: 'กาแฟ · อาหารและเบเกอรี่ · BPOST65 Express · โปรโมชั่น · EV Charging 2 สถานี', investment: '3.5–5 ล้านบาท', franchiseFee: '500,000 บาท', idealSitesEn: 'Main road · community mall · office · large petrol station', servicesEn: 'Coffee · food & bakery · BPOST65 Express · post office · 2 EV charging stations', investmentEn: 'THB 3.5–5M', franchiseFeeEn: 'THB 500,000' },
  L: { dimensions: '40 × 40 ม.', idealSites: 'ถนนหลัก · ทางเข้าตัวเมือง · จุดพักรถ · แหล่งท่องเที่ยว', services: 'กาแฟ · อาหารและเบเกอรี่ · BPOST65 Express · โปรโมชั่น · EV Charging 4 สถานี · Work Zone · Mobile Café · พื้นที่จัดงาน', investment: '7–10 ล้านบาทขึ้นไป', franchiseFee: '700,000 บาท', idealSitesEn: 'Main road · town entrance · rest stop · tourist destination', servicesEn: 'Coffee · food & bakery · BPOST65 Express · post office · 4 EV charging stations · work zone · mobile café · event space', investmentEn: 'THB 7–10M+', franchiseFeeEn: 'THB 700,000' },
} satisfies Record<'S' | 'M' | 'L', Record<string, string>>

export function AnalysisPage({ role }: { role: UserRole }) {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { t, language } = useI18n()
  const [rerunRadiusMeters, setRerunRadiusMeters] = useState<number>()
	const analysis = useQuery({ queryKey: ['analysis', id], queryFn: () => api.getAnalysis(id), enabled: Boolean(id), refetchInterval: query => ['pending', 'running'].includes(query.state.data?.status || '') ? 1000 : false })
  const plans = useQuery({ queryKey: ['franchise-plans'], queryFn: api.getFranchisePlans })
  const site = useQuery({ queryKey: ['site', analysis.data?.siteId], queryFn: () => api.getSite(analysis.data!.siteId), enabled: Boolean(analysis.data?.siteId) })
  const rerun = useMutation({ mutationFn: () => api.runAnalysis(analysis.data!.siteId, rerunRadiusMeters ?? analysis.data!.analysisRadiusMeters), onSuccess: run => navigate(`/analysis/${run.id}`) })
  const recalculate = useMutation({ mutationFn: () => api.recalculatePreliminary(id), onSuccess: run => { queryClient.setQueryData(['analysis', id], run) } })
  const updateInternet = useMutation({ mutationFn: (internetAvailable: boolean | undefined) => api.updateSite(analysis.data!.siteId, { ...site.data!, internetAvailable }), onSuccess: updated => { queryClient.setQueryData(['site', updated.id], updated) } })
  const aiNotes = useMutation({ mutationFn: ({ refresh, languages }: { refresh: boolean; languages: readonly ('th' | 'en')[] }) => api.generateAIAssessments(id, languages, refresh) })
	const { mutate: generateAINotes, reset: resetAINotes } = aiNotes
  const generatedForRun = useRef<string | undefined>(undefined)
  const canManageAnalysis = role === 'super_admin' || role === 'admin' || role === 'sales'
  const requestedLanguage = language
  const hasStoredAssessment = Boolean(analysis.data?.aiAssessments?.[requestedLanguage])
  useEffect(() => {
    const runID = analysis.data?.id
		if (!canManageAnalysis || !runID || analysis.data?.status !== 'completed' || hasStoredAssessment || generatedForRun.current === `${runID}:${requestedLanguage}`) return
    generatedForRun.current = `${runID}:${requestedLanguage}`
    resetAINotes()
    generateAINotes({ refresh: false, languages: [requestedLanguage] })
	}, [analysis.data?.id, analysis.data?.status, canManageAnalysis, generateAINotes, hasStoredAssessment, requestedLanguage, resetAINotes])
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
	if (run.status === 'pending' || run.status === 'running' || run.status === 'failed') {
		const failed = run.status === 'failed'
		return <section className="rounded-2xl border border-line bg-white p-8 shadow-panel" role="status">
			<h1 className="page-title">{failed ? (language === 'th' ? 'วิเคราะห์ไม่สำเร็จ' : 'Analysis failed') : (language === 'th' ? 'กำลังวิเคราะห์พื้นที่' : 'Analyzing site')}</h1>
			<p className="mt-3 text-sm text-muted">{failed ? (language === 'th' ? 'ระบบไม่สามารถรวบรวมข้อมูลได้ครบในรอบนี้ กรุณาลองใหม่' : 'The system could not complete this analysis. Please try again.') : (language === 'th' ? 'ระบบรับงานแล้วและกำลังรวบรวมข้อมูล ผลจะปรากฏในหน้านี้เมื่อเสร็จ' : 'The request is queued. This page will show the report when it is ready.')}</p>
			<p className="mt-2 text-xs text-muted">{language === 'th' ? 'สถานะ:' : 'Status:'} {run.status}</p>
			{failed && canManageAnalysis ? <button className="button-primary mt-5" onClick={() => rerun.mutate()} disabled={rerun.isPending}>{language === 'th' ? 'ลองวิเคราะห์ใหม่' : 'Try again'}</button> : null}
			{rerun.isError ? <div className="mt-4"><ErrorState error={rerun.error}/></div> : null}
		</section>
	}
	return <div className="analysis-print-report">
    <header className="print-company-header">
      <img className="print-company-logo" src="/rbc-group-logo.png" alt="RBC Group" />
	  <div><p className="print-company-name">{language === 'th' ? 'บริษัท รุ่งอรุณบรอดคาสติ้ง จำกัด' : 'Rung Arun Broadcasting Co., Ltd.'}</p><p>{language === 'th' ? 'เลขที่ 9 ซ.รามอินทรา 21 แยก 1 ถนนรามอินทรา แขวงท่าแร้ง เขตบางเขน กรุงเทพมหานคร 10220' : '9 Soi Ramintra 21 Yaek 1, Ramintra Road, Tha Raeng, Bang Khen, Bangkok 10220'}</p><p>{language === 'th' ? 'โทร. 02-970-7552' : 'Tel. 02-970-7552'}</p></div>
    </header>
	<div className="flex flex-wrap items-start justify-between gap-5"><div><h1 className="page-title">{site.data?.name || t('Analysis result')}</h1><p className="mt-2 text-sm text-muted">{t('Analysis result')} · {new Date(run.createdAt).toLocaleString(dateLocale)}</p></div><div className="print-hide flex flex-wrap items-center gap-3">{canManageAnalysis ? <><RadiusSelector value={rerunRadiusMeters ?? run.analysisRadiusMeters} onChange={setRerunRadiusMeters} /><button className="button-secondary" onClick={() => rerun.mutate()} disabled={rerun.isPending}><RefreshCw size={17} />{t(rerun.isPending ? 'Running…' : 'Run new analysis')}</button></> : null}<button type="button" className="button-secondary" onClick={downloadPdf} title={t('Save this analysis as a PDF')}><Download size={17} />{t('Download PDF')}</button></div></div>
	<div className="mt-8 grid gap-5 xl:grid-cols-2"><ScoreCard run={run} canRecalculate={canManageAnalysis} isRecalculating={recalculate.isPending} recalculationError={recalculate.error?.message} onRecalculate={() => recalculate.mutate()} /><section className="flex flex-col"><div className="mb-3 flex items-start justify-between gap-3"><div><h2 className="section-title">{t('Location context')}</h2>{site.data?.latitude !== undefined && site.data?.longitude !== undefined ? <p className="mt-1 text-xs leading-5 text-muted">{t('Customer site pin:')} {site.data.latitude.toFixed(6)}, {site.data.longitude.toFixed(6)} · <a className="font-semibold text-brand hover:underline print-hide" href={`https://www.google.com/maps?q=${site.data.latitude},${site.data.longitude}`} target="_blank" rel="noreferrer">{t('Open in Google Maps')}</a></p> : null}</div><RadiusLabel meters={run.analysisRadiusMeters} /></div>{site.data?.latitude !== undefined && site.data?.longitude !== undefined ? <div className="print-site-location"><PrintableLocationMap latitude={site.data.latitude} longitude={site.data.longitude} /></div> : null}<div className="screen-map h-[350px] sm:h-[420px]"><MapPanel latitude={site.data?.latitude} longitude={site.data?.longitude} radiusMeters={run.analysisRadiusMeters} className="h-full min-h-0" /></div></section></div>
	    <InvestmentRecommendation run={run} site={site.data} assessment={displayedAssessment} isGenerating={aiNotes.isPending} />
    {canManageAnalysis ? <StationRecommendation key={run.id} id={run.id} site={site.data} initialProposal={run.stationRecommendation} /> : null}
	 <MetricResultsTable metrics={run.metrics} renderMetric={metric => <MetricRow key={metric.id} metric={metric} site={site.data} canUpdateInternet={canUpdateInternet} onInternetChange={updateInternet.mutate} savingInternet={updateInternet.isPending} />} saveError={updateInternet.isError} />
    <ProjectRecommendation plans={plans.data} isLoading={plans.isLoading} site={site.data} />
    <AnalysisNotes assessment={displayedAssessment} isGenerating={aiNotes.isPending} error={displayedAssessmentError} canRegenerate={canManageAnalysis} onRegenerate={() => aiNotes.mutate({ refresh: true, languages: [language] })} fallback={run.recommendation} />
  </div>
}

function ScoreCard({ run, canRecalculate, isRecalculating, recalculationError, onRecalculate }: { run: import('../types/domain').AnalysisRun; canRecalculate: boolean; isRecalculating: boolean; recalculationError?: string; onRecalculate: () => void }) {
  const { t } = useI18n()
  const hasScore = finiteNumber(run.overallScore)
  const grade = getScoreGrade(hasScore ? run.overallScore : undefined)
	return <section className="rounded-2xl border border-line bg-white p-6 shadow-panel"><div className="flex items-center justify-between gap-3"><div><p className="text-xs font-bold uppercase tracking-[0.14em] text-brand">{t('Screening result')}</p><h2 className="mt-1 section-title">{t(hasScore ? 'Preliminary assessment' : 'Assessment unavailable')}</h2></div><DataStatus status={run.assessmentStatus} compact /></div><div className="mt-6 text-center"><div className="flex items-end justify-center gap-2"><span className="text-6xl font-extrabold tracking-tight text-ink">{run.overallScore?.toFixed(1) ?? '—'}</span>{hasScore ? <span className="pb-2 text-lg font-semibold text-muted">/100</span> : null}</div><p className="mt-2 font-semibold">{t('Overall location screening score')}</p>{grade ? <p className="mt-2 text-lg font-extrabold text-brand">{t('Grade')} {grade}</p> : null}</div>{canRecalculate && !run.scoring ? <button type="button" className="button-secondary mt-4 w-full" onClick={onRecalculate} disabled={isRecalculating}><Calculator size={16} />{t(isRecalculating ? 'Calculating preliminary score…' : 'Calculate preliminary score')}</button> : null}{recalculationError ? <p role="alert" className="mt-3 text-xs text-red-700">{t(errorMessageKey(recalculationError))}</p> : null}<div className="mt-5 border-t border-line pt-5"><DataStatusLegend /></div></section>
}

type InvestmentDecision = 'invest' | 'not_recommended'

function InvestmentRecommendation({ run, site, assessment, isGenerating }: { run: import('../types/domain').AnalysisRun; site?: Site; assessment?: AIAssessment; isGenerating: boolean }) {
  const { t, language } = useI18n()
  const score = finiteNumber(run.overallScore) ? run.overallScore : undefined
  const electrical = run.metrics.find(metric => metric.type === 'electrical')
  const weakMetrics = run.metrics.filter(metric => finiteNumber(metric.normalizedScore) && metric.normalizedScore < 50)
  const strongMetrics = run.metrics.filter(metric => finiteNumber(metric.normalizedScore) && metric.normalizedScore >= 70)
  const electricalNeedsCheck = !electrical || electrical.status === 'missing'
	const investmentBlockers = currentInvestmentBlockers(site, score, language)
	const decision: InvestmentDecision = investmentBlockers.length === 0 ? 'invest' : 'not_recommended'
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
	return <section className="mt-5 rounded-2xl border border-line bg-white p-5 shadow-panel sm:p-6"><div className="flex items-start gap-3"><span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-emerald-50 text-brand"><ClipboardList size={21} /></span><div><h2 className="section-title">{t('Investment recommendation summary')}</h2><p className="mt-1 text-sm text-muted">{t('The system determines the status from the screening score; AI explains the evidence.')} {language === 'th' ? 'ระบบไฟฟ้าใต้ดินตามนโยบายปัจจุบันอาจหยุดการลงทุนได้' : 'Underground electricity may also stop investment under the current policy.'}</p></div></div>{isGenerating ? <p className="mt-4 text-sm text-muted">{t('AI is analysing the recommendation…')}</p> : null}<div className="mt-5 grid gap-3 md:grid-cols-2">{(Object.keys(labels) as InvestmentDecision[]).map(option => { const selected = decision === option; const styles = optionStyles[option]; const description = descriptions[option]; return <div key={option} className={`rounded-xl border p-4 ${selected ? styles.selected : 'border-line bg-white opacity-60'}`}><div className="flex items-center gap-2"><span className={selected ? styles.icon : 'text-slate-400'}>{selected ? <CheckCircle2 size={20} /> : <Circle size={20} />}</span><p className="font-bold text-ink">{labels[option]}</p></div>{selected && description ? <p className="mt-2 text-sm leading-5 text-muted">{description}</p> : null}</div> })}</div>{investmentBlockers.length ? <div className="mt-5 rounded-xl border border-rose-200 bg-rose-50 p-4"><p className="text-sm font-bold text-rose-950">{language === 'th' ? 'เหตุผลที่ยังไม่ลงทุนในปัจจุบัน' : 'Reasons the site is not being invested in currently'}</p><ul className="mt-3 space-y-2 text-sm leading-6 text-rose-900">{investmentBlockers.map(reason => <li key={reason} className="flex gap-2"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-rose-600" />{reason}</li>)}</ul></div> : null}<div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4"><p className="text-sm font-bold text-ink">{t('Reasons from AI and system data')}</p><ul className="mt-3 space-y-2 text-sm leading-6 text-muted">{reasons.map(reason => <li key={reason} className="flex gap-2"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-brand" />{reason}</li>)}</ul></div></section>
}

function currentInvestmentBlockers(site: Site | undefined, score: number | undefined, language: 'th' | 'en'): string[] {
  const blockers: string[] = []
  if (score === undefined) blockers.push(language === 'th' ? 'ยังไม่มีคะแนนคัดกรองที่ใช้สรุปผลได้' : 'No screening score is available for a decision.')
  else if (score < 60) blockers.push(language === 'th' ? `คะแนนคัดกรอง ${score.toFixed(1)}/100 ต่ำกว่าเกณฑ์ส่งต่อเพื่อพิจารณาลงทุน 60/100` : `The ${score.toFixed(1)}/100 screening score is below the 60/100 investment-review threshold.`)
  if (site?.electricalSupplyType === 'underground') blockers.push(language === 'th' ? 'จุดเชื่อมต่อระบุว่าเป็นสายไฟฟ้าใต้ดิน ซึ่งนโยบายปัจจุบันยังไม่ลงทุนเพราะต้นทุนงานโยธาและเชื่อมต่อสูง' : 'The connection point is recorded as underground electricity; the current policy does not invest because civil-work and connection costs are high.')
  return blockers
}

function AnalysisNotes({ assessment, isGenerating, error, canRegenerate, onRegenerate, fallback }: { assessment?: AIAssessment; isGenerating: boolean; error: unknown; canRegenerate: boolean; onRegenerate: () => void; fallback: string }) {
  const { t } = useI18n()
	return <section className="mt-6 rounded-xl border border-line bg-white p-6"><div className="flex flex-wrap items-start justify-between gap-4"><div><h2 className="section-title">{t('Notes')}</h2><p className="mt-1 text-sm text-muted">{t('Analysis based on the evidence in this analysis run.')}</p></div>{canRegenerate ? <button type="button" className="button-secondary min-h-10" onClick={onRegenerate} disabled={isGenerating}><RefreshCw size={16} className={isGenerating ? 'animate-spin' : undefined} />{isGenerating ? t('AI is analysing the recommendation…') : t('Regenerate AI analysis')}</button> : null}</div>{isGenerating ? <div className="mt-5 rounded-xl border border-emerald-100 bg-emerald-50/60 p-4 text-sm text-muted">{t('AI is summarising site potential, electrical readiness, and required checks…')}</div> : assessment ? <div className="mt-5 space-y-4"><div className="rounded-xl border border-emerald-100 bg-emerald-50/60 p-4"><p className="text-sm font-bold text-ink">{t('Overview')}</p><p className="mt-2 text-sm leading-6 text-muted">{assessment.summary}</p></div><div className="grid gap-4 lg:grid-cols-3"><NoteList title={t('Location strengths')} values={assessment.strengths} tone="emerald" /><NoteList title={t('Risk factors')} values={assessment.risks} tone="rose" /><NoteList title={t('Further checks required')} values={assessment.requiredChecks} tone="amber" /></div></div> : <div className="mt-5 rounded-xl border border-amber-100 bg-amber-50/60 p-4"><p className="text-sm text-amber-800">{error ? t(errorMessageKey(error)) : fallback}</p>{error && canRegenerate ? <p className="mt-2 text-xs leading-5 text-muted">{t('Try “Regenerate AI analysis” again.')}</p> : null}</div>}</section>
}

function NoteList({ title, values, tone }: { title: string; values: string[]; tone: 'emerald' | 'rose' | 'amber' }) {
  const colors = tone === 'emerald' ? 'border-emerald-100 bg-emerald-50/60 marker:text-emerald-600' : tone === 'rose' ? 'border-rose-100 bg-rose-50/60 marker:text-rose-600' : 'border-amber-100 bg-amber-50/60 marker:text-amber-600'
  return <section className={`rounded-xl border p-4 ${colors}`}><h3 className="text-sm font-bold text-ink">{title}</h3><ul className="mt-3 list-disc space-y-2 pl-5 text-sm leading-6 text-muted">{values.length ? values.map(item => <li key={item}>{item}</li>) : <li>—</li>}</ul></section>
}

function MetricRow({ metric, site, canUpdateInternet, onInternetChange, savingInternet }: { metric: Metric; site?: Site; canUpdateInternet: boolean; onInternetChange: (value: boolean | undefined) => void; savingInternet: boolean }) {
  const { t } = useI18n(); const result = formatMetricResult(metric, t)
	if (metric.type === 'site_requirements') return <MetricRowView metric={metric} label={t('Internet signal access')} result={result} scoreLabel="" internetControl={<InternetAccessChecklist available={site?.internetAvailable} canEdit={canUpdateInternet} onChange={onInternetChange} saving={savingInternet} />} statusOverride={site?.internetAvailable === undefined ? 'missing' : 'preliminary'} />
  const aiAssisted = metric.assumptions.some(item => item.startsWith('Gemini assisted scoring:'))
  const poiObservation = metric.type === 'poi' ? getRawObservation(metric.rawValue) : undefined
  const siteSurface = metric.type === 'site_readiness' ? getSiteSurface(metric.rawValue) : undefined
  const unavailableReason = !result.available ? trafficUnavailableReason(metric) || metric.assumptions[0] : undefined
  if (metric.type === 'flood') result.note = [result.note, floodAccessWarning(metric.rawValue, t)].filter(Boolean).join(' · ')
  return <MetricRowView metric={metric} label={metricLabel(metric, t)} result={result} scoreLabel={t(aiAssisted ? 'AI-assisted metric score' : 'Preliminary metric score')} unavailableReason={unavailableReason ? t(unavailableReason) : undefined} detail={<>{siteSurface ? <SiteSurfaceBreakdown assessment={siteSurface} /> : null}{poiObservation ? <PoiCategoryBreakdown observation={poiObservation} /> : null}</>} />
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

function FranchisePlanDetails({ code, language }: { code: 'S' | 'M' | 'L'; language: 'th' | 'en' }) {
  const profile = franchisePlanProfiles[code]
  const isThai = language === 'th'
  const rows = [
    [isThai ? 'ผังโดยประมาณ' : 'Indicative layout', profile.dimensions],
    [isThai ? 'ทำเลที่เหมาะ' : 'Best fit', isThai ? profile.idealSites : profile.idealSitesEn],
    [isThai ? 'บริการหลัก' : 'Core services', isThai ? profile.services : profile.servicesEn],
    [isThai ? 'งบลงทุนโดยประมาณ' : 'Estimated investment', isThai ? profile.investment : profile.investmentEn],
    [isThai ? 'ค่าแฟรนไชส์เริ่มต้น' : 'Starting franchise fee', isThai ? profile.franchiseFee : profile.franchiseFeeEn],
  ]
  return <dl className="mt-4 space-y-2.5 border-t border-line pt-4 text-sm">{rows.map(([label, value]) => <div key={label} className="grid gap-0.5"><dt className="text-xs font-medium text-muted">{label}</dt><dd className="font-semibold leading-5 text-ink">{value}</dd></div>)}</dl>
}

function ProjectRecommendation({ plans, isLoading, site }: { plans?: FranchisePlan[]; isLoading: boolean; site?: Site }) {
	const { t, language } = useI18n()
  const [previewPlan, setPreviewPlan] = useState<'S' | 'M' | 'L' | null>(null)
  const { area, frontage, minimumFrontageMeters, widthMissing, widthInsufficient, selected, canRecommend } = selectProjectFormat(plans, site)
	const isPreliminary = false
	const reason = !selected ? (language === 'th' ? `พื้นที่ ${area?.toFixed(1) ?? '—'} ตร.วา ยังต่ำกว่าขนาดขั้นต่ำของแพ็กเกจ S (${plans?.find(plan => plan.code === 'S')?.minimumAreaSqWah ?? 50} ตร.วา)` : `The ${area?.toFixed(1) ?? '—'} sq wah land area is below the minimum for package S (${plans?.find(plan => plan.code === 'S')?.minimumAreaSqWah ?? 50} sq wah).`) : widthMissing ? (language === 'th' ? `พื้นที่มีขนาดเพียงพอสำหรับแพ็กเกจ ${selected.code}; แนะนำให้วัดหน้ากว้างทางเข้า–ออกก่อนออกแบบจริง` : `The land area is sufficient for package ${selected.code}; measure entrance frontage before the final design.`) : widthInsufficient ? (language === 'th' ? `พื้นที่มีขนาดเพียงพอสำหรับแพ็กเกจ ${selected.code}; แนะนำให้จัดทางเข้า–ออกกว้างอย่างน้อย ${minimumFrontageMeters} ม. เพื่อการใช้งานที่สะดวก` : `The land area is sufficient for package ${selected.code}; an entrance/exit at least ${minimumFrontageMeters} m wide is recommended for convenient use.`) : (language === 'th' ? `พื้นที่มีขนาดเพียงพอสำหรับรูปแบบ ${selected.code}` : `The land area is sufficient for format ${selected.code}.`)
 return <section className="mt-6 rounded-2xl border border-line bg-white p-6 shadow-panel"><div className="flex items-start gap-3"><span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-emerald-50 text-brand"><Calculator size={20} /></span><div><h2 className="section-title">{language === 'th' ? 'รูปแบบโครงการตามขนาดพื้นที่' : 'Project format by land size'}</h2><p className="mt-1 text-sm text-muted">{language === 'th' ? 'ใช้ขนาดที่ดินเพื่อเลือก S / M / L; หน้ากว้างทางเข้าเป็นคำแนะนำสำหรับการวางผัง' : 'Uses land size to select S / M / L; entrance frontage is a layout recommendation.'}</p></div></div>{isLoading ? <p className="mt-5 text-sm text-muted">{t('Loading project formats…')}</p> : <div className="mt-5 grid gap-4 lg:grid-cols-3">{plans?.map(plan => { const isSelected = canRecommend && plan.code === selected.code; const isIndividualPoster = true; return <article key={plan.code} className={`overflow-hidden rounded-2xl border ${isSelected ? 'border-emerald-400 bg-emerald-50/70 shadow-sm' : 'border-line bg-white'}`}><button type="button" onClick={() => setPreviewPlan(plan.code)} className="group relative block h-32 w-full overflow-hidden bg-slate-900 text-left" aria-label={language === 'th' ? `ดูรูปแบบโครงการ ${plan.code}` : `View ${plan.code} project format`}><span className="block h-full w-full bg-cover bg-no-repeat transition duration-300 group-hover:scale-[1.03]" style={{ backgroundImage: `url(${planPosterUrl(plan.code)})`, backgroundSize: isIndividualPoster ? 'cover' : '300% auto', backgroundPosition: isIndividualPoster ? '50% 50%' : '100% 35%' }}/><span className="absolute inset-x-0 bottom-0 flex items-center justify-between bg-gradient-to-t from-black/75 to-transparent px-4 pb-3 pt-8 text-xs font-bold text-white"><span>{language === 'th' ? 'กดดูผังและรายละเอียด' : 'View layout and details'}</span><Expand size={16}/></span></button><div className={`p-5 ${isSelected ? '' : 'opacity-75'}`}><div className="flex items-center justify-between"><span className="text-4xl font-extrabold text-brand">{plan.code}</span>{isSelected ? <span className="rounded-full bg-emerald-600 px-2.5 py-1 text-xs font-bold text-white">{language === 'th' ? (isPreliminary ? 'แนะนำเบื้องต้น' : 'รูปแบบที่เหมาะสม') : (isPreliminary ? 'Preliminary fit' : 'Recommended format')}</span> : null}</div><h3 className="mt-3 font-bold text-ink">{plan.name}</h3><dl className="mt-4 space-y-2 text-sm"><div className="flex justify-between gap-3"><dt className="text-muted">{t('Land size')}</dt><dd className="font-bold">{formatPlanArea(plan, language, t)}</dd></div><div className="flex justify-between gap-3"><dt className="text-muted">{language === 'th' ? 'ตู้ตามรูปแบบแฟรนไชส์' : 'Cabinets in franchise format'}</dt><dd className="font-bold">{plan.evChargingStations}</dd></div></dl></div></article> })}</div>}<div className={`mt-5 rounded-xl border p-4 ${canRecommend ? 'border-emerald-200 bg-emerald-50/60' : 'border-amber-200 bg-amber-50/70'}`}><p className="font-bold text-ink">{canRecommend ? `${language === 'th' ? (isPreliminary ? 'รูปแบบเบื้องต้น' : 'รูปแบบที่เหมาะสม') : (isPreliminary ? 'Preliminary format' : 'Recommended format')}: ${t('Plan')} ${selected.code}` : (language === 'th' ? 'ยังเลือกรูปแบบโครงการไม่ได้' : 'No project format selected yet')}</p><p className="mt-2 text-sm leading-6 text-muted">{reason}</p><p className="mt-3 text-sm font-medium text-ink">{language === 'th' ? 'จำนวนตู้ในรูปแบบแฟรนไชส์เป็นเป้าหมายการวางผัง ไม่ใช่จำนวนที่การไฟฟ้าอนุมัติ' : 'Cabinet quantity in a franchise format is a layout target, not utility-approved capacity.'}</p><ul className="mt-3 space-y-1.5 text-sm text-muted"><li>• {t('Land area used for calculation:')} {area !== undefined ? `${area.toFixed(1)} ${t('sq wah')}` : t('No data')}</li><li>• {t('Entrance frontage:')} {frontage !== undefined ? `${frontage.toFixed(1)} ${t('m')}` : t('No data')}</li><li>• {language === 'th' ? `คำแนะนำการวางผัง: ทางเข้า–ออกอย่างน้อย ${minimumFrontageMeters} ม.` : `Layout recommendation: entrance/exit at least ${minimumFrontageMeters} m.`}</li></ul></div>{previewPlan ? <PlanPreviewModal code={previewPlan} language={language} onClose={() => setPreviewPlan(null)} /> : null}</section>
}

function PlanPreviewModal({ code, language, onClose }: { code: 'S' | 'M' | 'L'; language: 'th' | 'en'; onClose: () => void }) {
  return <div className="fixed inset-0 z-50 grid place-items-center bg-slate-950/80 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label={language === 'th' ? `รายละเอียดรูปแบบ ${code}` : `${code} format details`} onMouseDown={onClose}>
    <div className="relative max-h-[calc(100vh-2rem)] w-full max-w-3xl overflow-y-auto rounded-2xl bg-white shadow-2xl" onMouseDown={event => event.stopPropagation()}>
      <div className="sticky top-0 z-10 flex items-center justify-between border-b border-line bg-white px-4 py-3 sm:px-5"><div><p className="text-lg font-extrabold text-ink">{code} — {code === 'S' ? 'Smart Café' : code === 'M' ? 'Lifestyle Café' : 'Lifestyle Hub'}</p><p className="mt-0.5 text-xs text-muted">{language === 'th' ? 'ผังตัวอย่าง บริการ และเงื่อนไขเบื้องต้นของแฟรนไชส์' : 'Illustrative layout, services, and preliminary franchise terms.'}</p></div><button type="button" onClick={onClose} className="grid h-10 w-10 place-items-center rounded-lg text-slate-600 hover:bg-slate-100" aria-label={language === 'th' ? 'ปิด' : 'Close'}><X size={20}/></button></div>
      <div className="flex h-[58vh] min-h-[360px] items-center justify-center bg-slate-950 p-3"><img src={planPosterUrl(code)} alt={language === 'th' ? `ผังรูปแบบ ${code}` : `${code} franchise layout`} className="h-full max-w-full object-contain" /></div>
      <div className="p-5"><h3 className="text-base font-extrabold text-ink">{language === 'th' ? 'รายละเอียดแพ็กเกจ' : 'Package details'}</h3><FranchisePlanDetails code={code} language={language} /><p className="mt-4 rounded-xl bg-slate-50 p-3 text-xs leading-5 text-muted">{language === 'th' ? 'ข้อมูลลงทุนและจำนวนสถานีเป็นข้อมูลเบื้องต้นสำหรับการวางแผน ทีมวิศวกรจะยืนยันผังจริง ระบบไฟฟ้า และงบประมาณหลังสำรวจพื้นที่' : 'Investment and station quantities are preliminary planning information. Engineering confirms the final layout, electrical capacity, and budget after site inspection.'}</p></div>
    </div>
  </div>
}

function PoiCategoryBreakdown({ observation }: { observation: RawObservation }) {
  const { t } = useI18n(); const groups = summarizePOICategoryGroups(observation.categoryCounts)
	if (!groups.length) return <p className="mt-2 text-xs text-amber-700">{t('No map records were found in this run. This does not mean there are no important places, so the zero result is excluded from scoring.')}</p>
  return <details className="mt-3 rounded-xl border border-emerald-100 bg-emerald-50/50 p-3"><summary className="cursor-pointer text-xs font-bold text-brand">{t('Category breakdown')}</summary><p className="mt-2 text-xs leading-5 text-muted">{t('The categories below total the observed places. EV charging stations are excluded and counted under competitors.')}</p><div className="mt-3 grid gap-2 sm:grid-cols-2">{groups.map(([category, count]) => <div key={category} className="flex items-center justify-between rounded-lg bg-white px-3 py-2 text-xs shadow-sm"><span className="font-medium text-ink">{t(category)}</span><span className="font-bold text-brand">{count.toLocaleString()}</span></div>)}</div></details>
}

function SiteSurfaceBreakdown({ assessment }: { assessment: SiteSurfaceValue }) {
  const { t } = useI18n()
  const entrance = assessment.entranceWidthEstimate
  return <details className="mt-3 rounded-xl border border-amber-100 bg-amber-50/60 p-3"><summary className="cursor-pointer text-xs font-bold text-amber-800">{t('Ground-surface details')}</summary><p className="mt-2 text-xs leading-5 text-muted">{assessment.summary}</p>{entrance ? <div className="mt-3 rounded-lg border border-blue-100 bg-white p-3"><p className="text-xs font-bold text-ink">{t('Approximate entrance width')}: {entrance.minimumMeters.toLocaleString(undefined, { maximumFractionDigits: 1 })}–{entrance.maximumMeters.toLocaleString(undefined, { maximumFractionDigits: 1 })} {t('m.')}</p><p className="mt-1 text-xs text-muted">{t('Confidence')}: {t(entrance.confidence)} · {entrance.visualEvidence}</p>{entrance.obstructions.length ? <p className="mt-1 text-xs text-muted">{t('Visible obstructions')}: {entrance.obstructions.join(', ')}</p> : null}<p className="mt-2 text-xs leading-5 text-amber-800">{t('Image estimate only — measure on site before engineering review.')}</p></div> : null}<div className="mt-3 grid gap-3 sm:grid-cols-3"><SurfaceList title="Surface types" values={assessment.surfaceTypes} /><SurfaceList title="Observed risks" values={assessment.observedRisks} /><SurfaceList title="Recommended improvements" values={assessment.recommendedImprovements} /></div><p className="mt-3 text-xs leading-5 text-muted">{assessment.disclaimer}</p></details>
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
type RoadValue = { radiusMeters: number; mappedMajorRoadCount: number; nearestMajorRoadMeters?: number; nearestRoadName?: string; nearestRoadRef?: string; nearestRoadClass?: string; roadClassCounts: Record<string, number> }
type AADTValue = { aadt: number; dataYear: number; roadAuthority?: 'DOH' | 'DRR'; roadCode: string; roadName?: string; controlSection: string; controlSectionName: string; surveyKm: string; distanceToSectionMeters?: number; distanceToRoadMeters?: number; matchRadiusMeters: number }
type TrafficPotentialValue = { assessmentType: 'osm_road_traffic_potential'; potentialScore: number; radiusMeters: number; mappedMajorRoadCount: number; nearestMajorRoadMeters?: number; nearestRoadName?: string; nearestRoadRef?: string; nearestRoadClass?: string; roadClassCounts: Record<string, number> }
type EVRegistrationValue = { province: string; registeredBev: number; datasetDate: string; previousRegisteredBev?: number; previousDatasetDate?: string; changePercent?: number; trend?: 'increased' | 'decreased' | 'unchanged'; provinceResolution: string }
type ElectricalPreliminaryValue = { assessmentType: string; matchingMethod?: 'point_in_published_area' | 'nearest_published_area'; distanceToAreaMeters?: number; mapYear?: number; voltageKv?: number; stationCode?: string; stationName?: string; publishedCapacityMw?: number; mapUrl?: string; serviceArea?: string; dataAvailable?: boolean }
type ElectricalGridValue = { assessmentType: 'pea_public_grid_evidence'; searchRadiusMeters: number; highVoltageLineCount: number; nearestHighVoltageLineMeters?: number; voltageCode?: string; conductorSize?: string; phaseCode?: string; feederId?: string; nearestStationMeters?: number; stationName?: string; stationSecondaryVoltageKv?: number }
type TerrainSamplingValue = { assessmentType: 'gistda_elevation_sampling'; sampleSpanMeters: number; sampleCount: number; minimumElevationMeters: number; maximumElevationMeters: number; elevationRangeMeters: number }
type SiteRequirementsValue = { assessmentType: 'automatic_site_requirements'; nearestPublishedElectricalKm?: number; terrain?: TerrainSamplingValue }
type DistrictFloodHistoryValue = { assessmentType: 'administrative_district_historical_reports'; province: string; district: string; periodStartYear: number; periodEndYear: number; reportedFloodYears: number[]; reportedFloodYearCount: number; reportedVillageIncidents: number; affectedSubdistrictCount: number }
type FloodLayerValue = { mappedFloodRiskAreaCount: number; radiusMeters: number; layerName: string }
type FloodValue = DistrictFloodHistoryValue | FloodLayerValue
type CompetitionSource = { name: string; referenceUri: string; coverage: string; count: number; retrievedAt: string }
type CompetitionPlace = { recordType: string; recordId: string; name?: string; operator?: string; sourceNames: string[] }
type CompetitionObservation = RawCount & { coverageMatched: boolean; places: CompetitionPlace[]; sources: CompetitionSource[] }
type EntranceWidthEstimate = { minimumMeters: number; maximumMeters: number; confidence: 'low' | 'moderate' | 'high'; visualEvidence: string; obstructions: string[] }
type SiteSurfaceValue = { imageCount: number; summary: string; suitability: 'low' | 'moderate' | 'high'; score: number; surfaceTypes: string[]; observedRisks: string[]; recommendedImprovements: string[]; disclaimer: string; entranceWidthEstimate?: EntranceWidthEstimate }
function getRecord(value: unknown): Record<string, unknown> | undefined { return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : undefined }
function finiteNumber(value: unknown): value is number { return typeof value === 'number' && Number.isFinite(value) }
function trafficUnavailableReason(metric: Metric): string | undefined {
  if (metric.type !== 'traffic') return undefined
  const raw = getRecord(metric.rawValue)
  const checks = Array.isArray(raw?.sourceChecks) ? raw.sourceChecks.map(getRecord).filter((item): item is Record<string, unknown> => !!item && typeof item.outcome === 'string') : []
  if (checks.some(check => check.outcome === 'unavailable')) return 'Official traffic data could not be retrieved. Please analyze again later.'
  if (checks.length && checks.every(check => check.outcome === 'no_match' || check.outcome === 'available')) return 'No official AADT road match was found within the selected radius. The system does not estimate a vehicle count.'
  return undefined
}
function getRawCount(value: unknown): RawCount | undefined { const item = getRecord(value); return item && finiteNumber(item.count) && item.count >= 0 && finiteNumber(item.radiusMeters) && item.radiusMeters > 0 ? { count: item.count, radiusMeters: item.radiusMeters } : undefined }
function getPopulation(value: unknown): PopulationValue | undefined { const item = getRecord(value); return item && finiteNumber(item.population) && finiteNumber(item.populationDensityPerKm2) && finiteNumber(item.areaKm2) && finiteNumber(item.radiusMeters) && finiteNumber(item.dataYear) && typeof item.resolution === 'string' ? item as unknown as PopulationValue : undefined }
function getRoad(value: unknown): RoadValue | undefined { const item = getRecord(value); return item && finiteNumber(item.mappedMajorRoadCount) && finiteNumber(item.radiusMeters) && getRecord(item.roadClassCounts) ? item as unknown as RoadValue : undefined }
function getAADT(value: unknown): AADTValue | undefined { const item = getRecord(value); return item && finiteNumber(item.aadt) && item.aadt >= 0 && finiteNumber(item.dataYear) && (item.roadAuthority === undefined || item.roadAuthority === 'DOH' || item.roadAuthority === 'DRR') && typeof item.roadCode === 'string' && typeof item.controlSection === 'string' && typeof item.controlSectionName === 'string' && typeof item.surveyKm === 'string' && (finiteNumber(item.distanceToSectionMeters) || finiteNumber(item.distanceToRoadMeters)) && finiteNumber(item.matchRadiusMeters) ? item as unknown as AADTValue : undefined }
function getTrafficPotential(value: unknown): TrafficPotentialValue | undefined { const item = getRecord(value); return item && item.assessmentType === 'osm_road_traffic_potential' && finiteNumber(item.potentialScore) && finiteNumber(item.radiusMeters) && finiteNumber(item.mappedMajorRoadCount) && getRecord(item.roadClassCounts) ? item as unknown as TrafficPotentialValue : undefined }
function nearestRoadLabel(value: Pick<TrafficPotentialValue, 'nearestRoadName' | 'nearestRoadRef' | 'nearestRoadClass'>, t: (key: string) => string): string | undefined {
  if (value.nearestRoadName) return value.nearestRoadRef ? `${value.nearestRoadName} (${value.nearestRoadRef})` : value.nearestRoadName
  if (value.nearestRoadRef) return value.nearestRoadRef
  return value.nearestRoadClass ? `${t('Mapped road class')} ${value.nearestRoadClass}` : undefined
}
function getEVRegistration(value: unknown): EVRegistrationValue | undefined { const item = getRecord(value); return item && typeof item.province === 'string' && finiteNumber(item.registeredBev) && item.registeredBev >= 0 && typeof item.datasetDate === 'string' ? item as unknown as EVRegistrationValue : undefined }
function getElectricalPreliminary(value: unknown): ElectricalPreliminaryValue | undefined { const item = getRecord(value); return item && (item.assessmentType === 'published_station_area_guideline' || item.assessmentType === 'official_public_map_guideline') ? item as unknown as ElectricalPreliminaryValue : undefined }
function getElectricalGrid(value: unknown): ElectricalGridValue | undefined { const item = getRecord(value); return item && item.assessmentType === 'pea_public_grid_evidence' && finiteNumber(item.searchRadiusMeters) && finiteNumber(item.highVoltageLineCount) ? item as unknown as ElectricalGridValue : undefined }
function getSiteRequirements(value: unknown): SiteRequirementsValue | undefined { const item = getRecord(value); if (!item || item.assessmentType !== 'automatic_site_requirements') return undefined; const terrain = getRecord(item.terrain); const validTerrain = terrain && terrain.assessmentType === 'gistda_elevation_sampling' && finiteNumber(terrain.sampleSpanMeters) && finiteNumber(terrain.sampleCount) && finiteNumber(terrain.minimumElevationMeters) && finiteNumber(terrain.maximumElevationMeters) && finiteNumber(terrain.elevationRangeMeters) ? terrain as unknown as TerrainSamplingValue : undefined; return { assessmentType: 'automatic_site_requirements', ...(finiteNumber(item.nearestPublishedElectricalKm) ? { nearestPublishedElectricalKm: item.nearestPublishedElectricalKm } : {}), ...(validTerrain ? { terrain: validTerrain } : {}) } }
function getFlood(value: unknown): FloodValue | undefined { const item = getRecord(value); if (!item) return undefined; if (item.assessmentType === 'administrative_district_historical_reports' && typeof item.province === 'string' && typeof item.district === 'string' && finiteNumber(item.periodStartYear) && finiteNumber(item.periodEndYear) && Array.isArray(item.reportedFloodYears) && item.reportedFloodYears.every(finiteNumber) && finiteNumber(item.reportedFloodYearCount) && finiteNumber(item.reportedVillageIncidents) && finiteNumber(item.affectedSubdistrictCount)) return item as unknown as DistrictFloodHistoryValue; return finiteNumber(item.mappedFloodRiskAreaCount) && item.mappedFloodRiskAreaCount >= 0 && finiteNumber(item.radiusMeters) && item.radiusMeters > 0 && typeof item.layerName === 'string' ? item as unknown as FloodLayerValue : undefined }
function getSiteSurface(value: unknown): SiteSurfaceValue | undefined { const item = getRecord(value); if (!item || (item.analysisScope !== 'ground_surface_only' && item.analysisScope !== 'visible_site_conditions') || !finiteNumber(item.imageCount) || !finiteNumber(item.score) || typeof item.summary !== 'string' || typeof item.suitability !== 'string' || typeof item.disclaimer !== 'string') return undefined; const strings = (key: string) => Array.isArray(item[key]) ? item[key].filter((entry): entry is string => typeof entry === 'string') : []; const estimate = getRecord(item.entranceWidthEstimate); const entranceWidthEstimate = estimate && finiteNumber(estimate.minimumMeters) && finiteNumber(estimate.maximumMeters) && estimate.minimumMeters > 0 && estimate.maximumMeters >= estimate.minimumMeters && typeof estimate.confidence === 'string' && (estimate.confidence === 'low' || estimate.confidence === 'moderate' || estimate.confidence === 'high') && typeof estimate.visualEvidence === 'string' ? { minimumMeters: estimate.minimumMeters, maximumMeters: estimate.maximumMeters, confidence: estimate.confidence as EntranceWidthEstimate['confidence'], visualEvidence: estimate.visualEvidence, obstructions: Array.isArray(estimate.obstructions) ? estimate.obstructions.filter((entry): entry is string => typeof entry === 'string') : [] } : undefined; return { imageCount: item.imageCount, summary: item.summary, suitability: item.suitability as SiteSurfaceValue['suitability'], score: item.score, surfaceTypes: strings('surfaceTypes'), observedRisks: strings('observedRisks'), recommendedImprovements: strings('recommendedImprovements'), disclaimer: item.disclaimer, ...(entranceWidthEstimate ? { entranceWidthEstimate } : {}) } }
function formatKilometers(meters: number): string { return (meters / 1000).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
function getRawObservation(value: unknown): RawObservation | undefined { const count = getRawCount(value); const candidate = getRecord(value); if (!count || !candidate) return undefined; const places: RawPlace[] = []; if (Array.isArray(candidate.places)) for (const place of candidate.places) { const item = getRecord(place); if (!item || typeof item.osmType !== 'string' || !finiteNumber(item.osmId) || typeof item.category !== 'string') continue; places.push({ osmType: item.osmType, osmId: item.osmId, category: item.category, ...(typeof item.name === 'string' && item.name ? { name: item.name } : {}) }) } const categoryCounts: Record<string, number> = {}; const rawCategoryCounts = getRecord(candidate.categoryCounts); if (rawCategoryCounts) for (const [category, categoryCount] of Object.entries(rawCategoryCounts)) if (finiteNumber(categoryCount) && categoryCount >= 0) categoryCounts[category] = categoryCount; return { ...count, places, ...(Object.keys(categoryCounts).length ? { categoryCounts } : {}), ...(finiteNumber(candidate.excludedChargingStationCount) ? { excludedChargingStationCount: candidate.excludedChargingStationCount } : {}), ...(finiteNumber(candidate.residentialBuildingCount) ? { residentialBuildingCount: candidate.residentialBuildingCount } : {}), ...(finiteNumber(candidate.communityAreaCount) ? { communityAreaCount: candidate.communityAreaCount } : {}) } }
function getCompetitionObservation(value: unknown): CompetitionObservation | undefined { const count = getRawCount(value); const candidate = getRecord(value); if (!count || !candidate || !Array.isArray(candidate.sources) || !Array.isArray(candidate.places)) return undefined; const sources: CompetitionSource[] = []; for (const source of candidate.sources) { const item = getRecord(source); if (!item || typeof item.name !== 'string' || typeof item.referenceUri !== 'string' || typeof item.coverage !== 'string' || !finiteNumber(item.count) || typeof item.retrievedAt !== 'string') continue; sources.push(item as unknown as CompetitionSource) } const places: CompetitionPlace[] = []; for (const place of candidate.places) { const item = getRecord(place); if (!item || typeof item.recordType !== 'string' || typeof item.recordId !== 'string' || !Array.isArray(item.sourceNames)) continue; places.push({ recordType: item.recordType, recordId: item.recordId, sourceNames: item.sourceNames.filter((source): source is string => typeof source === 'string'), ...(typeof item.name === 'string' ? { name: item.name } : {}), ...(typeof item.operator === 'string' ? { operator: item.operator } : {}) }) } return { ...count, coverageMatched: candidate.coverageMatched === true, sources, places } }

function formatMetricResult(metric: Metric, t: (key: string) => string) {
  const localTraffic = getRecord(metric.rawValue)
  if (localTraffic?.assessmentType === 'local_traffic_count' && finiteNumber(localTraffic.value) && finiteNumber(localTraffic.distanceMeters)) return { text: `${localTraffic.value.toLocaleString()} ${t(String(localTraffic.unit))} · ${String(localTraffic.station)}`, note: `${String(localTraffic.province)} · ${String(localTraffic.surveyDate)} · ${String(localTraffic.surveyHours)} ${t('survey hours')} · ${Math.round(localTraffic.distanceMeters)} ${t('m away')} · ${t('Local survey context; excluded from AADT scoring.')}`, available: true }
  if (metric.type === 'traffic' && metric.status === 'missing') return { text: t('No official AADT match'), available: false }
	const siteSurface = getSiteSurface(metric.rawValue); if (siteSurface) return { text: `${t('Ground-surface suitability')}: ${t(siteSurface.suitability)} · ${siteSurface.score.toFixed(0)}/100`, note: `${siteSurface.imageCount} ${t('site photos analyzed')} · ${t('This score is separate from the location score.')}`, available: true }
	const siteRequirements = getSiteRequirements(metric.rawValue); if (siteRequirements) { const electricalNote = siteRequirements.nearestPublishedElectricalKm === undefined ? t('No published electrical-distance reference was returned for this location.') : `${t('Nearest published electrical reference')} ${siteRequirements.nearestPublishedElectricalKm.toFixed(2)} ${t('km')}. ${t('This is not the actual cable-routing distance.')}`; const terrainText = siteRequirements.terrain ? `${t('Automatic terrain check: elevation range')} ${siteRequirements.terrain.elevationRangeMeters.toFixed(2)} ${t('m across')} ${siteRequirements.terrain.sampleCount} ${t('samples over')} ${siteRequirements.terrain.sampleSpanMeters} ${t('m')}.` : t('Internet, Wi-Fi 2.4 GHz, and land frontage still need a connected evidence source for confirmation.'); const terrainNote = siteRequirements.terrain ? t('This is a sample elevation reading at the pin, not a land survey or earthwork estimate.') : ''; return { text: terrainText, note: [electricalNote, terrainNote].filter(Boolean).join(' · '), available: true } }
	const electricalGrid = getElectricalGrid(metric.rawValue); if (electricalGrid) { const line = electricalGrid.nearestHighVoltageLineMeters === undefined ? t('No published PEA high-voltage line found within the search radius') : `${t('Nearest published PEA high-voltage line')} ${formatKilometers(electricalGrid.nearestHighVoltageLineMeters)} ${t('km away')}`; const station = electricalGrid.nearestStationMeters === undefined ? '' : ` · ${t('Nearest PEA station')} ${electricalGrid.stationName || '—'} ${formatKilometers(electricalGrid.nearestStationMeters)} ${t('km away')}${electricalGrid.stationSecondaryVoltageKv !== undefined ? ` · ${electricalGrid.stationSecondaryVoltageKv} kV` : ''}`; const codes = [electricalGrid.voltageCode && `${t('Voltage code')} ${electricalGrid.voltageCode}`, electricalGrid.phaseCode && `${t('Phase code')} ${electricalGrid.phaseCode}`, electricalGrid.conductorSize && `${t('Conductor size')} ${electricalGrid.conductorSize}`].filter(Boolean).join(' · '); return { text: `${line}${station}`, note: [codes, t('Public PEA GIS evidence only — it does not confirm a pole at the plot, remaining capacity, or three-phase connection.')].filter(Boolean).join(' · '), available: true } }
  const electrical = getElectricalPreliminary(metric.rawValue); if (electrical) { if (electrical.assessmentType === 'official_public_map_guideline') return { text: t('PEA Power Map available for this area'), note: t('Open the official PEA Power Map to review the preliminary area information; capacity and three-phase supply still require PEA confirmation.'), available: true }; const nearest = electrical.matchingMethod === 'nearest_published_area'; const distance = nearest && electrical.distanceToAreaMeters !== undefined ? ` · ${t('nearest published area')} ${formatKilometers(electrical.distanceToAreaMeters)} ${t('km from the published boundary')}` : ''; return { text: `${electrical.publishedCapacityMw?.toLocaleString()} MW · ${electrical.voltageKv} kV · ${electrical.mapYear}${electrical.stationCode ? ` · ${electrical.stationCode}` : ''}`, note: nearest ? `${t('Nearest MEA published-area reference only — not capacity for this plot.')}${distance}` : t('Preliminary MEA Power Map guideline — not verified remaining capacity for this plot.'), available: true } }
  if (metric.type === 'electrical' && metric.status !== 'verified') return { text: t('Utility capacity not verified'), available: false }
  const population = getPopulation(metric.rawValue); if (population) return { text: `${Math.round(population.population).toLocaleString()} ${t('estimated people')} · ${population.populationDensityPerKm2.toLocaleString(undefined, { maximumFractionDigits: 0 })} ${t('people per sq. km')}`, note: `${t('Population density within the selected radius')} · ${population.areaKm2.toFixed(2)} ${t('sq. km')} · ${t('Modelled estimate — not an official census count.')}`, available: true }
  const aadt = getAADT(metric.rawValue); if (aadt) { const authority = aadt.roadAuthority === 'DRR' ? t('Rural road') : t('Highway'); const distance = aadt.distanceToSectionMeters ?? aadt.distanceToRoadMeters; return { text: `${aadt.aadt.toLocaleString()} ${t('vehicles/day (AADT)')} · ${authority} ${aadt.roadCode} · ${aadt.roadName || aadt.controlSectionName}`, note: `${t(aadt.roadAuthority === 'DRR' ? 'Matched official rural-road route' : 'Matched official control section')} · ${distance === undefined ? '—' : `${Math.round(distance).toLocaleString()} ${t('m away')}`} · ${aadt.dataYear}`, available: true } }
  const trafficPotential = getTrafficPotential(metric.rawValue); if (trafficPotential) { const nearestRoad = nearestRoadLabel(trafficPotential, t); const distance = trafficPotential.nearestMajorRoadMeters === undefined ? '' : ` · ${Math.round(trafficPotential.nearestMajorRoadMeters).toLocaleString()} ${t('m away')}`; return { text: `${t('Road-based traffic potential')} ${trafficPotential.potentialScore.toFixed(0)}/100`, note: nearestRoad ? `${t('Nearest mapped main road')}: ${nearestRoad}${distance}` : `${trafficPotential.mappedMajorRoadCount.toLocaleString()} ${t('mapped major-road segments')} · ${trafficPotential.radiusMeters / 1000} ${t('km radius')}`, available: true } }
  const evRegistration = getEVRegistration(metric.rawValue); if (evRegistration) {
    const change = formatEVRegistrationTrend(evRegistration, t)
    return { text: `${evRegistration.registeredBev.toLocaleString()} ${t('registered BEVs')} · ${evRegistration.province}`, note: [change, t('Provincial indicator — not EV demand at this exact location.')].filter(Boolean).join(' · '), available: true }
  }
  const road = getRoad(metric.rawValue); if (road) return { text: `${road.mappedMajorRoadCount.toLocaleString()} ${t('mapped major-road segments')}${road.nearestMajorRoadMeters !== undefined ? ` · ${Math.round(road.nearestMajorRoadMeters).toLocaleString()} ${t('m to nearest mapped major road')}` : ''}`, note: t('Road-map assessment only — it does not confirm the actual entrance, turning movements, road width, or on-site obstructions.'), available: true }
  const provinceNote = metric.type === 'flood' ? provinceFloodNote(metric.rawValue, t) : ''
  const flood = getFlood(metric.rawValue)
  if (flood) {
    if ('assessmentType' in flood) {
      const startYear = t('BE') === 'พ.ศ.' ? flood.periodStartYear : flood.periodStartYear - 543
      const endYear = t('BE') === 'พ.ศ.' ? flood.periodEndYear : flood.periodEndYear - 543
      const historyNote = `${flood.reportedVillageIncidents.toLocaleString()} ${t('reported village flood occurrences')} · ${flood.affectedSubdistrictCount.toLocaleString()} ${t('affected subdistricts')} · ${startYear}–${endYear}. ${t('District-wide reports only — not a parcel-specific flood assessment.')}`
      return { text: `${t('District flood history')}: ${flood.reportedFloodYearCount} ${floodHistoryPeriod(flood.periodStartYear, flood.periodEndYear, t)} · ${flood.district}, ${flood.province}`, note: [historyNote, provinceNote].filter(Boolean).join(' · '), available: true }
    }
    return { text: flood.mappedFloodRiskAreaCount ? `${flood.mappedFloodRiskAreaCount.toLocaleString()} ${t('mapped flood-risk areas')} · ${flood.radiusMeters / 1000} ${t('km radius')}` : t('No mapped flood-risk area within the requested radius'), note: t('Published-layer overlap only — not a flood forecast or a no-risk conclusion.'), available: true }
  }
  if (provinceNote) return { text: t('District flood history unavailable'), note: provinceNote, available: false }
	const count = getRawCount(metric.rawValue); if (count) { const competition = metric.type === 'competition' ? getCompetitionObservation(metric.rawValue) : undefined; const label = metric.type === 'competition' ? t('charging-station records found') : t('points of interest'); const note = metric.type === 'competition' ? competition && !competition.coverageMatched ? t('The search completed, but the connected sources do not yet establish complete coverage for this area. This result is excluded from scoring.') : t('Deduplicated across connected sources — coverage is not nationwide.') : t('Observed count only — not a suitability score.'); return { text: `${count.count.toLocaleString()} ${label} · ${count.radiusMeters / 1000} ${t('km radius')}`, note, available: true } }
  if (finiteNumber(metric.normalizedScore)) return { text: metric.normalizedScore.toFixed(1), available: true }
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
  // OpenStreetMap prefixes categories (for example amenity:hospital), whereas
  // Google Places returns unprefixed primary types (for example hospital).
  // Keep both formats here so historical analyses render correctly too.
  if (['amenity:hospital', 'hospital', 'general_hospital'].includes(category)) return 'Hospitals'
  if (category === 'medical_center') return 'Health services'
  if (['building:commercial', 'shopping_mall', 'business_center', 'general_contractor'].includes(category)) return 'Commercial buildings'
  if (['tourism:hotel', 'building:hotel', 'hotel', 'resort_hotel', 'lodging', 'inn', 'farmstay', 'campground', 'bed_and_breakfast'].includes(category)) return 'Hotels'
  if (category === 'tourism:apartment') return 'Apartments'
  if (category === 'housing_complex') return 'Apartments / condominiums (OSM combined)'
  if (category === 'building:dormitory') return 'Dormitories'
  if (category === 'building:condominium') return 'Condominiums'
  if (category === 'building:apartments') return 'Apartments / condominiums (OSM combined)'
  if (/^tourism:(attraction|museum|zoo|theme_park)$/.test(category) || category === 'tourist_attraction') return 'Tourist attractions'
  if (category === 'building:office' || category.startsWith('office:')) return 'Office buildings'
  if (/^amenity:(restaurant|cafe|fast_food)$/.test(category) || /(^|_)(restaurant|cafe|coffee_shop|noodle_shop)$/.test(category)) return 'Food / cafe'
  if (category.startsWith('shop:') || category === 'amenity:marketplace' || ['market', 'farmers_market', 'flea_market', 'garden_center'].includes(category)) return 'Retail / shopping'
  if (/^amenity:(clinic|pharmacy)$/.test(category)) return 'Health services'
  if (/^amenity:(university|college|school|kindergarten)$/.test(category) || ['university', 'school', 'preschool', 'educational_institution'].includes(category)) return 'Education'
  if (category.startsWith('leisure:') || /^tourism:(guest_house|hostel|motel)$/.test(category)) return 'Other places'
  if (/^amenity:(fuel|bus_station|parking)$/.test(category) || /^shop:(car|car_repair|car_parts|motorcycle)$/.test(category) || category === 'gas_station') return 'Transport / automotive'
  return 'Other places'
}
function summarizePOICategoryGroups(categoryCounts?: Record<string, number>) { if (!categoryCounts) return []; const grouped = new Map<string, number>(['Hospitals', 'Commercial buildings', 'Hotels', 'Apartments', 'Dormitories', 'Condominiums', 'Tourist attractions', 'Office buildings'].map(category => [category, 0])); for (const [category, count] of Object.entries(categoryCounts)) { if (category === 'amenity:charging_station') continue; const group = poiCategoryGroup(category); grouped.set(group, (grouped.get(group) || 0) + count) } return Array.from(grouped.entries()).filter(([category, count]) => count > 0 || ['Hospitals', 'Commercial buildings', 'Hotels', 'Apartments', 'Dormitories', 'Condominiums', 'Tourist attractions', 'Office buildings'].includes(category)).sort(([firstCategory, firstCount], [secondCategory, secondCount]) => secondCount - firstCount || firstCategory.localeCompare(secondCategory)) }
function formatCategory(category: string) { return category.replace(/^[^:]+:/, '').replaceAll('_', ' ') }
function formatPlanArea(plan: FranchisePlan, language: string, t: (key: string) => string) { const locale = language === 'th' ? 'th-TH' : 'en-US'; const minimum = plan.minimumAreaSqWah.toLocaleString(locale); if (plan.maximumAreaSqWah !== undefined) return `${minimum}–${plan.maximumAreaSqWah.toLocaleString(locale)} ${t('sq wah')}`; return language === 'th' ? `${minimum} ${t('sq wah')} ขึ้นไป` : `${minimum}+ ${t('sq wah')}` }
