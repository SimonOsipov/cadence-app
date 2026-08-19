import { useState, type FormEvent } from 'react'

import { useAuth } from '../../auth/auth-context'
import { tokens } from '../../tokens/tokens'

/**
 * The door to the dashboard.
 *
 * There is no design for this screen in the frozen prototype — it draws the dashboard behind the
 * door — so it is built from the same tokens and says as little as a door can.
 */
export function SignInPage({ onSignedIn }: { onSignedIn?: () => void }) {
  const auth = useAuth()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [refusal, setRefusal] = useState<string | null>(null)
  const [asking, setAsking] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()

    setRefusal(null)
    setAsking(true)

    try {
      await auth.signIn({ email, password })
      onSignedIn?.()
    } catch (error) {
      setRefusal(error instanceof Error ? error.message : 'Не удалось войти. Повторите попытку.')
    } finally {
      setAsking(false)
    }
  }

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
      <form
        onSubmit={(event) => void submit(event)}
        style={{ width: 360, display: 'grid', gap: 16 }}
        aria-labelledby="sign-in-title"
      >
        <h1
          id="sign-in-title"
          style={{ fontFamily: tokens.fontDisplay, fontSize: 34, fontWeight: 400, margin: 0 }}
        >
          Кабинет врача
        </h1>

        <label style={{ display: 'grid', gap: 6 }}>
          Почта
          <input
            type="email"
            name="email"
            autoComplete="username"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            style={field}
          />
        </label>

        <label style={{ display: 'grid', gap: 6 }}>
          Пароль
          <input
            type="password"
            name="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            style={field}
          />
        </label>

        {/* The refusal is a live region: a screen reader user submitting a form hears why it was
            refused rather than finding the same form again. */}
        {refusal !== null && (
          <p role="alert" style={{ margin: 0, color: tokens.danger, background: tokens.dangerBg, padding: '10px 12px', borderRadius: 8 }}>
            {refusal}
          </p>
        )}

        <button type="submit" disabled={asking} style={submitButton}>
          {asking ? 'Входим…' : 'Войти'}
        </button>
      </form>
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
