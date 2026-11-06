import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '../i18n/I18nProvider'
import { SettingsPage } from '../pages/SettingsPage'
import { api } from '../services/api'
import type { LineNotificationStatus } from '../types/domain'
import { LineNotificationSettings } from './LineNotificationSettings'

const empty: LineNotificationStatus = { tokenConfigured: true, webhookConfigured: true, recipientConfigured: false, recipientSource: '', recipientType: '', maskedRecipient: '', canPair: true }
const linked: LineNotificationStatus = { ...empty, recipientConfigured: true, recipientSource: 'paired', recipientType: 'group', maskedRecipient: 'C012…cdef' }
let current: LineNotificationStatus
function show(language: 'th' | 'en' = 'en', role?: 'customer' | 'admin' | 'super_admin') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><I18nProvider initialLanguage={language}>{role ? <SettingsPage role={role} /> : <LineNotificationSettings />}</I18nProvider></QueryClientProvider>)
}
beforeEach(() => {
  current = { ...empty }
  vi.spyOn(api, 'getLineNotificationStatus').mockImplementation(async () => ({ ...current }))
  vi.spyOn(api, 'startLineNotificationPairing').mockImplementation(async () => { const expiresAt = new Date(Date.now() + 600000).toISOString(); current = { ...current, pairingExpiresAt: expiresAt }; return { command: 'RBC-LINK synthetic-one-time-command', expiresAt } })
  vi.spyOn(api, 'cancelLineNotificationPairing').mockImplementation(async () => { current = { ...empty }; return { cancelled: true } })
  vi.spyOn(api, 'testLineNotification').mockResolvedValue({ accepted: true })
  vi.spyOn(api, 'listUsers').mockResolvedValue([])
})
afterEach(() => { cleanup(); vi.restoreAllMocks() })

describe('owner LINE notification setup', () => {
  it.each(['th', 'en'] as const)('explains missing recipient without sending anything in %s', async language => {
    show(language)
    await screen.findByText(language === 'th' ? 'ยังไม่พร้อมแจ้งเตือนเจ้าหน้าที่' : 'Staff notifications are not ready')
    expect(screen.queryByRole('button', { name: /Send LINE test|ส่งข้อความทดสอบ LINE/ })).toBeNull()
    expect(api.startLineNotificationPairing).not.toHaveBeenCalled()
    expect(api.testLineNotification).not.toHaveBeenCalled()
  })
  it('generates a time-limited command only on owner action and can cancel it', async () => {
    show()
    fireEvent.click(await screen.findByRole('button', { name: 'Generate LINE pairing command' }))
    const command = await screen.findByRole('textbox', { name: 'LINE pairing command' })
    expect((command as HTMLInputElement).value).toBe('RBC-LINK synthetic-one-time-command')
    expect(screen.getByText(/Expires in/)).toBeDefined()
    expect(api.startLineNotificationPairing).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel pairing command' }))
    await waitFor(() => expect(screen.queryByRole('textbox', { name: 'LINE pairing command' })).toBeNull())
    expect(api.cancelLineNotificationPairing).toHaveBeenCalledTimes(1)
    expect(api.testLineNotification).not.toHaveBeenCalled()
  })
  it('keeps test delivery explicit and reports LINE acceptance rather than guaranteed receipt', async () => {
    current = { ...linked }
    show()
    const button = await screen.findByRole('button', { name: 'Send LINE test notification' })
    expect(api.testLineNotification).not.toHaveBeenCalled()
    fireEvent.click(button)
    await screen.findByText(/LINE accepted the test notification/)
    expect(api.testLineNotification).toHaveBeenCalledTimes(1)
  })
  it('shows failed test delivery with actionable checks', async () => {
    current = { ...linked }
    vi.mocked(api.testLineNotification).mockRejectedValue(new Error('upstream 401'))
    show()
    fireEvent.click(await screen.findByRole('button', { name: 'Send LINE test notification' }))
    await screen.findByText(/Test delivery failed. Check bot membership/)
    expect(screen.queryByText(/LINE accepted/)).toBeNull()
  })
  it('does not offer pairing if channel secrets have not been configured', async () => {
    current = { ...empty, tokenConfigured: false, webhookConfigured: false, canPair: false }
    show()
    await screen.findByText(/A server administrator must configure/)
    expect(screen.queryByRole('button', { name: 'Generate LINE pairing command' })).toBeNull()
  })
  it('preserves server-managed recipient instead of offering an override', async () => {
    current = { ...linked, recipientSource: 'environment', canPair: false }
    show()
    await screen.findByText(/This recipient is managed by server configuration/)
    expect(screen.queryByRole('button', { name: 'Generate LINE pairing command' })).toBeNull()
  })
  it.each(['customer', 'admin'] as const)('does not expose setup to role %s', role => {
    show('en', role)
    expect(screen.queryByRole('region', { name: 'Staff LINE notifications' })).toBeNull()
    expect(api.getLineNotificationStatus).not.toHaveBeenCalled()
  })
  it('exposes owner settings with masked recipient only', async () => {
    current = { ...linked }
    show('en', 'super_admin')
    await screen.findByRole('region', { name: 'Staff LINE notifications' })
    await screen.findByText(/Recipient linked · group C012…cdef/)
  })
})
