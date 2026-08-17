import { readFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { describe, expect, it } from 'vitest'

// The mobile app bundles these four and builds its scale from them; the dashboard links the same
// bytes. A typeface is one product decision, and two surfaces drifting apart on it is the class of
// divergence the token port exists to close — asserted here rather than agreed to, because a font
// swapped on one side alone is invisible until somebody puts the two screens next to each other.
const SHARED = 'CormorantGaramond CormorantGaramondItalic GolosText JetBrainsMono'.split(' ')

const digest = (path: string) => createHash('sha256').update(readFileSync(path)).digest('hex')

describe('the three faces', () => {
  it.each(SHARED)('%s is the file the mobile app ships', (face) => {
    const mobile = `../kmp/composeApp/src/commonMain/composeResources/font/${face}.ttf`

    expect(digest(`src/fonts/${face}.ttf`)).toBe(digest(mobile))
  })

  it('are all declared, and nothing else is', () => {
    const declared = [...readFileSync('src/fonts/fonts.css', 'utf8').matchAll(/url\('\.\/(.+?)'\)/g)]
      .map(([, file]) => file)

    expect([...new Set(declared)].sort()).toEqual(SHARED.map((face) => `${face}.ttf`).sort())
  })

  // Instrument Serif is named first in --font-display and has no @font-face on purpose: the stack
  // falls through to Cormorant Garamond, which is the substitution the prototype's own decision made
  // because Instrument Serif carries no Cyrillic. A @font-face for it would undo that silently.
  it('leave Instrument Serif to fall through', () => {
    // The declaration and not the word: the file explains the fall-through in prose right above, so a
    // search for the name matches the explanation and measures nothing.
    const families = [...readFileSync('src/fonts/fonts.css', 'utf8').matchAll(/font-family: '(.+?)'/g)]
      .map(([, family]) => family)

    expect(families).not.toContain('Instrument Serif')
    expect(readFileSync('src/tokens/colors_and_type.css', 'utf8')).toContain(
      "--font-display: 'Instrument Serif', 'Cormorant Garamond'",
    )
  })
})
