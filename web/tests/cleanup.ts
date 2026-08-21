/**
 * What the smoke test leaves behind, and how it is found again.
 *
 * Every address it creates carries the run that created it, so a teardown can delete exactly what this
 * run made and leave the seeded clinic — and any other run — alone.
 */
export const CLEANUP_PREFIX = 'e2e+'

let id: string | null = null

/** One id per process, so every address a run creates is found by one pattern. */
export function runId(): string {
  id ??= process.env.SMOKE_RUN_ID ?? String(Date.now())

  return id
}
