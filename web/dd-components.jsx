// Cadence — Doctor dashboard components
// Desktop-scale atoms built on the Cadence design language.
// Depends on globals from cadence-components.jsx (C, F, Icon) and dd-data.jsx.

// ── Avatar ────────────────────────────────────────────────────────────────
function Avatar({ pt, size = 40, ring }) {
  return (
    <div style={{
      width: size, height: size, borderRadius: 999,
      background: pt.bg, color: pt.fg,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      fontFamily: F.display, fontStyle: 'italic', fontSize: size * 0.44,
      flexShrink: 0, boxShadow: ring ? '0 0 0 3px ' + C.cream + ', 0 0 0 4px ' + ring : 'none',
    }}>{pt.initial}</div>
  );
}

// ── Desktop sparkline (area + line + endpoint) ─────────────────────────────
function DeskSpark({ data, color = C.forest700, fill = true, width = 116, height = 38, strokeWidth = 2 }) {
  const max = Math.max(...data), min = Math.min(...data);
  const pad = 3;
  const xs = data.map((_, i) => pad + i * (width - pad * 2) / (data.length - 1));
  const ys = data.map(v => pad + (max - v) / (max - min || 1) * (height - pad * 2));
  const d = xs.map((x, i) => (i === 0 ? 'M' : 'L') + x.toFixed(1) + ' ' + ys[i].toFixed(1)).join(' ');
  const dFill = d + ` L${xs[xs.length - 1].toFixed(1)} ${height - pad} L${pad} ${height - pad} Z`;
  const gid = 'sg' + Math.round(xs[0] * 1000 + color.charCodeAt(1));
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} style={{ display: 'block' }}>
      {fill && (
        <>
          <defs>
            <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor={color} stopOpacity="0.18" />
              <stop offset="1" stopColor={color} stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={dFill} fill={`url(#${gid})`} />
        </>
      )}
      <path d={d} stroke={color} strokeWidth={strokeWidth} fill="none" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={xs[xs.length - 1]} cy={ys[ys.length - 1]} r="2.6" fill={color} />
    </svg>
  );
}

// ── Flag pill ──────────────────────────────────────────────────────────────
function FlagPill({ kind, small }) {
  const meta = FLAG_META[kind];
  const palette = {
    danger:  { bg: C.dangerBg,  fg: '#6b2818', dot: C.danger },
    warning: { bg: C.warningBg, fg: '#7a4a06', dot: C.warning },
    info:    { bg: '#dfe6e9',   fg: '#2f4750', dot: '#4a6b7a' },
    forest:  { bg: C.forest50,  fg: C.forest800, dot: C.forest700 },
    sand:    { bg: C.sand100,   fg: '#6b4a25', dot: C.sand700 },
  }[meta.tone];
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: 5,
      padding: small ? '3px 8px' : '4px 10px', borderRadius: 999,
      background: palette.bg, color: palette.fg,
      fontFamily: F.body, fontSize: small ? 11 : 12, fontWeight: 500, whiteSpace: 'nowrap',
    }}>
      <Icon name={meta.icon} size={small ? 12 : 13} color={palette.dot} strokeWidth={1.8} />
      {meta.label}
    </span>
  );
}

// ── Status dot + label ───────────────────────────────────────────────────
function StatusTag({ status }) {
  const m = statusMeta(status);
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7 }}>
      <span style={{ width: 8, height: 8, borderRadius: 999, background: m.dot, flexShrink: 0 }} />
      <span style={{ fontFamily: F.body, fontSize: 13, fontWeight: 500, color: C.ink800 }}>{m.label}</span>
    </span>
  );
}

// ── Stat card (top strip) ──────────────────────────────────────────────────
function StatCard({ label, value, unit, sub, tone = 'paper', accent, icon }) {
  const isForest = tone === 'forest';
  return (
    <div style={{
      background: isForest ? C.forest800 : C.paper,
      color: isForest ? C.cream : C.ink900,
      border: isForest ? 'none' : '1px solid ' + C.bone,
      borderRadius: 18, padding: '18px 20px',
      boxShadow: isForest ? '0 8px 20px rgba(20,44,31,.18)' : '0 2px 6px rgba(46,38,24,.05)',
      display: 'flex', flexDirection: 'column', gap: 10, minWidth: 0,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
        <span style={{ fontFamily: F.body, fontSize: 12, fontWeight: 500, letterSpacing: '.12em', textTransform: 'uppercase', color: isForest ? 'rgba(246,241,234,.7)' : C.ink500 }}>{label}</span>
        {icon && (
          <span style={{ color: isForest ? C.sand500 : (accent || C.forest600), opacity: isForest ? 1 : .8 }}>
            <Icon name={icon} size={18} strokeWidth={1.7} />
          </span>
        )}
      </div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 5 }}>
        <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: 34, letterSpacing: '-0.03em', lineHeight: 1, color: isForest ? C.cream : C.ink900, fontVariantNumeric: 'tabular-nums' }}>{value}</span>
        {unit && <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 17, color: isForest ? C.sand300 : C.ink600 }}>{unit}</span>}
      </div>
      {sub && <div style={{ fontFamily: F.body, fontSize: 12.5, color: isForest ? 'rgba(246,241,234,.72)' : C.ink500 }}>{sub}</div>}
    </div>
  );
}

