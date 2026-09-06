import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'

import { AuthProvider, NOT_FOR_PATIENTS, useAuth } from '../../auth/auth-context'
import { readSession } from '../../auth/session'
import { defaultClient } from '../../data/queries'
import { AcceptInvitePage, PASSWORD_MIN_LENGTH, sessionFromFragment } from './accept-invite-page'
import { SignInPage } from './sign-in-page'

const API = 'https://api.example'
const PROVIDER = 'https://auth.example'

type Answer = { status?: number; body?: unknown }

/** A stand-in for both the provider and the API, answering by the path it is asked for. */
const tokensPresented: string[] = []

function serving(answers: Record<string, Answer | Answer[]>): { fetcher: typeof fetch; asked: string[] } {
  const asked: string[] = []
  const served: Record<string, number> = {}

  const fetcher = (url: string, init?: RequestInit) => {
    const path = String(url)
    asked.push(`${init?.method ?? 'GET'} ${path}`)
    tokensPresented.push(new Headers(init?.headers).get('authorization') ?? '')

    const match = Object.keys(answers).find((key) => path.includes(key))
    if (match === undefined) throw new Error(`nothing answers ${path}`)

    const candidates = answers[match]
    const list = Array.isArray(candidates) ? candidates : [candidates as Answer]
    const turn = served[match] ?? 0
    served[match] = turn + 1
    const answer = list[Math.min(turn, list.length - 1)] ?? {}

    return Promise.resolve(
      new Response(JSON.stringify(answer.body ?? {}), {
        status: answer.status ?? 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  }

  return { fetcher: fetcher as unknown as typeof fetch, asked }
}

function showing(children: ReactNode, fetcher: typeof fetch) {
  return render(
    <QueryClientProvider client={defaultClient()}>
      <AuthProvider apiUrl={API} providerUrl={PROVIDER} fetcher={fetcher} initial={null}>
        {children}
      </AuthProvider>
    </QueryClientProvider>,
  )
}

function SignedInAs() {
  const auth = useAuth()

  return <p>{auth.identity === null ? 'нет сессии' : `вошли: ${auth.identity.full_name ?? auth.identity.role}`}</p>
}

async function signInAs(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('Почта'), 'ksenia@clinic.example')
  await user.type(screen.getByLabelText('Пароль'), 'a-seeded-password')
  await user.click(screen.getByRole('button', { name: 'Войти' }))
}

describe('signing in', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('lets a doctor in and greets them by the name the clinic wrote', async () => {
    const user = userEvent.setup()
    const { fetcher } = serving({
      '/token': { body: { access_token: 'a', refresh_token: 'r', expires_at: 1 } },
      '/v1/me': { body: { sub: '3f2a', role: 'doctor', expires_at: '2026-08-19T12:00:00Z', full_name: 'Ксения Первеева' } },
    })

    showing(
      <>
        <SignInPage />
        <SignedInAs />
      </>,
      fetcher,
    )

    await signInAs(user)

    await waitFor(() => expect(screen.getByText('вошли: Ксения Первеева')).toBeInTheDocument())
    expect(readSession()?.accessToken).toBe('a')
  })

  // A patient's credentials are valid — the provider mints them a session — and this screen is still
  // not theirs. The refusal has to be the dashboard's, and the session it just took has to go.
  it('refuses a patient and keeps no session for them', async () => {
    const user = userEvent.setup()
    const { fetcher } = serving({
      '/token': { body: { access_token: 'a', refresh_token: 'r', expires_at: 1 } },
      '/v1/me': { body: { sub: '3f2a', role: 'patient', expires_at: '2026-08-19T12:00:00Z' } },
    })

    showing(<SignInPage />, fetcher)
    await signInAs(user)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(NOT_FOR_PATIENTS))
    expect(readSession()).toBeNull()
  })

  it('says what the provider refused, in the language of the product', async () => {
    const user = userEvent.setup()
    const { fetcher } = serving({
      '/token': { status: 400, body: { error_code: 'invalid_credentials' } },
    })

    showing(<SignInPage />, fetcher)
    await signInAs(user)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Неверная почта или пароль.'))
  })
})

