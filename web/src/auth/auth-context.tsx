import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

import { apiClient, type ApiClient } from '../data/api'
import type { Me } from '../api'
import {
  holdSession,
  readSession,
  signIn as signInWithProvider,
  storeSession,
  type Credentials,
  type HeldSession,
  type Session,
} from './session'

/** The roles this dashboard is for. A patient has an app; this screen would show them nothing. */
export const STAFF = ['doctor', 'admin']

export const NOT_FOR_PATIENTS =
  'Этот кабинет — для сотрудников клиники. Пациентам вход через мобильное приложение.'

export const NOT_IN_THE_CLINIC_YET =
  'Аккаунт ещё не заведён в клинике. Обратитесь к администратору.'

export type Auth = {
  /** Null when nobody is signed in. */
  session: Session | null

  /** Who the API says the caller is. Null until the first answer, and after signing out. */
  identity: Me | null

  signIn(credentials: Credentials): Promise<void>
  signOut(): void

  /** The API, with this session's token on every request. */
  api: ApiClient
}

const AuthContext = createContext<Auth | null>(null)

export function AuthProvider({
  children,
  apiUrl,
  providerUrl,
  fetcher,
  initial = readSession(),
}: {
  children: ReactNode
  apiUrl: string
  providerUrl: string
  fetcher?: typeof fetch | undefined
  initial?: Session | null
}) {
  const [session, setSession] = useState<Session | null>(initial)
  const [identity, setIdentity] = useState<Me | null>(null)

  // One holder for the life of the provider: it is where the single refresh lives, and a second one
  // would be a second refresh racing the first with a token only one of them can spend.
  const [holder] = useState<HeldSession>(() => holdSession({ providerUrl, fetcher }, initial))

  useEffect(() => holder.watch(setSession), [holder])

  const api = useMemo(
    () => apiClient({ baseUrl: apiUrl, session: holder, fetcher }),
    [apiUrl, holder, fetcher],
  )

  const cache = useQueryClient()

  // The cache goes with the session. What React Query holds after a doctor signs out is that
  // doctor's roster — their patients' names — and the next person to sign in on this machine would
  // be shown it for as long as it takes their own answer to arrive.
  const signOut = useCallback(() => {
    holder.signOut()
    setIdentity(null)
    cache.clear()
  }, [holder, cache])

  const signIn = useCallback(
    async (credentials: Credentials) => {
      const minted = await signInWithProvider({ providerUrl, fetcher }, credentials)

      // Stored before the role is known, because reading the role is a request this session has to
      // authenticate: the refusal below takes it away again.
      storeSession(minted)
      setSession(minted)

      const who = await apiClient({ baseUrl: apiUrl, session: staticAccess(minted), fetcher }).me()

      if (who.role === '') {
        signOut()

        throw new Error(NOT_IN_THE_CLINIC_YET)
      }
      if (!STAFF.includes(who.role)) {
        signOut()

        throw new Error(NOT_FOR_PATIENTS)
      }

      setIdentity(who)
    },
    [apiUrl, providerUrl, fetcher, signOut],
  )

  const value = useMemo<Auth>(
    () => ({ session, identity, signIn, signOut, api }),
    [session, identity, signIn, signOut, api],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

/**
 * Who the API says is signed in.
 *
 * A query and not the state above, because a session restored from sessionStorage has never been
 * asked: the tab was reloaded, and the role that decides whether this dashboard opens at all comes
 * from the API rather than from what the browser kept.
 */
export function useIdentity() {
  const { api, session } = useAuth()

  return useQuery({
    queryKey: ['me', session?.accessToken ?? null],
    queryFn: () => api.me(),
    enabled: session !== null,
    retry: false,
  })
}

export function useAuth(): Auth {
  const auth = useContext(AuthContext)
  if (auth === null) {
    throw new Error('this screen is outside an AuthProvider, so nobody is signed in as far as it knows')
  }

  return auth
}

/** The just-minted session, for the one call made before the holder is asked anything. */
function staticAccess(session: Session) {
  return {
    token: () => session.accessToken,
    refresh: () => Promise.reject(new Error('a session minted a moment ago is not refreshed')),
  }
}
