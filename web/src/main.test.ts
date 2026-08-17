import { waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

// The entry point is imported for its side effect, which is why both branches are reached by importing
// it rather than by calling something extracted out of it: an extracted function tested on its own
// leaves the argument at the call site — `document.getElementById('root')` — measured by nothing.
describe('the entry point', () => {
  beforeEach(() => {
    vi.resetModules()
    document.body.innerHTML = ''
  })

  it('refuses a document with no root rather than rendering nothing', async () => {
    await expect(import('./main')).rejects.toThrow('#root is missing from the document')
  })

  it('mounts into the root the document carries', async () => {
    document.body.innerHTML = '<div id="root"></div>'

    await import('./main')

    // Waited for rather than slept through: React 19 renders on a scheduler of its own, and a single
    // macrotask is an assumption about MessageChannel delivery beating timers, not a signal.
    await waitFor(() => {
      expect(document.querySelector('#root h1')?.textContent).toBe('Cadence')
    })
  })
})
