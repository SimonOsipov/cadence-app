import { defineConfig } from '@playwright/test'

/**
 * The smoke test's own runner, and it is not Vitest: `vite.config.ts` roots Vitest in `src/` so that
 * a spec in here is never collected by it.
 *
 * What this runs against is the local harness — Postgres, GoTrue, the provisioner, the API and the
 * dashboard's dev server, brought up by `make smoke`. It is not part of `scripts/gate/all.sh` and not
 * a CI job: the deployment those would run against is held by SKL-01 and SKL-06, and a job that can
 * only be red is a job whose red says nothing. Written down here rather than implied.
 */
const DASHBOARD = process.env.SMOKE_DASHBOARD_URL ?? 'http://localhost:5173'

export default defineConfig({
  testDir: 'tests',
  // One worker: the test creates a patient in a clinic the other tests read, and two of them racing
  // would be measuring each other.
  workers: 1,
  // Never here: a smoke test that passes on the second attempt is a smoke test that failed.
  retries: 0,
  timeout: 30_000,
  globalTeardown: './tests/teardown.ts',
  use: {
    baseURL: DASHBOARD,
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'npm run dev -- --port 5173 --strictPort',
    url: DASHBOARD,
    reuseExistingServer: true,
    timeout: 60_000,
    env: {
      VITE_API_URL: process.env.SMOKE_API_URL ?? 'http://localhost:8080',
      VITE_AUTH_URL: process.env.SMOKE_AUTH_URL ?? 'http://localhost:9999',
    },
  },
})
