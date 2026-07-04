// Cadence · Log Dose — shared atoms
// Exposes: COMPOUNDS, ZONES_FRONT, ZONES_BACK, SIDE_EFFECTS, VIALS, useLogState,
//          BodyDiagram, ZoneGrid, DoseStepper, SyringeBar, VialPicker,
//          MoodSlider, ChipsRow, PhotoSlot, WizardChrome, WarmConfirm, CelebrateOverlay

// ─────────────────────────────────────────────────────────────
// Data
// ─────────────────────────────────────────────────────────────
const COMPOUNDS = [
  { id: 'sema', name: 'Семаглутид', queued: true,  default: '0.25', unit: 'мг',  mode: 'п/к · еженедельно',  syringeMax: 100, syringeFill: 25 },
  { id: 'bpc',  name: 'BPC-157',     queued: false, default: '250',  unit: 'мкг', mode: 'п/к · 2× в день', syringeMax: 100, syringeFill: 50 },
  { id: 'tb',   name: 'TB-500',      queued: false, default: '2.5',  unit: 'мг',  mode: 'п/к · 2× в неделю', syringeMax: 100, syringeFill: 50 },
  { id: 'tes',  name: 'Тезаморелин', queued: false, default: '1.0',  unit: 'мг',  mode: 'п/к · ежедневно',     syringeMax: 100, syringeFill: 40 },
];

const ZONES_FRONT = [
  { id: 'r-delt',    label: 'Правое плечо',   cx: 54,  cy: 88 },
  { id: 'l-delt',    label: 'Левое плечо',    cx: 146, cy: 88 },
  { id: 'r-abdomen', label: 'Правый живот',   cx: 82,  cy: 148 },
  { id: 'l-abdomen', label: 'Левый живот',    cx: 118, cy: 148 },
  { id: 'r-thigh',   label: 'Правое бедро',   cx: 82,  cy: 230 },
  { id: 'l-thigh',   label: 'Левое бедро',    cx: 118, cy: 230 },
];

const ZONES_BACK = [
  { id: 'l-lback',   label: 'Левая поясница',   cx: 82,  cy: 175 },
  { id: 'r-lback',   label: 'Правая поясница',  cx: 118, cy: 175 },
  { id: 'l-glute',   label: 'Левая ягодица',    cx: 82,  cy: 210 },
  { id: 'r-glute',   label: 'Правая ягодица',   cx: 118, cy: 210 },
];

const ALL_ZONES = [...ZONES_FRONT, ...ZONES_BACK];
const zoneLabel = (id) => (ALL_ZONES.find(z => z.id === id) || {}).label || '—';

const SIDE_EFFECTS = [
  { id: 'nausea',    label: 'Тошнота' },
  { id: 'fatigue',   label: 'Усталость' },
  { id: 'headache',  label: 'Голова' },
  { id: 'bloating',  label: 'Вздутие' },
  { id: 'insomnia',  label: 'Бессонница' },
  { id: 'site',      label: 'Шишка' },
  { id: 'appetite',  label: 'Нет аппетита' },
  { id: 'none',      label: 'Ничего' },
];

const VIALS = [
  { id: 'v1', compound: 'sema', dose: '0,25 мг',  remaining: 8,  total: 12, opened: '2 апр', expires: '14 июн', active: true },
  { id: 'v2', compound: 'sema', dose: '0,25 мг',  remaining: 12, total: 12, opened: '—',     expires: '22 авг', active: false },
  { id: 'v3', compound: 'bpc',  dose: '250 мкг',  remaining: 14, total: 30, opened: '18 апр', expires: '6 июн', active: true, warn: true },
];

// ─────────────────────────────────────────────────────────────
// Shared state — defaults pre-filled from today's queued protocol
// ─────────────────────────────────────────────────────────────
function useLogState(initial = {}) {
  const [state, setState] = React.useState(() => ({
    compound: 'sema',
    dose: '0.25',
    unit: 'mg',
    vialId: 'v1',
    site: null,                  // selected zone id
    suggested: 'l-abdomen',      // rotation suggests left abdomen (last was right)
    lastUsed: ['r-abdomen'],
    view: 'front',
    mood: 3,                     // 1..5
    sides: [],                   // selected side-effect ids
    note: '',
    photo: null,                 // 'pending' | 'attached' | null
    time: '06:42',
    date: 'Сегодня',
    ...initial,
  }));
  const update = (patch) => setState(s => ({ ...s, ...(typeof patch === 'function' ? patch(s) : patch) }));
  return [state, update];
}

