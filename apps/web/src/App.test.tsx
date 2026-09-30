import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type React from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import App from './App'
import { I18nProvider } from './i18n/I18nProvider'

vi.mock('./components/AppShell', () => ({
  AppShell: ({ children, onLogout }: { children: React.ReactNode; onLogout: () => void }) => <div><button onClick={onLogout}>Log out</button>{children}</div>,
}))

vi.mock('./pages/DashboardPage', () => ({ DashboardPage: () => <p>Dashboard page</p> }))
vi.mock('./pages/APIUsagePage', () => ({ APIUsagePage: () => <p>API usage page</p> }))
vi.mock('./pages/NotInvestingPage', () => ({ NotInvestingPage: () => <p>Not investing page</p> }))

function renderApp(path = '/') {
  render(<MemoryRouter initialEntries={[path]}><I18nProvider><App /></I18nProvider></MemoryRouter>)
}

function saveSession(role: 'customer' | 'sales' | 'admin' | 'super_admin') {
  localStorage.setItem('rbc-session', JSON.stringify({ user: { id: 'user-1', displayName: 'Test user', role }, token: 'token-1' }))
  localStorage.setItem('rbc-session-token', 'token-1')
}

describe('App access boundaries', () => {
  beforeEach(() => {
    localStorage.clear()
    localStorage.setItem('rbc-language', 'en')
  })

  afterEach(cleanup)

  it('renders the public landing page when no persisted session exists', () => {
    renderApp()
    expect(screen.getByRole('heading', { name: 'Your EV station starts here Invest in EV with RBC Group' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Sign in' })).toBeTruthy()
  })

  it('opens sign in as a closable dialog over the landing page', async () => {
    renderApp('/login')

    expect(screen.getByRole('heading', { name: 'Your EV station starts here Invest in EV with RBC Group' })).toBeTruthy()
    expect(screen.getByRole('dialog')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Close sign-in dialog' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('heading', { name: 'Your EV station starts here Invest in EV with RBC Group' })).toBeTruthy()
  })
  it('keeps the landing page at the root route and navigates from the account logo without a reload', async () => {
    saveSession('admin')
    renderApp()

    expect(screen.getByRole('heading', { name: 'Your EV station starts here Invest in EV with RBC Group' })).toBeTruthy()
    expect(screen.getByText('Test user')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Open account menu' }))
    const dashboardLink = screen.getByRole('link', { name: 'Dashboard' })
    expect(dashboardLink.getAttribute('href')).toBe('/dashboard')

    fireEvent.click(dashboardLink)
    await waitFor(() => expect(screen.getByText('Dashboard page')).toBeTruthy())
  })

  it('offers logout from the account menu on the landing page', async () => {
    saveSession('admin')
    renderApp()

    fireEvent.click(screen.getByRole('button', { name: 'Open account menu' }))
    fireEvent.click(screen.getByRole('button', { name: 'Log out' }))

    await waitFor(() => expect(screen.getByRole('link', { name: 'Sign in' })).toBeTruthy())
    expect(localStorage.getItem('rbc-session')).toBeNull()
    expect(localStorage.getItem('rbc-session-token')).toBeNull()
  })
  it('redirects customers away from admin-only API usage to the landing page', async () => {
    saveSession('customer')
    renderApp('/api-usage')
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Your EV station starts here Invest in EV with RBC Group' })).toBeTruthy())
    expect(screen.queryByText('API usage page')).toBeNull()
  })

  it('allows an admin to view API usage and clears both persisted session keys on logout', async () => {
    saveSession('admin')
    renderApp('/api-usage')
    await waitFor(() => expect(screen.getByText('API usage page')).toBeTruthy())

    screen.getByRole('button', { name: 'Log out' }).click()

    expect(localStorage.getItem('rbc-session')).toBeNull()
    expect(localStorage.getItem('rbc-session-token')).toBeNull()
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Your EV station starts here Invest in EV with RBC Group' })).toBeTruthy())
  })

  it('reacts to an API expiry event by returning to the login screen', async () => {
    saveSession('sales')
    renderApp('/dashboard')
    await waitFor(() => expect(screen.getByText('Dashboard page')).toBeTruthy())

    window.dispatchEvent(new Event('rbc:auth-expired'))

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Your EV station starts here Invest in EV with RBC Group' })).toBeTruthy())
  })
})
