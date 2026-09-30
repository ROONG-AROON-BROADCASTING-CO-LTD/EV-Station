import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { I18nProvider } from '../i18n/I18nProvider'
import { api } from '../services/api'
import { LoginPage } from './LoginPage'

vi.mock('../services/api', () => ({
  api: {
    login: vi.fn(),
    requestRegistrationOTP: vi.fn(),
    register: vi.fn(),
  },
}))

const mockedAPI = api as unknown as {
  login: ReturnType<typeof vi.fn>
  requestRegistrationOTP: ReturnType<typeof vi.fn>
  register: ReturnType<typeof vi.fn>
}

function renderLogin(onAuthenticated = vi.fn()) {
  render(<MemoryRouter><I18nProvider><LoginPage onAuthenticated={onAuthenticated} /></I18nProvider></MemoryRouter>)
  return onAuthenticated
}

describe('LoginPage', () => {
  beforeEach(() => {
    localStorage.clear()
    localStorage.setItem('rbc-language', 'en')
    vi.clearAllMocks()
  })

  afterEach(cleanup)

  it('shows an error instead of authenticating when login fails', async () => {
    mockedAPI.login.mockRejectedValueOnce(new Error('invalid credentials'))
    const onAuthenticated = renderLogin()

    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'sales@example.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect((await screen.findByRole('alert')).textContent).toContain('Email or password is incorrect.')
    expect(onAuthenticated).not.toHaveBeenCalled()
  })

  it('requires OTP before registering and authenticates only after registration succeeds', async () => {
    mockedAPI.requestRegistrationOTP.mockResolvedValueOnce({ sent: true })
    mockedAPI.register.mockResolvedValueOnce({ id: 'customer-1' })
    const session = { user: { id: 'customer-1', displayName: 'Customer', role: 'customer' }, token: 'session-token' }
    mockedAPI.login.mockResolvedValueOnce(session)
    const onAuthenticated = renderLogin()

    fireEvent.click(screen.getByRole('tab', { name: 'Sign up' }))
    fireEvent.change(screen.getByLabelText('Display name'), { target: { value: 'Customer' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'customer@example.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send verification code' }))

    await waitFor(() => expect(mockedAPI.requestRegistrationOTP).toHaveBeenCalledWith('customer@example.com'))
    expect(screen.getByLabelText('Verification code')).toBeTruthy()
    expect(mockedAPI.register).not.toHaveBeenCalled()

    fireEvent.change(screen.getByLabelText('Verification code'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create account and sign in' }))

    await waitFor(() => expect(mockedAPI.register).toHaveBeenCalledWith('customer@example.com', 'Customer', 'password123', '123456'))
    expect(mockedAPI.login).toHaveBeenCalledWith('customer@example.com', 'password123')
    expect(onAuthenticated).toHaveBeenCalledWith(session)
  })
})
