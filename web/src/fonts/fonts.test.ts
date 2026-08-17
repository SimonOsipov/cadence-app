import { readFileSync, readdirSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { describe, expect, it } from 'vitest'

const MOBILE = '../kmp/composeApp/src/commonMain/composeResources/font'
const SHEET = readFileSync('src/fonts/fonts.css', 'utf8')

// Read rather than written down: a fifth face added to the mobile app is a fifth face this test then
// demands of the dashboard, instead of one nobody notices. Assert the set, not the size.
const SHARED = readdirSync(MOBILE)
  .filter((file) => file.endsWith('.ttf'))
  .map((file) => file.replace(/\.ttf$/, ''))
  .sort()

const digest = (path: string) => createHash('sha256').update(readFileSync(path)).digest('hex')

describe('the three faces', () => {
  // The set the dashboard links is the set the mobile app bundles. That is a new invariant this test
  // introduces, and it is deliberate: a typeface is one product decision for both surfaces. It is
  // stated here rather than left to an ENOENT deep inside a digest, which reads as a broken file
  // rather than as a policy.
  it('are the faces the mobile app bundles, neither more nor fewer', () => {
    const linked = readdirSync('src/fonts')
      .filter((file) => file.endsWith('.ttf'))
      .map((file) => file.replace(/\.ttf$/, ''))

    expect(SHARED.length).toBeGreaterThan(0)
    expect(linked.sort()).toEqual([...SHARED].sort())
  })

  // A typeface is one product decision and two surfaces. That they agree is measured, because a font
  // swapped on one side alone is invisible until somebody puts the two screens next to each other.
  it.each(SHARED)('%s is the file the mobile app ships', (face) => {
    expect(digest(`src/fonts/${face}.ttf`)).toBe(digest(`${MOBILE}/${face}.ttf`))
  })

  it('are all declared, and nothing else is', () => {
    const declared = [...SHEET.matchAll(/url\('\.\/(.+?)'\)/g)].map(([, file]) => file)

    expect([...new Set(declared)].sort()).toEqual(SHARED.map((face) => `${face}.ttf`).sort())
  })

  // Which file is linked is half the answer; the other half is how. Measured against this suite before
  // it existed: giving the italic face `font-style: normal` left both Cormorant faces claiming normal,
  // the browser free to pick either, and every test green.
  it('are declared with the style and weight each is for', () => {
    const declarations = [...SHEET.matchAll(/@font-face\s*\{([^}]+)\}/g)].map(([, body]) => {
      const read = (property: string) =>
        new RegExp(`${property}:\\s*([^;]+);`).exec(body ?? '')?.[1]?.trim()

      return [read('src')?.match(/\.\/(.+?\.ttf)/)?.[1], read('font-style'), read('font-display')].join(' ')
    })

    expect(declarations.sort()).toEqual(
      [
        'CormorantGaramond.ttf normal swap',
        'CormorantGaramondItalic.ttf italic swap',
        'GolosText.ttf normal swap',
        'JetBrainsMono.ttf normal swap',
      ].sort(),
    )
  })

  // The sentence three comments in this step give as the reason for linking locally, and until now it
  // was held by nothing: port.test.ts looks for that host in the *other* stylesheet. Measured — an
  // @import of Google Fonts added here passed the whole gate, and it survives minification without a
  // url() wrapper, so the built-asset check cannot see it either.
  it('reach for nothing over the network', () => {
    const everything = SHEET + readFileSync('src/tokens/colors_and_type.css', 'utf8')

    // What a stylesheet can be made to fetch, and not every mention of a scheme: a licence URL in a
    // comment above four bundled faces is an ordinary thing to write, and failing on it would be a red
    // gate whose diagnosis sends the next reader hunting a leak that is not there.
    //
    // The scheme is optional — `//host/path` resolves against the page's own and is as live a call.
    const fetched = [
      ...everything.matchAll(/@import\s+(?:url\()?["']?([^"');]+)/g),
      ...everything.matchAll(/src:\s*url\(["']?([^"')]+)/g),
    ].map(([, address]) => address ?? '')

    expect(fetched.length).toBeGreaterThan(0)
    expect(fetched.filter((address) => /^(?:https?:)?\/\//.test(address))).toEqual([])
  })

  // Instrument Serif is named first in --font-display and has no @font-face on purpose: the stack falls
  // through to Cormorant Garamond, which is the substitution the prototype's own decision made because
  // Instrument Serif carries no Cyrillic. A @font-face for it would undo that silently.
  it('leave Instrument Serif to fall through', () => {
    // The declaration and not the word: the file explains the fall-through in prose right above, so a
    // search for the name matches the explanation and measures nothing.
    const families = [...SHEET.matchAll(/font-family: '(.+?)'/g)].map(([, family]) => family)

    expect(families).not.toContain('Instrument Serif')
    expect(readFileSync('src/tokens/colors_and_type.css', 'utf8')).toContain(
      "--font-display: 'Instrument Serif', 'Cormorant Garamond'",
    )
  })
})
