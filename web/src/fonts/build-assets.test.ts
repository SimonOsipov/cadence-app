import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

import { REQUIRED_FACES, inspect } from '../../scripts/check-build-assets'

// The check that guards the fonts had no test, and the reasoning that excused it — «it is a check, not
// code» — is the reasoning that hid a fail-open in the stack filter for three review rounds. Two of
// them were here too: a build with no font references at all announced success, and a stylesheet
// reaching over the network was invisible because minified Vite writes the import without url().
function distWith(stylesheet: string, present: string[]): string {
  const dist = mkdtempSync(join(tmpdir(), 'cadence-dist-'))
  const assets = join(dist, 'assets')
  mkdirSync(assets)

  writeFileSync(join(assets, 'index-abc123.css'), stylesheet)
  for (const file of present) writeFileSync(join(assets, file), '')

  return dist
}

const wholeBuild = () => {
  const files = REQUIRED_FACES.map((face) => `${face}-hash.ttf`)
  const css = files.map((file) => `@font-face{src:url(/assets/${file})}`).join('')

  return distWith(css, files)
}

describe('inspecting a build', () => {
  it('accepts one that carries every face it names', () => {
    expect(inspect(wholeBuild(), REQUIRED_FACES)).toEqual({ ok: true, problems: [] })
  })

  it('refuses a reference to a file the build does not contain', () => {
    const dist = distWith('@font-face{src:url(./GolosText.ttf)}', [])

    expect(inspect(dist, []).problems).toContain(
      'index-abc123.css points at ./GolosText.ttf, which the build does not contain',
    )
  })

  // The one the old shape passed with a cheerful message: nothing was broken because nothing was there.
  it('refuses a build that references no font at all', () => {
    const dist = distWith('body{color:red}', [])

    expect(inspect(dist, REQUIRED_FACES).ok).toBe(false)
    expect(inspect(dist, REQUIRED_FACES).problems).toHaveLength(REQUIRED_FACES.length)
  })

  it('refuses a stylesheet that reaches over the network', () => {
    const whole = wholeBuild()
    const css = `@import "https://fonts.googleapis.com/css2?family=Inter";`
    writeFileSync(join(whole, 'assets', 'remote-xyz.css'), css)

    expect(inspect(whole, REQUIRED_FACES).problems).toContain(
      'remote-xyz.css reaches https://fonts.googleapis.com/css2?family=Inter over the network',
    )
  })

  it('refuses a build with no stylesheet rather than reporting nothing wrong', () => {
    const dist = mkdtempSync(join(tmpdir(), 'cadence-dist-'))
    mkdirSync(join(dist, 'assets'))

    expect(inspect(dist, REQUIRED_FACES)).toEqual({
      ok: false,
      problems: ['the build emitted no stylesheet, so this check measured nothing'],
    })
  })

  it('leaves an inlined asset alone', () => {
    const whole = wholeBuild()
    writeFileSync(join(whole, 'assets', 'inline-xyz.css'), 'i{background:url(data:image/png;base64,AA)}')

    expect(inspect(whole, REQUIRED_FACES).ok).toBe(true)
  })
})