const compoundById = (id) => COMPOUNDS.find(c => c.id === id) || COMPOUNDS[0];

// ─────────────────────────────────────────────────────────────
// Body diagram
// ─────────────────────────────────────────────────────────────
function BodyDiagram({ state, update, pal, size = 220, showToggle = true, compact = false }) {
  const zones = state.view === 'back' ? ZONES_BACK : ZONES_FRONT;
  const stroke = pal.border;
  const fill = pal.sunk;
  const select = (id) => update({ site: id });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10 }}>
      {showToggle && (
        <div style={{ display: 'inline-flex', background: pal.sunk, padding: 3, borderRadius: 999, gap: 2 }}>
          {[
            { id: 'front', label: 'Спереди' },
            { id: 'back',  label: 'Сзади' },
          ].map(t => {
            const on = state.view === t.id;
            return (
              <button key={t.id} onClick={() => update({ view: t.id })} style={{
                padding: '6px 14px', borderRadius: 999, border: 'none', cursor: 'pointer',
                background: on ? pal.paper : 'transparent',
                color: on ? pal.ink : pal.muted,
                boxShadow: on ? '0 1px 3px rgba(46,38,24,.1)' : 'none',
                fontFamily: F.body, fontWeight: 500, fontSize: 12,
                transition: 'all 180ms var(--ease-out)',
              }}>{t.label}</button>
            );
          })}
        </div>
      )}
      <svg viewBox="0 0 200 340" width={size} height={size * (340/200)} style={{ overflow: 'visible' }}>
        {/* Silhouette */}
        <g opacity="0.9">
          {/* head */}
          <ellipse cx="100" cy="38" rx="22" ry="24" fill={fill} stroke={stroke} strokeWidth="1.2" />
          {/* neck */}
          <path d="M92 60 L92 72 Q92 76 96 76 L104 76 Q108 76 108 72 L108 60 Z"
            fill={fill} stroke={stroke} strokeWidth="1.2" />
          {/* torso */}
          <path d={state.view === 'back'
            ? "M55 78 Q56 70 66 70 L134 70 Q144 70 145 78 L145 180 Q145 192 134 194 L66 194 Q55 192 55 180 Z"
            : "M55 78 Q56 70 66 70 L134 70 Q144 70 145 78 L145 175 Q145 188 134 190 L66 190 Q55 188 55 175 Z"
          } fill={fill} stroke={stroke} strokeWidth="1.2" />
          {/* arms */}
          <path d="M40 80 Q34 84 34 96 L34 196 Q34 204 41 206 L48 206 Q55 204 55 196 L55 90 Q55 80 47 78 Z"
            fill={fill} stroke={stroke} strokeWidth="1.2" />
          <path d="M160 80 Q166 84 166 96 L166 196 Q166 204 159 206 L152 206 Q145 204 145 196 L145 90 Q145 80 153 78 Z"
            fill={fill} stroke={stroke} strokeWidth="1.2" />
          {/* legs */}
          <path d="M68 190 L94 190 Q98 190 98 198 L98 320 Q98 328 91 328 L75 328 Q68 328 68 320 Z"
            fill={fill} stroke={stroke} strokeWidth="1.2" />
          <path d="M132 190 L106 190 Q102 190 102 198 L102 320 Q102 328 109 328 L125 328 Q132 328 132 320 Z"
            fill={fill} stroke={stroke} strokeWidth="1.2" />
        </g>

        {/* Last-used dots (small, subtle) */}
        {state.lastUsed.map(uid => {
          const z = ALL_ZONES.find(a => a.id === uid);
          if (!z) return null;
          const inView = (state.view === 'front' ? ZONES_FRONT : ZONES_BACK).some(zz => zz.id === uid);
          if (!inView) return null;
          return <circle key={'lu-' + uid} cx={z.cx} cy={z.cy} r="3" fill={pal.muted} opacity="0.6" />;
        })}

        {/* Zone targets */}
        {zones.map(z => {
          const isSel = state.site === z.id;
          const isSug = state.suggested === z.id && !state.site;
          return (
            <g key={z.id} onClick={() => select(z.id)} style={{ cursor: 'pointer' }}>
              {isSug && (
                <circle cx={z.cx} cy={z.cy} r="15" fill="none"
                  stroke={C.sand500} strokeWidth="1.5" strokeDasharray="3 3"
                  className="pulse" />
              )}
              <circle cx={z.cx} cy={z.cy} r={isSel ? 12 : 10}
                fill={isSel ? C.forest700 : 'rgba(246,241,234,0.92)'}
                stroke={isSel ? C.forest700 : pal.border}
                strokeWidth={isSel ? 0 : 1.5}
                style={{ transition: 'all 200ms var(--ease-out)' }}
              />
              {isSel && (
                <path d={`M${z.cx - 4} ${z.cy} L${z.cx - 1} ${z.cy + 3} L${z.cx + 5} ${z.cy - 3}`}
                  stroke={C.cream} strokeWidth="2" fill="none"
                  strokeLinecap="round" strokeLinejoin="round" />
              )}
              {/* Larger invisible hit target */}
              <circle cx={z.cx} cy={z.cy} r="18" fill="transparent" />
            </g>
          );
        })}
      </svg>

      {!compact && (
        <div style={{ minHeight: 32, textAlign: 'center' }}>
          {state.site
            ? <div className="fade-up" key={state.site}>
                <div style={{ fontFamily: F.display, fontSize: 17, color: pal.ink, letterSpacing: '-0.012em' }}>
                  {zoneLabel(state.site)}
                </div>
                <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}>
                  Tap a different zone to change
                </div>
              </div>
            : <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, fontStyle: 'italic' }}>
                <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: 999, background: C.sand500, marginRight: 6, verticalAlign: 'middle' }} />
                Предложено: <span style={{ color: pal.ink2, fontStyle: 'normal' }}>{zoneLabel(state.suggested)}</span> — следующее в ротации
              </div>
          }
        </div>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Zone grid — alternative site picker
