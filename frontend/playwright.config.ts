import { defineConfig, devices } from '@playwright/test';

/**
 * Cross-browser / cross-device test matrix for the Chess board component.
 * Run: npx playwright test
 * Report: npx playwright show-report
 */
export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  retries: process.env.CI ? 2 : 1,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { open: 'never' }], ['list']],
  timeout: 30_000,

  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:5174',
    trace: 'on-first-retry',
    video: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    // ── Desktop browsers ────────────────────────────────────────────
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },

    // ── Mobile — Android (Chromium engine, touch emulation) ─────────
    {
      name: 'android-pixel5',
      use: { ...devices['Pixel 5'] },
    },
    {
      name: 'android-galaxy-s9',
      use: { ...devices['Galaxy S9+'] },
    },

    // ── Mobile — iOS (WebKit engine, touch emulation) ───────────────
    {
      name: 'iphone-12',
      use: { ...devices['iPhone 12'] },
    },
    {
      name: 'ipad-pro',
      use: { ...devices['iPad Pro 11'] },
    },
  ],

  // Start the SvelteKit dev server before running tests
  webServer: {
    command: 'npm run dev -- --port 5174',
    url: process.env.BASE_URL ?? 'http://localhost:5174',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
