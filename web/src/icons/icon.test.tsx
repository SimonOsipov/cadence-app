import { render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { Icon } from './icon'
import { icons } from './icons'

const names = Object.keys(icons) as Array<keyof typeof icons>

// Taken from the set rather than named: the set is derived from what the screens draw, so naming one
// here both pins a screen's choice and — measured — puts that icon in the bundle, because the sweep
// read this file too.
const any = names[0]!

// cog, fire and camera carry two paths in heroicons; none of the three is on a screen, so the set the
// dashboard draws is entirely single-path and `icons[name].slice(0, 1)` would pass every case above.
// The module is substituted rather than the mapping extracted: an extracted helper tested on its own
// leaves the call site — which paths the component hands it — measured by nothing.
describe('an icon of more than one path', () => {
  it('draws every one of them, in order', async () => {
    const both = ['M1 1h2', 'M3 3h4']

    vi.resetModules()
    vi.doMock('./icons', () => ({ icons: { 'two-paths': both } }))

    const { Icon: Substituted } = await import('./icon')
    // @ts-expect-error the substituted module declares an icon the real IconName union does not
    const { container } = render(<Substituted name="two-paths" />)

    expect([...container.querySelectorAll('path')].map((path) => path.getAttribute('d'))).toEqual(both)

    vi.doUnmock('./icons')
    vi.resetModules()
  })
})

describe('the icon', () => {
  // This reads its expectation from icons.ts, the same module the component reads, so it cannot catch a
  // wrong path — only a component that drops or reorders one. The pin on the data itself is elsewhere:
  // derive-icons.ts --check re-derives the whole set from heroicons.js on every gate run.
  //
  // `toEqual` pins order and count per icon, so a component rendering only the first path of a two-path
  // icon would fail — but no icon this dashboard draws has two, so that arm is measured by the module
  // substitution below rather than by the set.
  it.each(names)('draws %s', (name) => {
    const { container } = render(<Icon name={name} />)

    const paths = [...container.querySelectorAll('path')].map((path) => path.getAttribute('d'))

    expect(paths).toEqual([...icons[name]])
  })

  // The assertion above already pins each icon's paths in order, so what this adds is the sentinel: if
  // the derived subset ever holds only single-path icons, the multi-path case is going untested and
  // that should be said out loud rather than passing vacuously. In the subset today it is cog alone —
  // fire and camera have two paths in heroicons.js and the dashboard draws neither.
  it('is sized and weighted by its caller, and takes the colour from the text', () => {
    const { container } = render(<Icon name={any} size={32} strokeWidth={2} />)
    const svg = container.querySelector('svg')

    expect(svg?.getAttribute('width')).toBe('32')
    expect(svg?.getAttribute('height')).toBe('32')
    expect(svg?.getAttribute('stroke')).toBe('currentColor')
    expect(svg?.getAttribute('stroke-width')).toBe('2')
  })

  // Both are properties of the data rather than of the caller: the paths are drawn for a 24x24 box, and
  // a different one crops all 22 at once — which reads as «the icons look wrong» and not as a bug.
  it('draws into the box the paths were made for, at the weight they were made for', () => {
    const svg = render(<Icon name={any} />).container.querySelector('svg')

    expect(svg?.getAttribute('viewBox')).toBe('0 0 24 24')
    expect(svg?.getAttribute('stroke-width')).toBe('1.5')
  })

  // The acceptance criterion is «an unknown icon name is a compile error», and this is what asserts
  // it: @ts-expect-error fails the typecheck when the line below turns out to compile. Vitest alone
  // would not notice — `vitest run` does not typecheck, which the gate learned by running tsc first.
  it('refuses a name nothing derives', () => {
    // @ts-expect-error the subset carries no such icon
    expect(() => render(<Icon name="not-an-icon" />)).toThrow()
  })

  // An icon beside its own label would otherwise be read out twice; one that stands alone needs the
  // label the caller gives it.
  it('is hidden from the accessibility tree unless it is given a label', () => {
    const { container: silent } = render(<Icon name={any} />)
    expect(silent.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true')

    const { container: labelled } = render(<Icon name={any} label="Добавить" />)
    expect(labelled.querySelector('svg')?.getAttribute('aria-hidden')).toBeNull()
    expect(labelled.querySelector('title')?.textContent).toBe('Добавить')
    // Without role="img" most screen readers skip the <title> and the label is not announced at all,
    // which is the outcome this test exists to prevent.
    expect(labelled.querySelector('svg')?.getAttribute('role')).toBe('img')
  })
})
