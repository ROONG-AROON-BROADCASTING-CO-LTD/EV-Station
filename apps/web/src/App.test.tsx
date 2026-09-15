import { cleanup, render, screen, waitFor } from '@testing-library/react'
import type React from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { I18nProvider } from './i18n/I18nProvider'

vi.mock('./components/AppShell', () => ({
  AppShell: ({ children, onLogout }: { children: React.ReactNode; onLogout: () => void }) => <div><button onClick={onLogout}>Log out</button>{children}</div>,
}))

vi.mock('./pages/DashboardPage', () => ({ DashboardPage: () => <p>Dashboard page</p> }))
vi.mock('./pages/APIUsagePage', () => ({ APIUsagePage: () => <p>API usage page</p> }))
vi.mock('./pages/NotInvestingPage', () => ({ NotInvestingPage: () => <p>Not investing page</p> }))

function renderApp() {
  render(<BrowserRouter><I18nProvider><App /></I18nProvider></BrowserRouter>)
}

function saveSession(role: 'customer' | 'sales' | 'admin' | 'super_admin') {
  localStorage.setItem('rbc-session', JSON.stringify({ user: { id: 'user-1', displayName: 'Test user', role }, token: 'token-1' }))
  localStorage.setItem('rbc-session-token', 'token-1')
}

describe('App access boundaries', () => {
  beforeEach(() => {
    localStorage.clear()
    localStorage.setItem('rbc-language', 'en')
    window.history.replaceState({}, '', '/')
  })

  afterEach(cleanup)

  it('renders the login screen when no persisted session exists', () => {
    renderApp()
    expect(screen.getByRole('tab', { name: 'Sign in' })).toBeTruthy()
  })

  it('redirects customers away from admin-only API usage', async () => {
    saveSession('customer')
    window.history.replaceState({}, '', '/api-usage')
    renderApp()
    await waitFor(() => expect(screen.getByText('Dashboard page')).toBeTruthy())
    expect(screen.queryByText('API usage page')).toBeNull()
  })

  it('allows an admin to view API usage and clears both persisted session keys on logout', async () => {
    saveSession('admin')
    window.history.replaceState({}, '', '/api-usage')
    renderApp()
    await waitFor(() => expect(screen.getByText('API usage page')).toBeTruthy())

    screen.getByRole('button', { name: 'Log out' }).click()

    expect(localStorage.getItem('rbc-session')).toBeNull()
    expect(localStorage.getItem('rbc-session-token')).toBeNull()
    await waitFor(() => expect(screen.getByRole('tab', { name: 'Sign in' })).toBeTruthy())
  })

  it('reacts to an API expiry event by returning to the login screen', async () => {
    saveSession('sales')
    renderApp()
    await waitFor(() => expect(screen.getByText('Dashboard page')).toBeTruthy())

    window.dispatchEvent(new Event('rbc:auth-expired'))

    await waitFor(() => expect(screen.getByRole('tab', { name: 'Sign in' })).toBeTruthy())
  })
})
