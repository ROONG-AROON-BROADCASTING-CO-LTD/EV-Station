import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 90_000,
  retries: 0,
  use: {
    baseURL: 'http://127.0.0.1:15173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['iPhone 13'] } },
  ],
  webServer: {
    command: 'pnpm dev --host 127.0.0.1 --port 15173',
    url: 'http://127.0.0.1:15173',
    reuseExistingServer: false,
    env: { VITE_API_BASE_URL: 'http://127.0.0.1:18080/api/v1' },
  },
})
