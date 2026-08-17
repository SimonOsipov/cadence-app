// Every asset the built CSS points at has to be in the build, and the fonts have to be among them.
//
// Measured on Vite 8: an asset it cannot resolve is a *warning* — "didn't resolve at build time, it
// will remain unchanged to be resolved at runtime" — and the build exits 0. For a font that ships a
// dashboard which 404s and falls back to a system face, in a product whose display face was chosen
// for Cyrillic. It does not travel Rollup's warning channel either, so `rollupOptions.onwarn` never
// sees it; that was tried, measured useless, and removed rather than left looking like a guard.
//
// So the output is read instead of the log. This asserts the result and not a sentence, which is the
// difference between a check and a string match on somebody else's wording.
import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

export type Verdict = { ok: boolean; problems: string[] }

/**
 * inspect reads a built `dist` and says whether its stylesheets are whole.
 *
 * Exported and taken as a path so it can be run against a fixture: this is the shape of check that
 * gets written once, measured by hand once, and then guards nothing when the thing it parses changes.
 */
export function inspect(dist: string, required: string[]): Verdict {
  const problems: string[] = []

  const assets = join(dist, 'assets')
  const stylesheets = existsSync(assets)
    ? readdirSync(assets).filter((name) => name.endsWith('.css'))
    : []

  if (stylesheets.length === 0) {
    return { ok: false, problems: ['the build emitted no stylesheet, so this check measured nothing'] }
  }

  const resolved: string[] = []

  for (const sheet of stylesheets) {
    const css = readFileSync(join(assets, sheet), 'utf8')

    // Counted as a failure rather than skipped: a stylesheet reaching over the network is the thing
    // linking the fonts locally exists to prevent, and minified Vite output writes a remote import as
    // `@import "https://…"` with no url() wrapper — so the sweep is over the whole text, not over
    // url() alone.
    for (const [remote] of css.matchAll(/https?:\/\/[^"')\s]+/g)) {
      problems.push(`${sheet} reaches ${remote} over the network`)
    }

    for (const [, reference] of css.matchAll(/url\(["']?([^"')]+)["']?\)/g)) {
      if (!reference || reference.startsWith('data:')) continue

      // Vite rewrites a resolved asset to /assets/<name>-<hash>.<ext>; anything still relative is a
      // reference it could not resolve.
      const path = reference.startsWith('/')
        ? join(dist, reference.slice(1))
        : join(assets, reference)

      if (existsSync(path)) resolved.push(reference)
      else problems.push(`${sheet} points at ${reference}, which the build does not contain`)
    }
  }

  // The positive, and it is the half that was missing: counting no failures is what a build shipping
  // no fonts at all looks like, and it announced success. What has to be true is that these four
  // arrived, not merely that nothing broke.
  for (const face of required) {
    if (!resolved.some((reference) => reference.includes(face))) {
      problems.push(`no stylesheet in the build points at ${face}`)
    }
  }

  return { ok: problems.length === 0, problems }
}

/** The faces the dashboard cannot render without. Named here so the check can fail for their absence. */
export const REQUIRED_FACES = ['CormorantGaramond', 'CormorantGaramondItalic', 'GolosText', 'JetBrainsMono']
