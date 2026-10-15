import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import { readFileSync, mkdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const root = 'http://127.0.0.1:18080/api/v1'
const password = 'E2e-password-123!'
const artifacts = join(tmpdir(), 'rbc-functional-qa')
mkdirSync(artifacts, { recursive: true })
const photo = readFileSync(join(process.cwd(), 'public/rbc-group-logo.png'))
// Valid one-page PDF: objects, xref and trailer, rather than a PDF signature only.
function fixturePDF(label = 'RBC QA site plan') {
  let pdf = '%PDF-1.4\n'
  const stream = `BT /F1 18 Tf 50 750 Td (${label}) Tj ET\n`
  const objects = ['<< /Type /Catalog /Pages 2 0 R >>', '<< /Type /Pages /Kids [3 0 R] /Count 1 >>', '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>', '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>', `<< /Length ${stream.length} >>\nstream\n${stream}endstream`]
  const offsets = [0]
  objects.forEach((object, i) => { offsets.push(Buffer.byteLength(pdf)); pdf += `${i + 1} 0 obj\n${object}\nendobj\n` })
  const xref = Buffer.byteLength(pdf)
  pdf += `xref\n0 6\n0000000000 65535 f \n${offsets.slice(1).map(offset => `${String(offset).padStart(10, '0')} 00000 n \n`).join('')}trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`
  return Buffer.from(pdf)
}
const files = [
  { name: 'site.png', mimeType: 'image/png', buffer: photo },
  { name: 'plan.pdf', mimeType: 'application/pdf', buffer: fixturePDF() },
]
async function loginAPI(request: APIRequestContext, email = 'owner@e2e.local') {
  const response = await request.post(`${root}/auth/login`, { data: { email, password } })
  expect(response.ok()).toBeTruthy()
  return (await response.json()).data
}
async function signIn(page: Page, email = 'owner@e2e.local') {
  await page.goto('/login')
  await page.getByLabel('Email', { exact: true }).fill(email)
  await page.getByLabel('Password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}
async function fillSite(page: Page, name: string) {
  await page.goto('/sites/new')
  await page.getByLabel('Project or location name *').fill(name)
  await page.getByLabel('Latitude', { exact: true }).fill('13.7563')
  await page.getByLabel('Longitude', { exact: true }).fill('100.5018')
  await page.getByLabel('Land size *', { exact: true }).fill('400')
}
async function attach(page: Page) {
  await page.locator('input[type=file]').nth(0).setInputFiles(files[0])
  await page.locator('input[type=file]').nth(1).setInputFiles(files[1])
}
async function noOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBeTruthy()
}
async function completed(request: APIRequestContext, headers: Record<string, string>, siteId: string) {
  const response = await request.post(`${root}/sites/${siteId}/analyses`, { headers, data: { radiusMeters: 3000 } })
  expect(response.ok()).toBeTruthy()
  let run = (await response.json()).data
  await expect.poll(async () => {
    run = (await (await request.get(`${root}/analyses/${run.id}`, { headers })).json()).data
    return run.status
  }, { timeout: 45_000 }).toBe('completed')
  return run
}
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => { if (!localStorage.getItem('rbc-language')) localStorage.setItem('rbc-language', 'en') })
})

test('ten valid documents survive reload and retries at the file quota', async ({ page, request }) => {
  await signIn(page)
  await fillSite(page, 'QA ten documents ' + Date.now())
  const documents = Array.from({ length: 10 }, (_, i) => ({ name: `plan-${i}.pdf`, mimeType: 'application/pdf', buffer: fixturePDF(`RBC QA plan ${i}`) }))
  await page.locator('input[type=file]').nth(1).setInputFiles(documents)
  // Selecting the same group replaces the selection; it must not count twice.
  await page.locator('input[type=file]').nth(1).setInputFiles(documents)
  await page.getByRole('button', { name: 'Save Site', exact: true }).click()
  await expect(page).toHaveURL(/\/sites\/[0-9a-f-]+$/)
  await expect(page.getByText('10 files', { exact: true })).toBeVisible()
  const siteId = new URL(page.url()).pathname.split('/').pop()
  const session = await loginAPI(request)
  const headers = { Authorization: `Bearer ${session.token}` }
  const replay = await request.post(`${root}/sites/${siteId}/images`, { headers, multipart: { images: { name: documents[0].name, mimeType: 'application/pdf', buffer: documents[0].buffer } } })
  expect(replay.ok()).toBeTruthy()
  const overQuota = await request.post(`${root}/sites/${siteId}/images`, { headers, multipart: { images: { name: 'extra.pdf', mimeType: 'application/pdf', buffer: fixturePDF('Extra document') } } })
  expect(overQuota.status()).toBe(400)
  await page.reload()
  await expect(page.getByText('10 files', { exact: true })).toBeVisible()
  await noOverflow(page)
})

