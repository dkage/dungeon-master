import { defineConfig } from '@playwright/test'

const port = process.env.E2E_PORT ?? '7899'

export default defineConfig({
  testDir: 'e2e',
  timeout: 30_000,
  workers: 1,
  fullyParallel: false,
  reporter: 'list',
  use: { baseURL: `http://127.0.0.1:${port}`, trace: 'retain-on-failure' },
  // Needs bin/dungeon: run `make build` first (or use `make test-e2e`).
  webServer: {
    command: 'node e2e/serve.mjs',
    url: `http://127.0.0.1:${port}/`,
    reuseExistingServer: false,
    timeout: 30_000,
    env: { E2E_PORT: port },
  },
})
