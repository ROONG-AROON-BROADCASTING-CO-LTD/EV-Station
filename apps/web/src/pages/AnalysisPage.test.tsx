import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n/I18nProvider'
import { api } from '../services/api'
import { downloadAnalysisPDF } from '../services/analysisPDF'
import type { AIAssessment, AnalysisRun, FranchisePlan, Metric, Site, UserRole } from '../types/domain'
import { AnalysisPage } from './AnalysisPage'

vi.mock('../services/analysisPDF', () => ({ downloadAnalysisPDF: vi.fn().mockResolvedValue(undefined) }))

vi.mock('../components/MapPanel', () => ({
  MapPanel: () => null,
  PrintableLocationMap: () => null,
  RadiusLabel: () => null,
  RadiusSelector: () => null,
}))

const timestamp = '2026-11-05T00:00:00Z'
const site: Site = { id: 'test-site', name: 'Test site', landSize: 100, landSizeUnit: 'sqwah', electricalSupplyType: 'unknown', inputStatus: 'preliminary', createdAt: timestamp, updatedAt: timestamp }
const surface = {
  analysisScope: 'visible_site_conditions',
  assessmentStatus: 'assessed',
  imageCount: 2,
  summary: 'Review the supplied ground photos.',
  suitability: 'high',
  score: 85,
  surfaceTypes: ['Concrete'],
  observedRisks: [],
  recommendedImprovements: ['Provide clear photos of the ground and entrance.'],
  disclaimer: 'A site survey is still required.',
  entranceWidthEstimate: { minimumMeters: 7, maximumMeters: 9, confidence: 'moderate', visualEvidence: 'Visible entrance', obstructions: [] },
}

const aiAssessment: AIAssessment = { summary: 'Current evidence summary.', decision: 'invest', recommendation: 'Current evidence recommendation.', strengths: [], risks: [], requiredChecks: ['Review the available evidence.'], disclaimer: 'Preliminary.', language: 'en', model: 'test', generatedAt: timestamp, decisionPolicy: 'test' }
const smallPlan: FranchisePlan = { code: 'S', name: 'Small station', recommendedAreaSqWah: 100, minimumAreaSqWah: 50, evChargingStations: 1, investmentMinThb: 1500000, investmentOpenEnded: false, franchiseFeeThb: 300000, locationProfile: [], coreServices: [], sourceStatus: 'user_supplied', sourceNote: 'Test package', roiAvailable: false, missingForRoi: [] }
type ReportOptions = { role?: UserRole; aiAssessments?: AnalysisRun['aiAssessments']; recommendation?: string; siteData?: Site; siteLoad?: () => Promise<Site>; plansLoad?: () => Promise<FranchisePlan[]>; aiLoad?: typeof api.generateAIAssessments; includeInternet?: boolean }

async function show(rawValue: unknown, language: 'th' | 'en' = 'en', normalizedScore?: number, assumptions: string[] = [], options: ReportOptions = {}) {
  const metric: Metric = { id: 'surface', analysisRunId: 'test-run', type: 'site_readiness', rawValue, normalizedScore, status: 'preliminary', source: { name: 'Gemini', type: 'ai', retrievedAt: timestamp }, assumptions, createdAt: timestamp }
  const run: AnalysisRun = { id: 'test-run', siteId: site.id, status: 'completed', analysisRadiusMeters: 1000, overallScore: 94.6, assessmentStatus: 'preliminary', recommendation: options.recommendation ?? '', aiAssessments: options.aiAssessments, metrics: [metric, ...(options.includeInternet ? [{ ...metric, id: 'internet', type: 'site_requirements', rawValue: { assessmentType: 'automatic_site_requirements' } }] : [])], startedAt: timestamp, createdAt: timestamp }
  vi.spyOn(api, 'getAnalysis').mockResolvedValue(run)
  vi.spyOn(api, 'getLatestAnalysisForSite').mockResolvedValue(run)
  vi.spyOn(api, 'getSite').mockImplementation(options.siteLoad ?? (() => Promise.resolve(options.siteData ?? site)))
  vi.spyOn(api, 'getFranchisePlans').mockImplementation(options.plansLoad ?? (() => Promise.resolve([])))
  vi.spyOn(api, 'generateAIAssessments').mockImplementation(options.aiLoad ?? (() => Promise.resolve({ assessments: {}, errors: {} })))
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/analysis/test-run']}><I18nProvider initialLanguage={language}><Routes><Route path="/analysis/:id" element={<AnalysisPage role={options.role ?? 'customer'} />} /></Routes></I18nProvider></MemoryRouter></QueryClientProvider>)
  const label = await screen.findByText(language === 'th' ? 'การวิเคราะห์สภาพพื้นที่จริง' : 'Real site condition analysis')
  if (!options.siteLoad && !options.plansLoad && !options.aiLoad) await waitFor(() => expect((screen.getByRole('button', { name: language === 'th' ? 'ดาวน์โหลด PDF' : 'Download PDF' }) as HTMLButtonElement).disabled).toBe(false))
  return label.closest('tr')!
}

