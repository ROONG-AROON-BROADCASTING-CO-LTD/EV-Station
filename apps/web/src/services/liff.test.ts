import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'
import { loadLiffConfig } from './liff'

afterEach(() => vi.restoreAllMocks())
describe('LIFF configuration', () => {
  it('reads through the configured API client', async () => {
    vi.spyOn(api, 'getLiffConfig').mockResolvedValue({ liffId: '2011499417-Test' })
    expect(await loadLiffConfig()).toEqual({ liffId: '2011499417-Test' })
  })
  it.each([undefined, '', 'https://example.invalid', '123-../../bad'])('rejects invalid LIFF id %s before building a link', async liffId => {
    vi.spyOn(api, 'getLiffConfig').mockResolvedValue({ liffId })
    await expect(loadLiffConfig()).rejects.toThrow('error.LIFF_CONFIGURATION')
  })
})
