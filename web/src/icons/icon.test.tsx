import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Icon } from './icon'
import { icons } from './icons'

const names = Object.keys(icons) as Array<keyof typeof icons>

describe('the icon', () => {
  it.each(names)('draws %s', (name) => {
    const { container } = render(<Icon name={name} />)

    const paths = [...container.querySelectorAll('path')].map((path) => path.getAttribute('d'))

    expect(paths).toEqual([...icons[name]])
  })

  // cog, fire and camera are two paths each in the source. A component rendering only the first would
  // draw a recognisable-but-wrong icon, which is the kind of thing nobody reports as a bug.
  it('draws every path of an icon that has more than one', () => {
    const twoPathed = names.filter((name) => icons[name].length > 1)

    expect(twoPathed.length).toBeGreaterThan(0)

    for (const name of twoPathed) {
      const { container } = render(<Icon name={name} />)

      expect(container.querySelectorAll('path')).toHaveLength(icons[name].length)
    }
  })

  it('is sized and coloured by its caller, and takes the colour from the text by default', () => {
    const { container } = render(<Icon name="plus" size={32} />)
    const svg = container.querySelector('svg')

    expect(svg?.getAttribute('width')).toBe('32')
    expect(svg?.getAttribute('height')).toBe('32')
    expect(svg?.getAttribute('stroke')).toBe('currentColor')
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
    const { container: silent } = render(<Icon name="plus" />)
    expect(silent.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true')

    const { container: labelled } = render(<Icon name="plus" label="Добавить" />)
    expect(labelled.querySelector('svg')?.getAttribute('aria-hidden')).toBeNull()
    expect(labelled.querySelector('title')?.textContent).toBe('Добавить')
  })
})
