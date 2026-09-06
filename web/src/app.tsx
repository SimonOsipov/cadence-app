import { QueryClientProvider } from '@tanstack/react-query'
import { useState } from 'react'
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom'

import { AuthProvider, NOT_FOR_PATIENTS, NOT_IN_THE_CLINIC_YET, STAFF, useAuth, useIdentity } from './auth/auth-context'
import { endpoints } from './config'
import { DataProvider, defaultClient } from './data/queries'
import { AcceptInvitePage } from './features/auth/accept-invite-page'
import { OpenInAppPage, type OpenInAppKind } from './features/auth/open-in-app-page'
import { SignInPage } from './features/auth/sign-in-page'
import { OverviewPage } from './features/overview/overview-page'
import { tokens } from './tokens/tokens'

/**
 * Where a patient's mail lands. Not `/accept-invite`: that one is the dashboard's own door and
 * expects a session, while these carry a token the app spends.
 *
 * A path and a kind paired by hand is a pair that can be swapped — and swapped, every invitation
 * goes to `cadence://recover`, which `/verify` refuses, so a patient is told a live link was
 * already used. The pairing is held by «serves each landing at the path its own scheme names»;
 * that these rows reach <Routes> at all, by «serves $path from the routes, not from the table».
 */
export const PATIENT_LANDINGS: ReadonlyArray<{ path: string; kind: OpenInAppKind }> = [
  { path: '/accept', kind: 'accept' },
  { path: '/recover', kind: 'recover' },
]

export function App() {
  const { apiUrl, providerUrl } = endpoints()

  // One client for the life of the app, and above the auth provider: signing out clears it, and a
  // client rebuilt on every render would cache nothing at all.
  const [client] = useState(defaultClient)

  return (
    <QueryClientProvider client={client}>
      <AuthProvider apiUrl={apiUrl} providerUrl={providerUrl}>
        <BrowserRouter>
          <Routes>
            <Route path="/sign-in" element={<SignInRoute />} />
            <Route path="/accept-invite" element={<AcceptInviteRoute providerUrl={providerUrl} />} />
            {PATIENT_LANDINGS.map(({ path, kind }) => (
              <Route key={path} path={path} element={<OpenInAppRoute kind={kind} />} />
            ))}
            <Route path="/" element={<DashboardRoute />} />
            {/* Anything else is the dashboard's door rather than a page saying nothing. */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}

function SignInRoute() {
  const auth = useAuth()
  const navigate = useNavigate()

  if (auth.session !== null) return <Navigate to="/" replace />

  return <SignInPage onSignedIn={() => void navigate('/', { replace: true })} />
}

function OpenInAppRoute({ kind }: { kind: OpenInAppKind }) {
  const { hash } = useLocation()

  return <OpenInAppPage kind={kind} fragment={hash} />
}

function AcceptInviteRoute({ providerUrl }: { providerUrl: string }) {
  const navigate = useNavigate()
  const { hash } = useLocation()

  return (
    <AcceptInvitePage
      providerUrl={providerUrl}
      fragment={hash}
      onAccepted={() => void navigate('/', { replace: true })}
    />
  )
}

/**
 * The dashboard, and the second half of the door.
 *
 * The role is checked here and not only at sign-in: a session restored from sessionStorage has never
 * been asked, and a patient's would otherwise reach a screen built to show them other people.
 */
function DashboardRoute() {
  const auth = useAuth()
  const identity = useIdentity()
  const { hash } = useLocation()

  // An invitation link lands wherever the provider was configured to send it — SITE_URL when the
  // invitation asked for nothing else — and what marks it is the fragment, not the path. Carried
  // across rather than read here, so the acceptance screen stays the one place that reads it.
  if (hash.includes('type=invite') || hash.includes('type=recovery') || hash.includes('type=magiclink')) {
    return <Navigate to={{ pathname: '/accept-invite', hash }} replace />
  }

  if (auth.session === null) return <Navigate to="/sign-in" replace />

  if (identity.isPending) return <Waiting>Открываем кабинет…</Waiting>

  if (identity.isError) {
    return (
      <Waiting>
        <span role="alert">{identity.error.message}</span>
      </Waiting>
    )
  }

  if (!STAFF.includes(identity.data.role)) {
    return (
      <Refused onLeave={() => auth.signOut()}>
        {identity.data.role === '' ? NOT_IN_THE_CLINIC_YET : NOT_FOR_PATIENTS}
      </Refused>
    )
  }

  return (
    <DataProvider api={auth.api}>
      <OverviewPage me={identity.data} onSignOut={() => auth.signOut()} />
    </DataProvider>
  )
}

function Waiting({ children }: { children: React.ReactNode }) {
  return (
    <main style={frame} role="status">
      {children}
    </main>
  )
}

function Refused({ children, onLeave }: { children: React.ReactNode; onLeave: () => void }) {
  return (
    <main style={frame}>
      <div style={{ width: 380, display: 'grid', gap: 16 }}>
        <p role="alert" style={{ margin: 0 }}>
          {children}
        </p>
        <button type="button" onClick={onLeave} style={{ padding: '11px 16px', borderRadius: 8, border: 'none', background: tokens.forest700, color: tokens.paper, font: 'inherit', cursor: 'pointer' }}>
          Выйти
        </button>
      </div>
    </main>
  )
}

const frame = {
  minHeight: '100vh',
  display: 'grid',
  placeItems: 'center',
  background: tokens.paper,
  fontFamily: tokens.fontBody,
  color: tokens.ink900,
}