describe('accepting an invitation', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  const landed = '#access_token=a&refresh_token=r&expires_at=1&type=invite'

  it('reads the session the provider left in the fragment', () => {
    expect(sessionFromFragment(landed)?.accessToken).toBe('a')
    expect(sessionFromFragment('#error=access_denied&error_code=otp_expired')).toBeNull()
  })

  // The floor the provider enforces, not a softer one of our own: refused here after the person
  // has typed is the shape this exists to prevent, and GOTRUE_PASSWORD_MIN_LENGTH is 10.
  it('refuses a password the provider would refuse, before asking it to', async () => {
    const user = userEvent.setup()
    const { fetcher, asked } = serving({ '/user': { status: 200, body: {} } })

    render(<AcceptInvitePage providerUrl={PROVIDER} fragment={landed} fetcher={fetcher} onAccepted={() => {}} />)

    await user.type(screen.getByLabelText('Пароль'), 'a'.repeat(PASSWORD_MIN_LENGTH - 1))
    await user.type(screen.getByLabelText('Ещё раз'), 'a'.repeat(PASSWORD_MIN_LENGTH - 1))
    await user.click(screen.getByRole('button', { name: 'Сохранить и войти' }))

    expect(await screen.findByRole('alert')).toBeTruthy()
    expect(asked).not.toContain(`PUT ${PROVIDER}/user`)
  })

  it('sets the password and keeps the session it arrived with', async () => {
    const user = userEvent.setup()
    const { fetcher, asked } = serving({ '/user': { status: 200, body: {} } })

    let accepted = false
    render(
      <AcceptInvitePage
        providerUrl={PROVIDER}
        fragment={landed}
        fetcher={fetcher}
        onAccepted={() => {
          accepted = true
        }}
      />,
    )

    await user.type(screen.getByLabelText('Пароль'), 'a-password-nobody-uses')
    await user.type(screen.getByLabelText('Ещё раз'), 'a-password-nobody-uses')
    await user.click(screen.getByRole('button', { name: 'Сохранить и войти' }))

    await waitFor(() => expect(accepted).toBe(true))
    expect(asked).toContain(`PUT ${PROVIDER}/user`)
    expect(readSession()?.accessToken).toBe('a')
  })

  // Two fields, so a typo is caught here rather than at the next sign-in, when the person has no
  // link left to try again with.
  it('refuses two passwords that differ, without asking the provider', async () => {
    const user = userEvent.setup()
    const { fetcher, asked } = serving({ '/user': { status: 200, body: {} } })

    render(<AcceptInvitePage providerUrl={PROVIDER} fragment={landed} fetcher={fetcher} />)

    await user.type(screen.getByLabelText('Пароль'), 'a-password-nobody-uses')
    await user.type(screen.getByLabelText('Ещё раз'), 'another-password')
    await user.click(screen.getByRole('button', { name: 'Сохранить и войти' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Пароли не совпадают.')
    expect(asked).toHaveLength(0)
    expect(readSession()).toBeNull()
  })

  // A link that was already opened, or one that sat in a mailbox past its three days: the provider
  // redirects here with an error in the fragment and no session in it.
  it('says a spent link is spent rather than showing a form that cannot work', () => {
    render(<AcceptInvitePage providerUrl={PROVIDER} fragment="#error_code=otp_expired" />)

    expect(screen.getByRole('alert')).toHaveTextContent('Ссылка недействительна')
    expect(screen.queryByLabelText('Пароль')).toBeNull()
  })
})

// What React Query holds after a doctor signs out is that doctor's roster — their patients' names.
// The next person to sign in on the same machine must not be shown it while their own answer is on
// its way.
describe('signing out', () => {
  it('takes the answers of the person who was signed in with it', async () => {
    const { fetcher } = serving({
      '/token': { body: { access_token: 'a', refresh_token: 'r', expires_at: 1 } },
      '/v1/me': { body: { sub: '3f2a', role: 'doctor', expires_at: '2026-08-19T12:00:00Z', full_name: 'Ксения Первеева' } },
    })

    const client = defaultClient()
    client.setQueryData(['roster'], { patients: [{ full_name: 'Марина Волкова' }] })

    function Door() {
      const auth = useAuth()

      return (
        <button type="button" onClick={() => auth.signOut()}>
          Выйти
        </button>
      )
    }

    render(
      <QueryClientProvider client={client}>
        <AuthProvider apiUrl={API} providerUrl={PROVIDER} fetcher={fetcher} initial={null}>
          <Door />
        </AuthProvider>
      </QueryClientProvider>,
    )

    await userEvent.setup().click(screen.getByRole('button', { name: 'Выйти' }))

    expect(client.getQueryData(['roster'])).toBeUndefined()
  })
})

// The bug the smoke test found and 365 tests did not: signing in stored the session and told React
// about it, and the holder — the one thing every request reads its token from — never heard. The
// dashboard signed in and then answered 401 to everything, on every screen.
describe('what the session is worth after signing in', () => {
  it('is on every request the API client makes next', async () => {
    const user = userEvent.setup()
    const { fetcher, asked } = serving({
      '/token': { body: { access_token: 'the-minted-token', refresh_token: 'r', expires_at: 1 } },
      '/v1/me': { body: { sub: '3f2a', role: 'doctor', expires_at: '2026-08-20T12:00:00Z', full_name: 'Ксения' } },
      '/v1/dashboard/overview': { body: { patients: [] } },
    })

    function AsksTheApi() {
      const auth = useAuth()

      return (
        <button type="button" onClick={() => void auth.api.roster({})}>
          Прочитать реестр
        </button>
      )
    }

    showing(
      <>
        <SignInPage />
        <AsksTheApi />
      </>,
      fetcher,
    )

    await signInAs(user)
    await waitFor(() => expect(asked.some((call) => call.includes('/v1/me'))).toBe(true))

    await user.click(screen.getByRole('button', { name: 'Прочитать реестр' }))

    await waitFor(() => expect(tokensPresented).toContain('Bearer the-minted-token'))
  })
})
