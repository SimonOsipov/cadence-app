import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { App } from './app'

describe('the dashboard shell', () => {
  it('renders the product name', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: 'Cadence' })).toBeInTheDocument()
  })
})
