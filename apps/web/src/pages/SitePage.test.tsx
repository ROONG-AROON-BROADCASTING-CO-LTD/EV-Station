import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n/I18nProvider'
import { api } from '../services/api'
import type { AnalysisRun, Site } from '../types/domain'
import { SitePage } from './SitePage'

vi.mock('../components/MapPanel', () => ({ MapPanel: () => null, RadiusLabel: () => null, RadiusSelector: () => null }))
const timestamp = '2026-11-05T00:00:00Z'
const site: Site = { id: 'site-qa', name: 'Customer site', landSize: 100, landSizeUnit: 'sqwah', electricalSupplyType: 'unknown', inputStatus: 'preliminary', createdAt: timestamp, updatedAt: timestamp }
const completed: AnalysisRun = { id: 'report-ready', siteId: site.id, status: 'completed', analysisRadiusMeters: 3000, overallScore: 80, assessmentStatus: 'preliminary', recommendation: '', metrics: [], startedAt: timestamp, createdAt: timestamp }

async function show(result: AnalysisRun | null, language: 'en' | 'th' = 'en') {
  vi.spyOn(api, 'getSite').mockResolvedValue(site)
  vi.spyOn(api, 'listSiteImages').mockResolvedValue([])
  vi.spyOn(api, 'getLatestAnalysisForSite').mockResolvedValue(result)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/sites/site-qa']}><I18nProvider initialLanguage={language}><Routes><Route path="/sites/:id" element={<SitePage role="customer" />} /></Routes></I18nProvider></MemoryRouter></QueryClientProvider>)
  await screen.findByText(site.name)
}

afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.useRealTimers() })

describe('customer site report availability', () => {
  it.each(['en', 'th'] as const)('links to the completed report instead of showing a waiting message in %s', async language => {
    await show(completed, language)
    expect((await screen.findByRole('link', { name: language === 'th' ? 'ดูรายงานวิเคราะห์' : 'View analysis report' })).getAttribute('href')).toBe('/analysis/report-ready')
    expect(screen.queryByText('The RBC team will review your information and begin the site analysis when it is ready.')).toBeNull()
  })

  it('checks for a completed report after an in-progress run and then stops polling', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    await show({ ...completed, status: 'running' })
    await screen.findByText('The site analysis is in progress. Your report will appear here when it is ready.')
    vi.mocked(api.getLatestAnalysisForSite).mockResolvedValue(completed)
    await act(async () => { vi.advanceTimersByTime(3000) })
    await screen.findByRole('link', { name: 'View analysis report' })
    const count = vi.mocked(api.getLatestAnalysisForSite).mock.calls.length
    await act(async () => { vi.advanceTimersByTime(60_000) })
    expect(api.getLatestAnalysisForSite).toHaveBeenCalledTimes(count)
  })

  it.each(['pending', null] as const)('does not poll indefinitely while report status is %s', async status => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    await show(status ? { ...completed, status } : null)
    await screen.findByText(status ? 'The site analysis is in progress. Your report will appear here when it is ready.' : 'The RBC team will review your information and begin the site analysis when it is ready.')
    await act(async () => { vi.advanceTimersByTime(6 * 60_000) })
    await waitFor(() => expect(api.getLatestAnalysisForSite).toHaveBeenCalled())
    const count = vi.mocked(api.getLatestAnalysisForSite).mock.calls.length
    await act(async () => { vi.advanceTimersByTime(6 * 60_000) })
    expect(api.getLatestAnalysisForSite).toHaveBeenCalledTimes(count)
  })

  it('detects a newly completed report while the endpoint initially returns no completed report', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    await show(null)
    await screen.findByText('The RBC team will review your information and begin the site analysis when it is ready.')
    vi.mocked(api.getLatestAnalysisForSite).mockResolvedValue(completed)
    await act(async () => { vi.advanceTimersByTime(30_000) })
    expect((await screen.findByRole('link', { name: 'View analysis report' })).getAttribute('href')).toBe('/analysis/report-ready')
  })

  it('keeps a clear no-report state and lets the customer check again', async () => {
    await show(null)
    await screen.findByText('The RBC team will review your information and begin the site analysis when it is ready.')
    expect(screen.queryByRole('link', { name: 'View analysis report' })).toBeNull()
    vi.mocked(api.getLatestAnalysisForSite).mockResolvedValue(completed)
    fireEvent.click(screen.getByRole('button', { name: 'Check report status' }))
    expect((await screen.findByRole('link', { name: 'View analysis report' })).getAttribute('href')).toBe('/analysis/report-ready')
  })

  it('shows failed analysis separately and does not give customers staff controls', async () => {
    await show({ ...completed, status: 'failed' })
    await screen.findByText('The last analysis could not be completed. The RBC team needs to retry it.')
    expect(screen.queryByRole('link', { name: 'View analysis report' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Run Analysis' })).toBeNull()
  })

  it('shows a status lookup failure instead of claiming there is no report', async () => {
    await show(null)
    vi.mocked(api.getLatestAnalysisForSite).mockRejectedValue(new Error('network unavailable'))
    fireEvent.click(screen.getByRole('button', { name: 'Check report status' }))
    expect((await screen.findByRole('alert')).textContent).toContain('Unable to check the report status. Please try again.')
  })
})
