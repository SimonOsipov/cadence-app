import { waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// The entry point is imported for its side effect, which is why both branches are reached by importing
// it rather than by calling something extracted out of it: an extracted function tested on its own
// leaves the argument at the call site — `document.getElementById('root')` — measured by nothing.
describe('the entry point', () => {
  beforeEach(() => {
    vi.resetModules()
    document.body.innerHTML = ''

    // The addresses this build points at. config.ts refuses to start without them, deliberately, so a
    // test of the entry point has to provide them or it measures that refusal instead.
    vi.stubEnv('VITE_API_URL', 'https://api.example')
    vi.stubEnv('VITE_AUTH_URL', 'https://auth.example')
  })

  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('refuses a document with no root rather than rendering nothing', async () => {
    await expect(import('./main')).rejects.toThrow('#root is missing from the document')
  })

  it('mounts into the root the document carries', async () => {
    document.body.innerHTML = '<div id="root"></div>'

    await import('./main')

    // Waited for rather than slept through: React 19 renders on a scheduler of its own, and a single
    // macrotask is an assumption about MessageChannel delivery beating timers, not a signal. The
    // door and not the dashboard behind it, because what this measures is that something mounted —
    // and nobody is signed in in a fresh document.
    await waitFor(() => {
      expect(document.querySelector('#root h1')?.textContent).toContain('Кабинет врача')
    })
  })
})
