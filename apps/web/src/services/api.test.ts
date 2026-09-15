import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { APIError, api, errorMessageKey } from './api'

describe('API client authentication boundary', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  afterEach(() => vi.unstubAllGlobals())

  it('sends the persisted bearer token and returns the response payload', async () => {
    localStorage.setItem('rbc-session-token', 'stored-token')
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { user: { id: 'user-1' }, token: 'new-token' } }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    const session = await api.login('sales@example.com', 'password123')

    expect(session.token).toBe('new-token')
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/auth/login'),
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer stored-token', 'Content-Type': 'application/json' }),
      }),
    )
  })

  it('clears a stale session and emits the expiry event when the API rejects the token', async () => {
    localStorage.setItem('rbc-session', JSON.stringify({ token: 'expired-token' }))
    localStorage.setItem('rbc-session-token', 'expired-token')
    const onExpired = vi.fn()
    window.addEventListener('rbc:auth-expired', onExpired)
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: 'INVALID_TOKEN', message: 'expired' } }), { status: 401, headers: { 'Content-Type': 'application/json' } })))

    await expect(api.listSites()).rejects.toMatchObject({ code: 'INVALID_TOKEN', status: 401 })

    expect(localStorage.getItem('rbc-session')).toBeNull()
    expect(localStorage.getItem('rbc-session-token')).toBeNull()
    expect(onExpired).toHaveBeenCalledTimes(1)
    window.removeEventListener('rbc:auth-expired', onExpired)
  })

  it('keeps non-authentication failures mapped to a stable translation key', () => {
    expect(errorMessageKey(new APIError('SITE_ACCESS_DENIED', 'denied', 403))).toBe('error.SITE_ACCESS_DENIED')
    expect(errorMessageKey(new Error('network failed'))).toBe('error.REQUEST_FAILED')
  })
})
