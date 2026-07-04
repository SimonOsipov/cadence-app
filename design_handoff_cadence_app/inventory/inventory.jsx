// Cadence — Аптечка (Vials) landing screen.
// Exposes: VialsScreen, VialCard, VialRow

function VialsScreen({ pal, platform, onBack, onOpenVial, onAddVial, onChangeTab, sealedOpen, setSealedOpen }) {
  const [filter, setFilter] = React.useState('all');
  const inv = VIAL_INVENTORY;
  const sum = inventorySummary(inv);

  // Filter inventory based on chip
  const filterVial = (v) => {
    if (filter === 'all')     return true;
    if (filter === 'active')  return v.opened !== null && v.status !== 'expiring';
    if (filter === 'expiring')return v.status === 'expiring';
    if (filter === 'sealed')  return v.opened === null;
    return v.compound === filter;
  };

  const active = inv.filter(v => v.opened !== null).filter(filterVial);
  const sealed = inv.filter(v => v.opened === null).filter(filterVial);

  // Distinct compounds for chip filter
  const compounds = Array.from(new Set(inv.map(v => v.compound)));

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 130,
      }}>
        {/* Top bar */}
        <div style={{
          padding: '12px 20px 14px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 12,
        }}>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginBottom: 4 }}>
              {inv.length} {inv.length === 1 ? 'флакон' : inv.length < 5 ? 'флакона' : 'флаконов'} в холодильнике
            </div>
            <div style={{
              fontFamily: F.display, fontSize: 36, color: pal.ink,
              lineHeight: 1.0, letterSpacing: '-0.018em',
            }}>
              Ваша <span style={{ fontStyle: 'italic', color: C.forest700 }}>аптечка</span>
            </div>
          </div>
          <button
            onClick={onAddVial}
            className="press"
            style={{
              width: 44, height: 44, borderRadius: 999,
              background: C.forest700, color: C.cream,
              border: 'none', cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              boxShadow: '0 4px 12px rgba(45,95,63,.28)',
            }}>
            <Icon name="plus" size={20} strokeWidth={2} />
          </button>
        </div>

        {/* Summary hero card */}
        <div style={{ padding: '0 16px 14px' }}>
          <InventorySummaryCard sum={sum} pal={pal} />
        </div>

        {/* Filter chips */}
        <div style={{
          padding: '0 16px 14px',
          display: 'flex', gap: 6, overflowX: 'auto',
          WebkitOverflowScrolling: 'touch',
        }}>
          {[
            { id: 'all',      label: 'Все', count: inv.length },
            { id: 'active',   label: 'Активные', count: sum.active.length },
            { id: 'expiring', label: 'Истекают', count: sum.expiring.length, danger: sum.expiring.length > 0 },
            { id: 'sealed',   label: 'Запас', count: sum.sealed.length },
            ...compounds.map(c => ({ id: c, label: COMPOUND_META[c]?.name || c, count: inv.filter(v => v.compound === c).length })),
          ].map(chip => {
            const on = filter === chip.id;
            const danger = chip.danger && !on;
            return (
              <button
                key={chip.id}
                onClick={() => setFilter(chip.id)}
                className="press"
                style={{
                  flexShrink: 0,
                  padding: '8px 12px', borderRadius: 999, cursor: 'pointer',
                  background: on ? C.forest700 : danger ? '#fbeed1' : pal.paper,
                  color: on ? C.cream : danger ? '#c2780a' : pal.muted,
                  border: on ? '1px solid transparent' : `1px solid ${danger ? 'transparent' : pal.border}`,
                  fontFamily: F.body, fontSize: 13, fontWeight: 500,
                  whiteSpace: 'nowrap',
                  display: 'inline-flex', alignItems: 'center', gap: 6,
                  transition: 'background-color 160ms var(--ease-out), color 160ms var(--ease-out)',
                }}
              >
                {chip.label}
                <span style={{
                  fontFamily: F.mono, fontSize: 10,
                  fontVariantNumeric: 'tabular-nums',
                  color: on ? 'rgba(246,241,234,.75)' : danger ? '#c2780a' : pal.subtle,
                  fontWeight: 400,
                }}>{chip.count}</span>
              </button>
            );
          })}
        </div>

        {/* Warning + reorder cards (only when on relevant filters) */}
        {(filter === 'all' || filter === 'expiring') && sum.expiring.map(v => (
          <div key={v.id} style={{ padding: '0 16px 10px' }}>
            <WarningCard
              tone="warn"
              icon="exclamation-circle"
              title={`${v.compoundMeta.name} истекает через ${v.daysToExpiry} ${v.daysToExpiry === 1 ? 'день' : v.daysToExpiry < 5 ? 'дня' : 'дней'}`}
              sub={v.opened ? `Открыт ${v.openedLabel} · ${v.remaining}/${v.total} доз` : `Запечатан · до ${v.expiresLabel}`}
              ctaLabel="Открыть"
              pal={pal}
              onClick={() => onOpenVial(v.id)}
            />
          </div>
        ))}

        {(filter === 'all') && sum.reorder.map(r => (
          <div key={r.compound} style={{ padding: '0 16px 10px' }}>
            <WarningCard
              tone="info"
              icon="information-circle"
              title={`${r.meta.name} закончится через ~${r.weeksLeft} ${r.weeksLeft === 1 ? 'неделю' : r.weeksLeft < 5 ? 'недели' : 'недель'}`}
              sub="Запасного флакона нет"
              ctaLabel="Добавить"
              pal={pal}
              onClick={onAddVial}
            />
          </div>
        ))}

        {/* Empty state */}
        {inv.length === 0 && (
          <div style={{ padding: '24px 16px 0' }}>
            <EmptyState pal={pal} onAddVial={onAddVial} />
          </div>
        )}

        {/* Active section */}
        {active.length > 0 && (
          <div style={{ padding: '4px 16px 14px' }}>
            <div style={{
              display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
              padding: '0 4px 10px',
            }}>
              <Eyebrow style={{ color: pal.subtle }}>Активные</Eyebrow>
              <span style={{
                fontFamily: F.mono, fontSize: 11, color: pal.subtle,
                fontVariantNumeric: 'tabular-nums',
              }}>{active.length} в работе</span>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {active.map(v => (
                <VialCard key={v.id} vial={v} pal={pal} onClick={() => onOpenVial(v.id)} />
              ))}
            </div>
          </div>
        )}

        {/* Sealed / spare section — collapsible */}
        {sealed.length > 0 && (
          <div style={{ padding: '4px 16px 0' }}>
            <button
              onClick={() => setSealedOpen(!sealedOpen)}
              className="press"
              style={{
                display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
                padding: '4px 4px 10px',
                width: '100%', background: 'transparent', border: 'none',
                cursor: 'pointer', textAlign: 'left',
              }}>
              <Eyebrow style={{ color: pal.subtle }}>В запасе</Eyebrow>
              <span style={{
                fontFamily: F.body, fontSize: 12, color: pal.muted,
                display: 'inline-flex', alignItems: 'center', gap: 4,
              }}>
                <span style={{
                  fontFamily: F.mono, fontSize: 11,
                  fontVariantNumeric: 'tabular-nums', color: pal.subtle,
                }}>{sealed.length}</span>
                <Icon name={sealedOpen ? 'chevron-up' : 'chevron-down'} size={14} />
              </span>
            </button>
            {sealedOpen && (
              <div style={{
                background: pal.paper, borderRadius: 18,
                border: `1px solid ${pal.hairline}`,
              }}>
                {sealed.map((v, i) => (
                  <React.Fragment key={v.id}>
                    <VialRow vial={v} pal={pal} onClick={() => onOpenVial(v.id)} />
                    {i < sealed.length - 1 && <div style={{ height: 1, background: pal.hairline, marginLeft: 60 }} />}
                  </React.Fragment>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Tab bar */}
      {platform === 'android'
        ? <CadenceAndroidNav active="inventory" onChange={onChangeTab} pal={pal} />
        : <CadenceTabBar     active="inventory" onChange={onChangeTab} pal={pal} />}
    </div>
  );
}

// ── Summary hero card ───────────────────────────────────────────────

function InventorySummaryCard({ sum, pal }) {
  const total = sum.active.length + sum.sealed.length;
  return (
    <div style={{
      background: pal.paper, borderRadius: 20, padding: 18,
      border: `1px solid ${pal.hairline}`,
      boxShadow: '0 2px 8px rgba(46,38,24,.05)',
    }}>
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start',
        marginBottom: 14,
      }}>
        <div>
          <Eyebrow style={{ color: pal.subtle, marginBottom: 4 }}>Запас в норме</Eyebrow>
          <div style={{
            fontFamily: F.display, fontSize: 22, color: pal.ink,
            lineHeight: 1.15, letterSpacing: '-0.012em',
          }}>
            <span style={{ color: C.forest700 }}>{sum.active.length}</span> открыты,
            {' '}<span style={{ color: C.sand700 }}>{sum.sealed.length}</span> в запасе
          </div>
        </div>
      </div>

      {/* Status dots row — visual representation of every vial */}
      <div style={{
        display: 'flex', gap: 5, marginBottom: 14, flexWrap: 'wrap',
      }}>
        {VIAL_INVENTORY.map(v => (
          <span key={v.id} title={`${v.compoundMeta.name} · ${v.status}`} style={{
            width: 10, height: 10, borderRadius: 999,
            background:
              v.status === 'expiring' ? '#c2780a' :
              v.status === 'low'      ? '#b8503c' :
              v.status === 'sealed'   ? C.sand500 :
                                        C.forest700,
            opacity: v.status === 'sealed' ? 0.6 : 1,
          }} />
        ))}
      </div>

      {/* Mini stats grid */}
      <div style={{
        display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12,
        paddingTop: 12, borderTop: `1px solid ${pal.hairline}`,
      }}>
        <SummaryStat label="истекает срок годности" value={sum.expiring.length} color="#c2780a" pal={pal} />
        <SummaryStat label="заканчиваются"            value={sum.low.length}      color="#b8503c" pal={pal} />
        <SummaryStat label="всего"                   value={total}                color={C.forest700} pal={pal} />
      </div>
    </div>
  );
}

function SummaryStat({ label, value, color, pal }) {
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
        <span style={{
          fontFamily: F.mono, fontSize: 22, fontWeight: 500,
          color: value === 0 ? pal.subtle : color,
          letterSpacing: '-0.02em',
          fontVariantNumeric: 'tabular-nums',
          lineHeight: 1,
        }}>{value}</span>
      </div>
      <Eyebrow style={{ color: pal.subtle, marginTop: 4, fontSize: 10, whiteSpace: 'normal', lineHeight: 1.25 }}>{label}</Eyebrow>
    </div>
  );
}

// ── WarningCard — amber for warn, slate for info ────────────────────

function WarningCard({ tone, icon, title, sub, ctaLabel, onClick, pal }) {
  const palette = tone === 'warn'
    ? { bg: '#fbeed1', border: 'rgba(194,120,10,.18)', fg: '#7a4a06', accent: '#c2780a' }
    : { bg: pal.sunk, border: pal.hairline,            fg: pal.ink2, accent: C.forest700 };
  return (
    <button
      onClick={onClick}
      className="press"
      style={{
        display: 'grid', gridTemplateColumns: '40px 1fr auto',
        gap: 12, alignItems: 'center',
        width: '100%', textAlign: 'left',
        background: palette.bg, border: `1px solid ${palette.border}`,
        borderRadius: 16, padding: 14, cursor: 'pointer',
      }}
    >
      <div style={{
        width: 40, height: 40, borderRadius: 12,
        background: tone === 'warn' ? 'rgba(194,120,10,.18)' : pal.paper,
        color: palette.accent,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <Icon name={icon} size={20} />
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: palette.fg }}>{title}</div>
        <div style={{ fontFamily: F.body, fontSize: 11.5, color: palette.fg, opacity: 0.7, marginTop: 1, lineHeight: 1.35 }}>{sub}</div>
      </div>
      <div style={{
        display: 'inline-flex', alignItems: 'center', gap: 4,
        fontFamily: F.body, fontSize: 12, fontWeight: 500, color: palette.accent,
      }}>
        {ctaLabel}
        <Icon name="arrow-right" size={14} />
      </div>
    </button>
  );
}

// ── VialCard — paper card for active vials ──────────────────────────

function VialCard({ vial, pal, onClick }) {
  const status = vial.status;
  const meta = vial.compoundMeta;

  const statusPill = (() => {
    if (status === 'expiring') return { label: 'Истекает', bg: '#fbeed1',     fg: '#c2780a' };
    if (status === 'low')      return { label: 'На исходе', bg: '#f4dfd6',    fg: '#b8503c' };
    return                            { label: 'Активный', bg: C.forest50,    fg: C.forest800 };
  })();

  const barColor =
    status === 'expiring' ? '#c2780a' :
    status === 'low'      ? '#b8503c' :
                            C.forest700;

  return (
    <button
      onClick={onClick}
      className="press"
      style={{
        display: 'block', width: '100%', textAlign: 'left',
        background: pal.paper, borderRadius: 18, padding: 16,
        border: `1px solid ${pal.hairline}`,
        boxShadow: '0 2px 6px rgba(46,38,24,.04)',
        cursor: 'pointer',
      }}
    >
      {/* Header row */}
      <div style={{
        display: 'grid', gridTemplateColumns: '40px 1fr auto',
        gap: 12, alignItems: 'center', marginBottom: 14,
      }}>
        <div style={{
          width: 40, height: 40, borderRadius: 12,
          background: pal.sunk, color: pal.ink2,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <Icon name={meta.icon || 'beaker'} size={20} />
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{
            fontFamily: F.display, fontSize: 19, color: pal.ink,
            lineHeight: 1.15, letterSpacing: '-0.012em',
          }}>{meta.name}</div>
          <div style={{
            fontFamily: F.body, fontSize: 12, color: pal.muted,
            marginTop: 1,
          }}>{vial.dose}</div>
        </div>
        <span style={{
          padding: '4px 10px', borderRadius: 999,
          fontFamily: F.body, fontSize: 11, fontWeight: 500,
          background: statusPill.bg, color: statusPill.fg,
          whiteSpace: 'nowrap',
        }}>{statusPill.label}</span>
      </div>

      {/* Doses bar */}
      <div style={{ marginBottom: 12 }}>
        <div style={{
          display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
          marginBottom: 4,
        }}>
          <span style={{
            fontFamily: F.body, fontSize: 11.5, color: pal.muted,
          }}>Доз осталось</span>
          <span style={{
            fontFamily: F.mono, fontSize: 12.5, color: pal.ink2,
            fontVariantNumeric: 'tabular-nums',
          }}>
            <span style={{ color: pal.ink, fontWeight: 500 }}>{vial.remaining}</span>
            <span style={{ color: pal.subtle }}> / {vial.total}</span>
            <span style={{ marginLeft: 8, color: pal.subtle }}>{Math.round(vial.pct * 100)}%</span>
          </span>
        </div>
        <div style={{
          height: 6, background: pal.sunk, borderRadius: 999, overflow: 'hidden',
        }}>
          <div style={{
            width: `${vial.pct * 100}%`, height: '100%',
            background: barColor, borderRadius: 999,
            transition: 'width 320ms var(--ease-out)',
          }} />
        </div>
      </div>

      {/* Meta strip — opened · last dose · expiry */}
      <div style={{
        display: 'grid', gridTemplateColumns: '1fr 1fr 1fr',
        gap: 12, paddingTop: 13, marginBottom: 4,
        borderTop: `1px solid ${pal.hairline}`,
      }}>
        <div style={{ minWidth: 0 }}>
          <Eyebrow style={{ color: pal.subtle, fontSize: 10, marginBottom: 3 }}>Открыт</Eyebrow>
          <div style={{
            fontFamily: F.body, fontSize: 12.5, color: pal.ink2,
          }}>{vial.daysSinceOpened} {vial.daysSinceOpened === 1 ? 'день' : vial.daysSinceOpened < 5 ? 'дня' : 'дней'} назад</div>
        </div>
        <div style={{ minWidth: 0 }}>
          <Eyebrow style={{ color: pal.subtle, fontSize: 10, marginBottom: 3 }}>Последняя</Eyebrow>
          <div style={{
            fontFamily: F.body, fontSize: 12.5, color: pal.ink2, whiteSpace: 'nowrap',
          }}>{vial.lastDose ? vial.lastDose.dateLabel : '—'}</div>
        </div>
        <div style={{ minWidth: 0, textAlign: 'right' }}>
          <Eyebrow style={{ color: pal.subtle, fontSize: 10, marginBottom: 3 }}>До истечения</Eyebrow>
          <div style={{
            fontFamily: F.body, fontSize: 12.5,
            color: status === 'expiring' ? '#c2780a' : pal.ink2,
            fontWeight: status === 'expiring' ? 500 : 400,
            whiteSpace: 'nowrap',
          }}>{vial.expiresLabel}</div>
        </div>
      </div>

      {/* Lot + location */}
      <div style={{
        paddingTop: 12, marginTop: 6,
        display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12,
      }}>
        <span style={{
          fontFamily: F.mono, fontSize: 10, color: pal.subtle,
          fontVariantNumeric: 'tabular-nums', letterSpacing: '-0.01em',
        }}>Лот {vial.lot}</span>
        <span style={{
          fontFamily: F.body, fontSize: 11, color: pal.subtle,
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>{vial.location}</span>
      </div>
    </button>
  );
}

// ── VialRow — compact list row for sealed/spare vials ───────────────

function VialRow({ vial, pal, onClick }) {
  const meta = vial.compoundMeta;
  return (
    <button
      onClick={onClick}
      className="press"
      style={{
        display: 'grid', gridTemplateColumns: 'auto 1fr auto auto',
        gap: 12, alignItems: 'center', padding: '12px 14px',
        width: '100%', background: 'transparent', border: 'none',
        cursor: 'pointer', textAlign: 'left',
      }}
    >
      <div style={{
        width: 32, height: 32, borderRadius: 999,
        border: `1.5px solid ${pal.border}`,
        background: 'transparent',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: pal.muted,
      }}>
        <Icon name={meta.icon || 'beaker'} size={16} />
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: pal.ink }}>
          {meta.name} · {vial.dose}
        </div>
        <div style={{
          fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 1,
          fontVariantNumeric: 'tabular-nums',
        }}>
          {vial.remaining}/{vial.total} доз · лот {vial.lot}
        </div>
      </div>
      <div style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
        <div style={{
          fontFamily: F.body, fontSize: 11, color: pal.subtle,
        }}>до</div>
        <div style={{
          fontFamily: F.mono, fontSize: 11.5, color: pal.ink2,
          fontVariantNumeric: 'tabular-nums',
        }}>{vial.expiresLabel}</div>
      </div>
      <Icon name="chevron-right" size={14} color={pal.subtle} />
    </button>
  );
}

// ── UsageSpark removed — weekly-usage line was dropped from VialCard ─

// ── Empty state ─────────────────────────────────────────────────────

function EmptyState({ pal, onAddVial }) {
  return (
    <div style={{
      background: pal.paper, borderRadius: 22, padding: '32px 24px',
      border: `1px dashed ${pal.border}`,
      textAlign: 'center',
    }}>
      <div style={{
        width: 64, height: 64, borderRadius: 999,
        background: C.sand100, color: C.sand700,
        display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
        marginBottom: 16,
      }}>
        <Icon name="beaker" size={28} />
      </div>
      <div style={{
        fontFamily: F.display, fontSize: 26, color: pal.ink,
        lineHeight: 1.1, letterSpacing: '-0.018em', marginBottom: 8,
      }}>
        Аптечка <span style={{ fontStyle: 'italic', color: C.sand700 }}>пуста</span>
      </div>
      <div style={{
        fontFamily: F.body, fontSize: 13, color: pal.muted,
        lineHeight: 1.5, marginBottom: 18, maxWidth: 260,
        marginLeft: 'auto', marginRight: 'auto',
      }}>
        Добавьте первый флакон, чтобы отсчёт сроков и расход доз пошёл сам.
      </div>
      <button
        onClick={onAddVial}
        className="press"
        style={{
          padding: '12px 24px', borderRadius: 999, border: 'none',
          background: C.forest700, color: C.cream, cursor: 'pointer',
          fontFamily: F.body, fontSize: 14, fontWeight: 500,
          display: 'inline-flex', alignItems: 'center', gap: 8,
        }}>
        Добавить флакон
        <span style={{ fontFamily: F.display, fontStyle: 'italic' }}>→</span>
      </button>
    </div>
  );
}

Object.assign(window, { VialsScreen, VialCard, VialRow, InventorySummaryCard });
