// Cadence Dashboard — shared atoms for the 6 variants
// Exposes: getPalette, PullToRefresh, TimeOfDayChips, BiomarkerSheet,
//          CadenceTabBar, CadenceAndroidNav, CoachLine, useCoach,
//          CycleRing, RhythmDot, StatusStrip, FuelStrip, IconTile

// ─────────────────────────────────────────────────────────────
// Palette swap (light / dark)
// ─────────────────────────────────────────────────────────────
function getPalette(dark) {
  if (!dark) {
    return {
      bg:        C.cream,
      paper:     C.paper,
      sunk:      C.linen,
      bone:      C.bone,
      border:    C.border,
      ink:       C.ink900,
      ink2:      C.ink800,
      muted:     C.ink600,
      subtle:    C.ink500,
      placeholder: C.ink400,
      forestBg:  C.forest800,        // forest hero card bg
      forestDeep: C.forest900,
      forestFg:  C.cream,
      forestPill:{ bg: C.forest50, fg: C.forest800 },
      sand:      C.sand500,
      sand3:     C.sand300,
      sand1:     C.sand100,
      sand7:     C.sand700,
      glassDark: 'rgba(20,44,31,.35)',
      hairline:  'rgba(26,26,26,0.08)',
      onForest:  C.cream,
      tabBarGrad:'linear-gradient(180deg, rgba(246,241,234,0) 0%, rgba(246,241,234,0.85) 40%, rgba(246,241,234,1) 100%)',
    };
  }
  // Dark mode — midnight forest, sand accents
  return {
    bg:        '#0e1e16',       // very dark forest
    paper:     '#16291f',       // slightly elevated
    sunk:      '#0a1610',       // sunk
    bone:      'rgba(232, 212, 184, .12)',
    border:    'rgba(232, 212, 184, .18)',
    ink:       '#f3ead7',       // warm cream text
    ink2:      '#e8ddc3',
    muted:     'rgba(243, 234, 215, .65)',
    subtle:    'rgba(243, 234, 215, .45)',
    placeholder: 'rgba(243, 234, 215, .3)',
    forestBg:  '#1a3527',
    forestDeep: '#0b1d13',
    forestFg:  '#f6f1ea',
    forestPill:{ bg: 'rgba(168, 200, 178, .14)', fg: '#cfe5d6' },
    sand:      C.sand500,
    sand3:     C.sand300,
    sand1:     'rgba(232, 212, 184, .14)',
    sand7:     C.sand700,
    glassDark: 'rgba(0, 0, 0, .5)',
    hairline:  'rgba(243, 234, 215, .08)',
    onForest:  C.cream,
    tabBarGrad:'linear-gradient(180deg, rgba(14,30,22,0) 0%, rgba(14,30,22,.85) 40%, rgba(14,30,22,1) 100%)',
  };
}

