import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const FROZEN = 'prototype/design-system/colors_and_type.css'
const PORTED = 'src/tokens/colors_and_type.css'

// The two edits the port is allowed to make, and nothing else is. Written as a list rather than as a
// tolerance in a diff, because «the CSS stays the source of truth» is a claim that needs a mechanism:
// without one, a colour changed here and not in the prototype is invisible until two surfaces disagree
// in front of a doctor.
const PERMITTED = [
  {
    why: 'the Google Fonts import becomes a local @font-face — no runtime call leaves the browser',
    line: 6,
    from:
      "@import url('https://fonts.googleapis.com/css2?family=Instrument+Serif:ital@0;1" +
      '&family=DM+Sans:opsz,wght@9..40,300;9..40,400;9..40,500;9..40,600;9..40,700' +
      "&family=JetBrains+Mono:wght@400;500&display=swap');",
    to: "@import '../fonts/fonts.css';",
  },
  {
    why: 'DM Sans has no Cyrillic and every string in this product is Russian — see the 2026-07-28 decision',
    line: 70,
    from: "  --font-body:    'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;",
    to: "  --font-body:    'Golos Text', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;",
  },
]

describe('the ported tokens', () => {
  it('differ from the frozen prototype only where the port is permitted to', () => {
    const frozen = readFileSync(FROZEN, 'utf8').split('\n')

    for (const { line, from, to } of PERMITTED) {
      // The line the exception names has to be the line it describes. Without this the list drifts as
      // the file moves and starts excusing whichever line happens to sit at that index.
      expect(frozen[line - 1]).toBe(from)
      frozen[line - 1] = to
    }

    expect(readFileSync(PORTED, 'utf8')).toBe(frozen.join('\n'))
  })

  it('asks the network for nothing', () => {
    expect(readFileSync(PORTED, 'utf8')).not.toContain('fonts.googleapis.com')
  })
})
