import { useMemo, useState } from 'react'

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

  // The schedule names patients by id; this is the lookup, and it is a lookup rather than a search
  // through the rows on every line.
  const byId = useMemo(
    () => new Map((roster.data?.items ?? []).concat(overview.data?.triage ?? []).map((p) => [p.id, p])),
    [roster.data, overview.data],
  )

  if (overview.isPending) return <Waiting />
  if (overview.isError) return <Failed error={overview.error} onRetry={() => void overview.refetch()} />

  const { aggregates, triage, schedule } = overview.data

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: tokens.cream }}>
      <SideMenu current="overview" unread={aggregates.unread} />

      <main style={{ flex: 1, minWidth: 0, padding: '34px 40px 60px', maxWidth: 1320 }}>
        <StatsStrip aggregates={aggregates} />
        <Triage patients={triage} onOpen={setOpened} />

        {roster.isError ? (
          <Failed error={roster.error} onRetry={() => void roster.refetch()} />
        ) : (
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
          />
        )}

        <Schedule entries={schedule} patients={byId} />
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
          borderRadius: 999,
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
