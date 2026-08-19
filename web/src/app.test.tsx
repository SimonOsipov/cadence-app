import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { App } from './app'
import { readSession } from './auth/session'

const API = 'https://api.example'
const PROVIDER = 'https://auth.example'

/** Answers /v1/me with the given role, and refuses anything else this test did not arrange. */
function providerAnswering(me: { role: string; full_name?: string }) {
  return vi.fn((url: string) => {
    if (String(url).includes('/v1/me')) {
      return Promise.resolve(
        new Response(JSON.stringify({ sub: '3f2a', expires_at: '2026-08-19T12:00:00Z', ...me }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    }

    return Promise.resolve(new Response(JSON.stringify({ patients: [] }), { status: 200 }))
  })
}

function signedIn() {
  sessionStorage.setItem(
    'cadence.session',
    JSON.stringify({ accessToken: 'a', refreshToken: 'r', expiresAt: 1 }),
  )
}

describe('the door to the dashboard', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', API)
    vi.stubEnv('VITE_AUTH_URL', PROVIDER)
    sessionStorage.clear()
    window.history.pushState({}, '', '/')
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  it('asks whoever arrives without a session to sign in', async () => {
    vi.stubGlobal('fetch', providerAnswering({ role: 'doctor' }))

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Кабинет врача' })).toBeInTheDocument()
  })

  it('opens for a doctor whose session survived a reload', async () => {
    signedIn()
    vi.stubGlobal('fetch', providerAnswering({ role: 'doctor', full_name: 'Ксения Первеева' }))

    render(<App />)

    expect(await screen.findByText(/Ксения/)).toBeInTheDocument()
  })

  it('opens for an administrator too', async () => {
    signedIn()
    vi.stubGlobal('fetch', providerAnswering({ role: 'admin', full_name: 'Пётр Аверин' }))

    render(<App />)

    expect(await screen.findByText(/Пётр/)).toBeInTheDocument()
  })

  // The role is the API's answer and not the browser's — see DashboardRoute.
  it('refuses a patient whose session was restored, and offers them the way out', async () => {
    signedIn()
    vi.stubGlobal('fetch', providerAnswering({ role: 'patient' }))

    render(<App />)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('для сотрудников клиники'))
    expect(screen.getByRole('button', { name: 'Выйти' })).toBeInTheDocument()
  })

  it('refuses an account the clinic has not finished creating', async () => {
    signedIn()
    vi.stubGlobal('fetch', providerAnswering({ role: '' }))

    await waitFor(() => {
      render(<App />)
    })

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('ещё не заведён в клинике'))
  })
})

describe('signing out and coming back in', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', API)
    vi.stubEnv('VITE_AUTH_URL', PROVIDER)
    sessionStorage.clear()
    window.history.pushState({}, '', '/')
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  it('puts a signed-in doctor back at the door and keeps nothing', async () => {
    const user = userEvent.setup()
    signedIn()
    vi.stubGlobal('fetch', providerAnswering({ role: 'doctor', full_name: 'Ксения Первеева' }))

    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Выйти' }))

    expect(await screen.findByRole('heading', { name: 'Кабинет врача' })).toBeInTheDocument()
    expect(readSession()).toBeNull()
  })

  // The link lands wherever the provider was configured to send it, and what says «this is an
  // invitation» is the fragment. Landing on the dashboard with one and being asked to sign in would
  // ask the person for a password they have not set yet.
  it('takes an invitation that landed on the dashboard to the screen that finishes it', async () => {
    vi.stubGlobal('fetch', providerAnswering({ role: 'doctor' }))
    window.history.pushState({}, '', '/#access_token=a&refresh_token=r&type=invite')

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Задайте пароль' })).toBeInTheDocument()
  })
})

// The second thing the smoke test found, and only the router could: the form that was about to say
// why was replaced before it could. See AuthProvider.signIn for what changed.
describe('a patient at the door', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', API)
    vi.stubEnv('VITE_AUTH_URL', PROVIDER)
    sessionStorage.clear()
    window.history.pushState({}, '', '/')
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
  })

  it('is told why, on the form they filled in', async () => {
    const user = userEvent.setup()

    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) =>
        Promise.resolve(
          new Response(
            JSON.stringify(
              String(url).includes('/token')
                ? { access_token: 'a', refresh_token: 'r', expires_at: 1 }
                : { sub: '3f2a', role: 'patient', expires_at: '2026-08-20T12:00:00Z' },
            ),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          ),
        ),
      ),
    )

    render(<App />)

    await user.type(await screen.findByLabelText('Почта'), 'marina@clinic.example')
    await user.type(screen.getByLabelText('Пароль'), 'a-seeded-password')
    await user.click(screen.getByRole('button', { name: 'Войти' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('для сотрудников клиники')
    expect(readSession()).toBeNull()
  })
})
