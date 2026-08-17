// Every asset the built CSS points at has to be in the build.
//
// Measured on Vite 8: an asset it cannot resolve is a *warning* — "didn't resolve at build time, it
// will remain unchanged to be resolved at runtime" — and the build exits 0. For a font that ships a
// dashboard which 404s and falls back to a system face, in a product whose display face was chosen
// for Cyrillic. It does not travel Rollup's warning channel either, so `rollupOptions.onwarn` never
// sees it; that was tried and removed rather than left looking like a guard.
//
// So the output is read instead of the log. This asserts the result and not a sentence, which is the
// difference between a check and a string match on somebody else's wording.
import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

const DIST = 'dist'
const stylesheets = readdirSync(join(DIST, 'assets')).filter((name) => name.endsWith('.css'))

if (stylesheets.length === 0) {
  console.error('the build emitted no stylesheet, so this check measured nothing')
  process.exit(1)
}

let dangling = 0

for (const sheet of stylesheets) {
  const css = readFileSync(join(DIST, 'assets', sheet), 'utf8')

  for (const [, reference] of css.matchAll(/url\(["']?([^"')]+)["']?\)/g)) {
    if (reference.startsWith('data:') || reference.startsWith('http')) continue

    // Vite rewrites a resolved asset to /assets/<name>-<hash>.<ext>; anything still relative is a
    // reference it could not resolve.
    const path = reference.startsWith('/')
      ? join(DIST, reference.slice(1))
      : join(DIST, 'assets', reference)

    if (!existsSync(path)) {
      console.error(`${sheet} points at ${reference}, which the build does not contain`)
      dangling += 1
    }
  }
}

if (dangling > 0) {
  process.exit(1)
}

console.log(`every asset referenced by ${stylesheets.length} stylesheet(s) is in the build`)
