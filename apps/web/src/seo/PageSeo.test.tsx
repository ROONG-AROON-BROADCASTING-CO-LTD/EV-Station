import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { I18nProvider, LanguageSwitcher } from '../i18n/I18nProvider'
import { PublicLink } from '../components/PublicLink'
import { PageSeo } from './PageSeo'

afterEach(() => { cleanup(); document.head.querySelectorAll('[data-rbc-seo]').forEach(node => node.remove()) })

it('updates metadata on language switches and navigation, and removes public metadata on private routes', async () => {
  render(<MemoryRouter><I18nProvider initialLanguage="th"><PageSeo /><LanguageSwitcher /><PublicLink to="/investment">Investment</PublicLink><PublicLink to="/dashboard">Workspace</PublicLink></I18nProvider></MemoryRouter>)
  expect(document.title).toContain('ลงทุนสถานีชาร์จ EV')
  fireEvent.click(screen.getByText('EN', { selector: 'button' }))
  await waitFor(() => expect(document.title).toContain('EV Charging Station Investment'))
  expect(screen.getByText('Investment').getAttribute('href')).toBe('/investment?lang=en')
  fireEvent.click(screen.getByText('Investment'))
  await waitFor(() => expect(document.title).toContain('AC/DC Charging Solutions'))
  expect(document.head.querySelectorAll('link[rel="canonical"]')).toHaveLength(1)
  expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://www.rbcevstation.com/investment?lang=en')
  fireEvent.click(screen.getByText('Workspace'))
  await waitFor(() => expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex, nofollow'))
  expect(document.head.querySelector('link[rel="canonical"]')).toBeNull()
  expect(document.head.querySelector('script[type="application/ld+json"]')).toBeNull()
})