// ── Attention card (triage queue) ──────────────────────────────────────────
function AttentionCard({ pt, onOpen, onMessage }) {
  const [hover, setHover] = React.useState(false);
  return (
    <div
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={() => onOpen(pt)}
      style={{
        background: C.paper, border: '1px solid ' + (hover ? C.border : C.bone),
        borderRadius: 18, padding: 20, cursor: 'pointer', width: '100%',
        boxShadow: hover ? '0 10px 26px rgba(46,38,24,.10)' : '0 2px 6px rgba(46,38,24,.05)',
        transform: hover ? 'translateY(-2px)' : 'none',
        transition: 'all 200ms var(--ease-out)',
        display: 'flex', flexDirection: 'column', gap: 13,
        position: 'relative', overflow: 'hidden',
      }}>
      {/* terracotta edge for attention */}
      <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 4,
        background: pt.status === 'attention' ? C.danger : C.warning }} />
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <Avatar pt={pt} size={44} />
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ fontFamily: F.body, fontWeight: 600, fontSize: 15, color: C.ink900, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{pt.name}</div>
          <div style={{ fontFamily: F.body, fontSize: 12.5, color: C.ink500 }}>{pt.compound} · {pt.dose} · нед. {pt.week}</div>
        </div>
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
        {pt.flags.map(f => <FlagPill key={f} kind={f} small />)}
      </div>
      <div style={{ fontFamily: F.body, fontSize: 13.5, lineHeight: 1.5, color: C.ink600, flex: 1 }}>{pt.note}</div>
      <div style={{ display: 'flex', gap: 8, marginTop: 2 }}>
        <button onClick={(e) => { e.stopPropagation(); onOpen(pt); }} style={{
          flex: 1, fontFamily: F.body, fontWeight: 500, fontSize: 13, padding: '9px 0', borderRadius: 999,
          background: C.forest700, color: C.cream, border: 'none', cursor: 'pointer',
        }}>Открыть</button>
        <button onClick={(e) => { e.stopPropagation(); onMessage && onMessage(pt); }} style={{
          fontFamily: F.body, fontWeight: 500, fontSize: 13, padding: '9px 16px', borderRadius: 999,
          background: 'transparent', color: C.ink800, border: '1px solid ' + C.border, cursor: 'pointer',
          display: 'inline-flex', alignItems: 'center', gap: 6,
        }}><Icon name="paper-airplane" size={14} />Написать</button>
      </div>
    </div>
  );
}

