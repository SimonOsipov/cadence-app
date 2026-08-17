import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

import { tokens } from './tokens'

// The mobile app shipped its palette first, and in two places it followed the prototype's JS
// components rather than the CSS the port takes as the source. Generating from the CSS without
// reconciling would split the two surfaces at the first token — so the disagreements are enumerated
// here, each with the decision that settles it, and the table is asserted in both directions: a
// recorded divergence that has quietly gone away fails just as loudly as a new one.
const KOTLIN = '../kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceColors.kt'

const RECONCILED: Record<string, { css: string; kotlin: string; decision: string }> = {
  border: {
    css: '#e4dac6',
    kotlin: '#cdc1a8',
    decision:
      'The CSS calls #e4dac6 --border and #cdc1a8 --border-strong; the prototype JS palette calls ' +
      '#cdc1a8 border, and both shipped surfaces drew with it. The port may not edit the CSS, so the ' +
      'dashboard reads borderStrong wherever the prototype read C.border. Nothing is renamed on ' +
      'either side; what is written down is that they are the same colour under two names.',
  },
}

// Present in Kotlin and in no CSS declaration. Listed rather than fixed: the CSS is the source, and a
// token it does not carry is not invented on one surface — that is how the two drift.
const KOTLIN_ONLY = [
  'ink700', // #3f3b35 — the prototype yields undefined for it, so it never rendered on the web
  'sand600', // from a literal in the mobile app's NutritionScreen
  'slateBg',
  'slateFg',
  'clayBg',
  'clayFg', // from literals in the mobile app's Recipes and Learn screens
  'dangerFg',
  'warningFg', // likewise
  'forestScrim', // private in Kotlin: the ink behind the two below
  'glassDark', // mobile/src/theme/index.ts:73 names it; this CSS has no glass token at all
  'glassSoft', // the same ink at a lighter weight, inline in the mobile app's ConfirmToast
]

function kotlinPalette(): Record<string, string> {
  const source = readFileSync(KOTLIN, 'utf8')
  const palette: Record<string, string> = {}

  // Two shapes, and the second matters: hairline is written as an ink with an alpha applied, which is
  // the same colour the CSS spells rgba(...). Normalised rather than excused — a divergence table that
  // lists agreements stops being read.
  for (const [, name, hex] of source.matchAll(/val (\w+) = Color\(0xFF([0-9A-Fa-f]{6})\)\s*$/gm)) {
    if (name && hex) palette[name] = `#${hex.toLowerCase()}`
  }

  for (const [, name, hex, alpha] of source.matchAll(
    /val (\w+) = (?:Color\(0xFF([0-9A-Fa-f]{6})\)|\w+)\.copy\(alpha = ([\d.]+)f\)/g,
  )) {
    // The `else` reaches the ink a named private colour holds — glassDark and glassSoft are written
    // as forestScrim with an alpha, so the base is looked up rather than matched.
    const base = hex ? `#${hex.toLowerCase()}` : palette.forestScrim
    if (!name || !alpha || !base) continue

    const [r, g, b] = [1, 3, 5].map((at) => parseInt(base.slice(at, at + 2), 16))

    palette[name] = `rgba(${r},${g},${b},${Number(alpha)})`
  }

  return palette
}

describe('the dashboard and the mobile app', () => {
  const kotlin = kotlinPalette()
  const shared = Object.keys(tokens).filter((name) => name in kotlin)

  it('name enough colours in common for this comparison to mean anything', () => {
    expect(shared.length).toBeGreaterThan(25)
  })

  it.each(shared.filter((name) => !(name in RECONCILED)))('agree about %s', (name) => {
    expect(tokens[name as keyof typeof tokens]).toBe(kotlin[name])
  })

  it.each(Object.entries(RECONCILED))('still disagree about %s, as recorded', (name, recorded) => {
    expect(tokens[name as keyof typeof tokens]).toBe(recorded.css)
    expect(kotlin[name]).toBe(recorded.kotlin)
  })

  it.each(KOTLIN_ONLY)('%s is the mobile app’s alone', (name) => {
    expect(kotlin).toHaveProperty(name)
    expect(tokens).not.toHaveProperty(name)
  })

  // The list above is the whole of what the two do not share, in that direction. Without this a name
  // added to Kotlin and to nothing else would be a silent third divergence.
  it('have nothing else the mobile app carries alone', () => {
    const unshared = Object.keys(kotlin).filter((name) => !(name in tokens))

    expect(unshared.sort()).toEqual([...KOTLIN_ONLY].sort())
  })
})
