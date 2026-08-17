import type { Patient, ScheduleEntry } from '../../data/overview'
import { Icon } from '../../icons/icon'
import { tokens } from '../../tokens/tokens'
import { SectionHead } from './patient-bits'

const STATE = {
  done: { label: 'выполнено', colour: tokens.forest600 },
  now: { label: 'сейчас', colour: tokens.sand700 },
  due: { label: 'предстоит', colour: tokens.ink500 },
} as const

export function Schedule({
  entries,
  patients,
}: {
  entries: readonly ScheduleEntry[]
  patients: ReadonlyMap<string, Patient>
}) {
  return (
    <section aria-label="Расписание">
      <SectionHead eyebrow="Сегодня" title="Расписание" />

      <div
        style={{
          background: tokens.paper,
          border: `1px solid ${tokens.bone}`,
          borderRadius: tokens.rLg,
          padding: '20px 24px',
          boxShadow: tokens.shadowXs,
        }}
      >
        {entries.length === 0 ? (
          <p style={{ fontFamily: tokens.fontBody, fontSize: 14, color: tokens.ink500, margin: 0 }}>
            На сегодня ничего не назначено.
          </p>
        ) : (
          <ul
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
              gap: 14,
              listStyle: 'none',
              margin: 0,
              padding: 0,
            }}
          >
            {entries.map((entry) => {
              const state = STATE[entry.state]
              const patient = patients.get(entry.patientId)

              return (
                <li key={entry.id} style={{ display: 'flex', gap: 11, alignItems: 'flex-start' }}>
                  <span style={{ color: state.colour, marginTop: 2 }}>
                    <Icon name={entry.kind === 'dose' ? 'beaker' : 'clock'} size={16} />
                  </span>
                  <div>
                    <div style={{ fontFamily: tokens.fontMono, fontSize: 12, color: tokens.ink500 }}>
                      {entry.at}
                    </div>
                    <div style={{ fontFamily: tokens.fontBody, fontSize: 13.5, color: tokens.ink800 }}>
                      {patient?.name ?? entry.patientId}
                    </div>
                    <div style={{ fontFamily: tokens.fontBody, fontSize: 12.5, color: tokens.ink600 }}>
                      {entry.label}
                    </div>
                    <div style={{ fontFamily: tokens.fontBody, fontSize: 11.5, color: state.colour }}>
                      {state.label}
                    </div>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </section>
  )
}
