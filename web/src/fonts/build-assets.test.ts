import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it, onTestFinished } from 'vitest'

import { REQUIRED_FACES, inspect } from '../../scripts/check-build-assets'

// The check that guards the fonts had no test, and the reasoning that excused it — «it is a check, not
// code» — is the reasoning that hid a fail-open in the stack filter for three review rounds. Two of
// them were here too: a build with no font references at all announced success, and a stylesheet
// reaching over the network was invisible because minified Vite writes the import without url().
function distWith(stylesheet: string, present: string[]): string {
  const dist = mkdtempSync(join(tmpdir(), 'cadence-dist-'))
  onTestFinished(() => rmSync(dist, { recursive: true, force: true }))
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
  //
  // The two assertions are a pair and neither is redundant: the length has REQUIRED_FACES on both
  // sides, so on its own an emptied list would satisfy it as 0 === 0. What kills that is the line
  // above, where an empty requirement makes the verdict ok.
  it('refuses a build that references no font at all', () => {
    const dist = distWith('body{color:red}', [])

    expect(inspect(dist, REQUIRED_FACES).ok).toBe(false)
    expect(inspect(dist, REQUIRED_FACES).problems).toHaveLength(REQUIRED_FACES.length)
  })

  // One case per face, and the reason is a name that contains another: a build carrying only
  // CormorantGaramondItalic satisfied the requirement for CormorantGaramond by substring, so the
  // requirement it exists to enforce was open on exactly one of the four. The next pair of names
  // sharing a prefix would reopen it, which is why this is four cases and not one.
  it.each(REQUIRED_FACES)('refuses a build that has dropped %s', (missing) => {
    const shipped = REQUIRED_FACES.filter((face) => face !== missing).map((face) => `${face}-hash.ttf`)
    const css = shipped.map((file) => `@font-face{src:url(/assets/${file})}`).join('')

    expect(inspect(distWith(css, shipped), REQUIRED_FACES).problems).toEqual([
      `no stylesheet in the build points at ${missing}`,
    ])
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

  // An SVG and not a base64 PNG, which is the one inline encoding that carries no URL inside it and so
  // could not have caught this: every SVG holds xmlns="http://www.w3.org/2000/svg", and Vite inlines
  // small assets as data URIs, so the network sweep read the namespace as a call.
  it('leaves an inlined asset alone, xmlns and all', () => {
    const whole = wholeBuild()
    const svg = `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'%3E%3C/svg%3E")`
    writeFileSync(join(whole, 'assets', 'inline-xyz.css'), `i{background:${svg}}`)

    expect(inspect(whole, REQUIRED_FACES)).toEqual({ ok: true, problems: [] })
  })

  // Resolved against the page's own scheme, so it is as live a call as either spelling — and it passed
  // both halves of the network guard until this case.
  it('refuses a scheme-relative address', () => {
    const whole = wholeBuild()
    writeFileSync(join(whole, 'assets', 'relative-xyz.css'), '@import "//fonts.googleapis.com/css2";')

    expect(inspect(whole, REQUIRED_FACES).problems).toContain(
      'relative-xyz.css reaches //fonts.googleapis.com/css2 over the network',
    )
  })

  // What a build that emitted nothing actually leaves behind: no assets directory at all, rather than
  // an empty one.
  it('refuses a build with no assets directory', () => {
    const dist = mkdtempSync(join(tmpdir(), 'cadence-dist-'))

    expect(inspect(dist, REQUIRED_FACES).ok).toBe(false)
  })
})
