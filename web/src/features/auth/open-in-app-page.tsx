import { useEffect, useState } from 'react'

import { tokens } from '../../tokens/tokens'

/** Where each link hands off. The app registers both hosts; see `ACCEPT_LINK` in the shared module. */
export const DESTINATIONS = {
  accept: 'cadence://accept',
  recover: 'cadence://recover',
} as const

export type OpenInAppKind = keyof typeof DESTINATIONS

export const THE_APP_DID_NOT_ANSWER = 'Похоже, приложение Cadence не установлено'

const THE_NEXT_STEP = 'Установите его, вернитесь в письмо и откройте ссылку ещё раз.'

const A_LINK_WITH_NOTHING_IN_IT =
  'Ссылка неполная — откройте её из письма целиком, не копируя по частям.'

// Long enough for the system to hand the link over and put the app in front of this tab, short
// enough that a patient without the app is not left watching nothing. Not measured against a
// device — the by-hand pass on both platforms is what settles it.
export const WAIT_FOR_THE_APP_MS = 1500

/**
 * The token, or null when the fragment is not one.
 *
 * Compared as a whole parameter rather than by prefix: `token_hashy=` starts with the name this
 * page reads and is somebody else's field.
 */
export function tokenFromFragment(fragment: string): string | null {
  const carried = new URLSearchParams(fragment.startsWith('#') ? fragment.slice(1) : fragment)
  const token = carried.get('token_hash')

  // Present-but-empty is absent: `#token_hash=` is what a mail client leaves behind when it eats
  // the value, and handing an empty token to the app spends a request to be told nothing.
  return token === null || token === '' ? null : token
}

/**
 * What the invitation and the recovery mails link to, and the only thing standing between a patient
 * without the app and a tab that does nothing.
 *
 * The token rides in the fragment, which a browser does not send to any server — so it stays out of
 * access logs and out of `Referer`. This page reads it on the client and hands it to the scheme;
 * it never puts it into a request, and there is no fetch on this path at all.
 */
export function OpenInAppPage({
  kind,
  fragment,
  open = (url) => {
    window.location.assign(url)
  },
  schedule = (fn) => window.setTimeout(fn, WAIT_FOR_THE_APP_MS),
  cancel = (timer) => {
    if (timer !== undefined) window.clearTimeout(timer)
  },
  stillHere = () => document.visibilityState === 'visible',
}: {
  kind: OpenInAppKind
  /** The URL fragment as the mail client left it, with or without its leading '#'. */
  fragment: string
  open?: (url: string) => void
  schedule?: (fn: () => void) => number | undefined
  cancel?: (timer: number | undefined) => void
  stillHere?: () => boolean
}) {
  const token = tokenFromFragment(fragment)
  const [unanswered, setUnanswered] = useState(false)

  useEffect(() => {
    if (token === null) return

    // Encoded on the way out because it was decoded on the way in: a `%23` in the fragment would
    // otherwise arrive at the app as a real `#`, and Ktor's parser reads everything after it as
    // the link's fragment rather than as the token.
    open(`${DESTINATIONS[kind]}?token_hash=${encodeURIComponent(token)}`)
    // Nothing tells a page that a scheme found no handler. What it can see is that it is still the
    // thing in front of the patient once the system has had its chance.
    const timer = schedule(() => {
      if (stillHere()) setUnanswered(true)
    })

    // A second mail opened in the same tab changes the fragment in place, and without this the
    // first link's timer outlives it and can answer for the second.
    return () => cancel(timer)
    // Once per token: re-running it would hand the same link over again on any render around it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, kind])

  if (token === null) {
    return (
      <Frame>
        <p role="alert" style={refusalStyle}>
          {A_LINK_WITH_NOTHING_IN_IT}
        </p>
      </Frame>
    )
  }

  return (
    <Frame>
      <p style={leadStyle}>Открываем Cadence…</p>
      {unanswered && (
        <>
          <p style={refusalStyle}>{THE_APP_DID_NOT_ANSWER}</p>
          <p style={leadStyle}>{THE_NEXT_STEP}</p>
        </>
      )}
    </Frame>
  )
}

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <main
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 12,
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        padding: 24,
        background: tokens.paper,
        fontFamily: tokens.fontBody,
        color: tokens.ink900,
      }}
    >
      {children}
    </main>
  )
}

const leadStyle = { color: tokens.ink900, margin: 0, textAlign: 'center' } as const

const refusalStyle = { color: tokens.danger, margin: 0, textAlign: 'center' } as const
