import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n/I18nProvider'
import { api } from '../services/api'
import { loadLiffConfig, loadLiffSDK } from '../services/liff'
import { LiffSiteSubmissionPage } from './LiffSiteSubmissionPage'
import { LiffMySitesPage } from './LiffMySitesPage'

vi.mock('../services/liff', () => ({ loadLiffConfig: vi.fn(), loadLiffSDK: vi.fn() }))
vi.mock('./NewSitePage', () => ({ NewSitePage: ({ liff }: { liff: { onSubmitted: (accepted: boolean) => void } }) => <button onClick={() => liff.onSubmitted(true)}>Submit test site</button> }))
beforeEach(() => {
  vi.mocked(loadLiffConfig).mockResolvedValue({ liffId: '123-Configured' })
  vi.mocked(loadLiffSDK).mockResolvedValue({ init: vi.fn().mockResolvedValue(undefined), isLoggedIn: () => true, login: vi.fn(), getIDToken: () => 'local-test-token' })
})
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.clearAllMocks() })
describe('LINE customer flow navigation', () => {
  it('lets a customer track submitted sites after saving through LIFF', async () => {
    vi.spyOn(api, 'getLiffCustomerProfile').mockResolvedValue(null)
    render(<I18nProvider initialLanguage="en"><LiffSiteSubmissionPage/></I18nProvider>)
    fireEvent.click(await screen.findByRole('button', { name: 'Submit test site' }))
    expect(screen.getByRole('heading', { name: 'Site information submitted' })).toBeDefined()
    expect(screen.getByRole('link', { name: 'My sites' }).getAttribute('href')).toBe('/liff/site-submission?view=my-sites')
    expect(screen.getByRole('link', { name: 'Return to RBC EV Station chat' }).getAttribute('href')).toBe('https://line.me/R/ti/p/@424cdtee')
  })
  it('uses the current LIFF endpoint when a customer submits another site', async () => {
    vi.spyOn(api, 'listLiffSites').mockResolvedValue([])
    render(<I18nProvider initialLanguage="en"><LiffMySitesPage/></I18nProvider>)
    expect(await screen.findByText('No submitted sites yet')).toBeDefined()
    expect(screen.getByRole('link', { name: 'Submit site information' }).getAttribute('href')).toBe('/liff/site-submission')
  })
})
