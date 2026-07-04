// Cadence — Chat landing (care-team thread list).
// Exposes: ChatLanding

function ChatLanding({ pal, platform, onBack, onOpenThread }) {
  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 30,
      }}>
        {/* Top bar */}
        <div style={{
          padding: '8px 16px 10px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <button onClick={onBack} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none',
            background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{
            fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.muted,
          }}>Команда</div>
          <div style={{ width: 40 }} />
        </div>

        {/* Hero */}
        <div style={{ padding: '4px 24px 20px' }}>
          <Eyebrow style={{ color: pal.subtle, marginBottom: 6 }}>Связь с командой</Eyebrow>
          <div style={{
            fontFamily: F.display, fontSize: 32, color: pal.ink,
            lineHeight: 1.05, letterSpacing: '-0.018em',
          }}>
            Кому <span style={{ fontStyle: 'italic', color: C.forest700 }}>напишем</span>?
          </div>
        </div>

        {/* Thread list */}
        <div style={{ padding: '0 16px', display: 'flex', flexDirection: 'column', gap: 10 }}>
          {CARE_TEAM.map(t => (
            <button
              key={t.id}
              onClick={() => onOpenThread(t.id)}
              className="press"
              style={{
                display: 'grid', gridTemplateColumns: '52px 1fr auto',
                gap: 14, alignItems: 'center',
                background: pal.paper, border: `1px solid ${pal.hairline}`,
                borderRadius: 18, padding: 14, cursor: 'pointer',
                textAlign: 'left',
                boxShadow: '0 2px 6px rgba(46,38,24,.04)',
              }}
            >
              <div style={{
                width: 52, height: 52, borderRadius: 999,
                background: t.avatarBg, color: t.avatarFg,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontFamily: F.display, fontStyle: 'italic', fontSize: 22,
              }}>{t.initial}</div>
              <div style={{ minWidth: 0 }}>
                <div style={{
                  display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
                  gap: 8, marginBottom: 4,
                }}>
                  <span style={{
                    fontFamily: F.body, fontSize: 14, fontWeight: 500, color: pal.ink,
                    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                  }}>{t.name}</span>
                  <span style={{
                    fontFamily: F.mono, fontSize: 10.5, color: pal.subtle,
                    fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap',
                  }}>{t.preview.time}</span>
                </div>
                <div style={{
                  fontFamily: F.body, fontSize: 11.5, color: C.sand700,
                  marginBottom: 4, fontWeight: 500,
                }}>{t.role}</div>
                <div style={{
                  fontFamily: F.body, fontSize: 12.5, color: pal.muted, lineHeight: 1.4,
                  overflow: 'hidden', textOverflow: 'ellipsis',
                  display: '-webkit-box', WebkitLineClamp: 1, WebkitBoxOrient: 'vertical',
                }}>{t.preview.text}</div>
              </div>
              {t.preview.unread > 0 ? (
                <span style={{
                  width: 22, height: 22, borderRadius: 999,
                  background: C.forest700, color: C.cream,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontFamily: F.mono, fontSize: 11, fontWeight: 500,
                  fontVariantNumeric: 'tabular-nums',
                }}>{t.preview.unread}</span>
              ) : (
                <Icon name="chevron-right" size={16} color={pal.subtle} />
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { ChatLanding });
