import { defineConfig, devices } from '@playwright/test';

/**
 * Browser E2E for the guide — a standalone package (own lockfile) run from
 * the HOST against the EXTERNALLY managed dev stack, never inside a
 * container: the toolchain containers run unprivileged, so Chromium's
 * system libraries cannot be installed there.
 *
 * No `webServer` block on purpose: the suite never boots anything itself.
 * The default base URL is the client dev server (Vite inside
 * go_developers_guide_client_dev, published on :5175), whose /api proxy
 * forwards to the Go server inside the dev container — the same front
 * door a developer's browser uses. E2E_BASE_URL overrides it (e.g. to
 * drive a production-image front door instead).
 */
export default defineConfig({
  testDir: '.',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  // Deterministic by design: a red e2e test is a wiring signal, and
  // re-rolling it throws the information away. Zero retries, one worker —
  // the specs share a single stack and one shared scenario toggle.
  retries: 0,
  workers: 1,
  fullyParallel: false,
  reporter: 'line',
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:5175',
    headless: true,
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
