import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

const password = 'E2e-password-123!'
const nonce = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
const owner = { email: `owner-${nonce}@e2e.local`, name: 'E2E Owner' }
const sales = { email: `sales-${nonce}@e2e.local`, name: 'E2E Sales' }
const admin = { email: `admin-${nonce}@e2e.local`, name: 'E2E Admin' }
const customer = { email: `customer-${nonce}@e2e.local`, name: 'E2E Customer' }

async function otpFor(request: APIRequestContext, email: string) {
  await expect.poll(async () => {
    const response = await request.get('http://127.0.0.1:18025/api/v2/messages')
    const body = await response.json()
    return body.items?.find((message: { To?: Array<{ Mailbox?: string; Domain?: string }> }) => message.To?.some(recipient => `${recipient.Mailbox}@${recipient.Domain}`.toLowerCase() === email))
  }).not.toBeUndefined()
  const response = await request.get('http://127.0.0.1:18025/api/v2/messages')
  const body = await response.json()
  const message = body.items.find((candidate: { To?: Array<{ Mailbox?: string; Domain?: string }> }) => candidate.To?.some(recipient => `${recipient.Mailbox}@${recipient.Domain}`.toLowerCase() === email))
  const match = String(message?.Content?.Body || '').match(/\b(\d{6})\b/)
  expect(match, 'registration email must contain a six-digit OTP').toBeTruthy()
  return match![1]
}

async function signUp(page: Page, request: APIRequestContext, account: { email: string; name: string }) {
  await page.goto('/')
  await page.getByRole('tab', { name: 'สมัครสมาชิก' }).click()
  await page.getByLabel('ชื่อที่แสดง').fill(account.name)
  await page.getByLabel('อีเมล').fill(account.email)
  await page.getByLabel('รหัสผ่าน').fill(password)
  await page.getByRole('button', { name: 'ส่งรหัสยืนยัน' }).click()
  await page.getByLabel('รหัสยืนยัน 6 หลัก').fill(await otpFor(request, account.email))
  await page.getByRole('button', { name: 'สร้างบัญชีและเข้าสู่ระบบ' }).click()
  await expect(page.getByRole('main')).toContainText('RBC')
}

async function signIn(page: Page, account: { email: string }) {
  await page.goto('/')
  await page.getByLabel('อีเมล').fill(account.email)
  await page.getByLabel('รหัสผ่าน').fill(password)
  await page.getByRole('button', { name: 'เข้าสู่ระบบ' }).click()
  await expect(page.getByRole('button', { name: 'ออกจากระบบ' })).toBeVisible()
}

test.describe.serial('isolated end-to-end customer-site workflow', () => {
  test('super admin signs up and creates sales/admin team accounts', async ({ page, request }) => {
    await signUp(page, request, owner)
    await page.getByRole('link', { name: 'ตั้งค่า' }).click()
    await expect(page.getByRole('heading', { name: 'สร้างบัญชีทีมงาน' })).toBeVisible()
    for (const account of [sales, admin]) {
      await page.getByLabel('ชื่อที่แสดง').last().fill(account.name)
      await page.getByLabel('อีเมล').last().fill(account.email)
      await page.getByLabel('รหัสผ่าน').last().fill(password)
      await page.getByLabel('บทบาท').last().selectOption(account === sales ? 'sales' : 'admin')
      await page.getByRole('button', { name: 'สร้างผู้ใช้' }).click()
      await expect(page.getByText(account.email)).toBeVisible()
    }
  })

  test('sales creates a site, uploads an image/PDF, edits it, and runs analysis', async ({ page }) => {
    const pageErrors: string[] = []
    const browserErrors: string[] = []
    page.on('pageerror', error => pageErrors.push(error.message))
    page.on('console', message => { if (message.type() === 'error') browserErrors.push(message.text()) })
    await signIn(page, sales)
    await page.getByRole('link', { name: 'ส่งข้อมูลพื้นที่' }).click()
    await page.getByLabel('ชื่อโครงการหรือสถานที่ตั้ง *').fill('E2E Station Candidate')
    await page.getByLabel('ละติจูด').fill('13.7563')
    await page.getByLabel('ลองจิจูด').fill('100.5018')
    await page.getByLabel('ขนาดพื้นที่ *').fill('400')
    await page.getByLabel('หน่วยพื้นที่ *').selectOption('sqwah')
    await page.getByLabel('หน้ากว้างทางเข้า (เมตร)').fill('12')
    const fileInputs = page.locator('input[type=file]')
    await fileInputs.nth(0).setInputFiles({ name: 'site.png', mimeType: 'image/png', buffer: Buffer.from('89504e470d0a1a0a', 'hex') })
    await fileInputs.nth(1).setInputFiles({ name: 'deed.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4\n% E2E\n') })
    await page.getByRole('button', { name: 'บันทึกพื้นที่' }).click()
    await expect(page.getByRole('heading', { name: 'E2E Station Candidate' })).toBeVisible()
    await expect(page.getByText('2 ไฟล์')).toBeVisible()
    await page.getByRole('link', { name: 'แดชบอร์ด' }).click()
    await page.getByText('E2E Station Candidate', { exact: true }).first().click()
    await page.getByRole('link', { name: 'แก้ไข' }).click()
    await page.getByLabel('ชื่อโครงการหรือสถานที่ตั้ง *').fill('E2E Station Updated')
    await page.getByRole('button', { name: 'บันทึกการแก้ไข' }).click()
    await expect(page.getByRole('heading', { name: 'E2E Station Updated' })).toBeVisible()
    await page.getByRole('button', { name: 'เริ่มวิเคราะห์' }).click()
    await expect(page).toHaveURL(/\/analysis\//)
    await expect.poll(() => pageErrors, { message: 'analysis page must not throw a browser runtime error' }).toEqual([])
    await expect.poll(() => browserErrors, { message: 'analysis page must not log browser errors' }).toEqual([])
    await expect(page.getByRole('button', { name: 'ดาวน์โหลด PDF' })).toBeVisible({ timeout: 30_000 })
  })

  test('admin and customer retain their intended UI permissions', async ({ browser, request }) => {
    const adminContext = await browser.newContext()
    const adminPage = await adminContext.newPage()
    await signIn(adminPage, admin)
    await expect(adminPage.getByRole('link', { name: 'การใช้งาน API' })).toBeVisible()
    await expect(adminPage.getByText('สร้างบัญชีทีมงาน')).toHaveCount(0)
    await adminContext.close()

    const customerPage = await browser.newPage()
    await signUp(customerPage, request, customer)
    await expect(customerPage.getByRole('link', { name: 'การใช้งาน API' })).toHaveCount(0)
    await expect(customerPage.getByRole('link', { name: 'พื้นที่ที่ยังไม่ลงทุน' })).toHaveCount(0)
    await customerPage.goto('/sites/new')
    await expect(customerPage.getByRole('heading', { name: 'เพิ่มพื้นที่ใหม่' })).toBeVisible()
    await expect(customerPage.getByLabel('ละติจูด')).toHaveCount(0)
    await expect(customerPage.getByLabel('ลองจิจูด')).toHaveCount(0)
  })
})

test('mobile login view renders the primary account controls', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'mobile', 'mobile-only smoke coverage')
  await page.goto('/')
  await expect(page.getByRole('tablist', { name: 'การเข้าใช้งานบัญชี' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'เข้าสู่ระบบ' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'สมัครสมาชิก' })).toBeVisible()
})