test('owner uploads valid files, edits, analyzes and reopens a saved report', async ({ page, request }, info) => {
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  await signIn(page)
  const name = `QA ${info.project.name} ${Date.now()}`
  await fillSite(page, name)
  await attach(page)
  await page.getByRole('button', { name: 'Save Site', exact: true }).click()
  await expect(page).toHaveURL(/\/sites\/[0-9a-f-]+$/)
  await expect(page.getByText('2 files', { exact: true })).toBeVisible()
  const siteURL = page.url()
  await page.reload()
  await expect(page.getByRole('img', { name: 'Site photo' })).toBeVisible()
  expect(await page.getByRole('img', { name: 'Site photo' }).evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
  await page.goto(`${siteURL}/edit`)
  await page.getByLabel('Project or location name *').fill(`${name} edited`)
  await page.reload()
  await expect(page.getByLabel('Project or location name *')).toHaveValue(`${name} edited`)
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByRole('heading', { name: `${name} edited` })).toBeVisible()
  await page.getByRole('button', { name: 'Run Analysis', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeVisible({ timeout: 45_000 })
  await page.reload()
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeVisible()
  await noOverflow(page)
  await page.screenshot({ path: join(artifacts, `${info.project.name}-analysis.png`), fullPage: true })
  expect(errors).toEqual([])
  if (info.project.name === 'desktop') {
    await page.evaluate(() => { window.print = () => { document.body.dataset.printInvoked = 'yes' } })
    await page.getByRole('button', { name: 'Download PDF', exact: true }).click()
    expect(await page.locator('body').getAttribute('data-print-invoked')).toBe('yes')
    await page.evaluate(() => document.fonts.ready)
    await page.pdf({ path: join(artifacts, 'report-missing-data.pdf'), printBackground: true, preferCSSPageSize: true })
    // Synthetic assumptions exercise pagination; these are not assessment data.
    const session = await loginAPI(request)
    const runId = new URL(page.url()).pathname.split('/').pop()
    const response = await request.get(`${root}/analyses/${runId}`, { headers: { Authorization: `Bearer ${session.token}` } })
    const body = await response.json()
    for (const metric of body.data.metrics) metric.assumptions = [Array.from({ length: 12 }, (_, i) => `QA pagination paragraph ${i}: ${'Evidence requires field verification. '.repeat(8)}`).join(' ') + ' QA-END-OF-LONG-REPORT']
    await page.route(`**/api/v1/analyses/${runId}`, route => route.fulfill({ json: body }))
    await page.reload()
    await expect(page.getByText('QA-END-OF-LONG-REPORT', { exact: false }).first()).toBeVisible()
    await page.evaluate(() => document.fonts.ready)
    await page.pdf({ path: join(artifacts, 'report-long.pdf'), printBackground: true, preferCSSPageSize: true })
  }
})

for (const failure of ['create-response-lost', 'upload-response-lost', 'upload-rejected'] as const) {
  test(`retry ${failure} produces one site and one copy of each file`, async ({ page, request }) => {
    await signIn(page)
    const name = `QA ${failure} ${Date.now()}`
    await fillSite(page, name)
    await attach(page)
    let failed = false
    await page.route('**/api/v1/sites**', async route => {
      const path = new URL(route.request().url()).pathname
      const target = failure === 'create-response-lost' ? path === '/api/v1/sites' : /\/sites\/[^/]+\/images$/.test(path)
      if (failed || !target || route.request().method() !== 'POST') return route.continue()
      failed = true
      if (failure === 'upload-rejected') return route.fulfill({ status: 503, json: { error: { code: 'UPLOAD_FAILED', message: 'QA temporary failure' } } })
      const accepted = await route.fetch()
      expect(accepted.ok()).toBeTruthy()
      await route.abort('connectionreset')
    })
    await page.getByRole('button', { name: 'Save Site', exact: true }).click()
    await expect(page.getByRole('alert').last()).toBeVisible()
    if (failure !== 'upload-rejected') {
      await page.reload()
      await expect(page.getByLabel('Project or location name *')).toHaveValue(name)
      await attach(page)
    }
    await page.getByRole('button', { name: 'Save Site', exact: true }).click()
    await expect(page).toHaveURL(/\/sites\/[0-9a-f-]+$/)
    await expect(page.getByText('2 files', { exact: true })).toBeVisible()
    const session = await loginAPI(request)
    const result = await request.get(`${root}/sites`, { headers: { Authorization: `Bearer ${session.token}` } })
    expect((await result.json()).data.filter((site: { name: string }) => site.name === name)).toHaveLength(1)
  })
}

test('offline, reload and expired sessions preserve the unfinished form', async ({ page, context }) => {
  await signIn(page)
  const name = `QA recovery ${Date.now()}`
  await fillSite(page, name)
  await context.setOffline(true)
  await page.getByRole('button', { name: 'Save Site', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Saving…', exact: true })).toBeDisabled()
  await context.setOffline(false)
  await expect(page).toHaveURL(/\/sites\/[0-9a-f-]+$/)
  await fillSite(page, name + ' draft')
  await page.reload()
  await expect(page.getByLabel('Project or location name *')).toHaveValue(name + ' draft')
  await page.route('**/api/v1/sites', route => route.request().method() === 'POST' ? route.fulfill({ status: 401, json: { error: { code: 'INVALID_TOKEN' } } }) : route.continue())
  await page.getByRole('button', { name: 'Save Site', exact: true }).click()
  await expect(page).toHaveURL(/\/login\?reason=expired/)
  await expect(page.getByRole('status')).toContainText('session expired')
  await page.unroute('**/api/v1/sites')
  await page.getByLabel('Email', { exact: true }).fill('owner@e2e.local')
  await page.getByLabel('Password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/sites\/new$/)
  await expect(page.getByLabel('Project or location name *')).toHaveValue(name + ' draft')
  await page.getByRole('button', { name: 'Save Site', exact: true }).click()
  await expect(page).toHaveURL(/\/sites\/[0-9a-f-]+$/)
})

test('sales and customer access is enforced by the API and reports reopen', async ({ page, request }, info) => {
  const owner = await loginAPI(request)
  const ownerHeaders = { Authorization: `Bearer ${owner.token}` }
  const unique = `${info.project.name}-${Date.now()}`
  const email = `sales-${unique}@e2e.local`
  expect((await request.post(`${root}/users`, { headers: ownerHeaders, data: { email, displayName: 'QA Sales', password, role: 'sales' } })).ok()).toBeTruthy()
  await signIn(page, email)
  await fillSite(page, `QA Sales ${unique}`)
  await attach(page)
  await page.getByRole('button', { name: 'Save Site', exact: true }).click()
  await expect(page).toHaveURL(/\/sites\/[0-9a-f-]+$/)
  const salesSiteId = new URL(page.url()).pathname.split('/').pop()!
  await page.getByRole('button', { name: 'Run Analysis', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeVisible({ timeout: 45_000 })
  await page.reload()
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeVisible()
  const sales = await loginAPI(request, email)
  expect((await request.get(`${root}/users`, { headers: { Authorization: `Bearer ${sales.token}` } })).status()).toBe(403)
  const customerEmail = `customer-${unique}@e2e.local`
  expect((await request.post(`${root}/auth/register/request-otp`, { data: { email: customerEmail } })).ok()).toBeTruthy()
  let otp = ''
  await expect.poll(async () => {
    const mail = await (await request.get('http://127.0.0.1:18025/api/v2/messages')).json()
    const message = mail.items.find((item: { To: { Mailbox: string; Domain: string }[] }) => item.To.some(to => `${to.Mailbox}@${to.Domain}` === customerEmail))
    otp = String(message?.Content?.Body || '').match(/\b\d{6}\b/)?.[0] || ''
    return otp
  }).toMatch(/^\d{6}$/)
  expect((await request.post(`${root}/auth/register`, { data: { email: customerEmail, displayName: 'QA Customer', password, otp } })).ok()).toBeTruthy()
  const customer = await loginAPI(request, customerEmail)
  const headers = { Authorization: `Bearer ${customer.token}` }
  expect((await request.get(`${root}/sites/${salesSiteId}`, { headers })).status()).toBe(403)
  expect((await request.get(`${root}/users`, { headers })).status()).toBe(403)
  const site = await request.post(`${root}/sites`, { headers, data: { name: 'พื้นที่ทดสอบรายงานภาษาไทย ' + 'สถานีชาร์จรถยนต์ไฟฟ้า '.repeat(5), latitude: 13.7563, longitude: 100.5018, landSize: 400, landSizeUnit: 'sqm' } })
  expect(site.ok()).toBeTruthy()
  const siteId = (await site.json()).data.id
  expect((await request.post(`${root}/sites/${siteId}/analyses`, { headers, data: { radiusMeters: 3000 } })).status()).toBe(403)
  const run = await completed(request, ownerHeaders, siteId)
  await page.evaluate(session => { localStorage.setItem('rbc-session', JSON.stringify(session)); localStorage.setItem('rbc-session-token', session.token) }, customer)
  await page.goto(`/analysis/${run.id}`)
  await expect(page.getByRole('button', { name: 'Download PDF', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Run new analysis', exact: true })).toHaveCount(0)
  await noOverflow(page)
  if (info.project.name === 'desktop') {
    await page.evaluate(() => localStorage.setItem('rbc-language', 'th'))
    await page.reload()
    await expect(page.getByRole('button', { name: 'ดาวน์โหลด PDF', exact: true })).toBeVisible()
    await page.evaluate(() => document.fonts.ready)
    await page.pdf({ path: join(artifacts, 'report-customer.pdf'), printBackground: true, preferCSSPageSize: true })
  }
})
