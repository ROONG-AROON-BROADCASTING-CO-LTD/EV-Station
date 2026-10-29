import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { I18nProvider } from '../i18n/I18nProvider'
import { PublicHeader } from './PublicHeader'

beforeEach(() => {
  localStorage.setItem('rbc-language', 'en')
  vi.stubGlobal('matchMedia', vi.fn(() => ({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })))
})

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function renderHeader(authenticated = false, onLogout = vi.fn()) {
  render(<MemoryRouter><I18nProvider><PublicHeader authenticated={authenticated} accountName="RBC Owner" onLogout={onLogout}/></I18nProvider></MemoryRouter>)
  return onLogout
}

describe('PublicHeader navigation', () => {
  it('keeps sign-in reachable from the mobile navigation', () => {
    renderHeader()
    fireEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    const menu = screen.getByRole('navigation', { name: 'Mobile navigation' })
    expect(within(menu).getByRole('link', { name: 'Sign in' }).getAttribute('href')).toBe('/login')
    expect(within(menu).getByRole('link', { name: 'Submit a site' }).getAttribute('href')).toBe('/login?mode=register')
  })

  it('keeps authenticated account actions reachable on mobile', () => {
    const onLogout = renderHeader(true)
    fireEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    const menu = screen.getByRole('navigation', { name: 'Mobile navigation' })
    expect(within(menu).getByRole('link', { name: 'Dashboard' }).getAttribute('href')).toBe('/dashboard')
    expect(within(menu).getByRole('link', { name: 'Settings' }).getAttribute('href')).toBe('/settings')
    fireEvent.click(within(menu).getByRole('button', { name: 'Log out' }))
    expect(onLogout).toHaveBeenCalledOnce()
    expect(screen.queryByRole('navigation', { name: 'Mobile navigation' })).toBeNull()
  })

  it('closes the disclosure with Escape and restores trigger focus', () => {
    renderHeader(true)
    fireEvent.click(screen.getByRole('button', { name: 'Open account menu' }))
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('navigation', { name: 'Account menu' })).toBeNull()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Open account menu' }))
  })

  it('closes mobile navigation after a destination is selected or an outside click', () => {
    renderHeader()
    fireEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    fireEvent.click(within(screen.getByRole('navigation', { name: 'Mobile navigation' })).getByRole('link', { name: 'Contact us' }))
    expect(screen.queryByRole('navigation', { name: 'Mobile navigation' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    fireEvent.pointerDown(document.body)
    expect(screen.queryByRole('navigation', { name: 'Mobile navigation' })).toBeNull()
  })
})