// ─────────────────────────────────────────────────────────────
function ZoneGrid({ state, update, pal }) {
  const all = [
    { id: 'r-delt',    label: 'Прав. плечо'   },
    { id: 'l-delt',    label: 'Лев. плечо'    },
    { id: 'r-abdomen', label: 'Прав. живот'   },
    { id: 'l-abdomen', label: 'Лев. живот'    },
    { id: 'r-thigh',   label: 'Прав. бедро'   },
    { id: 'l-thigh',   label: 'Лев. бедро'    },
    { id: 'r-glute',   label: 'Прав. ягодица' },
    { id: 'l-glute',   label: 'Лев. ягодица'  },
  ];
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
      {all.map(z => {
        const sel = state.site === z.id;
        const sug = state.suggested === z.id && !state.site;
        const used = state.lastUsed.includes(z.id);
        return (
          <button key={z.id}
            onClick={() => update({ site: z.id })}
            className="press"
            style={{
              position: 'relative',
              padding: '13px 14px', borderRadius: 14, cursor: 'pointer',
              background: sel ? C.forest700 : pal.paper,
              color: sel ? C.cream : pal.ink,
              border: sel ? 'none' : `1px solid ${sug ? C.sand500 : pal.border}`,
              fontFamily: F.body, fontSize: 13, fontWeight: 500,
              display: 'flex', alignItems: 'center', justifyContent: 'space-between',
              transition: 'all 180ms var(--ease-out)',
            }}>
            <span>{z.label}</span>
            {sel && <Icon name="check-circle" size={16} />}
            {!sel && sug && <span style={{ fontFamily: F.body, fontSize: 10, color: C.sand700, fontWeight: 500 }}>далее</span>}
            {!sel && used && !sug && <span style={{ width: 5, height: 5, borderRadius: 999, background: pal.muted }} />}
          </button>
        );
      })}
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Dose stepper — big mono number + - / +
// ─────────────────────────────────────────────────────────────
function DoseStepper({ value, onChange, step = 0.05, unit = 'mg', size = 'lg', pal }) {
  const v = parseFloat(value);
  const num = isNaN(v) ? 0 : v;
  const fmt = (n) => unit === 'mcg' ? String(Math.round(n)) : n.toFixed(2).replace(/\.?0+$/, '') || '0';

  const SZ = size === 'lg'
    ? { num: 72, btn: 52, btnIcon: 22 }
    : { num: 44, btn: 40, btnIcon: 18 };

  const bump = (delta) => {
    const next = Math.max(0, +(num + delta).toFixed(3));
    onChange(fmt(next));
  };

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 18 }}>
      <button onClick={() => bump(-step)} className="press" style={{
        width: SZ.btn, height: SZ.btn, borderRadius: 999, cursor: 'pointer',
        background: pal.paper, border: `1px solid ${pal.border}`, color: pal.ink2,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <svg width={SZ.btnIcon} height="2" viewBox="0 0 22 2"><line x1="0" y1="1" x2="22" y2="1" stroke="currentColor" strokeWidth="2" strokeLinecap="round" /></svg>
      </button>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, minWidth: 140, justifyContent: 'center' }}>
        <span style={{
          fontFamily: F.mono, fontSize: SZ.num, fontWeight: 500, color: pal.ink,
          letterSpacing: '-0.04em', fontVariantNumeric: 'tabular-nums', lineHeight: 1,
        }}>{fmt(num)}</span>
        <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: SZ.num * 0.35, color: pal.muted }}>{unit}</span>
      </div>
      <button onClick={() => bump(+step)} className="press" style={{
        width: SZ.btn, height: SZ.btn, borderRadius: 999, cursor: 'pointer',
        background: pal.paper, border: `1px solid ${pal.border}`, color: pal.ink2,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <Icon name="plus" size={SZ.btnIcon} />
      </button>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Syringe bar — horizontal pen showing dose on a 100-unit scale
// ─────────────────────────────────────────────────────────────
function SyringeBar({ fill = 25, max = 100, pal, height = 22, showUnits = true }) {
  const pct = Math.min(100, Math.max(0, (fill / max) * 100));
  return (
    <div>
      <div style={{
        position: 'relative', height,
        background: pal.sunk, borderRadius: 999,
        border: `1px solid ${pal.border}`,
        overflow: 'hidden',
      }}>
        {/* fill */}
        <div style={{
          position: 'absolute', top: 0, bottom: 0, left: 0,
          width: `${pct}%`, background: C.sand500,
          transition: 'width 320ms var(--ease-out)',
        }} />
        {/* tick marks */}
        {[10, 20, 30, 40, 50, 60, 70, 80, 90].map(t => (
          <div key={t} style={{
            position: 'absolute', top: 4, bottom: 4, left: `${t}%`,
            width: 1, background: pal.border, opacity: t % 50 === 0 ? 0.6 : 0.25,
          }} />
        ))}
        {/* needle */}
        <div style={{
          position: 'absolute', right: -2, top: '50%', transform: 'translateY(-50%)',
          width: 18, height: 2, background: pal.border,
        }} />
      </div>
      {showUnits && (
        <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 6, fontFamily: F.mono, fontSize: 10, color: pal.subtle, fontVariantNumeric: 'tabular-nums' }}>
          <span>0u</span>
          <span style={{ color: pal.ink2 }}>{Math.round(fill)} ед. в шприце</span>
          <span>{max}u</span>
        </div>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Vial picker — segmented list with dose-remaining bar
// ─────────────────────────────────────────────────────────────
function VialPicker({ state, update, pal, compoundFilter, compact = false }) {
  const vials = compoundFilter
    ? VIALS.filter(v => v.compound === compoundFilter)
    : VIALS;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      {vials.map(v => {
        const sel = state.vialId === v.id;
        const pct = Math.round((v.remaining / v.total) * 100);
        return (
          <button key={v.id}
            onClick={() => update({ vialId: v.id })}
            className="press"
            style={{
              padding: compact ? '10px 12px' : '12px 14px',
              borderRadius: 14, cursor: 'pointer', textAlign: 'left',
              background: sel ? pal.paper : 'transparent',
              border: `1px solid ${sel ? C.forest700 : pal.border}`,
              display: 'grid', gridTemplateColumns: 'auto 1fr auto', gap: 12, alignItems: 'center',
              transition: 'all 180ms var(--ease-out)',
            }}>
            <div style={{
              width: 28, height: 28, borderRadius: 999,
              border: `2px solid ${sel ? C.forest700 : pal.border}`,
              background: sel ? C.forest700 : 'transparent', color: C.cream,
              display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
            }}>
              {sel && <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"/></svg>}
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
                <span style={{ fontFamily: F.body, fontWeight: 500, fontSize: 13, color: pal.ink }}>
                  Флакон · {v.dose}
                </span>
                {v.active && !v.warn && <span style={{ fontFamily: F.body, fontSize: 10, color: C.forest700, fontWeight: 500 }}>активен</span>}
                {v.warn && <span style={{ fontFamily: F.body, fontSize: 10, color: '#c2780a', fontWeight: 500 }}>до 6 июн</span>}
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 4 }}>
                <div style={{ flex: 1, height: 4, background: pal.bone, borderRadius: 999, overflow: 'hidden', maxWidth: 120 }}>
                  <div style={{ height: '100%', width: `${pct}%`, background: v.warn ? '#c2780a' : C.forest600 }} />
                </div>
                <span style={{ fontFamily: F.mono, fontSize: 10, color: pal.subtle, fontVariantNumeric: 'tabular-nums' }}>{v.remaining}/{v.total} доз</span>
              </div>
            </div>
            {v.opened !== '—' && (
              <span style={{ fontFamily: F.mono, fontSize: 10, color: pal.subtle, whiteSpace: 'nowrap' }}>открыт {v.opened}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Mood slider — 5 dots
// ─────────────────────────────────────────────────────────────
function MoodSlider({ value, onChange, pal }) {
  const labels = ['Никак', 'Слабо', 'Ровно', 'Хорошо', 'Светло'];
  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        {[1,2,3,4,5].map(n => {
          const on = value === n;
          return (
            <button key={n} onClick={() => onChange(n)} className="press" style={{
              width: 34, height: 34, borderRadius: 999, cursor: 'pointer',
              background: on ? C.forest700 : 'transparent',
              border: `1.5px solid ${on ? C.forest700 : pal.border}`,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontFamily: F.mono, fontSize: 11, color: on ? C.cream : pal.muted, fontWeight: 500,
              transition: 'all 180ms var(--ease-out)',
            }}>{n}</button>
          );
        })}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontFamily: F.body, fontSize: 10, color: pal.subtle, padding: '0 4px' }}>
        <span>Никак</span>
        <span style={{ fontStyle: 'italic', color: pal.ink2 }}>{labels[value - 1]}</span>
        <span>Светло</span>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Side-effect chips
// ─────────────────────────────────────────────────────────────
function ChipsRow({ value = [], onChange, pal, items = SIDE_EFFECTS }) {
  const toggle = (id) => {
    const has = value.includes(id);
    let next;
    if (id === 'none') next = has ? [] : ['none'];
    else next = (has ? value.filter(v => v !== id) : [...value.filter(v => v !== 'none'), id]);
    onChange(next);
  };
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
      {items.map(i => {
        const on = value.includes(i.id);
        return (
          <button key={i.id} onClick={() => toggle(i.id)} className="press" style={{
            padding: '7px 12px', borderRadius: 999, cursor: 'pointer',
            background: on ? C.ink900 : 'transparent',
            color: on ? C.cream : pal.ink2,
            border: `1px solid ${on ? C.ink900 : pal.border}`,
            fontFamily: F.body, fontSize: 12, fontWeight: 500,
            transition: 'all 160ms var(--ease-out)',
          }}>{i.label}</button>
        );
      })}
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Photo slot — simulated capture
// ─────────────────────────────────────────────────────────────
function PhotoSlot({ state, update, pal }) {
  const onTap = () => {
    if (state.photo === 'attached') {
      update({ photo: null });
      return;
    }
    update({ photo: 'pending' });
    setTimeout(() => update({ photo: 'attached' }), 900);
  };
  return (
    <button onClick={onTap} className="press" style={{
      width: '100%', borderRadius: 14, cursor: 'pointer',
      background: state.photo === 'attached' ? pal.sunk : 'transparent',
      border: `1.5px dashed ${pal.border}`,
      padding: '14px',
      display: 'flex', alignItems: 'center', gap: 12,
    }}>
      <div style={{
        width: 44, height: 44, borderRadius: 12, flexShrink: 0,
        background: state.photo === 'attached' ? C.forest700 : pal.sunk,
        color: state.photo === 'attached' ? C.cream : pal.muted,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        {state.photo === 'pending' ? (
          <div style={{
            width: 18, height: 18, borderRadius: 999,
            border: `2px solid ${pal.muted}`, borderTopColor: 'transparent',
            animation: 'spin 700ms linear infinite',
          }} />
        ) : state.photo === 'attached' ? (
          <svg className="tick" width="20" height="20" viewBox="0 0 16 16" fill="none">
            <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        ) : (
          <Icon name="camera" size={20} />
        )}
      </div>
      <div style={{ textAlign: 'left', flex: 1 }}>
        <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 13, color: pal.ink }}>
          {state.photo === 'attached' ? 'Фото добавлено' : state.photo === 'pending' ? 'Снимаем…' : 'Добавить фото'}
        </div>
        <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}>
          {state.photo === 'attached' ? 'Нажмите, чтобы убрать' : 'Место или флакон · по желанию'}
        </div>
      </div>
    </button>
  );
}

// ─────────────────────────────────────────────────────────────
// Wizard chrome — header w/ Cancel · step counter, progress bar, footer w/ prev+next
// ─────────────────────────────────────────────────────────────
function WizardChrome({ step, total, onCancel, onPrev, onNext, nextLabel = 'Continue', nextDisabled, dense, platform = 'ios', pal, children, eyebrow, title, sub, footerExtra }) {
  const topPad = platform === 'android' ? 8 : (dense ? 50 : 54);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Header */}
      <div style={{
        padding: `${topPad}px 18px 6px`,
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      }}>
        <button onClick={onCancel} style={{
          background: 'transparent', border: 'none', cursor: 'pointer',
          fontFamily: F.body, fontSize: 14, color: pal.muted, padding: 0,
        }}>Отмена</button>
        <span style={{ fontFamily: F.mono, fontSize: 11, color: pal.subtle, fontVariantNumeric: 'tabular-nums' }}>
          Шаг {step} из {total}
        </span>
        <div style={{ width: 50 }} />
      </div>

      {/* Progress bar */}
      <div style={{ padding: '8px 18px 0', display: 'flex', gap: 4 }}>
        {Array.from({ length: total }).map((_, i) => (
          <div key={i} style={{
            flex: 1, height: 3, borderRadius: 999,
            background: i < step ? C.forest700 : pal.bone,
            transition: 'background 280ms var(--ease-out)',
          }} />
        ))}
      </div>

      {/* Step heading */}
      {(eyebrow || title) && (
        <div style={{ padding: dense ? '20px 20px 14px' : '28px 24px 18px' }} className="slide-r" key={step}>
          {eyebrow && (
            <div style={{
              fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em',
              textTransform: 'uppercase', color: pal.subtle, marginBottom: 8,
            }}>{eyebrow}</div>
          )}
          {title && (
            <div style={{
              fontFamily: F.display, fontSize: dense ? 28 : 34,
              color: pal.ink, lineHeight: 1.05, letterSpacing: '-0.02em',
            }}>{title}</div>
          )}
          {sub && (
            <div style={{
              fontFamily: F.body, fontSize: 13, color: pal.muted,
              lineHeight: 1.5, marginTop: 8, maxWidth: 320,
            }}>{sub}</div>
          )}
        </div>
      )}

      {/* Step body */}
      <div className="ds-scroll slide-r" key={'body-' + step} style={{
        flex: 1, minHeight: 0, overflowY: 'auto',
        padding: dense ? '0 16px' : '0 18px',
        paddingBottom: 100,
      }}>
        {children}
      </div>

      {/* Footer */}
      <div style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        padding: '12px 18px 34px',
        background: `linear-gradient(180deg, ${pal.bg === C.cream ? 'rgba(246,241,234,0)' : 'rgba(14,30,22,0)'} 0%, ${pal.bg} 30%)`,
        backdropFilter: 'blur(8px)',
        display: 'flex', flexDirection: 'column', gap: 8,
      }}>
        {footerExtra}
        <div style={{ display: 'flex', gap: 8 }}>
          {step > 1 && onPrev && (
            <button onClick={onPrev} className="press" style={{
              padding: '14px 18px', borderRadius: 999, cursor: 'pointer',
              background: pal.sunk, color: pal.ink, border: 'none',
              fontFamily: F.body, fontSize: 14, fontWeight: 500,
              display: 'flex', alignItems: 'center', gap: 4,
            }}>
              <Icon name="chevron-left" size={16} />
              Назад
            </button>
          )}
          <button onClick={onNext} disabled={nextDisabled} className="press" style={{
            flex: 1, padding: '14px 20px', borderRadius: 999, cursor: nextDisabled ? 'not-allowed' : 'pointer',
            background: nextDisabled ? pal.sunk : C.forest700,
            color: nextDisabled ? pal.subtle : C.cream,
            border: 'none', fontFamily: F.body, fontSize: 14, fontWeight: 500,
            opacity: nextDisabled ? 0.65 : 1,
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
          }}>
            {nextLabel}
            {nextLabel !== 'Сохранить дозу' && <Icon name="arrow-right" size={16} />}
          </button>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Warm confirmation — serif moment, auto-dismiss
// ─────────────────────────────────────────────────────────────
function WarmConfirm({ open, onClose, state, pal, variant = 'sheet' }) {
  React.useEffect(() => {
    if (!open || variant === 'celebration') return;
    const t = setTimeout(() => onClose && onClose(), 2600);
    return () => clearTimeout(t);
  }, [open, variant]);

  if (!open) return null;

  if (variant === 'toast') {
    return (
      <div className="fade-up" style={{
        position: 'absolute', left: 16, right: 16, bottom: 110, zIndex: 90,
        background: C.ink900, color: C.cream,
        padding: '12px 16px', borderRadius: 14,
        display: 'flex', alignItems: 'center', gap: 10,
        boxShadow: '0 12px 28px rgba(0,0,0,.2)',
      }}>
        <div style={{ width: 22, height: 22, borderRadius: 999, background: C.forest700, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <svg className="tick" width="12" height="12" viewBox="0 0 16 16" fill="none">
            <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </div>
        <span style={{ fontFamily: F.body, fontSize: 13, flex: 1 }}>Доза записана · {zoneLabel(state.site)}</span>
        <span style={{ fontFamily: F.mono, fontSize: 11, color: 'rgba(246,241,234,.55)' }}>{state.time}</span>
      </div>
    );
  }

  // Default warm sheet
  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 90 }}>
      <div className="scrim" style={{ position: 'absolute', inset: 0, background: 'rgba(20,44,31,0.45)', backdropFilter: 'blur(4px)' }} />
      <div className="sheet" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28,
        padding: '20px 24px 38px',
      }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '0 auto 20px' }} />
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 14 }}>
          <div style={{
            width: 56, height: 56, borderRadius: 999, background: C.forest700, color: C.cream,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <svg className="tick" width="28" height="28" viewBox="0 0 16 16" fill="none">
              <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </div>
        </div>
        <div style={{ fontFamily: F.display, fontSize: 32, color: pal.ink, lineHeight: 1.05, letterSpacing: '-0.02em', textAlign: 'center', marginBottom: 8 }}>
          Записано. <span style={{ fontStyle: 'italic', color: C.forest700 }}>Отлично.</span>
        </div>
        <div style={{ fontFamily: F.body, fontSize: 13, color: pal.muted, textAlign: 'center', lineHeight: 1.5 }}>
          {compoundById(state.compound).name} · {state.dose} {state.unit} · {zoneLabel(state.site)} в {state.time}
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Celebration confirm — fuller editorial moment with next-dose preview
// ─────────────────────────────────────────────────────────────
function CelebrateOverlay({ open, onClose, state, pal }) {
  if (!open) return null;
  return (
    <div className="fade-in" style={{ position: 'absolute', inset: 0, zIndex: 100, background: pal.bg, display: 'flex', flexDirection: 'column' }}>
      {/* Top bar */}
      <div style={{ padding: '50px 20px 0', display: 'flex', justifyContent: 'flex-end' }}>
        <button onClick={onClose} style={{
          background: pal.sunk, border: 'none', cursor: 'pointer',
          width: 36, height: 36, borderRadius: 999, color: pal.ink2,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <Icon name="x-mark" size={18} />
        </button>
      </div>

      <div style={{ flex: 1, padding: '20px 24px', display: 'flex', flexDirection: 'column' }}>
        {/* Big tick */}
        <div className="fade-up" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', marginTop: 18, marginBottom: 24 }}>
          <div style={{
            width: 84, height: 84, borderRadius: 999,
            background: C.forest700, color: C.cream,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            boxShadow: '0 12px 28px rgba(45,95,63,.35)',
          }}>
            <svg className="tick" width="44" height="44" viewBox="0 0 16 16" fill="none">
              <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </div>
        </div>

        {/* Editorial moment */}
        <div className="fade-up" style={{ textAlign: 'center', marginBottom: 26 }}>
          <div style={{
            fontFamily: F.body, fontSize: 11, fontWeight: 500,
            letterSpacing: '.14em', textTransform: 'uppercase',
            color: C.sand700, marginBottom: 10,
          }}>4-я неделя · доза 4 из 12</div>
          <div style={{ fontFamily: F.display, fontSize: 40, color: pal.ink, lineHeight: 1.02, letterSpacing: '-0.024em' }}>
            Вы стали<br/>
            <span style={{ fontStyle: 'italic', color: C.forest700 }}>легче</span>. Записано.
          </div>
          <div style={{ fontFamily: F.body, fontSize: 14, color: pal.muted, lineHeight: 1.5, marginTop: 14, maxWidth: 280, marginLeft: 'auto', marginRight: 'auto' }}>
            Ровно там, где мы и надеялись. Держите утренний ритм.
          </div>
        </div>

        {/* Cycle progress */}
        <div style={{ padding: '16px 18px', background: pal.sunk, borderRadius: 18, marginBottom: 14 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <span style={{
              fontFamily: F.body, fontSize: 11, fontWeight: 500,
              letterSpacing: '.14em', textTransform: 'uppercase', color: pal.subtle,
            }}>Прогресс цикла</span>
            <span style={{ fontFamily: F.mono, fontSize: 11, color: pal.ink2, fontVariantNumeric: 'tabular-nums' }}>4 / 12</span>
          </div>
          <div style={{ display: 'flex', gap: 3 }}>
            {Array.from({ length: 12 }).map((_, i) => (
              <div key={i} style={{
                flex: 1, height: 6, borderRadius: 2,
                background: i < 4 ? C.forest700 : pal.bone,
              }} />
            ))}
          </div>
        </div>

        {/* Next dose preview */}
        <div style={{ padding: '14px 16px', background: pal.paper, borderRadius: 16, border: `1px solid ${pal.hairline}`, display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{ width: 40, height: 40, borderRadius: 12, background: C.sand100, color: C.sand700, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <Icon name="calendar" size={20} />
          </div>
          <div style={{ flex: 1 }}>
            <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginBottom: 2 }}>Следующая доза</div>
            <div style={{ fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.ink }}>
              <span style={{ fontFamily: F.mono }}>Воскресенье, 31 мая</span> · {compoundById(state.compound).name} 0,25 мг
            </div>
          </div>
        </div>

        <div style={{ flex: 1 }} />

        <button onClick={onClose} className="press" style={{
          padding: '14px 20px', borderRadius: 999, cursor: 'pointer',
          background: C.forest700, color: C.cream, border: 'none',
          fontFamily: F.body, fontSize: 14, fontWeight: 500,
          marginBottom: 32,
        }}>Вернуться на Сегодня</button>
      </div>
    </div>
  );
}

Object.assign(window, {
  COMPOUNDS, ZONES_FRONT, ZONES_BACK, ALL_ZONES, SIDE_EFFECTS, VIALS,
  useLogState, compoundById, zoneLabel,
  BodyDiagram, ZoneGrid, DoseStepper, SyringeBar, VialPicker,
  MoodSlider, ChipsRow, PhotoSlot, WizardChrome,
  WarmConfirm, CelebrateOverlay,
});
