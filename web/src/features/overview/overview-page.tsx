import { useState } from 'react'

import type { Patient } from '../../data/overview'
import { useLiveRoster, useOverview } from '../../data/queries'
import { tokens } from '../../tokens/tokens'
import { PatientCard } from './patient-card'
import { Roster } from './roster'
import { Schedule } from './schedule'
import { SideMenu } from './side-menu'
import { StatsStrip } from './stats-strip'
import { Triage } from './triage'

/**
 * The Overview, assembled.
 *
 * Three states rather than one, and they are here rather than inside each section: what fails is the
 * request, not the stats strip. A screen that draws only the happy path acquires its loading and error
 * states one component at a time, later, under pressure — the move to `/v1` is exactly when.
 */
/**
 * greetedAs is the signed-in person's own name, passed in rather than read from the fixture: the
 * fixture's doctor is whoever the design drew, and the person in front of the screen is whoever
 * signed in. Left empty while the API has not answered yet — a greeting with a stranger's name in it
 * for one frame is worse than a greeting with none.
 */
export function OverviewPage({
  greetedAs = '',
  onSignOut,
}: {
  greetedAs?: string
  onSignOut?: (() => void) | undefined
}) {
  const [cursor, setCursor] = useState<string | null>(null)
  const [opened, setOpened] = useState<Patient | null>(null)

  // One section live and five on fixtures until M6 extends this same endpoint with what they draw —
  // flags, adherence, doses, appointments. Four of the five carry the mark a doctor can see; the side
  // menu does not, because its only fixture datum is a counter on a destination that does not exist
  // yet and is already drawn as unavailable.
  const overview = useOverview()
  const roster = useLiveRoster(cursor === null ? {} : { cursor })

  if (overview.isPending) return <Waiting />
  if (overview.isError) return <Failed error={overview.error} onRetry={() => void overview.refetch()} />

  const { aggregates, triage, schedule } = overview.data

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: tokens.cream }}>
      <SideMenu current="overview" unread={aggregates.unread} />

      <main style={{ flex: 1, minWidth: 0, padding: '34px 40px 60px', maxWidth: 1320 }}>
        {/* The prototype opens with a date, a greeting, a search field and a «Новый пациент» button.
            The greeting is here; the other two are not, and for the reason the side menu drops four
            destinations — searching the roster and creating a patient are things this MVP cannot do
            yet, and a control that does nothing is the dead control invariant 4 forbids. */}
        <header style={{ marginBottom: 28, position: 'relative' }}>
          <h1
            style={{
              fontFamily: tokens.fontDisplay,
              fontSize: 34,
              color: tokens.ink900,
              margin: 0,
              fontWeight: 400,
            }}
          >
            Здравствуйте
            {greetedAs !== '' && (
              <>
                ,{' '}
                <span style={{ fontStyle: 'italic' }}>{greetedAs.split(' ')[0]}</span>
              </>
            )}
          </h1>
          {onSignOut !== undefined && (
            <button
              type="button"
              onClick={onSignOut}
              style={{
                position: 'absolute',
                top: 34,
                right: 40,
                padding: '8px 14px',
                borderRadius: 8,
                border: `1px solid ${tokens.ink300}`,
                background: 'transparent',
                font: 'inherit',
                color: tokens.ink600,
                cursor: 'pointer',
              }}
            >
              Выйти
            </button>
          )}
        </header>

        <StatsStrip aggregates={aggregates} />
        <Triage patients={triage} onOpen={setOpened} />

        <Roster
          page={roster.data}
          onPage={setCursor}
          loading={roster.isFetching}
          error={roster.isError ? roster.error : undefined}
          onRetry={() => void roster.refetch()}
        />

        <Schedule entries={schedule} />
      </main>

      {opened !== null && <PatientCard patient={opened} onClose={() => setOpened(null)} />}
    </div>
  )
}

function Waiting() {
  return (
    <p
      role="status"
      style={{ fontFamily: tokens.fontBody, fontSize: 15, color: tokens.ink500, padding: 40 }}
    >
      Загружаем дашборд…
    </p>
  )
}

function Failed({ error, onRetry }: { error: Error; onRetry: () => void }) {
  return (
    <div role="alert" style={{ fontFamily: tokens.fontBody, padding: 40 }}>
      <p style={{ fontSize: 15, color: tokens.ink800 }}>Не удалось загрузить данные.</p>
      {/* The reason is shown rather than swallowed: a doctor reporting «не грузится» and an engineer
          reading a log is two conversations, and one sentence on screen removes the first. */}
      <p style={{ fontSize: 13, color: tokens.ink500 }}>{error.message}</p>
      <button
        type="button"
        onClick={onRetry}
        style={{
          fontFamily: tokens.fontBody,
          fontSize: 13,
          padding: '8px 16px',
          borderRadius: tokens.rPill,
          border: `1px solid ${tokens.borderStrong}`,
          background: tokens.paper,
          cursor: 'pointer',
        }}
      >
        Повторить
      </button>
    </div>
  )
}
