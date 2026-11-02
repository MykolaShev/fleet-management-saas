import { defineConfig } from '@playwright/test';

// QA2: E2E against F5 (this SPA). The Go API + its infra (Postgres,
// Redis) must already be running at localhost:8080 — Playwright only
// starts the frontend's own dev server, it has no business launching a
// Go binary.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  retries: 0,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 30_000,
  },
});