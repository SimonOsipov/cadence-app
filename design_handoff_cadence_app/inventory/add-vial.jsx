// Cadence — Add a vial flow.
// Manual form: compound, dose, expiration date, lot number, location.
// Mirrors the structure of the LogDose wizard but as a single screen
// (less granular — vial adds are one-shot, not multi-step).

function AddVialScreen({ dark, platform, onCancel, onComplete }) {
  const pal = getPalette(dark);
  const [compound, setCompound] = React.useState('sema');
  const [dose, setDose]         = React.useState('0,25 мг');
  const [lot, setLot]           = React.useState('');
  const [expires, setExpires]   = React.useState('');     // free-form date, e.g. '14 авг'
  const [location, setLocation] = React.useState('Холодильник, полка 2');
  const [opened, setOpened]     = React.useState(false);  // also set as active?
  const [photo, setPhoto]       = React.useState(null);   // 'pending' | 'attached' | null

  // Auto-fill dose from compound default
  React.useEffect(() => {
    const meta = COMPOUND_META[compound];
    if (meta) setDose(meta.defaultDose);
  }, [compound]);

  // Photo "capture" (mocked)
  const onPhotoTap = () => {
    if (photo === 'attached') { setPhoto(null); return; }
    setPhoto('pending');
    setTimeout(() => setPhoto('attached'), 900);
  };

  // Pretty-format placeholder for expiration
  const expirePlaceholder = '14 сен';

  const canSave = lot.trim().length > 0 && expires.trim().length > 0;

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 110,
      }}>
        {/* Top bar */}
        <div style={{
          padding: '8px 16px 8px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <button onClick={onCancel} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none',
            background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}>
            <Icon name="x-mark" size={18} />
          </button>
          <div style={{
            fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.muted,
          }}>Новый флакон</div>
          <div style={{ width: 40 }} />
        </div>

        {/* Hero */}
        <div style={{ padding: '4px 24px 18px' }}>
          <Eyebrow style={{ color: C.sand700, marginBottom: 6 }}>В аптечку</Eyebrow>
          <div style={{
            fontFamily: F.display, fontSize: 32, color: pal.ink,
            lineHeight: 1.05, letterSpacing: '-0.018em',
          }}>
            Что <span style={{ fontStyle: 'italic', color: C.sand700 }}>прибыло?</span>
          </div>
        </div>

        {/* Compound chooser */}
        <div style={{ padding: '0 16px 14px' }}>
          <FieldLabel pal={pal}>Препарат</FieldLabel>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
            {Object.entries(COMPOUND_META).map(([id, meta]) => {
              const sel = compound === id;
              return (
                <button
                  key={id}
                  onClick={() => setCompound(id)}
                  className="press"
                  style={{
                    padding: '12px 14px', borderRadius: 14, cursor: 'pointer',
                    background: sel ? pal.paper : 'transparent',
                    border: `1px solid ${sel ? C.forest700 : pal.border}`,
                    color: sel ? pal.ink : pal.ink2,
                    fontFamily: F.body, fontSize: 13.5, fontWeight: 500,
                    textAlign: 'left',
                    display: 'flex', alignItems: 'center', gap: 10,
                    transition: 'all 180ms var(--ease-out)',
                  }}
                >
                  <span style={{
                    width: 18, height: 18, borderRadius: 999,
                    border: `1.5px solid ${sel ? C.forest700 : pal.border}`,
                    background: sel ? C.forest700 : 'transparent', color: C.cream,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    flexShrink: 0,
                  }}>
                    {sel && (
                      <svg width="10" height="10" viewBox="0 0 16 16" fill="none">
                        <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    )}
                  </span>
                  {meta.name}
                </button>
              );
            })}
          </div>
        </div>

        {/* Dose */}
        <div style={{ padding: '0 16px 14px' }}>
          <FieldLabel pal={pal}>Дозировка</FieldLabel>
          <TextField
            value={dose}
            onChange={setDose}
            placeholder="0,25 мг"
            pal={pal}
          />
        </div>

        {/* Lot + expires row */}
        <div style={{ padding: '0 16px 14px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
          <div>
            <FieldLabel pal={pal}>Лот</FieldLabel>
            <TextField
              value={lot}
              onChange={setLot}
              placeholder="A24-0312"
              mono
              pal={pal}
            />
          </div>
          <div>
            <FieldLabel pal={pal}>До какого</FieldLabel>
            <TextField
              value={expires}
              onChange={setExpires}
              placeholder={expirePlaceholder}
              pal={pal}
            />
          </div>
        </div>

        {/* Location */}
        <div style={{ padding: '0 16px 14px' }}>
          <FieldLabel pal={pal}>Хранение</FieldLabel>
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {['Холодильник, полка 1','Холодильник, полка 2','Холодильник, полка 3','Морозильник','Комната'].map(opt => {
              const on = location === opt;
              return (
                <button key={opt}
                  onClick={() => setLocation(opt)}
                  className="press"
                  style={{
                    padding: '8px 12px', borderRadius: 999, cursor: 'pointer',
                    background: on ? C.forest700 : 'transparent',
                    color: on ? C.cream : pal.ink2,
                    border: `1px solid ${on ? C.forest700 : pal.border}`,
                    fontFamily: F.body, fontSize: 12, fontWeight: 500,
                    transition: 'background-color 160ms var(--ease-out), color 160ms var(--ease-out)',
                  }}>{opt}</button>
              );
            })}
          </div>
        </div>

        {/* Open now toggle */}
        <div style={{ padding: '0 16px 14px' }}>
          <button
            onClick={() => setOpened(!opened)}
            className="press"
            style={{
              width: '100%', textAlign: 'left',
              background: pal.paper, border: `1px solid ${pal.hairline}`,
              borderRadius: 14, padding: 14, cursor: 'pointer',
              display: 'grid', gridTemplateColumns: '1fr auto', gap: 12, alignItems: 'center',
            }}
          >
            <div>
              <div style={{
                fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: pal.ink,
              }}>Открыть сразу</div>
              <div style={{
                fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 2, lineHeight: 1.4,
              }}>
                Сделаем флакон активным и начнём отсчёт сроков.
              </div>
            </div>
            <div style={{
              width: 44, height: 26, borderRadius: 999,
              background: opened ? C.forest700 : pal.border,
              padding: 2, transition: 'background-color 160ms var(--ease-out)',
              flexShrink: 0,
            }}>
              <div style={{
                width: 22, height: 22, borderRadius: 999, background: C.cream,
                transform: opened ? 'translateX(18px)' : 'translateX(0)',
                transition: 'transform 180ms var(--ease-out)',
                boxShadow: '0 1px 3px rgba(0,0,0,.18)',
              }} />
            </div>
          </button>
        </div>

        {/* Photo */}
        <div style={{ padding: '0 16px 14px' }}>
          <button onClick={onPhotoTap} className="press" style={{
            width: '100%', borderRadius: 14, cursor: 'pointer',
            background: photo === 'attached' ? pal.sunk : 'transparent',
            border: `1.5px dashed ${pal.border}`,
            padding: '14px',
            display: 'flex', alignItems: 'center', gap: 12,
          }}>
            <div style={{
              width: 44, height: 44, borderRadius: 12, flexShrink: 0,
              background: photo === 'attached' ? C.forest700 : pal.sunk,
              color: photo === 'attached' ? C.cream : pal.muted,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              {photo === 'pending' ? (
                <div style={{
                  width: 18, height: 18, borderRadius: 999,
                  border: `2px solid ${pal.muted}`, borderTopColor: 'transparent',
                  animation: 'spin 700ms linear infinite',
                }} />
              ) : photo === 'attached' ? (
                <svg className="tick" width="20" height="20" viewBox="0 0 16 16" fill="none">
                  <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              ) : (
                <Icon name="camera" size={20} />
              )}
            </div>
            <div style={{ textAlign: 'left', flex: 1, minWidth: 0 }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 13, color: pal.ink }}>
                {photo === 'attached' ? 'Фото добавлено' : photo === 'pending' ? 'Снимаем…' : 'Снимок этикетки'}
              </div>
              <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}>
                {photo === 'attached' ? 'Нажмите, чтобы убрать' : 'По желанию'}
              </div>
            </div>
          </button>
        </div>
      </div>

      {/* Sticky Save button */}
      <div style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        padding: '14px 16px 28px',
        background: `linear-gradient(180deg, rgba(246,241,234,0) 0%, ${pal.bg} 40%, ${pal.bg} 100%)`,
      }}>
        <button
          className="press"
          disabled={!canSave}
          onClick={() => onComplete({
            compound, dose, lot, expires, location, opened, photo,
          })}
          style={{
            width: '100%', padding: '15px 20px', borderRadius: 999,
            background: canSave ? C.forest700 : pal.sunk,
            color: canSave ? C.cream : pal.subtle,
            border: 'none',
            fontFamily: F.body, fontSize: 14, fontWeight: 500,
            cursor: canSave ? 'pointer' : 'not-allowed',
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
          }}
        >
          {canSave ? <>Добавить в аптечку <span style={{ fontFamily: F.display, fontStyle: 'italic' }}>→</span></> : 'Заполните лот и срок'}
        </button>
      </div>
    </div>
  );
}

// ── FieldLabel — uppercase eyebrow for form fields ──────────────────

function FieldLabel({ children, pal }) {
  return (
    <div style={{
      fontFamily: F.body, fontSize: 11, fontWeight: 500,
      letterSpacing: '.14em', textTransform: 'uppercase',
      color: pal.subtle, marginBottom: 8, padding: '0 4px',
    }}>{children}</div>
  );
}

// ── TextField — simple paper-card input ─────────────────────────────

function TextField({ value, onChange, placeholder, mono, pal }) {
  return (
    <input
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      style={{
        width: '100%', padding: '12px 14px', boxSizing: 'border-box',
        background: pal.paper, border: `1px solid ${pal.border}`,
        borderRadius: 12, outline: 'none',
        fontFamily: mono ? F.mono : F.body,
        fontSize: 14, color: pal.ink,
        fontVariantNumeric: mono ? 'tabular-nums' : 'normal',
        letterSpacing: mono ? '-0.01em' : 'normal',
      }}
    />
  );
}

Object.assign(window, { AddVialScreen });
