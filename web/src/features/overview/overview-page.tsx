import { useState } from 'react'

import type { Patient, RosterFilter } from '../../data/overview'
import { useOverview, useRoster } from '../../data/queries'
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
export function OverviewPage() {
  const [filter, setFilter] = useState<RosterFilter>('all')
  const [cursor, setCursor] = useState<string | null>(null)
  const [opened, setOpened] = useState<Patient | null>(null)

  const overview = useOverview()
  const roster = useRoster({ filter, cursor })

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
        <header style={{ marginBottom: 28 }}>
          <h1
            style={{
              fontFamily: tokens.fontDisplay,
              fontSize: 34,
              color: tokens.ink900,
              margin: 0,
              fontWeight: 400,
            }}
          >
            Здравствуйте,{' '}
            <span style={{ fontStyle: 'italic' }}>{overview.data.doctor.name.split(' ')[0]}</span>
          </h1>
        </header>

        <StatsStrip aggregates={aggregates} />
        <Triage patients={triage} onOpen={setOpened} />

        <Roster
            page={roster.data}
            aggregates={aggregates}
            filter={filter}
            onFilter={(next) => {
              // The cursor belongs to the filter it was taken from: kept across a tab change it names a
              // patient the new filter has never heard of, and the seam refuses it.
              setFilter(next)
              setCursor(null)
            }}
            onPage={setCursor}
            onOpen={setOpened}
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
