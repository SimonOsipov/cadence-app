import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// Imported from scripts/ rather than living in src/: the parser runs at build time and has no business
// in the bundle. The test is in src/ because that is the only place Vitest collects from — see
// frame.test.ts for why that rule is worth keeping.
import { parseTokens, renderModule } from '../../scripts/tokens'

const root = (body: string) => `:root {\n${body}\n}\n`

describe('reading the tokens out of the CSS', () => {
  it('takes a declaration as it is written', () => {
    expect(parseTokens(root('  --paper: #fbf8f3;'))).toEqual([
      { name: 'paper', cssVariable: '--paper', value: '#fbf8f3' },
    ])
  })

  it('keeps a value that contains commas, quotes and calls', () => {
    const values = [
      "'Golos Text', -apple-system, 'Segoe UI', sans-serif",
      'clamp(3.5rem, 8vw, 5.5rem)',
      '0 2px 6px rgba(46, 38, 24, 0.06), 0 1px 2px rgba(46, 38, 24, 0.04)',
      'cubic-bezier(0.34, 1.56, 0.64, 1)',
    ]

    for (const value of values) {
      expect(parseTokens(root(`  --x: ${value};`))[0]?.value).toBe(value)
    }
  })

  it('resolves an alias to the value it ends at, however many hops away', () => {
    const parsed = parseTokens(
      root('  --forest-700: #2d5f3f;\n  --primary: var(--forest-700);\n  --cta: var(--primary);'),
    )

    expect(parsed.map((token) => token.value)).toEqual(['#2d5f3f', '#2d5f3f', '#2d5f3f'])
  })

  // The DOM needs the name and the code needs the value; a component reading `--primary` off an
  // element and a component importing `tokens.primary` must be talking about the same declaration.
  it('carries the CSS name beside the resolved value', () => {
    const [token] = parseTokens(root('  --forest-700: #2d5f3f;\n  --primary: var(--forest-700);'))

    expect(token).toEqual({ name: 'forest700', cssVariable: '--forest-700', value: '#2d5f3f' })
  })

  it('ignores comments, wherever they sit', () => {
    const parsed = parseTokens(
      root('  /* Surfaces */\n  --paper: #fbf8f3;  /* lightest */\n  --cream: #f6f1ea;'),
    )

    expect(parsed.map((token) => token.value)).toEqual(['#fbf8f3', '#f6f1ea'])
  })

  it('reads only the first :root, not whatever follows it', () => {
    const css = `${root('  --paper: #fbf8f3;')}\n.card { color: var(--paper); border: 1px solid red; }`

    expect(parseTokens(css)).toHaveLength(1)
  })
})

describe('what the parser refuses', () => {
  it('an alias to a name nothing declares', () => {
    expect(() => parseTokens(root('  --primary: var(--forest-700);'))).toThrow('--forest-700')
  })

  it('a cycle', () => {
    expect(() => parseTokens(root('  --a: var(--b);\n  --b: var(--a);'))).toThrow(/cycle/i)
  })

  // Two CSS names can fold to one TypeScript key, and the loser would vanish from the generated module
  // without a word — the import that used it still compiles, because the winner answers to that key.
  it('two names that fold to one key', () => {
    expect(() => parseTokens(root('  --t-body: 1rem;\n  --tBody: 2rem;'))).toThrow(/tBody/)
  })

  // A missing semicolon is the same shape of loss: CSS drops both declarations, and this used to return
  // one token whose value held the other. The last declaration of a block was already refused; the ones
  // in the middle were not.
  it.each([
    ['in the middle of the block', '  --a: 1rem\n  --b: 2rem;\n  --c: 3rem;'],
    ['at the end of the block', '  --a: 1rem;\n  --b: 2rem'],
  ])('a declaration missing its semicolon %s', (_, body) => {
    expect(() => parseTokens(root(body))).toThrow(/semicolon/)
  })

  // Both reach tokens.ts as the literal string otherwise, which is not a value and which the module's
  // own contract says it never carries.
  it.each([
    ['a fallback', '  --a: #000;\n  --b: var(--a, #fff);'],
    ['a reference inside a call', '  --a: 2px;\n  --b: calc(var(--a) * 2);'],
  ])('an alias it cannot follow: %s', (_, body) => {
    expect(() => parseTokens(root(body))).toThrow(/cannot resolve/)
  })

  it('a declaration with no value at all', () => {
    expect(() => parseTokens(root('  --a:;'))).toThrow(/empty value/)
  })
})

// Three shapes that used to end the scan early or leave it inside a string, taking every declaration
// after them with it. None is reachable while the CSS is pinned byte-for-byte to the prototype; all
// three are one edit away from being.
describe('what the scanner survives', () => {
  it.each([
    ['a closing brace inside a value', `  --a: "}";\n  --b: 2rem;`],
    ['an escaped quote inside a value', `  --a: 'it\\'s';\n  --b: 2rem;`],
  ])('%s', (_, body) => {
    expect(parseTokens(root(body))).toHaveLength(2)
  })
})

// --check compares the committed module against a regeneration by the same parser, so it is
// self-consistent by construction: it catches drift and can never catch a parser defect. The
// reconciliation against the mobile palette covers 32 names and all of them colours. These are the
// families nothing else pins — a parser change that mangled every shadow would have regenerated
// cleanly and passed everything.
describe('the real stylesheet', () => {
  const parsed = parseTokens(readFileSync('src/tokens/colors_and_type.css', 'utf8'))
  const valueOf = (name: string) => parsed.find((token) => token.name === name)?.value

  it('yields every declaration it carries', () => {
    expect(parsed).toHaveLength(89)
  })

  it.each([
    ['shadowSm', '0 2px 6px rgba(46, 38, 24, 0.06), 0 1px 2px rgba(46, 38, 24, 0.04)'],
    ['shadowInset', 'inset 0 1px 0 rgba(255,255,255,0.6), inset 0 -1px 0 rgba(46,38,24,0.04)'],
    ['tDisplay', 'clamp(3.5rem, 8vw, 5.5rem)'],
    ['easeSpring', 'cubic-bezier(0.34, 1.56, 0.64, 1)'],
    ['r2xl', '32px'],
    ['s7', '24px'],
    ['hairline', 'rgba(26,26,26,0.08)'],
    ['fontBody', "'Golos Text', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif"],
  ])('carries %s through as written', (name, value) => {
    expect(valueOf(name)).toBe(value)
  })

  it('leaves no reference behind', () => {
    for (const token of parsed) {
      expect(token.value).not.toContain('var(')
    }
  })
})

describe('the generated module', () => {
  const tokens = [{ name: 'paper', cssVariable: '--paper', value: '#fbf8f3' }]

  it('says it is generated and where from', () => {
    expect(renderModule(tokens)).toContain('generate-tokens')
  })

  it('exports the values and the CSS names under the same keys', () => {
    const module = renderModule(tokens)

    expect(module).toContain("paper: '#fbf8f3'")
    expect(module).toContain("paper: '--paper'")
  })
})
