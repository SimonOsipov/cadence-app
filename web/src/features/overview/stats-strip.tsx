import { Icon } from '../../icons/icon'
import type { IconName } from '../../icons/icons'
import type { OverviewAggregates } from '../../data/overview'
import { whole } from '../../format'
import { tokens } from '../../tokens/tokens'

/**
 * The five numbers across the top, every one of them handed down.
 *
 * The prototype builds this strip out of expressions over the patient array — including the average
 * adherence invariant 2 names by itself. None of that arithmetic is ported; the lint rule in
 * eslint.config.js is what keeps it from coming back.
 */
export function StatsStrip({ aggregates }: { aggregates: OverviewAggregates }) {
  const cards: Array<{ label: string; value: string; unit?: string; sub: string; icon: IconName }> = [
    { label: 'Пациентов', value: whole(aggregates.patients), sub: 'активных протоколов', icon: 'user' },
    { label: 'Внимание', value: whole(aggregates.attention), sub: 'пациента ожидают', icon: 'exclamation-circle' },
    {
      label: 'Дозы сегодня',
      value: `${whole(aggregates.dosesDone)}/${whole(aggregates.dosesToday)}`,
      sub: 'выполнено',
      icon: 'beaker',
    },
    { label: 'Сообщения', value: whole(aggregates.unread), sub: 'без ответа', icon: 'chat-bubble' },
    {
      label: 'Регулярность',
      value: whole(aggregates.averageAdherence),
      unit: '%',
      sub: 'в среднем',
      icon: 'check-circle',
    },
  ]

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: tokens.s5,
        marginBottom: 36,
      }}
    >
      {cards.map((card) => (
        <article
          key={card.label}
          style={{
            background: tokens.paper,
            border: `1px solid ${tokens.bone}`,
            borderRadius: tokens.rLg,
            padding: '16px 18px',
            boxShadow: tokens.shadowXs,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: tokens.ink500 }}>
            <Icon name={card.icon} size={16} />
            <span style={{ fontFamily: tokens.fontBody, fontSize: 12.5 }}>{card.label}</span>
          </div>

          <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginTop: 10 }}>
            <span style={{ fontFamily: tokens.fontDisplay, fontSize: 30, color: tokens.ink900 }}>
              {card.value}
            </span>
            {card.unit !== undefined && (
              <span style={{ fontFamily: tokens.fontDisplay, fontStyle: 'italic', fontSize: 17, color: tokens.ink600 }}>
                {card.unit}
              </span>
            )}
          </div>

          <div style={{ fontFamily: tokens.fontBody, fontSize: 12, color: tokens.ink500, marginTop: 2 }}>
            {card.sub}
          </div>
        </article>
      ))}
    </div>
  )
}
