import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { App } from './app'

describe('the dashboard', () => {
  // The shell, and the only thing this asserts: App composes the provider with the screen, so a screen
  // rendered outside a DataProvider throws rather than rendering half a dashboard. What the Overview
  // draws is overview-page.test.tsx's subject.
  it('puts the Overview inside a data provider', async () => {
    render(<App />)

    expect(await screen.findByRole('status')).toHaveTextContent('Загружаем дашборд')
  })
})
