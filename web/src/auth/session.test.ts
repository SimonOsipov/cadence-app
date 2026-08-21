import { beforeEach, describe, expect, it } from 'vitest'

import { holdSession, readSession, signIn, storeSession, type Session } from './session'

const PROVIDER = 'https://auth.example'

function answering(...answers: { status: number; body: unknown }[]): {
  fetcher: typeof fetch
  calls: { url: string; init: RequestInit | undefined }[]
} {
  const calls: { url: string; init: RequestInit | undefined }[] = []
  let served = 0

  const fetcher = (url: string, init?: RequestInit) => {
    calls.push({ url, init })

    const answer = answers[Math.min(served, answers.length - 1)]
    served += 1

    return Promise.resolve(
      new Response(JSON.stringify(answer?.body), {
        status: answer?.status ?? 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  }

  return { fetcher: fetcher as unknown as typeof fetch, calls }
}

const aSession: Session = {
  accessToken: 'the-access-token',
  refreshToken: 'the-refresh-token',
  expiresAt: 1_800_000_000,
}

describe('signing in', () => {
  it('asks the identity provider itself, not the API', async () => {
    const { fetcher, calls } = answering({
      status: 200,
      body: { access_token: 'a', refresh_token: 'r', expires_at: 1_800_000_000 },
    })

    const session = await signIn(
      { providerUrl: PROVIDER, fetcher },
      { email: 'ksenia@clinic.example', password: 'a-seeded-password' },
    )

    expect(calls[0]?.url).toBe(`${PROVIDER}/token?grant_type=password`)
    expect(session).toEqual({ accessToken: 'a', refreshToken: 'r', expiresAt: 1_800_000_000 })
  })

  // The provider answers `invalid_grant` for a wrong password and for an address it has never seen
  // alike, and it answers in English. The person at the keyboard reads Russian.
  it('refuses in the language of the product, and says nothing about which half was wrong', async () => {
    const { fetcher } = answering({
      status: 400,
      body: { error_code: 'invalid_credentials', msg: 'Invalid login credentials' },
    })

    await expect(
      signIn({ providerUrl: PROVIDER, fetcher }, { email: 'ksenia@clinic.example', password: 'wrong' }),
    ).rejects.toThrow('Неверная почта или пароль')
  })

  it('says an unconfirmed address is not a wrong password', async () => {
    const { fetcher } = answering({
      status: 400,
      body: { error_code: 'email_not_confirmed', msg: 'Email not confirmed' },
    })

    await expect(
      signIn({ providerUrl: PROVIDER, fetcher }, { email: 'ksenia@clinic.example', password: 'right' }),
    ).rejects.toThrow('Приглашение ещё не принято')
  })
})

describe('the session a browser holds', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  // sessionStorage and not localStorage: the session survives a reload, dies with the tab, and is not
  // shared with a second one. Recorded as the decision it is — both are readable by any script that
  // reaches the page, so what this buys is the window, not secrecy.
  it('survives a reload and is gone when the tab is', () => {
    storeSession(aSession)

    expect(readSession()).toEqual(aSession)

    // The decision itself, asserted where it is made rather than described in a comment: the store is
    // sessionStorage, so a second tab starts signed out and closing this one ends the session.
    expect(sessionStorage.getItem('cadence.session')).not.toBeNull()
    expect(globalThis.localStorage?.getItem('cadence.session') ?? null).toBeNull()
  })

  it('reads nothing rather than throwing when the stored value is not a session', () => {
    sessionStorage.setItem('cadence.session', '{"half":')

    expect(readSession()).toBeNull()
  })

  it('reads nothing when the stored value is missing a half of the session', () => {
    sessionStorage.setItem('cadence.session', JSON.stringify({ accessToken: 'a' }))

    expect(readSession()).toBeNull()
  })
})

describe('the held session', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  // The criterion, and the reason the refresh is in one place: every request that meets a 401 asks
  // for a new token, and a provider given two refresh calls with the same token answers the second
  // one «already used».
  it('refreshes once however many callers meet a 401 at the same moment', async () => {
    const { fetcher, calls } = answering({
      status: 200,
      body: { access_token: 'fresh', refresh_token: 'r2', expires_at: 1_900_000_000 },
    })

    const held = holdSession({ providerUrl: PROVIDER, fetcher }, aSession)

    const [first, second, third] = await Promise.all([held.refresh(), held.refresh(), held.refresh()])

    expect(calls.length).toBe(1)
    expect([first, second, third]).toEqual(['fresh', 'fresh', 'fresh'])
    expect(held.token()).toBe('fresh')
  })

  it('asks again the next time, rather than answering the first refresh for ever', async () => {
    const { fetcher, calls } = answering({
      status: 200,
      body: { access_token: 'fresh', refresh_token: 'r2', expires_at: 1_900_000_000 },
    })

    const held = holdSession({ providerUrl: PROVIDER, fetcher }, aSession)

    await held.refresh()
    await held.refresh()

    expect(calls.length).toBe(2)
  })

  it('ends the session when the provider refuses the refresh token', async () => {
    const { fetcher } = answering({ status: 400, body: { error_code: 'invalid_grant' } })

    const held = holdSession({ providerUrl: PROVIDER, fetcher }, aSession)

    await expect(held.refresh()).rejects.toThrow()
    expect(held.token()).toBe('')
    expect(readSession()).toBeNull()
  })

  it('forgets the session on the way out', () => {
    const { fetcher } = answering({ status: 200, body: {} })

    const held = holdSession({ providerUrl: PROVIDER, fetcher }, aSession)
    held.signOut()

    expect(held.token()).toBe('')
    expect(readSession()).toBeNull()
  })

  it('tells whoever is watching that the session changed', async () => {
    const { fetcher } = answering({
      status: 200,
      body: { access_token: 'fresh', refresh_token: 'r2', expires_at: 1_900_000_000 },
    })

    const seen: (Session | null)[] = []
    const held = holdSession({ providerUrl: PROVIDER, fetcher }, aSession)
    held.watch((session) => seen.push(session))

    await held.refresh()
    held.signOut()

    expect(seen.map((session) => session?.accessToken ?? null)).toEqual(['fresh', null])
  })
})
