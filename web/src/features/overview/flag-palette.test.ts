import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

import { FLAG_INK } from './flag-palette'

const KOTLIN = '../kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceColors.kt'

// These three sit outside the token file by necessity — the CSS never named them and the port may not
// add one. That makes them the one place the two surfaces can drift without the reconciliation
// noticing, so the two the mobile app does name are checked against it here.
describe('the flag inks the token file does not carry', () => {
  const kotlin = new Map<string, string>(
    [...readFileSync(KOTLIN, 'utf8').matchAll(/val (\w+) = Color\(0xFF([0-9A-Fa-f]{6})\)/g)].map(
      ([, name, hex]) => [name ?? '', `#${(hex ?? '').toLowerCase()}`] as const,
    ),
  )

  it.each([
    ['danger', 'dangerFg'],
    ['warning', 'warningFg'],
  ] as const)('%s is the ink the mobile app calls %s', (tone, mobile) => {
    expect(kotlin.get(mobile)).toBeDefined()
    expect(FLAG_INK[tone]).toBe(kotlin.get(mobile))
  })

  // Recorded rather than asserted: neither the CSS nor the mobile palette names it, so there is nothing
  // to compare against. This test exists so that stops being true loudly — the day somebody adds it on
  // either side, this is where the two are joined up.
  it('has one ink that is nobody else’s', () => {
    expect([...kotlin.values()]).not.toContain(FLAG_INK.info)
  })
})
