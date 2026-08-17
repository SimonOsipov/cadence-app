import { ESLint } from 'eslint'
import { describe, expect, it } from 'vitest'

import viteConfig from '../vite.config'

// `web/prototype/` is the frozen visual specification: in-browser Babel JSX that never compiles and is
// never shipped. Left inside any runner it fails the gate at once and there is nothing to fix — the
// only repair is the exclusion this asserts.
describe('the frozen prototype', () => {
  it('is ignored by the linter', async () => {
    // Vitest runs at the Vite root, which is web/ — where the configuration lives.
    const eslint = new ESLint({ cwd: process.cwd() })

    await expect(eslint.isPathIgnored('prototype/dd-app.jsx')).resolves.toBe(true)
  })

  it('is outside every path Vitest looks in', () => {
    const { test } = viteConfig as { test: { include: string[]; exclude: string[] } }

    // Asserted on the include side rather than the exclude side: an exclusion can be out-argued by a
    // wider include, and «every pattern is rooted in src/» cannot.
    for (const pattern of test.include) {
      expect(pattern.startsWith('src/')).toBe(true)
    }

    expect(test.exclude).toContain('prototype/**')
  })
})

// The smoke test of step 5 lives in tests/ and is Playwright's. Vitest picking it up would import a
// runner that is not running and go red permanently, on a branch that changed nothing about it.
describe('the two runners', () => {
  it('do not overlap', () => {
    const { test } = viteConfig as { test: { exclude: string[] } }

    expect(test.exclude).toContain('tests/**')
  })
})
