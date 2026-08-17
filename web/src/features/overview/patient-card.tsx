import type { Patient } from '../../data/overview'
import { Icon } from '../../icons/icon'
import { tokens } from '../../tokens/tokens'
import { FLAGS, STATUS_LABEL } from './flags'
import { FlagPill, GoalBar, Spark } from './patient-bits'

/**
 * One patient, opened beside the roster.
 *
 * Everything it shows arrives worked out: what they have lost, how far along the goal they are, and
 * whether each biomarker is where it should be. The prototype computed the first two here and compared
 * the third against three numbers written into the markup — `dd-app.jsx:214-216` — which is the
 * hard-coding the acceptance criterion moves into the data.
 */
export function PatientCard({ patient, onClose }: { patient: Patient; onClose: () => void }) {
  return (
    <aside
      aria-label={`Карточка: ${patient.name}`}
      style={{
        position: 'fixed',
        top: 0,
        right: 0,
        bottom: 0,
        width: 420,
        maxWidth: '100vw',
        background: tokens.paper,
        borderLeft: `1px solid ${tokens.borderStrong}`,
        boxShadow: tokens.shadowLg,
        padding: '24px 26px',
        overflowY: 'auto',
      }}
    >
      <header style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
        <div>
          <h2 style={{ fontFamily: tokens.fontDisplay, fontSize: 27, color: tokens.ink900, margin: 0 }}>
            {patient.name}
          </h2>
          <div style={{ fontFamily: tokens.fontBody, fontSize: 13, color: tokens.ink500, marginTop: 4 }}>
            {patient.age} лет · {STATUS_LABEL[patient.status]} · был{patient.lastSeen.startsWith('в') ? '' : 'а'} {patient.lastSeen}
          </div>
        </div>
        <button
          type="button"
          onClick={onClose}
          style={{ background: 'none', border: 'none', cursor: 'pointer', color: tokens.ink600, padding: 4 }}
        >
          <Icon name="x-mark" size={20} label="Закрыть карточку" />
        </button>
      </header>

      <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', margin: '16px 0' }}>
        {patient.flags.map((flag) => (
          <FlagPill key={flag} kind={flag} />
        ))}
      </div>

      <p style={{ fontFamily: tokens.fontBody, fontSize: 13.5, color: tokens.ink600, marginTop: 0 }}>
        {patient.note}
      </p>

      <section style={{ marginTop: 22 }}>
        <h3 style={{ fontFamily: tokens.fontBody, fontSize: 12, letterSpacing: '.09em', textTransform: 'uppercase', color: tokens.ink500 }}>
          Протокол
        </h3>
        <div style={{ fontFamily: tokens.fontBody, fontSize: 14, color: tokens.ink800 }}>
          {patient.compound} · {patient.dose} · {patient.cadence}
        </div>
        <div style={{ fontFamily: tokens.fontMono, fontSize: 12.5, color: tokens.ink500, marginTop: 2 }}>
          неделя {patient.week} из {patient.cycleLength} · регулярность {patient.adherence}%
        </div>
      </section>

      <section style={{ marginTop: 22 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontFamily: tokens.fontDisplay, fontSize: 24, color: tokens.ink900 }}>
              {patient.weight} {patient.unit}
            </div>
            <div style={{ fontFamily: tokens.fontBody, fontSize: 12.5, color: tokens.forest600, marginTop: 4 }}>
              ↓ {patient.lostKg} {patient.unit} с начала · цель {patient.goal} {patient.unit}
            </div>
          </div>
          <Spark points={patient.spark} tone={tokens.forest600} />
        </div>
        <div style={{ marginTop: 12 }}>
          <GoalBar patient={patient} />
        </div>
      </section>

      <section style={{ marginTop: 22 }}>
        <h3 style={{ fontFamily: tokens.fontBody, fontSize: 12, letterSpacing: '.09em', textTransform: 'uppercase', color: tokens.ink500 }}>
          Биомаркеры
        </h3>
        <dl style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, margin: '8px 0 0' }}>
          {patient.biomarkers.map((marker) => (
            <div key={marker.key}>
              <dt style={{ fontFamily: tokens.fontBody, fontSize: 11.5, color: tokens.ink500 }}>{marker.label}</dt>
              <dd
                style={{
                  fontFamily: tokens.fontMono,
                  fontSize: 15,
                  margin: '2px 0 0',
                  color: marker.withinRange ? tokens.forest700 : FLAGS.biomarker.fg,
                }}
              >
                {marker.value}
                <span style={{ fontSize: 11, color: tokens.ink500 }}> {marker.unit}</span>
              </dd>
            </div>
          ))}
        </dl>
      </section>
    </aside>
  )
}
