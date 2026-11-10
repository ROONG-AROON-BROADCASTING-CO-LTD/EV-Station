import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n/I18nProvider'
import { loadLiffConfig } from '../services/liff'
import { SubmissionChannels } from './SubmissionChannels'

vi.mock('../services/liff', () => ({ loadLiffConfig: vi.fn() }))
afterEach(() => { cleanup(); vi.resetAllMocks() })
function show(authenticated = false) {
  render(<MemoryRouter><I18nProvider initialLanguage="en"><SubmissionChannels authenticated={authenticated}/></I18nProvider></MemoryRouter>)
}
describe('customer submission channels', () => {
  it('offers website registration with a return path and the configured LIFF submission/tracking links', async () => {
    vi.mocked(loadLiffConfig).mockResolvedValue({ liffId: '123-Test' })
    show()
    expect(screen.getByRole('link', { name: 'Submit through the website' }).getAttribute('href')).toBe('/login?mode=register&returnTo=%2Fsites%2Fnew&from=%2Fcontact%23submit-site&lang=en')
    expect(screen.getByRole('link', { name: 'Add LINE to talk to our team' }).getAttribute('href')).toBe('https://line.me/R/ti/p/@424cdtee')
    expect((await screen.findByRole('link', { name: 'Open the form in LINE' })).getAttribute('href')).toBe('https://liff.line.me/123-Test')
    expect(screen.getByRole('link', { name: 'Track submitted sites' }).getAttribute('href')).toBe('https://liff.line.me/123-Test?view=my-sites')
  })
  it('lets an existing website account open the form directly', async () => {
    vi.mocked(loadLiffConfig).mockResolvedValue({ liffId: '123-Test' })
    show(true)
    expect(screen.getByRole('link', { name: 'Submit through the website' }).getAttribute('href')).toBe('/sites/new')
    await screen.findByRole('link', { name: 'Open the form in LINE' })
  })
  it('keeps website and OA contact available when LIFF config fails, and allows recovery', async () => {
    vi.mocked(loadLiffConfig).mockRejectedValueOnce(new Error('offline')).mockResolvedValue({ liffId: '123-Recovered' })
    show()
    await screen.findByRole('button', { name: 'Try again' })
    expect(screen.queryByRole('link', { name: 'Open the form in LINE' })).toBeNull()
    expect(screen.getByRole('link', { name: 'Submit through the website' })).toBeDefined()
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.getByRole('link', { name: 'Open the form in LINE' }).getAttribute('href')).toBe('https://liff.line.me/123-Recovered'))
  })
})
