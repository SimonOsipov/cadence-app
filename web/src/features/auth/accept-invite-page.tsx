import { useState, type FormEvent } from 'react'

import { setPassword, storeSession, type Session } from '../../auth/session'
import { tokens } from '../../tokens/tokens'

/**
 * Where an invitation link lands.
 *
 * The provider verifies the link itself and redirects here with a session in the URL fragment — the
 * fragment, because a browser does not send it to the server, so the token that would sign this
 * person in never reaches an access log. What is left for this screen is the password: without one,
 * the person can be signed in exactly once, by a link that has already been spent.
 */
export function AcceptInvitePage({
  providerUrl,
  fragment,
  fetcher,
  onAccepted,
}: {
  providerUrl: string
  /** The URL fragment as the provider left it, without the leading '#'. */
  fragment: string
  fetcher?: typeof fetch | undefined
  onAccepted?: () => void
}) {
  const landed = sessionFromFragment(fragment)

  if (landed === null) {
    return (
      <Frame>
        <p role="alert" style={refusalStyle}>
          Ссылка недействительна или уже использована. Попросите администратора прислать приглашение
          заново.
        </p>
      </Frame>
    )
  }

  return <ChooseAPassword providerUrl={providerUrl} landed={landed} fetcher={fetcher} onAccepted={onAccepted} />
}

function ChooseAPassword({
  providerUrl,
  landed,
  fetcher,
  onAccepted,
}: {
  providerUrl: string
  landed: Session
  fetcher?: typeof fetch | undefined
  onAccepted?: (() => void) | undefined
}) {
  const [password, setChosen] = useState('')
  const [repeated, setRepeated] = useState('')
  const [refusal, setRefusal] = useState<string | null>(null)
  const [asking, setAsking] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    setRefusal(null)

    if (password !== repeated) {
      setRefusal('Пароли не совпадают.')

      return
    }
    if (password.length < 8) {
      setRefusal('Пароль должен быть не короче восьми символов.')

      return
    }

    setAsking(true)

    try {
      await setPassword({ providerUrl, fetcher }, landed.accessToken, password)

      // Stored only once the password is set: a session kept for an account whose password was
      // refused is a person signed in who cannot sign in again.
      storeSession(landed)
      onAccepted?.()
    } catch (error) {
      setRefusal(error instanceof Error ? error.message : 'Не удалось задать пароль. Повторите попытку.')
    } finally {
      setAsking(false)
    }
  }

  return (
    <Frame>
      <form onSubmit={(event) => void submit(event)} style={{ display: 'grid', gap: 16 }} aria-labelledby="accept-title">
        <h1 id="accept-title" style={{ fontFamily: tokens.fontDisplay, fontSize: 34, fontWeight: 400, margin: 0 }}>
          Задайте пароль
        </h1>

        <label style={{ display: 'grid', gap: 6 }}>
          Пароль
          <input
            type="password"
            name="password"
            autoComplete="new-password"
            required
            value={password}
            onChange={(event) => setChosen(event.target.value)}
            style={field}
          />
        </label>

        <label style={{ display: 'grid', gap: 6 }}>
          Ещё раз
          <input
            type="password"
            name="repeat"
            autoComplete="new-password"
            required
            value={repeated}
            onChange={(event) => setRepeated(event.target.value)}
            style={field}
          />
        </label>

        {refusal !== null && (
          <p role="alert" style={refusalStyle}>
            {refusal}
          </p>
        )}

        <button type="submit" disabled={asking} style={submitButton}>
          {asking ? 'Сохраняем…' : 'Сохранить и войти'}
        </button>
      </form>
    </Frame>
  )
}

/**
 * Reads the session the provider left in the fragment.
 *
 * Both halves or nothing: an access token with no refresh token signs the person in for an hour and
 * then drops them at a form with no way back, which reads as the invitation having failed.
 */
export function sessionFromFragment(fragment: string): Session | null {
  const landed = new URLSearchParams(fragment.replace(/^#/, ''))

  const accessToken = landed.get('access_token')
  const refreshToken = landed.get('refresh_token')
  if (accessToken === null || refreshToken === null) return null

  return { accessToken, refreshToken, expiresAt: Number(landed.get('expires_at') ?? 0) }
}

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <main
      style={{
        minHeight: '100vh',
        display: 'grid',
        placeItems: 'center',
        background: tokens.paper,
        fontFamily: tokens.fontBody,
        color: tokens.ink900,
      }}
    >
      <div style={{ width: 360 }}>{children}</div>
    </main>
  )
}

const field = {
  padding: '10px 12px',
  borderRadius: 8,
  border: `1px solid ${tokens.ink300}`,
  background: tokens.cream,
  font: 'inherit',
  color: 'inherit',
}

const submitButton = {
  padding: '11px 16px',
  borderRadius: 8,
  border: 'none',
  background: tokens.forest700,
  color: tokens.paper,
  font: 'inherit',
  cursor: 'pointer',
}

const refusalStyle = {
  margin: 0,
  color: tokens.danger,
  background: tokens.dangerBg,
  padding: '10px 12px',
  borderRadius: 8,
}
