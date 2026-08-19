/**
 * The session a signed-in member of staff holds, and the two places it comes from: the identity
 * provider, which mints it, and sessionStorage, which keeps it across a reload.
 *
 * Sign-in goes straight to the provider and not through the API. The API verifies tokens and issues
 * none — it holds no admin key and no password — so a sign-in route on it would be a second door to
 * the same lock, with this product's most dangerous credential behind it.
 */

/** Where the session is kept. sessionStorage: it survives a reload, dies with the tab, and is not
 * shared with a second one. Both stores are readable by any script that reaches the page, so what
 * this buys is the window it exists in, not secrecy — recorded as a decision at step 6. */
const STORAGE_KEY = 'cadence.session'

export type Session = {
  accessToken: string
  refreshToken: string

  /** Seconds since the epoch, as the provider states it. */
  expiresAt: number
}

export type ProviderOptions = {
  /** Where the identity provider listens. No trailing slash. */
  providerUrl: string

  /** Injected so a test can answer without a provider, and never used to reach anything else. */
  fetcher?: typeof fetch | undefined
}

export type Credentials = {
  email: string
  password: string
}

/**
 * The refusals a person can do something about, in the language they read.
 *
 * `invalid_credentials` covers a wrong password and an address the provider has never seen, and it is
 * one sentence here for the same reason it is one there: telling them apart tells whoever is guessing
 * which half of the guess was right.
 */
const REFUSALS: Record<string, string> = {
  invalid_credentials: 'Неверная почта или пароль.',
  email_not_confirmed: 'Приглашение ещё не принято. Откройте ссылку из письма и задайте пароль.',
  user_banned: 'Доступ к аккаунту закрыт. Обратитесь к администратору клиники.',
  over_request_rate_limit: 'Слишком много попыток. Повторите через минуту.',
}

const REFUSED = 'Не удалось войти. Повторите попытку.'

export async function signIn(options: ProviderOptions, credentials: Credentials): Promise<Session> {
  return grant(options, 'password', { email: credentials.email, password: credentials.password })
}

/** Sets a password on the account the token belongs to: how an invited doctor finishes accepting. */
export async function setPassword(options: ProviderOptions, accessToken: string, password: string): Promise<void> {
  const send = options.fetcher ?? fetch

  const answer = await send(`${options.providerUrl}/user`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${accessToken}` },
    body: JSON.stringify({ password }),
  })

  if (!answer.ok) throw await refusal(answer)
}

async function grant(
  options: ProviderOptions,
  type: 'password' | 'refresh_token',
  body: Record<string, string>,
): Promise<Session> {
  const send = options.fetcher ?? fetch

  const answer = await send(`${options.providerUrl}/token?grant_type=${type}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  if (!answer.ok) throw await refusal(answer)

  const minted = (await answer.json()) as {
    access_token?: string
    refresh_token?: string
    expires_at?: number
  }

  if (!minted.access_token || !minted.refresh_token) {
    throw new Error(REFUSED)
  }

  return {
    accessToken: minted.access_token,
    refreshToken: minted.refresh_token,
    expiresAt: minted.expires_at ?? 0,
  }
}

async function refusal(answer: Response): Promise<Error> {
  const said = await answer.text()

  try {
    const refused = JSON.parse(said) as { error_code?: string; error?: string }
    const code = refused.error_code ?? refused.error ?? ''

    return new Error(REFUSALS[code] ?? REFUSED)
  } catch {
    // Not the provider's own shape: a proxy in front of it, or nothing at all.
    return new Error(REFUSED)
  }
}

export function readSession(): Session | null {
  const stored = sessionStorage.getItem(STORAGE_KEY)
  if (stored === null) return null

  try {
    const session = JSON.parse(stored) as Partial<Session>

    // Both halves, because a session with no refresh token expires in an hour with no way back, and
    // the screen it leaves behind looks like the API refusing rather than like a session running out.
    if (typeof session.accessToken !== 'string' || typeof session.refreshToken !== 'string') return null

    return {
      accessToken: session.accessToken,
      refreshToken: session.refreshToken,
      expiresAt: typeof session.expiresAt === 'number' ? session.expiresAt : 0,
    }
  } catch {
    return null
  }
}

export function storeSession(session: Session): void {
  sessionStorage.setItem(STORAGE_KEY, JSON.stringify(session))
}

export function forgetSession(): void {
  sessionStorage.removeItem(STORAGE_KEY)
}

export type HeldSession = {
  /** The access token to present, or the empty string once there is no session. */
  token(): string

  /** A fresh access token. Concurrent callers share one call to the provider. */
  refresh(): Promise<string>

  signOut(): void

  /** Called whenever the session changes, with null once it is gone. Returns the way to stop. */
  watch(onChange: (session: Session | null) => void): () => void
}

/**
 * Holds the session and is the only thing that refreshes it.
 *
 * One place, because a refresh token may be spent once: two requests meeting a 401 at the same moment
 * would each ask, and the provider answers the second one «already used» — which ends the session of
 * somebody who did nothing wrong. So concurrent callers wait on the one call that is already running.
 */
export function holdSession(options: ProviderOptions, initial: Session | null): HeldSession {
  let current = initial
  let refreshing: Promise<string> | null = null
  const watchers: ((session: Session | null) => void)[] = []

  const settle = (session: Session | null) => {
    current = session

    if (session === null) {
      forgetSession()
    } else {
      storeSession(session)
    }

    for (const watcher of watchers) watcher(session)
  }

  return {
    token: () => current?.accessToken ?? '',

    refresh() {
      if (refreshing !== null) return refreshing

      const held = current
      if (held === null) return Promise.reject(new Error('нет сессии для обновления'))

      refreshing = grant(options, 'refresh_token', { refresh_token: held.refreshToken })
        .then((session) => {
          settle(session)

          return session.accessToken
        })
        .catch((error: unknown) => {
          // A refusal here is the session ending: the token cannot be spent twice, and holding a
          // dead one only makes the next request fail in a way nobody can read.
          settle(null)

          throw error
        })
        .finally(() => {
          refreshing = null
        })

      return refreshing
    },

    signOut: () => settle(null),

    watch(onChange) {
      watchers.push(onChange)

      // Returned rather than left to be forgotten: a provider remounted — which React does on its own
      // in development — would otherwise leave the old subscriber holding a setState of a component
      // that is gone.
      return () => {
        const at = watchers.indexOf(onChange)
        if (at >= 0) watchers.splice(at, 1)
      }
    },
  }
}
