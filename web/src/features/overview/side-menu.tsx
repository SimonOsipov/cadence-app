import { Icon } from '../../icons/icon'
import { tokens } from '../../tokens/tokens'

/**
 * Two destinations and no more.
 *
 * The prototype draws six; «Пациенты», «Расписание», «Аналитика» and «Протоколы» are outside the MVP
 * and are removed rather than shown disabled — invariant 4 of the component note forbids a dead
 * control by the end of M10, and honouring it now is cheaper than finding them all later. The roster
 * below *is* the patient list, so «Пациенты» would be a second door to one room.
 */
const DESTINATIONS = [
  { id: 'overview', icon: 'home', label: 'Обзор' },
  { id: 'messages', icon: 'chat-bubble', label: 'Сообщения' },
] as const

export function SideMenu({ current, unread }: { current: string; unread: number }) {
  return (
    <aside
      aria-label="Боковая панель"
      style={{
        width: 248,
        flexShrink: 0,
        minHeight: '100vh',
        position: 'sticky',
        top: 0,
        background: tokens.forest900,
        color: tokens.cream,
        display: 'flex',
        flexDirection: 'column',
        padding: '22px 16px',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 11, padding: '4px 8px 22px' }}>
        <span style={{ fontFamily: tokens.fontDisplay, fontSize: 22, letterSpacing: '.01em' }}>Cadence</span>
      </div>

      <nav aria-label="Разделы" style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {DESTINATIONS.map((destination) => {
          const here = destination.id === current

          return (
            <a
              key={destination.id}
              href={`/${destination.id}`}
              aria-current={here ? 'page' : undefined}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 11,
                padding: '10px 12px',
                borderRadius: tokens.rMd,
                textDecoration: 'none',
                fontFamily: tokens.fontBody,
                fontSize: 14,
                color: here ? tokens.cream : tokens.forest300,
                background: here ? tokens.forest800 : 'transparent',
              }}
            >
              <Icon name={destination.icon} size={18} />
              {destination.label}
              {destination.id === 'messages' && unread > 0 && (
                <span
                  aria-label={`${unread} без ответа`}
                  style={{
                    marginLeft: 'auto',
                    fontFamily: tokens.fontMono,
                    fontSize: 11,
                    background: tokens.sand500,
                    color: tokens.forest900,
                    borderRadius: 999,
                    padding: '1px 7px',
                  }}
                >
                  {unread}
                </span>
              )}
            </a>
          )
        })}
      </nav>
    </aside>
  )
}
