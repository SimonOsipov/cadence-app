import type { OverviewAggregates, Patient, RosterFilter, RosterPage } from '../../data/overview'
import { Icon } from '../../icons/icon'
import { tokens } from '../../tokens/tokens'
import { SectionHead, StatusDot } from './patient-bits'

/**
 * The roster, one page at a time.
 *
 * The tab counters come from the aggregates rather than from the rows, and the rows are a page rather
 * than the list. The prototype does both the other way round — four `.filter().length` calls in the
 * render, and the whole array filtered and sorted on the client — which is exactly what stops working
 * the day the server owns the query.
 */
const TABS: Array<{ id: RosterFilter; label: string; of: keyof OverviewAggregates['byStatus'] }> = [
  { id: 'all', label: 'Все', of: 'all' },
  { id: 'attention', label: 'Внимание', of: 'attention' },
  { id: 'watch', label: 'Наблюдение', of: 'watch' },
  { id: 'track', label: 'В норме', of: 'track' },
]

export function Roster({
  page,
  aggregates,
  filter,
  onFilter,
  onPage,
  onOpen,
  loading,
}: {
  page: RosterPage | undefined
  aggregates: OverviewAggregates
  filter: RosterFilter
  onFilter: (filter: RosterFilter) => void
  onPage: (cursor: string | null) => void
  onOpen: (patient: Patient) => void
  loading: boolean
}) {
  return (
    <section style={{ marginBottom: 40 }} aria-label="Журнал протоколов">
      <SectionHead eyebrow="Все пациенты" title="Журнал протоколов" />

      <div role="tablist" aria-label="Фильтр" style={{ display: 'flex', gap: 7, flexWrap: 'wrap', marginBottom: 14 }}>
        {TABS.map((tab) => {
          const on = tab.id === filter

          return (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={on}
              onClick={() => onFilter(tab.id)}
              style={{
                fontFamily: tokens.fontBody,
                fontSize: 13,
                fontWeight: 500,
                padding: '7px 14px',
                borderRadius: 999,
                cursor: 'pointer',
                background: on ? tokens.ink900 : 'transparent',
                color: on ? tokens.cream : tokens.ink600,
                border: `1px solid ${on ? tokens.ink900 : tokens.borderStrong}`,
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
              }}
            >
              {tab.label}
              <span style={{ fontFamily: tokens.fontMono, fontSize: 11, opacity: 0.7 }}>
                {aggregates.byStatus[tab.of]}
              </span>
            </button>
          )
        })}
      </div>

      <div
        style={{
          background: tokens.paper,
          border: `1px solid ${tokens.bone}`,
          borderRadius: tokens.rLg,
          boxShadow: tokens.shadowXs,
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1.7fr 1.25fr 0.8fr 1.15fr',
            gap: 16,
            padding: '12px 18px',
            borderBottom: `1px solid ${tokens.bone}`,
            background: tokens.cream,
          }}
        >
          {['Пациент', 'Протокол', 'Цикл', 'Вес'].map((heading) => (
            <div
              key={heading}
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

        {page === undefined ? (
          <p style={{ padding: '44px 18px', textAlign: 'center', fontFamily: tokens.fontBody, color: tokens.ink500 }}>
            Загружаем журнал…
          </p>
        ) : page.items.length === 0 ? (
          <p style={{ padding: '44px 18px', textAlign: 'center', fontFamily: tokens.fontBody, color: tokens.ink500 }}>
            Никого не нашлось — попробуйте другой фильтр.
          </p>
        ) : (
          page.items.map((patient) => (
            <button
              key={patient.id}
              type="button"
              onClick={() => onOpen(patient)}
              style={{
                display: 'grid',
                gridTemplateColumns: '1.7fr 1.25fr 0.8fr 1.15fr',
                gap: 16,
                width: '100%',
                textAlign: 'left',
                padding: '14px 18px',
                borderBottom: `1px solid ${tokens.linen}`,
                background: 'none',
                border: 'none',
                cursor: 'pointer',
                fontFamily: tokens.fontBody,
              }}
            >
              <span style={{ display: 'flex', alignItems: 'center', gap: 9, color: tokens.ink900, fontSize: 14 }}>
                <StatusDot status={patient.status} />
                {patient.name}
              </span>
              <span style={{ color: tokens.ink600, fontSize: 13 }}>
                {patient.compound} · {patient.dose}
              </span>
              <span style={{ color: tokens.ink600, fontSize: 13, fontFamily: tokens.fontMono }}>
                {patient.week}/{patient.cycleLength}
              </span>
              <span style={{ color: tokens.ink600, fontSize: 13 }}>
                {patient.weight} {patient.unit} · ↓{patient.lostKg}
              </span>
            </button>
          ))
        )}
      </div>

      <nav
        aria-label="Страницы журнала"
        style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 12 }}
      >
        <button
          type="button"
          onClick={() => onPage(null)}
          disabled={loading}
          style={pagerStyle}
        >
          В начало
        </button>
        <button
          type="button"
          onClick={() => onPage(page?.cursor ?? null)}
          disabled={loading || page?.cursor == null}
          style={pagerStyle}
        >
          Дальше
          <Icon name="chevron-right" size={14} />
        </button>
        <span style={{ fontFamily: tokens.fontBody, fontSize: 12.5, color: tokens.ink500 }}>
          {page === undefined ? '' : `${page.items.length} из ${page.total}`}
        </span>
      </nav>
    </section>
  )
}

const pagerStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: 5,
  fontFamily: tokens.fontBody,
  fontSize: 13,
  padding: '7px 13px',
  borderRadius: 999,
  border: `1px solid ${tokens.borderStrong}`,
  background: tokens.paper,
  color: tokens.ink800,
  cursor: 'pointer',
} as const
