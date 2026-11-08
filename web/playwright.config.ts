import { defineConfig } from '@playwright/test';
import os from 'node:os';
import path from 'node:path';
import { OIDC_ALLOWED_EMAIL, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_ISSUER, OIDC_SERVER_PORT, OIDC_SERVER_URL } from './e2e/oidcProvider';

/**
 * End-to-end tests run the real Go server (built from ../cmd/go-red) with a
 * fresh data directory, serving the production build from ./dist. Run
 * `npm run build` first, or `npm run e2e:full`.
 *
 * A second instance on OIDC_SERVER_PORT runs with OIDC sign-in against the
 * fake provider that e2e/oidc.spec.ts starts (discovery is lazy, so the
 * provider does not have to be up before the server).
 */

const port = Number(process.env.GO_RED_E2E_PORT || 8081);
const dataDir = process.env.GO_RED_E2E_DATA || path.join(os.tmpdir(), `go-red-e2e-${process.pid}`);
const oidcDataDir = path.join(os.tmpdir(), `go-red-e2e-oidc-${process.pid}`);
const oidcFlags = [
  `-oidc-issuer ${OIDC_ISSUER}`,
  `-oidc-client-id ${OIDC_CLIENT_ID}`,
  `-oidc-redirect-url ${OIDC_SERVER_URL}/auth/callback`,
  `-oidc-allowed-emails ${OIDC_ALLOWED_EMAIL}`,
  '-session-secret e2e-session-secret-0123456789abcdef',
  '-rate-limit 0',
].join(' ');

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
  webServer: [
    {
      command: `cd .. && go build -o bin/go-red-e2e ./cmd/go-red && bin/go-red-e2e -port ${port} -data-dir "${dataDir}" -log-level warn`,
      url: `http://localhost:${port}/api/health`,
      reuseExistingServer: false,
      timeout: 240_000,
    },
    {
      // Its own binary, so the two builds never write the same file.
      command: `cd .. && go build -o bin/go-red-e2e-oidc ./cmd/go-red && bin/go-red-e2e-oidc -port ${OIDC_SERVER_PORT} -data-dir "${oidcDataDir}" -log-level warn ${oidcFlags}`,
      env: { GORED_OIDC_CLIENT_SECRET: OIDC_CLIENT_SECRET },
      url: `${OIDC_SERVER_URL}/api/health`,
      reuseExistingServer: false,
      timeout: 240_000,
    },
  ],
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
});
