import { describe, expect, it } from 'vitest'

import { decimal, quantity, whole } from './format'

// The prototype writes «110,0 кг» and «0,25 мг»; a component interpolating the number writes «110» and
// «0.25». Which one a doctor sees is the whole of what this module decides.
describe('numbers as this surface writes them', () => {
  it.each([
    [104.2, 1, '104,2'],
    [110, 1, '110'],
    [71.75, 1, '71,8'],
    // A dose is written to two places, and one place would round 0,25 мг to 0,3 мг — a different dose.
    [0.25, 2, '0,25'],
  ])('writes %s to %s place(s) as %s', (value, places, expected) => {
    expect(decimal(value, places)).toBe(expected)
  })

  it('never writes a decimal point', () => {
    for (const value of [1.5, 0.25, 104.2, 1000.75]) {
      expect(decimal(value, 2)).not.toContain('.')
    }
  })

  it('puts a unit after the number, spaced', () => {
    expect(quantity(104.2, 'кг')).toBe('104,2 кг')
    expect(quantity(0.25, 'мг', 2)).toBe('0,25 мг')
  })

  it('writes a count without a fraction', () => {
    expect(whole(25)).toBe('25')
    expect(whole(93)).toBe('93')
  })
})