// ─────────────────────────────────────────────────────────────
// Pull-to-refresh scroll wrapper
// ─────────────────────────────────────────────────────────────
function PullToRefresh({ children, onRefresh, pal }) {
  const scrollRef = React.useRef(null);
  const [pull, setPull] = React.useState(0);     // px pulled
  const [refreshing, setRefreshing] = React.useState(false);
  const start = React.useRef(null);

  const THRESH = 70;
  const MAX = 110;

  const onTouchStart = (e) => {
    if (!scrollRef.current) return;
    if (scrollRef.current.scrollTop <= 0) {
      start.current = e.touches ? e.touches[0].clientY : e.clientY;
    } else {
      start.current = null;
    }
  };
  const onTouchMove = (e) => {
    if (start.current == null || refreshing) return;
    const y = e.touches ? e.touches[0].clientY : e.clientY;
    const dy = y - start.current;
    if (dy > 0 && scrollRef.current.scrollTop <= 0) {
      const eased = Math.min(MAX, dy * 0.55);
      setPull(eased);
    } else if (dy < 0) {
      setPull(0);
    }
  };
  const onTouchEnd = () => {
    if (pull >= THRESH) {
      setRefreshing(true);
      setPull(48);
      setTimeout(() => {
        setRefreshing(false);
        setPull(0);
        if (onRefresh) onRefresh();
      }, 900);
    } else {
      setPull(0);
    }
    start.current = null;
  };

  const progress = Math.min(1, pull / THRESH);

  return (
    <div style={{ position: 'relative', height: '100%', overflow: 'hidden' }}>
      {/* Pull indicator */}
      <div className="pull-indicator" style={{
        position: 'absolute', top: 0, left: 0, right: 0, zIndex: 5,
        display: 'flex', justifyContent: 'center',
        transform: `translateY(${Math.max(0, pull - 36)}px)`,
        opacity: pull > 12 || refreshing ? 1 : 0,
        pointerEvents: 'none',
      }}>
        <div style={{
          width: 28, height: 28, borderRadius: 999,
          background: pal.paper,
          boxShadow: '0 2px 8px rgba(46,38,24,.18)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          {refreshing ? (
            <div style={{
              width: 14, height: 14, borderRadius: 999,
              border: `2px solid ${pal.muted}`, borderTopColor: pal.forestBg,
              animation: 'spin 700ms linear infinite',
            }} />
          ) : (
            <div style={{
              width: 14, height: 14, borderRadius: 999,
              border: `1.5px solid ${pal.muted}`,
              borderTopColor: 'transparent',
              transform: `rotate(${progress * 270}deg)`,
              transition: 'transform 80ms linear',
            }} />
          )}
        </div>
      </div>
      <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
      <div
        ref={scrollRef}
        className="ds-scroll"
        onTouchStart={onTouchStart}
        onTouchMove={onTouchMove}
        onTouchEnd={onTouchEnd}
        onPointerDown={onTouchStart}
        onPointerMove={onTouchMove}
        onPointerUp={onTouchEnd}
        onPointerCancel={onTouchEnd}
        style={{
          height: '100%', overflowY: 'auto', overflowX: 'hidden',
          transform: `translateY(${pull}px)`,
          transition: start.current == null ? 'transform 260ms var(--ease-out)' : 'none',
        }}
      >
        {children}
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Time-of-day chips (Morning / Afternoon / Evening)
// ─────────────────────────────────────────────────────────────
const TIMES = [
  { id: 'morning',   label: 'Утро',      sub: '06:42' },
  { id: 'afternoon', label: 'День',      sub: '13:18' },
  { id: 'evening',   label: 'Вечер',     sub: '20:04' },
];

function TimeOfDayChips({ value, onChange, pal, accent }) {
  return (
    <div style={{ display: 'flex', gap: 6, padding: '0 16px 14px', overflowX: 'auto' }} className="ds-scroll">
      {TIMES.map(t => {
        const on = value === t.id;
        return (
          <button
            key={t.id}
            onClick={() => onChange(t.id)}
            className="tod-chip"
            style={{
              flexShrink: 0, padding: '8px 14px', borderRadius: 999, cursor: 'pointer',
              background: on ? (accent || pal.forestBg) : 'transparent',
              color: on ? pal.forestFg : pal.muted,
              border: on ? '1px solid transparent' : `1px solid ${pal.border}`,
              fontFamily: F.body, fontSize: 13, fontWeight: 500,
              display: 'inline-flex', alignItems: 'center', gap: 8,
            }}>
            <span>{t.label}</span>
            <span style={{ fontFamily: F.mono, fontSize: 10, opacity: 0.7 }}>{t.sub}</span>
          </button>
        );
      })}
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Biomarker bottom sheet
// ─────────────────────────────────────────────────────────────
function BiomarkerSheet({ open, onClose, biomarker, pal, onOpenTrend }) {
  if (!open || !biomarker) return null;
  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 80 }}>
      <div className="scrim" onClick={onClose} style={{
        position: 'absolute', inset: 0, background: pal.glassDark,
        backdropFilter: 'blur(4px)',
      }} />
      <div className="sheet" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28,
        padding: '12px 20px 32px',
        boxShadow: '0 -18px 40px rgba(0,0,0,.18)',
      }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '0 auto 18px' }} />
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 14 }}>
          <div>
            <div style={{
              fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em',
              textTransform: 'uppercase', color: pal.subtle, marginBottom: 6,
            }}>{biomarker.eyebrow}</div>
            <div style={{ fontFamily: F.display, fontSize: 32, color: pal.ink, lineHeight: 1.05, letterSpacing: '-0.02em' }}>
              {biomarker.title}
            </div>
          </div>
          <button onClick={onClose} style={{
            width: 36, height: 36, borderRadius: 999, border: 'none', cursor: 'pointer',
            background: pal.sunk, color: pal.ink2, display: 'flex',
            alignItems: 'center', justifyContent: 'center',
          }}>
            <Icon name="x-mark" size={18} />
          </button>
        </div>

        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 6 }}>
          <span style={{ fontFamily: F.mono, fontSize: 56, fontWeight: 500, color: pal.ink, letterSpacing: '-0.03em', lineHeight: 1, fontVariantNumeric: 'tabular-nums' }}>{biomarker.value}</span>
          <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 22, color: pal.muted }}>{biomarker.unit}</span>
        </div>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: 20 }}>
          <Pill tone={biomarker.trend === 'down' ? 'forest' : biomarker.trend === 'up' ? 'forest' : 'neutral'} style={{ fontSize: 11 }}>
            {biomarker.delta}
          </Pill>
          <span style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle }}>к прошлой неделе</span>
        </div>

        {/* Mini chart */}
        <div style={{ padding: 16, background: pal.sunk, borderRadius: 18, marginBottom: 16 }}>
          <Spark data={biomarker.series} color={pal.forestBg === C.forest800 ? C.forest700 : '#a6c2af'} width={320} height={70} />
          <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 8, fontFamily: F.mono, fontSize: 10, color: pal.subtle }}>
            <span>Пн</span><span>Ср</span><span>Пт</span><span>Сегодня</span>
          </div>
        </div>

        <div style={{ fontFamily: F.body, fontSize: 13, lineHeight: 1.55, color: pal.ink2, marginBottom: 18 }}>
          {biomarker.note}
        </div>

        <Btn kind="primary" full onClick={() => {
          onClose && onClose();
          onOpenTrend && onOpenTrend(biomarker.trendId || 'weight');
        }}>Открыть детали тренда</Btn>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Tab bar — Cadence iOS style (overrides design-system TabBar bg for dark)
