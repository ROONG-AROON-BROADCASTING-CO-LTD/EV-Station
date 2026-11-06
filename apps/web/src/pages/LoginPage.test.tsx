import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { I18nProvider } from '../i18n/I18nProvider'
import { api, APIError } from '../services/api'
import { LoginPage } from './LoginPage'

vi.mock('../services/api', async importOriginal => ({
  ...await importOriginal<typeof import('../services/api')>(),
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
    vi.resetAllMocks()
  })

  afterEach(() => { cleanup(); vi.useRealTimers() })

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

  function beginSignup() {
    renderLogin()
    fireEvent.click(screen.getByRole('tab', { name: 'Sign up' }))
    fireEvent.change(screen.getByLabelText('Display name'), { target: { value: 'Customer' } })
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'customer@example.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'password123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Send verification code' }))
  }

  it('confirms the destination and sent time, counts down, and resends only after cooldown', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    mockedAPI.requestRegistrationOTP.mockResolvedValue({ sent: true })
    beginSignup()
    const confirmation = await screen.findByRole('status')
    expect(confirmation.textContent).toContain('customer@example.com')
    expect(confirmation.textContent).toContain('Sent at')
    expect(confirmation.querySelector('time')?.dateTime).toBeTruthy()
    const resend = screen.getByRole('button', { name: 'Resend verification code' }) as HTMLButtonElement
    expect(resend.disabled).toBe(true)
    expect(screen.getByText(/Send again in 60 seconds/)).toBeDefined()
    fireEvent.change(screen.getByLabelText('Verification code'), { target: { value: '123456' } })
    await act(async () => { vi.advanceTimersByTime(60_000) })
    expect(resend.disabled).toBe(false)
    fireEvent.click(resend)
    await waitFor(() => expect(mockedAPI.requestRegistrationOTP).toHaveBeenCalledTimes(2))
    expect((screen.getByLabelText('Verification code') as HTMLInputElement).value).toBe('')
    expect(screen.getByText(/Send again in 60 seconds/)).toBeDefined()
    expect(mockedAPI.register).not.toHaveBeenCalled()
  })

  it('prevents duplicate OTP requests while delivery is pending', async () => {
    let resolve!: (value: { sent: boolean }) => void
    mockedAPI.requestRegistrationOTP.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    beginSignup()
    fireEvent.submit(screen.getByLabelText('Email').closest('form')!)
    fireEvent.submit(screen.getByLabelText('Email').closest('form')!)
    expect(mockedAPI.requestRegistrationOTP).toHaveBeenCalledTimes(1)
    expect((screen.getByLabelText('Email') as HTMLInputElement).disabled).toBe(true)
    await act(async () => { resolve({ sent: true }) })
    expect((await screen.findByRole('status')).textContent).toContain('customer@example.com')
  })

  it('does not claim a code was sent after a delivery failure', async () => {
    mockedAPI.requestRegistrationOTP.mockRejectedValueOnce(new APIError('OTP_DELIVERY_FAILED', 'delivery failed', 502))
    beginSignup()
    expect((await screen.findByRole('alert')).textContent).toContain('Unable to send verification code. Please try again.')
    expect(screen.queryByRole('status')).toBeNull()
    expect(screen.queryByLabelText('Verification code')).toBeNull()
  })

  it('explains invalid or expired OTP instead of a generic account-creation error', async () => {
    mockedAPI.requestRegistrationOTP.mockResolvedValueOnce({ sent: true })
    mockedAPI.register.mockRejectedValueOnce(new APIError('OTP_INVALID', 'invalid or expired', 400))
    beginSignup()
    fireEvent.change(await screen.findByLabelText('Verification code'), { target: { value: '000000' } })
    fireEvent.click(screen.getByRole('button', { name: 'Create account and sign in' }))
    expect((await screen.findByRole('alert')).textContent).toContain('The verification code is invalid or expired. Request a new code and try again.')
    expect(mockedAPI.login).not.toHaveBeenCalled()
  })

  it('respects the server rate-limit period before allowing another delivery attempt', async () => {
    mockedAPI.requestRegistrationOTP.mockRejectedValueOnce(new APIError('AUTH_RATE_LIMITED', 'rate limited', 429))
    beginSignup()
    expect((await screen.findByRole('alert')).textContent).toContain('Too many attempts. Please wait 15 minutes before trying again.')
    expect(screen.getByText(/Send again in 900 seconds/)).toBeDefined()
    expect((screen.getByRole('button', { name: 'Send verification code' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('requires a new code after changing the email, without using the earlier destination', async () => {
    mockedAPI.requestRegistrationOTP.mockResolvedValueOnce({ sent: true })
    beginSignup()
    await screen.findByLabelText('Verification code')
    fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'changed@example.com' } })
    expect(screen.queryByLabelText('Verification code')).toBeNull()
    expect(screen.queryByRole('status')).toBeNull()
    expect(screen.getByRole('button', { name: 'Send verification code' })).toBeDefined()
    expect(mockedAPI.register).not.toHaveBeenCalled()
  })

  it('shows the sent confirmation and resend control in Thai', async () => {
    localStorage.setItem('rbc-language', 'th')
    mockedAPI.requestRegistrationOTP.mockResolvedValueOnce({ sent: true })
    renderLogin()
    fireEvent.click(screen.getByRole('tab', { name: 'สมัครสมาชิก' }))
    fireEvent.change(screen.getByLabelText('ชื่อที่แสดง'), { target: { value: 'ลูกค้าทดสอบ' } })
    fireEvent.change(screen.getByLabelText('อีเมล'), { target: { value: 'customer@example.com' } })
    fireEvent.change(screen.getByLabelText('รหัสผ่าน'), { target: { value: 'password123' } })
    fireEvent.click(screen.getByRole('button', { name: 'ส่งรหัสยืนยัน' }))
    expect((await screen.findByRole('status')).textContent).toContain('ส่งรหัสยืนยันไปที่ customer@example.com')
    expect(screen.getByRole('button', { name: 'ส่งรหัสยืนยันอีกครั้ง' })).toBeDefined()
    expect(screen.getByText(/ส่งอีกครั้งได้ใน 60 วินาที/)).toBeDefined()
  })
})
