import { defineConfig, devices } from '@playwright/test'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

export default defineConfig({
  testDir: './e2e',
  testMatch: 'seo.spec.ts',
  outputDir: join(tmpdir(), 'rbc-seo-qa'),
  workers: 1,
  use: { baseURL: 'http://127.0.0.1:15273', screenshot: 'only-on-failure' },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
  webServer: { command: 'npx vite preview --host 127.0.0.1 --port 15273 --strictPort', url: 'http://127.0.0.1:15273' },
})