// ─────────────────────────────────────────────────────────────
function CadenceTabBar({ active, onChange, pal }) {
  const tabs = [
    { id: 'today',     name: 'home',       label: 'Сегодня' },
    { id: 'inventory', name: 'beaker',     label: 'Аптечка' },
    { id: 'log',       name: 'plus',       label: 'Записать', primary: true },
    { id: 'insights',  name: 'chart-bar',  label: 'Тренды' },
    { id: 'nutrition', name: 'cake',       label: 'Питание' },
  ];
  return (
    <div style={{
      position: 'absolute', left: 0, right: 0, bottom: 0, zIndex: 40,
      paddingBottom: 34, paddingTop: 8, paddingLeft: 8, paddingRight: 8,
      background: pal.tabBarGrad,
      backdropFilter: 'blur(10px)',
    }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5,1fr)', alignItems: 'end' }}>
        {tabs.map(t => {
          if (t.primary) {
            return (
              <button key={t.id} onClick={() => onChange(t.id)} className="press" style={{
                justifySelf: 'center', width: 52, height: 52, borderRadius: 999,
                background: pal.forestBg === C.forest800 ? C.forest700 : C.sand500,
                color: pal.forestBg === C.forest800 ? C.cream : C.ink900,
                border: 'none', cursor: 'pointer',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                boxShadow: '0 6px 16px rgba(45,95,63,.35)',
              }}>
                <Icon name="plus" size={24} strokeWidth={2} />
              </button>
            );
          }
          const on = active === t.id;
          return (
            <button key={t.id} onClick={() => onChange(t.id)} style={{
              background: 'none', border: 'none', cursor: 'pointer',
              display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2,
              padding: '6px 0',
              color: on ? (pal.forestBg === C.forest800 ? C.forest700 : C.sand500) : pal.subtle,
            }}>
              <Icon name={t.name} size={22} strokeWidth={on ? 1.8 : 1.5} />
              <span style={{ fontFamily: F.body, fontSize: 10, fontWeight: 500, letterSpacing: '.02em' }}>{t.label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

// Android-style bottom nav (Material 3 NavigationBar) — pill indicator behind active icon
function CadenceAndroidNav({ active, onChange, pal }) {
  const tabs = [
    { id: 'today',     name: 'home',       label: 'Сегодня' },
    { id: 'inventory', name: 'beaker',     label: 'Аптечка' },
    { id: 'log',       name: 'plus',       label: 'Записать' },
    { id: 'insights',  name: 'chart-bar',  label: 'Тренды' },
    { id: 'nutrition', name: 'cake',       label: 'Питание' },
  ];
  const isDark = pal.forestBg !== C.forest800;
  return (
    <div style={{
      position: 'absolute', left: 0, right: 0, bottom: 0, zIndex: 40,
      paddingTop: 12, paddingBottom: 16, paddingLeft: 4, paddingRight: 4,
      background: isDark ? 'rgba(14,30,22,.96)' : 'rgba(246,241,234,.96)',
      borderTop: `1px solid ${pal.hairline}`,
      backdropFilter: 'blur(10px)',
    }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5,1fr)' }}>
        {tabs.map(t => {
          const on = active === t.id;
          return (
            <button key={t.id} onClick={() => onChange(t.id)} style={{
              background: 'none', border: 'none', cursor: 'pointer',
              display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4,
              padding: '4px 0',
            }}>
              <div style={{
                width: 56, height: 30, borderRadius: 999,
                background: on ? (isDark ? 'rgba(232,212,184,.18)' : C.forest50) : 'transparent',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                transition: 'background 180ms var(--ease-out)',
                color: on ? (isDark ? C.sand300 : C.forest800) : pal.muted,
              }}>
                <Icon name={t.name} size={20} strokeWidth={on ? 2 : 1.5} />
              </div>
              <span style={{ fontFamily: F.body, fontSize: 10, fontWeight: on ? 600 : 500,
                color: on ? pal.ink : pal.muted }}>{t.label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Coach line — cycles through messages
// ─────────────────────────────────────────────────────────────
const COACH_LINES = [
  { lead: "Вы стали", emph: "легче", tail: "и спите глубже.", note: "Ровно там, где мы и надеялись. Держите утренний ритм." },
  { lead: "Восемь дней стабильного сна —", emph: "HRV растёт.", tail: "", note: "+3 мс за неделю. Восстановление догоняет." },
  { lead: "Половина четвёртой недели.", emph: "Держите ритм.", tail: "", note: "Следующая доза — воскресенье. Та же ротация, то же утро." },
  { lead: "Хотите среду помягче?", emph: "Вы заслужили тихий день.", tail: "", note: "Перенесите BPC-157 на вечер, силовую — пропустите." },
];
function useCoach(coachIndex) {
  return COACH_LINES[coachIndex % COACH_LINES.length];
}

// ─────────────────────────────────────────────────────────────
// Cycle ring — week N of 12
// ─────────────────────────────────────────────────────────────
function CycleRing({ week = 4, total = 12, size = 132, stroke = 10, pal, color }) {
  const r = (size - stroke) / 2;
  const cx = size / 2, cy = size / 2;
  const circ = 2 * Math.PI * r;
  const pct = week / total;
  const dash = circ * pct;
  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      <circle cx={cx} cy={cy} r={r} stroke={pal.bone} strokeWidth={stroke} fill="none" />
      <circle cx={cx} cy={cy} r={r}
        stroke={color || (pal.forestBg === C.forest800 ? C.forest700 : C.sand500)}
        strokeWidth={stroke} fill="none"
        strokeDasharray={`${dash} ${circ - dash}`}
        strokeDashoffset={circ / 4}
        strokeLinecap="round"
        transform={`rotate(-90 ${cx} ${cy})`}
        style={{ transition: 'stroke-dasharray 600ms var(--ease-out)' }}
      />
      {/* week tick marks every 1 week */}
      {Array.from({ length: total }).map((_, i) => {
        const a = (i / total) * Math.PI * 2 - Math.PI / 2;
        const inner = r - stroke / 2 - 4;
        const outer = r - stroke / 2 - 1;
        const x1 = cx + Math.cos(a) * inner;
        const y1 = cy + Math.sin(a) * inner;
        const x2 = cx + Math.cos(a) * outer;
        const y2 = cy + Math.sin(a) * outer;
        return <line key={i} x1={x1} y1={y1} x2={x2} y2={y2} stroke={pal.subtle} strokeWidth={1} opacity={0.5} />;
      })}
    </svg>
  );
}

Object.assign(window, {
  getPalette, PullToRefresh, TimeOfDayChips, BiomarkerSheet,
  CadenceTabBar, CadenceAndroidNav, useCoach, COACH_LINES, CycleRing, TIMES,
});
