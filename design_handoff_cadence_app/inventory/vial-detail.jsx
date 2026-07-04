// Cadence — Vial detail bottom sheet.
// Full info + actions: open / dispose / edit / set active / move to spare /
// view dose history / add photo.

function VialDetailSheet({ open, vial, pal, onClose, onActivate, onMarkOpened, onDispose, onMoveToSpare, onEdit, onAddPhoto, onLogDoseFromVial }) {
  if (!open || !vial) return null;
  const meta = vial.compoundMeta;
  const status = vial.status;

  const statusPill = (() => {
    if (status === 'expiring') return { label: 'Истекает скоро', bg: '#fbeed1',   fg: '#c2780a' };
    if (status === 'low')      return { label: 'На исходе',      bg: '#f4dfd6',   fg: '#b8503c' };
    if (status === 'sealed')   return { label: 'Запечатан',      bg: C.sand100,   fg: '#6b4a25' };
    return                            { label: 'Активный',       bg: C.forest50,  fg: C.forest800 };
  })();

  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 80 }}>
      <div className="scrim" onClick={onClose} style={{
        position: 'absolute', inset: 0,
        background: 'rgba(20,44,31,.35)',
        backdropFilter: 'blur(4px)',
      }} />
      <div className="sheet ds-scroll" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28,
        padding: '12px 0 28px',
        maxHeight: '92%',
        overflowY: 'auto', overflowX: 'hidden',
        boxShadow: '0 -18px 40px rgba(0,0,0,.18)',
      }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '0 auto 14px' }} />

        {/* Hero */}
        <div style={{ padding: '0 20px 18px' }}>
          <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start',
            marginBottom: 14, gap: 12,
          }}>
            <div style={{ minWidth: 0, flex: 1 }}>
              <Eyebrow style={{ color: pal.subtle, marginBottom: 6 }}>Флакон · лот {vial.lot}</Eyebrow>
              <div style={{
                fontFamily: F.display, fontSize: 28, color: pal.ink,
                lineHeight: 1.05, letterSpacing: '-0.018em',
              }}>
                {meta.name}<br/>
                <span style={{ fontStyle: 'italic', color: C.forest700 }}>{vial.dose}</span>
              </div>
            </div>
            <button onClick={onClose} style={{
              width: 36, height: 36, borderRadius: 999, border: 'none', cursor: 'pointer',
              background: pal.sunk, color: pal.ink2, display: 'flex',
              alignItems: 'center', justifyContent: 'center', flexShrink: 0,
            }}>
              <Icon name="x-mark" size={18} />
            </button>
          </div>
          <span style={{
            padding: '4px 10px', borderRadius: 999,
            fontFamily: F.body, fontSize: 11, fontWeight: 500,
            background: statusPill.bg, color: statusPill.fg,
            display: 'inline-block',
          }}>{statusPill.label}</span>
        </div>

        {/* Doses bar — only if opened */}
        {vial.opened && (
          <div style={{ padding: '0 20px 16px' }}>
            <div style={{
              background: pal.paper, borderRadius: 16, padding: 14,
              border: `1px solid ${pal.hairline}`,
            }}>
              <div style={{
                display: 'flex', alignItems: 'baseline', justifyContent: 'space-between',
                marginBottom: 8, gap: 12,
              }}>
                <div>
                  <Eyebrow style={{ color: pal.subtle, marginBottom: 4, fontSize: 10 }}>Доз осталось</Eyebrow>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
                    <span style={{
                      fontFamily: F.mono, fontSize: 28, fontWeight: 500,
                      color: pal.ink, letterSpacing: '-0.025em',
                      fontVariantNumeric: 'tabular-nums', lineHeight: 1,
                    }}>{vial.remaining}</span>
                    <span style={{
                      fontFamily: F.body, fontSize: 13, color: pal.subtle,
                    }}>/ {vial.total}</span>
                  </div>
                </div>
                <Pill tone="forest" style={{ fontSize: 11 }}>{Math.round(vial.pct * 100)}%</Pill>
              </div>
              <div style={{
                height: 6, background: pal.sunk, borderRadius: 999, overflow: 'hidden',
              }}>
                <div style={{
                  width: `${vial.pct * 100}%`, height: '100%',
                  background:
                    status === 'expiring' ? '#c2780a' :
                    status === 'low'      ? '#b8503c' :
                                            C.forest700,
                  borderRadius: 999,
                }} />
              </div>
            </div>
          </div>
        )}

        {/* Key facts grid */}
        <div style={{ padding: '0 20px 16px' }}>
          <div style={{
            display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10,
          }}>
            <FactCard
              label="Открыт"
              value={vial.opened ? `${vial.daysSinceOpened} ${vial.daysSinceOpened === 1 ? 'день' : vial.daysSinceOpened < 5 ? 'дня' : 'дней'} назад` : 'Запечатан'}
              sub={vial.opened ? vial.openedLabel : null}
              pal={pal}
            />
            <FactCard
              label="Истекает"
              value={`${vial.daysToExpiry} ${vial.daysToExpiry === 1 ? 'день' : vial.daysToExpiry < 5 ? 'дня' : 'дней'}`}
              sub={vial.expiresLabel}
              pal={pal}
              accent={vial.daysToExpiry <= 14 ? '#c2780a' : null}
            />
            <FactCard
              label="Лот"
              value={vial.lot}
              mono
              pal={pal}
            />
            <FactCard
              label="Хранится"
              value={vial.location.split(',')[1]?.trim() || vial.location}
              sub={vial.location.split(',')[0]}
              pal={pal}
            />
          </div>
        </div>

        {/* Recent doses */}
        {vial.recent && vial.recent.length > 0 && (
          <div style={{ padding: '0 20px 16px' }}>
            <div style={{
              display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
              padding: '0 0 10px',
            }}>
              <Eyebrow style={{ color: pal.subtle }}>Последние записи</Eyebrow>
              <span style={{
                fontFamily: F.mono, fontSize: 11, color: pal.subtle,
                fontVariantNumeric: 'tabular-nums',
              }}>{vial.recent.length}</span>
            </div>
            <div style={{
              background: pal.paper, borderRadius: 16,
              border: `1px solid ${pal.hairline}`,
            }}>
              {vial.recent.map((r, i, arr) => (
                <React.Fragment key={i}>
                  <div style={{
                    display: 'grid', gridTemplateColumns: '1fr auto auto',
                    gap: 12, alignItems: 'center', padding: '11px 14px',
                  }}>
                    <div>
                      <div style={{ fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.ink }}>
                        {formatDayLabel(r.day)}
                      </div>
                      <div style={{
                        fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 1,
                      }}>{r.site}</div>
                    </div>
                    <span style={{
                      fontFamily: F.mono, fontSize: 12, color: pal.ink2,
                      fontVariantNumeric: 'tabular-nums',
                    }}>{r.dose}</span>
                  </div>
                  {i < arr.length - 1 && <div style={{ height: 1, background: pal.hairline, marginLeft: 14 }} />}
                </React.Fragment>
              ))}
            </div>
          </div>
        )}

        {/* Action buttons */}
        <div style={{ padding: '0 20px' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {!vial.opened && (
              <ActionRow
                icon="check-circle"
                tone="primary"
                title="Открыть флакон"
                sub="Начать отсчёт сроков и доз"
                onClick={() => onMarkOpened && onMarkOpened(vial.id)}
                pal={pal}
              />
            )}
            {vial.opened && (
              <ActionRow
                icon="paper-airplane"
                tone="primary"
                title="Записать дозу из этого флакона"
                sub="Откроем мастер с пред-заполненным флаконом"
                onClick={() => onLogDoseFromVial && onLogDoseFromVial(vial.id)}
                pal={pal}
              />
            )}
            <ActionRow
              icon="pencil"
              title="Изменить лот, дату или дозу"
              onClick={() => onEdit && onEdit(vial.id)}
              pal={pal}
            />
            <ActionRow
              icon="camera"
              title="Прикрепить фото"
              sub="Этикетка или коробка"
              onClick={() => onAddPhoto && onAddPhoto(vial.id)}
              pal={pal}
            />
            {vial.opened ? (
              <ActionRow
                icon="trash"
                tone="danger"
                title="Утилизировать"
                sub="Флакон пуст или испорчен"
                onClick={() => onDispose && onDispose(vial.id)}
                pal={pal}
              />
            ) : (
              <ActionRow
                icon="arrow-path"
                title="Перенести в запас"
                onClick={() => onMoveToSpare && onMoveToSpare(vial.id)}
                pal={pal}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ── FactCard — labelled stat tile ───────────────────────────────────

function FactCard({ label, value, sub, mono, accent, pal }) {
  return (
    <div style={{
      background: pal.paper, borderRadius: 14, padding: 12,
      border: `1px solid ${pal.hairline}`,
    }}>
      <Eyebrow style={{ color: pal.subtle, fontSize: 10, marginBottom: 4 }}>{label}</Eyebrow>
      <div style={{
        fontFamily: mono ? F.mono : F.body, fontSize: 14, fontWeight: 500,
        color: accent || pal.ink,
        fontVariantNumeric: mono ? 'tabular-nums' : 'normal',
        letterSpacing: mono ? '-0.01em' : 'normal',
      }}>{value}</div>
      {sub && (
        <div style={{
          fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2,
        }}>{sub}</div>
      )}
    </div>
  );
}

// ── ActionRow — one action button in the sheet ──────────────────────

function ActionRow({ icon, title, sub, tone, onClick, pal }) {
  const palette =
    tone === 'primary' ? { bg: pal.paper, fg: pal.ink, accent: C.forest700, accentBg: C.forest50 }
    : tone === 'danger' ? { bg: pal.paper, fg: '#b8503c', accent: '#b8503c', accentBg: '#f4dfd6' }
    :                     { bg: pal.paper, fg: pal.ink, accent: pal.ink2, accentBg: pal.sunk };
  return (
    <button
      onClick={onClick}
      className="press"
      style={{
        display: 'grid', gridTemplateColumns: '40px 1fr auto',
        gap: 12, alignItems: 'center',
        width: '100%', background: palette.bg, border: `1px solid ${pal.hairline}`,
        borderRadius: 14, padding: 12, cursor: 'pointer',
        textAlign: 'left',
      }}
    >
      <div style={{
        width: 40, height: 40, borderRadius: 12,
        background: palette.accentBg, color: palette.accent,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <Icon name={icon} size={18} />
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{
          fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: palette.fg,
        }}>{title}</div>
        {sub && (
          <div style={{
            fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 1, lineHeight: 1.35,
          }}>{sub}</div>
        )}
      </div>
      <Icon name="chevron-right" size={16} color={pal.subtle} />
    </button>
  );
}

Object.assign(window, { VialDetailSheet });
