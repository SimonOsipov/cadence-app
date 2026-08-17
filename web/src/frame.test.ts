import { ESLint } from 'eslint'
import { describe, expect, it } from 'vitest'

import viteConfig from '../vite.config'

// `web/prototype/` is the frozen visual specification: in-browser Babel JSX that never compiles and is
// never shipped.
describe('the frozen prototype', () => {
  // heroicons.js and not one of the .jsx files, and the difference is the whole test: no config object
  // here carries a `files` pattern matching .jsx, and flat config reports a file matched by nothing as
  // ignored. Measured — asked about a .jsx path this passes with the ignore entry deleted, and passes
  // for a .jsx path outside prototype/ altogether.
  it('is ignored by the linter', async () => {
    // Vitest runs at the Vite root, which is web/ — where the configuration lives.
    const eslint = new ESLint({ cwd: process.cwd() })

    await expect(eslint.isPathIgnored('prototype/design-system/heroicons.js')).resolves.toBe(true)
    await expect(eslint.isPathIgnored('src/app.tsx')).resolves.toBe(false)
  })

  it('is outside every path Vitest looks in', () => {
    const { test } = viteConfig as { test: { include: string[] } }

    // Non-empty first: the loop below is vacuously true over an empty list, and an empty include is a
    // plausible edit — Vitest's own «no test files found» is what would catch it, one layer away.
    expect(test.include.length).toBeGreaterThan(0)

    for (const pattern of test.include) {
      expect(pattern.startsWith('src/')).toBe(true)
    }
  })
})
