import { defineConfig } from '@playwright/test';
import os from 'node:os';
import path from 'node:path';

/**
 * End-to-end tests run the real Go server (built from ../cmd/go-red) with a
 * fresh data directory, serving the production build from ./dist. Run
 * `npm run build` first, or `npm run e2e:full`.
 */

const port = Number(process.env.GO_RED_E2E_PORT || 8081);
const dataDir = process.env.GO_RED_E2E_DATA || path.join(os.tmpdir(), `go-red-e2e-${process.pid}`);

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL: `http://localhost:${port}`,
    trace: 'retain-on-failure',
    viewport: { width: 1440, height: 900 },
  },
  webServer: {
    command: `cd .. && go build -o bin/go-red-e2e ./cmd/go-red && bin/go-red-e2e -port ${port} -data-dir "${dataDir}" -web-dir web/dist -log-level warn`,
    url: `http://localhost:${port}/api/health`,
    reuseExistingServer: false,
    timeout: 240_000,
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
});