beforeEach(() => { vi.mocked(downloadAnalysisPDF).mockReset().mockResolvedValue(undefined); vi.spyOn(window, 'print').mockImplementation(() => {}) })
afterEach(() => { cleanup(); vi.restoreAllMocks() })

describe('site-condition evidence status in analysis reports', () => {
  it.each(['pending', 'running'] as const)('renders a %s report with null metrics and opens it after completion', async status => {
    // Queued PostgreSQL records have no metric rows yet and serialize as null.
    const run = { id: 'test-run', siteId: site.id, status, analysisRadiusMeters: 1000, metrics: null, assessmentStatus: 'preliminary', startedAt: timestamp, createdAt: timestamp } as unknown as AnalysisRun
    const getAnalysis = vi.spyOn(api, 'getAnalysis').mockResolvedValue(run)
    vi.spyOn(api, 'getSite').mockResolvedValue(site)
    vi.spyOn(api, 'getFranchisePlans').mockResolvedValue([smallPlan])
    vi.spyOn(api, 'getLatestAnalysisForSite').mockResolvedValue(null)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/analysis/test-run']}>
          <I18nProvider initialLanguage="en">
            <Routes><Route path="/analysis/:id" element={<AnalysisPage role="customer" />} /></Routes>
          </I18nProvider>
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect(await screen.findByRole('heading', { name: 'Analyzing site' })).toBeDefined()
    expect(screen.queryByRole('button', { name: 'Download PDF' })).toBeNull()
    getAnalysis.mockResolvedValue({ ...run, status: 'completed', metrics: [] })
    await act(async () => { await client.invalidateQueries({ queryKey: ['analysis', run.id] }) })
    expect(await screen.findByRole('button', { name: 'Download PDF' })).toBeDefined()
  })

  it.each(['th', 'en'] as const)('does not call a large site too small when no package definitions exist in %s', async language => {
    await show(surface, language, undefined, [], { siteData: { ...site, landSize: 200 } })
    const text = screen.getByText(language === 'th' ? 'รูปแบบโครงการตามขนาดพื้นที่' : 'Project format by land size').closest('section')!.textContent
    expect(text).toContain(language === 'th' ? 'ยังไม่มีข้อมูลแพ็กเกจสำหรับเปรียบเทียบขนาดพื้นที่' : 'Package definitions are unavailable for comparing land sizes.')
    expect(text).not.toContain(language === 'th' ? 'ต่ำกว่าขนาดขั้นต่ำ' : 'below the minimum')
    expect((screen.getByRole('button', { name: language === 'th' ? 'ดาวน์โหลด PDF' : 'Download PDF' }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('requires land-size data instead of saying an unknown area is below the minimum', async () => {
    // Simulate an incomplete legacy API record rather than a valid submission.
    const missingAreaSite = { ...site, landSize: undefined } as unknown as Site
    await show(surface, 'en', undefined, [], { siteData: missingAreaSite, plansLoad: () => Promise.resolve([smallPlan]) })
    expect(await screen.findByText('Land size is not available. Confirm the area before selecting a project format.')).toBeDefined()
    expect(screen.queryByText(/land area is below the minimum/)).toBeNull()
  })

  it('keeps the minimum-area explanation when known land size is genuinely too small', async () => {
    await show(surface, 'en', undefined, [], { siteData: { ...site, landSize: 20 }, plansLoad: () => Promise.resolve([smallPlan]) })
    expect(await screen.findByText('The 20.0 sq wah land area is below the minimum for package S (50 sq wah).')).toBeDefined()
  })

  it('marks the explicit map link for inclusion in the PDF', async () => {
    await show(surface, 'en', undefined, [], { siteData: { ...site, latitude: 13.77, longitude: 100.58 } })
    const link = screen.getByRole('link', { name: 'Open in Google Maps' })
    expect(link.getAttribute('data-pdf-link')).toBe('true')
    expect(link.getAttribute('href')).toBe('https://www.google.com/maps?q=13.77,100.58')
  })

  it('starts only one PDF export for repeated clicks while an export is pending', async () => {
    let finish!: () => void
    vi.mocked(downloadAnalysisPDF).mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve }))
    await show(surface)
    const button = screen.getByRole('button', { name: 'Download PDF' })
    fireEvent.click(button)
    fireEvent.click(button)
    await waitFor(() => expect(downloadAnalysisPDF).toHaveBeenCalledOnce())
    await act(async () => { finish() })
    await waitFor(() => expect((screen.getByRole('button', { name: 'Download PDF' }) as HTMLButtonElement).disabled).toBe(false))
  })

  it.each(['site', 'plans'] as const)('does not snapshot a report while %s data is loading', async dependency => {
    let release!: () => void
    const siteLoad = () => new Promise<Site>(resolve => { release = () => resolve(site) })
    const plansLoad = () => new Promise<FranchisePlan[]>(resolve => { release = () => resolve([]) })
    await show(surface, 'en', undefined, [], dependency === 'site' ? { siteLoad } : { plansLoad })
    const button = screen.getByRole('button', { name: 'Download PDF' }) as HTMLButtonElement
    expect(button.disabled).toBe(true)
    expect(screen.getByText('Loading report data before PDF download…')).toBeDefined()
    fireEvent.click(button)
    expect(downloadAnalysisPDF).not.toHaveBeenCalled()
    await act(async () => { release() })
    await waitFor(() => expect(button.disabled).toBe(false))
    fireEvent.click(button)
    await waitFor(() => expect(downloadAnalysisPDF).toHaveBeenCalledOnce())
    expect(vi.mocked(downloadAnalysisPDF).mock.calls[0][0].title).toBe(site.name)
  })

  it.each(['site', 'plans'] as const)('offers a retry instead of exporting when %s data cannot be loaded', async dependency => {
    const load = vi.fn().mockRejectedValueOnce(new Error('unavailable')).mockResolvedValue(dependency === 'site' ? site : [])
    await show(surface, 'en', undefined, [], dependency === 'site' ? { siteLoad: load } : { plansLoad: load })
    const button = screen.getByRole('button', { name: 'Download PDF' }) as HTMLButtonElement
    expect((await screen.findByRole('alert')).textContent).toContain('Required report data could not be loaded. Retry before downloading the PDF.')
    expect(button.disabled).toBe(true)
    fireEvent.click(button)
    expect(downloadAnalysisPDF).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Retry report data' }))
    await waitFor(() => expect(button.disabled).toBe(false))
    fireEvent.click(button)
    await waitFor(() => expect(downloadAnalysisPDF).toHaveBeenCalledOnce())
  })

  it('waits for current AI commentary before exporting the report', async () => {
    let resolve!: (value: Awaited<ReturnType<typeof api.generateAIAssessments>>) => void
    const promise = new Promise<Awaited<ReturnType<typeof api.generateAIAssessments>>>(done => { resolve = done })
    await show(surface, 'en', undefined, [], { role: 'admin', aiLoad: () => promise })
    const button = screen.getByRole('button', { name: 'Download PDF' }) as HTMLButtonElement
    expect(button.disabled).toBe(true)
    expect(await screen.findByText('Preparing AI commentary before PDF download…')).toBeDefined()
    fireEvent.click(button)
    expect(downloadAnalysisPDF).not.toHaveBeenCalled()
    await act(async () => { resolve({ assessments: { en: aiAssessment }, errors: {} }) })
    await waitFor(() => expect(button.disabled).toBe(false))
    fireEvent.click(button)
    await waitFor(() => expect(downloadAnalysisPDF).toHaveBeenCalledOnce())
    expect(vi.mocked(downloadAnalysisPDF).mock.calls[0][0].element.textContent).toContain(aiAssessment.summary)
  })

  it.each([[true, 'Available'], [false, 'Unavailable'], [undefined, 'Pending verification']] as const)('preserves internet availability %s as export text', async (internetAvailable, label) => {
    await show(surface, 'en', undefined, [], { siteData: { ...site, internetAvailable }, includeInternet: true })
    const row = screen.getByText('Internet signal access', { exact: true }).closest('tr')!
    expect(row.querySelector('[data-pdf-text]')?.getAttribute('data-pdf-text')).toBe(label)
  })

  it('marks only the inactive investment decision for omission from the PDF', async () => {
    await show(surface)
    expect(screen.getByText('Not yet recommended to invest', { exact: true }).closest('[data-pdf-ignore="true"]')).not.toBeNull()
    expect(screen.getByText('Recommended to invest', { exact: true }).closest('[data-pdf-ignore="true"]')).toBeNull()
  })

  it('automatically recovers an expired fallback once without an endless loop', async () => {
    await show(surface, 'en', undefined, [], { role: 'admin', aiAssessments: { en: { ...aiAssessment, model: 'system-fallback', generatedAt: '2020-01-01T00:00:00Z' } } })
    await waitFor(() => expect(api.generateAIAssessments).toHaveBeenCalledWith('test-run', ['en'], false), { timeout: 2500 })
    await new Promise(resolve => window.setTimeout(resolve, 1150))
    expect(api.generateAIAssessments).toHaveBeenCalledOnce()
  })

  it('does not regenerate cached fallback for customers', async () => {
    await show(surface, 'en', undefined, [], { aiAssessments: { en: { ...aiAssessment, model: 'system-fallback', generatedAt: '2020-01-01T00:00:00Z' } } })
    expect(api.generateAIAssessments).not.toHaveBeenCalled()
  })

  it('shows a retryable export error and does not fall back to printing', async () => {
    vi.mocked(downloadAnalysisPDF).mockRejectedValueOnce(new Error('PDF_FONT_UNAVAILABLE'))
    await show(surface)
    fireEvent.click(screen.getByRole('button', { name: 'Download PDF' }))
    expect((await screen.findByRole('alert')).textContent).toContain('The PDF could not be created')
    expect(window.print).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Download PDF' }))
    await waitFor(() => expect(downloadAnalysisPDF).toHaveBeenCalledTimes(2))
  })

  it.each(['en', 'th'] as const)('shows an unavailable assessment with its next checks in %s', async language => {
    const row = await show({ ...surface, assessmentStatus: 'insufficient_evidence', score: null, suitability: 'not_assessable' }, language)
    expect(row.textContent).toContain(language === 'th' ? 'ยังประเมินไม่ได้' : 'Unable to assess yet')
    expect(row.textContent).toContain(language === 'th' ? 'หลักฐานหน้างานยังไม่เพียงพอ' : 'There is not enough site evidence')
    expect(row.textContent).not.toContain('/100')
    expect(row.textContent).toContain(language === 'th' ? 'ไม่มีข้อมูล' : 'Missing data')
    fireEvent.click(within(row).getByText(language === 'th' ? 'รายละเอียดสภาพพื้น' : 'Ground-surface details'))
    expect(row.textContent).toContain(surface.summary)
    expect(row.textContent).toContain(surface.recommendedImprovements[0])
    expect(row.textContent).not.toContain(language === 'th' ? 'หน้ากว้างทางเข้าโดยประมาณ' : 'Approximate entrance width')
    // Export the sanitized report directly, without opening the print dialog.
    fireEvent.click(screen.getByRole('button', { name: language === 'th' ? 'ดาวน์โหลด PDF' : 'Download PDF' }))
    await waitFor(() => expect(downloadAnalysisPDF).toHaveBeenCalledOnce())
    expect(window.print).not.toHaveBeenCalled()
    expect(row.textContent).not.toContain('/100')
    expect(screen.getByText('94.6')).toBeDefined()
  })

  it.each([0, 85])('ignores contradictory score %s, low rating, and width when evidence is insufficient', async score => {
    const row = await show({ ...surface, assessmentStatus: 'insufficient_evidence', score, suitability: 'low' }, 'en', score)
    expect(row.textContent).toContain('Unable to assess yet')
    expect(row.textContent).not.toContain('/100')
    expect(row.textContent).not.toContain('Ground-surface suitability')
    expect(row.textContent).not.toContain('Approximate entrance width')
    expect(screen.queryByText(/Factors to watch:/)).toBeNull()
  })

  it.each([0, 85])('requires a new analysis for legacy reports with score %s and no evidence status', async score => {
    const row = await show({ ...surface, assessmentStatus: undefined, score }, 'en', score)
    expect(row.textContent).toContain('Unable to assess yet')
    expect(row.textContent).toContain('The evidence status in this saved report has not been confirmed. Ask staff to run a new analysis.')
    expect(row.textContent).not.toContain('/100')
    expect(row.textContent).not.toContain('Approximate entrance width')
    expect(row.textContent).toContain(surface.recommendedImprovements[0])
  })

  it.each([0, 85])('keeps a valid assessed score of %s and the supported width estimate', async score => {
    const row = await show({ ...surface, score, suitability: score === 0 ? 'low' : 'high' })
    expect(row.textContent).toContain(`Ground-surface suitability: ${score === 0 ? 'low' : 'high'} · ${score}/100`)
    expect(row.textContent).toContain('Approximate entrance width')
    expect(row.textContent).not.toContain('Unable to assess yet')
  })

  it.each([null, {}, { ...surface, assessmentStatus: 'assessed', score: null }, { ...surface, assessmentStatus: 'assessed', score: -1 }, { ...surface, assessmentStatus: 'assessed', score: 101 }])('does not invent a score for a missing or invalid assessment', async rawValue => {
    const row = await show(rawValue, 'en', 0)
    expect(row.textContent).toContain('Unable to assess yet')
    expect(row.textContent).not.toContain('/100')
    expect(row.textContent).not.toContain('Approximate entrance width')
  })

  it.each(['en', 'th'] as const)('keeps the real AI failure reason without claiming the supplied evidence was insufficient in %s', async language => {
    const reason = 'Site attachments were supplied, but Gemini timed out.'
    const row = await show(null, language, 0, [reason])
    expect(row.textContent).toContain(language === 'th' ? 'ยังประเมินไม่ได้' : 'Unable to assess yet')
    expect(row.textContent).toContain(language === 'th' ? 'ยังไม่ได้ประเมินสภาพพื้นที่ กรุณาตรวจหลักฐานและสถานะการวิเคราะห์ด้านล่าง' : 'Site conditions have not been assessed yet. Review the evidence and analysis status below')
    expect(row.textContent).toContain(reason)
    expect(row.textContent).not.toContain(language === 'th' ? 'หลักฐานหน้างานยังไม่เพียงพอ' : 'There is not enough site evidence')
    expect(row.textContent).not.toContain('/100')
  })

  it.each(['en', 'th'] as const)('hides cached AI conclusions about unconfirmed legacy surface evidence in %s', async language => {
    const stale = { ...aiAssessment, summary: 'STALE summary: site is low, 0/100.', recommendation: 'STALE investment recommendation because surface is low, 0/100.', risks: ['STALE ground risk, 0/100.'] }
    await show({ ...surface, assessmentStatus: undefined, score: 0, suitability: 'low' }, language, undefined, [], { aiAssessments: { en: stale, th: stale } })
    expect(screen.queryByText(/STALE/)).toBeNull()
    expect(screen.getByText(language === 'th' ? 'ยังไม่แสดงคำอธิบาย AI เดิม เพราะยังไม่ได้ยืนยันหลักฐานสภาพพื้นที่ กรุณาให้เจ้าหน้าที่วิเคราะห์พื้นที่ใหม่ แล้วสร้างคำอธิบาย AI อีกครั้ง' : 'Saved AI commentary is hidden until a new site analysis confirms the evidence status. Ask staff to run a new analysis and regenerate the commentary.')).toBeDefined()
    expect(screen.getByText('94.6')).toBeDefined()
  })

  it('does not auto-generate or offer manual AI generation against an unconfirmed legacy run', async () => {
    await show({ ...surface, assessmentStatus: undefined }, 'en', undefined, [], { role: 'super_admin', recommendation: 'STALE fallback: low surface 0/100.' })
    expect(api.generateAIAssessments).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: 'Regenerate AI analysis' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Generate updated recommendation' })).toBeNull()
    expect(screen.queryByText(/STALE/)).toBeNull()
    expect(screen.getByRole('button', { name: 'Run new analysis' })).toBeDefined()
  })

  it.each([
    ['assessed', surface],
    ['insufficient', { ...surface, assessmentStatus: 'insufficient_evidence', score: null, suitability: 'not_assessable' }],
    ['no raw result', null],
  ])('retains cached AI commentary for %s evidence', async (_name, rawValue) => {
    await show(rawValue, 'en', undefined, [], { aiAssessments: { en: aiAssessment } })
    expect(screen.getByText(aiAssessment.summary)).toBeDefined()
    expect(screen.queryByText(/Saved AI commentary is hidden/)).toBeNull()
  })

  it.each([
    ['assessed', surface],
    ['insufficient', { ...surface, assessmentStatus: 'insufficient_evidence', score: null, suitability: 'not_assessable' }],
    ['no raw result', null],
  ])('still allows staff to generate AI notes for %s evidence', async (_name, rawValue) => {
    await show(rawValue, 'en', undefined, [], { role: 'super_admin' })
    await waitFor(() => expect(api.generateAIAssessments).toHaveBeenCalledTimes(1))
    const button = await screen.findByRole('button', { name: 'Regenerate AI analysis' })
    fireEvent.click(button)
    await waitFor(() => expect(api.generateAIAssessments).toHaveBeenCalledTimes(2))
  })
})