// ── Roster row (table) ─────────────────────────────────────────────────────
function RosterRow({ pt, onOpen, last }) {
  const [hover, setHover] = React.useState(false);
  const sm = statusMeta(pt.status);
  const lostKg = lost(pt);
  const pct = lostPct(pt);
  const sparkColor = pt.status === 'attention' ? C.danger : pt.status === 'watch' ? C.warning : C.forest700;
  return (
    <div
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={() => onOpen(pt)}
      style={{
        display: 'grid',
        gridTemplateColumns: '1.7fr 1.25fr 0.8fr 1.15fr 0.5fr',
        alignItems: 'center', gap: 16, padding: '14px 18px', cursor: 'pointer',
        background: hover ? C.cream : 'transparent',
        borderBottom: last ? 'none' : '1px solid ' + C.hairline,
        transition: 'background 140ms',
      }}>
      {/* patient */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
        <div style={{ position: 'relative' }}>
          <Avatar pt={pt} size={38} />
          <span style={{ position: 'absolute', right: -1, bottom: -1, width: 11, height: 11, borderRadius: 999, background: sm.dot, border: '2px solid ' + C.paper }} />
        </div>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontFamily: F.body, fontWeight: 600, fontSize: 14, color: C.ink900, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{pt.name}</div>
          <div style={{ fontFamily: F.body, fontSize: 12, color: C.ink500 }}>{pt.age} лет · {pt.lastSeen}</div>
        </div>
      </div>
      {/* protocol */}
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 13.5, color: C.ink800, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{pt.compound}</div>
        <div style={{ fontFamily: F.mono, fontSize: 12, color: C.ink500, fontVariantNumeric: 'tabular-nums' }}>{pt.dose} · {pt.cadence}</div>
      </div>
      {/* cycle */}
      <div>
        <div style={{ fontFamily: F.mono, fontSize: 13, color: C.ink800, fontVariantNumeric: 'tabular-nums' }}>{pt.week}/{pt.cycleLen}</div>
        <div style={{ height: 4, borderRadius: 999, background: C.bone, marginTop: 5, overflow: 'hidden', width: 54 }}>
          <div style={{ height: '100%', width: (pt.week / pt.cycleLen * 100) + '%', background: sm.dot, borderRadius: 999 }} />
        </div>
      </div>
      {/* weight + spark */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <DeskSpark data={pt.spark} color={sparkColor} width={70} height={30} strokeWidth={1.8} />
        <div>
          <div style={{ fontFamily: F.mono, fontSize: 13.5, color: C.ink900, fontVariantNumeric: 'tabular-nums' }}>{pt.weight.toFixed(1).replace('.', ',')}<span style={{ fontSize: 11, color: C.ink500 }}> {pt.unit}</span></div>
          <div style={{ fontFamily: F.body, fontSize: 11.5, color: lostKg > 0 ? C.forest600 : C.ink500 }}>↓ {lostKg.toFixed(1).replace('.', ',')} кг</div>
        </div>
      </div>
      {/* adherence — removed */}
      {/* chevron */}
      <div style={{ display: 'flex', justifyContent: 'flex-end', color: hover ? C.forest700 : C.ink400, transition: 'color 140ms, transform 140ms', transform: hover ? 'translateX(2px)' : 'none' }}>
        <Icon name="chevron-right" size={18} />
      </div>
    </div>
  );
}

// ── Schedule item ────────────────────────────────────────────────────────
function ScheduleItem({ item, pt, onOpen }) {
  const state = {
    done: { color: C.forest600, bg: C.forest50, label: 'выполнено' },
    now:  { color: C.danger,    bg: C.dangerBg, label: 'сейчас' },
    due:  { color: C.ink500,    bg: C.linen,    label: 'предстоит' },
  }[item.state];
  const isCheck = item.kind === 'checkin';
  return (
    <div onClick={() => onOpen(pt)} style={{
      display: 'grid', gridTemplateColumns: '46px 1fr', gap: 12, alignItems: 'flex-start',
      cursor: 'pointer', padding: '4px 0',
    }}>
      <div style={{ fontFamily: F.mono, fontSize: 13, color: item.state === 'now' ? C.danger : C.ink600, fontVariantNumeric: 'tabular-nums', paddingTop: 1, fontWeight: item.state === 'now' ? 500 : 400 }}>{item.time}</div>
      <div style={{ display: 'flex', gap: 11, alignItems: 'center', minWidth: 0,
        opacity: item.state === 'done' ? 0.62 : 1 }}>
        <Avatar pt={pt} size={32} />
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 13.5, color: C.ink900, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{pt.name.split(' ')[0]} {pt.name.split(' ')[1] ? pt.name.split(' ')[1][0] + '.' : ''}</div>
          <div style={{ fontFamily: F.body, fontSize: 12, color: C.ink500, display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            <Icon name={isCheck ? 'chat-bubble' : 'beaker'} size={12} color={C.ink400} />
            {item.label}
          </div>
        </div>
        <span style={{ fontFamily: F.body, fontSize: 10.5, fontWeight: 500, color: state.color, background: state.bg, padding: '3px 8px', borderRadius: 999, whiteSpace: 'nowrap' }}>{state.label}</span>
      </div>
    </div>
  );
}

// ── Activity item ──────────────────────────────────────────────────────────
function ActivityItem({ act, pt, onOpen }) {
  const tones = {
    forest:  { bg: C.forest50, fg: C.forest700 },
    sand:    { bg: C.sand100,  fg: '#6b4a25' },
    warning: { bg: C.warningBg, fg: C.warning },
    danger:  { bg: C.dangerBg, fg: C.danger },
    info:    { bg: '#dfe6e9',  fg: '#4a6b7a' },
  }[act.tone];
  return (
    <div onClick={() => onOpen(pt)} style={{ display: 'flex', gap: 11, alignItems: 'flex-start', cursor: 'pointer', padding: '4px 0' }}>
      <div style={{ width: 30, height: 30, borderRadius: 9, background: tones.bg, color: tones.fg, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
        <Icon name={act.icon} size={15} strokeWidth={1.7} />
      </div>
      <div style={{ minWidth: 0, flex: 1, paddingTop: 1 }}>
        <div style={{ fontFamily: F.body, fontSize: 13, color: C.ink800, lineHeight: 1.4 }}>
          <span style={{ fontWeight: 600, color: C.ink900 }}>{pt.name.split(' ')[0]}</span> {act.text}
        </div>
        <div style={{ fontFamily: F.body, fontSize: 12, color: C.ink500 }}>{act.sub} · {act.time}</div>
      </div>
    </div>
  );
}

Object.assign(window, {
  Avatar, DeskSpark, FlagPill, StatusTag, StatCard,
  AttentionCard, RosterRow, ScheduleItem, ActivityItem,
});
