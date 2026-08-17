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

    // React 19 renders through a scheduler, so the assertion waits a macrotask rather than reading
    // straight after the import.
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(document.querySelector('#root h1')?.textContent).toBe('Cadence')
  })
})
