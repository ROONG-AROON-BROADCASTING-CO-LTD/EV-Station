import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'

test('registers via SMTP OTP after a wrong code and resend', async ({ page, request }) => {
  const email = `otp-${randomUUID()}@e2e.local`
  const password = 'E2e-password-123!'
  const auth = 'http://127.0.0.1:18080/api/v1/auth'
  const codes = async () => {
    const response = await request.get('http://127.0.0.1:18025/api/v2/search', {
      params: { kind: 'to', query: email },
    })
    expect(response.ok()).toBeTruthy()
    const mailbox: { items: Array<{ Created: string; Content: { Body: string } }> } = await response.json()
    return mailbox.items
      .sort((left, right) => right.Created.localeCompare(left.Created))
      .map(message => message.Content.Body.match(/Your verification code is (\d{6})/)?.[1])
      .filter((code): code is string => code !== undefined)
  }

  await page.clock.install()
  await page.goto('/login?mode=register&lang=en')
  await page.getByLabel('Display name', { exact: true }).fill('QA Registration')
  await page.getByLabel('Email', { exact: true }).fill(email)
  await page.getByLabel('Password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Send verification code', exact: true }).click()
  await expect(page.getByText('Verification code sent to')).toBeVisible()
  await expect.poll(async () => (await codes()).length).toBe(1)

  const [originalCode] = await codes()
  await page.getByLabel('Verification code', { exact: true }).fill(originalCode === '123456' ? '654321' : '123456')
  await page.getByRole('button', { name: 'Create account and sign in' }).click()
  await expect(page.getByRole('alert')).toHaveText('The verification code is invalid or expired. Request a new code and try again.')

  const resend = page.getByRole('button', { name: 'Resend verification code' })
  await expect(resend).toBeDisabled()
  await page.clock.fastForward(61_000)
  await expect(resend).toBeEnabled()
  await resend.click()
  await expect.poll(async () => (await codes()).length).toBe(2)
  const [newCode] = await codes()
  await page.getByLabel('Verification code', { exact: true }).fill(newCode)
  await page.getByRole('button', { name: 'Create account and sign in' }).click()
  await expect(page).toHaveURL(/\/dashboard$/)

  const reuse = await request.post(`${auth}/register`, {
    data: { email, displayName: 'QA Registration', password, otp: newCode },
  })
  expect(reuse.status()).toBe(400)
  expect((await reuse.json()).error.code).toBe('OTP_INVALID')
})
