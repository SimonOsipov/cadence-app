import type { Patient } from '../../data/overview'
import { tokens } from '../../tokens/tokens'
import { FlagPill, GoalBar, SectionHead, Spark } from './patient-bits'

/**
 * The patients needing attention, chosen on the far side of the seam.
 *
 * The prototype filters the whole array here in the render. What arrives now is `overview.triage`,
 * already the right people — which is also what lets the server decide, later, that «needs attention»
 * means something more than a status column.
 */
export function Triage({ patients, onOpen }: { patients: readonly Patient[]; onOpen: (patient: Patient) => void }) {
  return (
    <section style={{ marginBottom: 40 }} aria-label="Требуют внимания">
      <SectionHead eyebrow="Триаж" title="Требуют внимания" />

      {patients.length === 0 ? (
        <p style={{ fontFamily: tokens.fontBody, fontSize: 14, color: tokens.ink500 }}>
          Никто не ждёт решения — на сегодня всё разобрано.
        </p>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: tokens.s5 }}>
          {patients.map((patient) => (
            <article
              key={patient.id}
              style={{
                background: tokens.paper,
                border: `1px solid ${tokens.borderStrong}`,
                borderRadius: tokens.rLg,
                padding: '16px 18px',
                boxShadow: tokens.shadowXs,
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10 }}>
                <button
                  type="button"
                  onClick={() => onOpen(patient)}
                  style={{
                    fontFamily: tokens.fontDisplay,
                    fontSize: 19,
                    color: tokens.ink900,
                    background: 'none',
                    border: 'none',
                    padding: 0,
                    cursor: 'pointer',
                  }}
                >
                  {patient.name}
                </button>
                <Spark points={patient.spark} tone={tokens.danger} />
              </div>

              <p style={{ fontFamily: tokens.fontBody, fontSize: 13, color: tokens.ink600, margin: '8px 0 12px' }}>
                {patient.note}
              </p>

              <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 12 }}>
                {patient.flags.map((flag) => (
                  <FlagPill key={flag} kind={flag} small />
                ))}
              </div>

              <GoalBar patient={patient} />
            </article>
          ))}
        </div>
      )}
    </section>
  )
}
