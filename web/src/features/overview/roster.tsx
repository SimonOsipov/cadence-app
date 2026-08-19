import type { RosterPage, RosterRow } from '../../api'
import { Icon } from '../../icons/icon'
import { tokens } from '../../tokens/tokens'
import { whole } from '../../format'
import { SectionHead } from './patient-bits'

/**
 * The roster, live, one page at a time.
 *
 * It draws what `/v1/dashboard/overview` answers and nothing else. The protocol, the cycle and the
 * weight the prototype's table carries are M6's — they belong to the dosing and measurements
 * contexts, which have no endpoint yet — and so are the status tabs above it and the card a row used
 * to open. A column filled with «—» would be a promise; a tab that filters nothing would be the dead
 * control invariant 4 forbids. They come back when the data does.
 *
 * There is no total either, and that is keyset paging rather than an omission: the set moves under a
 * doctor as assignments change, and a count taken at one page is wrong by the next.
 */
export function Roster({
  page,
  loading,
  onPage,
  error,
  onRetry,
}: {
  page: RosterPage | undefined
  loading: boolean
  onPage: (cursor: string | null) => void

  /** Shown in place of the rows. The pager stays: a doctor whose journal failed still has to be able
   * to go back to the first page, and replacing the whole section takes that away. */
  error?: Error | undefined
  onRetry?: (() => void) | undefined
}) {
  return (
    <section style={{ marginBottom: 40 }} aria-label="Журнал протоколов">
      <SectionHead eyebrow="Все пациенты" title="Журнал протоколов" />

      <div
        role="table"
        aria-label="Пациенты"
        style={{
          background: tokens.paper,
          border: `1px solid ${tokens.bone}`,
          borderRadius: tokens.rLg,
          boxShadow: tokens.shadowXs,
          overflow: 'hidden',
        }}
      >
        <div role="row" style={{ ...gridRow, padding: '12px 18px', borderBottom: `1px solid ${tokens.bone}`, background: tokens.cream }}>
          {['Пациент', 'Возраст', 'Приглашение'].map((heading) => (
            <div
              key={heading}
              role="columnheader"
              style={{
                fontFamily: tokens.fontBody,
                fontSize: 11,
                fontWeight: 500,
                letterSpacing: '.1em',
                textTransform: 'uppercase',
                color: tokens.ink500,
              }}
            >
              {heading}
            </div>
          ))}
        </div>

        {error !== undefined ? (
          <div role="alert" style={{ padding: '36px 18px', textAlign: 'center', fontFamily: tokens.fontBody }}>
            <p style={{ color: tokens.ink800, margin: 0 }}>Не удалось загрузить журнал.</p>
            <p style={{ color: tokens.ink500, fontSize: 13 }}>{error.message}</p>
            {onRetry !== undefined && (
              <button type="button" onClick={onRetry} style={pagerStyle}>
                Повторить
              </button>
            )}
          </div>
        ) : page === undefined ? (
          <p style={{ padding: '44px 18px', textAlign: 'center', fontFamily: tokens.fontBody, color: tokens.ink500 }}>
            Загружаем журнал…
          </p>
        ) : page.patients.length === 0 ? (
          <p style={{ padding: '44px 18px', textAlign: 'center', fontFamily: tokens.fontBody, color: tokens.ink500 }}>
            Пациентов пока нет — создайте первого.
          </p>
        ) : (
          page.patients.map((patient) => <Row key={patient.user_id} patient={patient} />)
        )}
      </div>

      <nav
        aria-label="Страницы журнала"
        style={{ display: 'flex', alignItems: 'center', gap: tokens.s4, marginTop: 12 }}
      >
        <button type="button" onClick={() => onPage(null)} disabled={loading} style={pagerStyle}>
          В начало
        </button>
        <button
          type="button"
          onClick={() => onPage(page?.next_cursor ?? null)}
          disabled={loading || page?.next_cursor === undefined}
          style={pagerStyle}
        >
          Дальше
          <Icon name="chevron-right" size={14} />
        </button>
        <span style={{ fontFamily: tokens.fontBody, fontSize: 12.5, color: tokens.ink500 }}>
          {page === undefined ? '' : `${whole(page.patients.length)} на странице`}
        </span>
      </nav>
    </section>
  )
}

function Row({ patient }: { patient: RosterRow }) {
  return (
    <div role="row" style={{ ...gridRow, padding: '14px 18px', borderBottom: `1px solid ${tokens.linen}`, fontFamily: tokens.fontBody }}>
      <span role="cell" style={{ color: tokens.ink900, fontSize: 14 }}>
        {patient.full_name}
      </span>
      {/* Absent rather than nought: the server answers null for a patient whose date of birth the
          clinic never entered, and «0 лет» is a number this screen would be inventing. */}
      <span role="cell" style={{ color: tokens.ink600, fontSize: 13, fontFamily: tokens.fontMono }}>
        {patient.age === null ? '' : `${whole(patient.age)} лет`}
      </span>
      <span role="cell" style={{ color: tokens.ink600, fontSize: 13 }}>
        {INVITE_STATES[patient.invite_state]}
      </span>
    </div>
  )
}

/**
 * What each state of an invitation says to a doctor.
 *
 * «Статус неизвестен» is the honest one: unknown means the identity provider could not be asked, and
 * a row drawn as «отправлено» would be a claim nobody made.
 */
const INVITE_STATES: Record<RosterRow['invite_state'], string> = {
  accepted: 'В приложении',
  pending: 'Приглашение отправлено',
  expired: 'Приглашение истекло',
  unknown: 'Статус неизвестен',
}

const gridRow = {
  display: 'grid',
  gridTemplateColumns: '2fr 0.8fr 1.2fr',
  gap: tokens.s5,
  alignItems: 'center',
} as const

const pagerStyle = {
  fontFamily: tokens.fontBody,
  fontSize: 13,
  padding: '7px 13px',
  borderRadius: tokens.rPill,
  border: `1px solid ${tokens.borderStrong}`,
  background: 'transparent',
  color: tokens.ink600,
  cursor: 'pointer',
  display: 'inline-flex',
  alignItems: 'center',
  gap: 6,
}
