import { expect, test } from '@playwright/test'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

test('serves useful public HTML even without JavaScript, and noindex for private URLs', async ({ browser, request }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, baseURL: 'http://127.0.0.1:15273' })
  const page = await context.newPage()
  await page.goto('/investment?lang=en')
  await expect(page).toHaveTitle(/AC\/DC Charging Solutions/)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Invest with evidence, not guesswork')
  await expect(page.getByText('DC 120 kW', { exact: false }).first()).toBeAttached()
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute('href', 'https://www.rbcevstation.com/investment?lang=en')
  for (const url of ['/login', '/analysis/customer-test', '/sites/customer-test/documents', '/guide/admin']) {
    const response = await request.get(url)
    expect(response.status()).toBe(200)
    expect(response.headers()['x-robots-tag']).toBe('noindex, nofollow')
    expect(await response.text()).toContain('content="noindex, nofollow"')
  }
  const missing = await request.get('/missing-page', { headers: { accept: 'text/html' } })
  expect(missing.status()).toBe(404)
  await context.close()
})

test('keeps English URLs and metadata across reload, navigation, language switch and back', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto('/?lang=en', { waitUntil: 'domcontentloaded' })
  await expect(page).toHaveTitle(/EV Charging Station Investment/)
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
  // Header navigation is hidden on mobile; the language links remain visible.
  if (test.info().project.name === 'mobile') await page.getByRole('button', { name: 'Open menu', exact: true }).click()
  await page.getByRole('link', { name: 'Invest in EV', exact: true }).filter({ visible: true }).first().click()
  await expect(page).toHaveURL(/\/investment\?lang=en/)
  await expect(page).toHaveTitle(/AC\/DC Charging Solutions/)
  await page.reload({ waitUntil: 'domcontentloaded' })
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
  await page.evaluate(() => window.scrollTo(0, 0))
  await expect(page.locator('h1')).toHaveCount(1)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await page.screenshot({ path: join(tmpdir(), `rbc-seo-${test.info().project.name}.png`) })
  if (test.info().project.name === 'mobile') await page.getByRole('button', { name: 'Open menu', exact: true }).click()
  await page.getByRole('button', { name: 'ไทย', exact: true }).filter({ visible: true }).first().click()
  await expect(page).toHaveURL(/\/investment$/)
  await expect(page).toHaveTitle(/เครื่องชาร์จ AC\/DC/)
  await page.goBack({ waitUntil: 'domcontentloaded' })
  await expect(page).toHaveTitle(/EV Charging Station Investment/)
  expect(errors).toEqual([])
})
