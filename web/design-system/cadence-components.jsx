// Cadence — shared brand components for the mobile UI kit
// Exposes: Eyebrow, Title, Body, Meta, Num, Pill, Chip, Btn, IconBtn,
//          Card, Section, ListRow, TabBar, AppHeader, Spark

const C = {
  paper: '#fbf8f3',
  cream: '#f6f1ea',
  linen: '#ede5d6',
  bone: '#e4dac6',
  border: '#cdc1a8',
  ink900: '#1a1a1a',
  ink800: '#2a2a2a',
  ink600: '#5c5852',
  ink500: '#8a857d',
  ink400: '#b3ad9f',
  forest900: '#142c1f',
  forest800: '#1f4530',
  forest700: '#2d5f3f',
  forest600: '#3d7a52',
  forest100: '#d8e5db',
  forest50: '#eaf0eb',
  sand700: '#b8895a',
  sand500: '#d4a574',
  sand300: '#e8d4b8',
  sand100: '#f3e8d6',
  danger: '#b8503c',
  dangerBg: '#f4dfd6',
  warning: '#c2780a',
  warningBg: '#fbeed1'
};
const F = {
  display: '"Instrument Serif", Georgia, serif',
  body: '"DM Sans", -apple-system, system-ui, sans-serif',
  mono: '"JetBrains Mono", ui-monospace, Menlo, monospace'
};

const Eyebrow = ({ children, color, style }) =>
<div style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em', textTransform: 'uppercase', color: color || C.ink500, whiteSpace: 'nowrap', ...style }}>{children}</div>;

const Title = ({ children, size = 28, italic, color, style }) =>
<div style={{ fontFamily: F.display, fontWeight: 400, fontStyle: italic ? 'italic' : 'normal', fontSize: size, lineHeight: 1.04, letterSpacing: '-0.018em', color: color || C.ink900, ...style }}>{children}</div>;

const Body = ({ children, size = 14, color, style }) =>
<div style={{ fontFamily: F.body, fontSize: size, lineHeight: 1.5, color: color || C.ink800, ...style }}>{children}</div>;

const Meta = ({ children, style }) =>
<div style={{ fontFamily: F.body, fontSize: 12, color: C.ink500, ...style }}>{children}</div>;

const Num = ({ value, unit, size = 44, color, unitColor, style }) =>
<div style={{ display: 'flex', alignItems: 'baseline', gap: 4, ...style }}>
    <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: size, letterSpacing: '-0.03em', color: color || C.ink900, fontVariantNumeric: 'tabular-nums', lineHeight: 1 }}>{value}</span>
    {unit && <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: size * 0.36, color: unitColor || C.ink600 }}>{unit}</span>}
  </div>;


const Pill = ({ children, tone = 'forest', style }) => {
  const palette = {
    forest: { bg: C.forest50, fg: C.forest800, dot: C.forest700 },
    sand: { bg: C.sand100, fg: '#6b4a25', dot: C.sand700 },
    warning: { bg: C.warningBg, fg: '#7a4a06', dot: C.warning },
    danger: { bg: C.dangerBg, fg: '#6b2818', dot: C.danger },
    neutral: { bg: C.linen, fg: C.ink800, dot: C.ink500 },
    dark: { bg: C.ink900, fg: C.sand300, dot: C.sand500 }
  }[tone];
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 10px', borderRadius: 999, background: palette.bg, color: palette.fg, fontFamily: F.body, fontSize: 11, fontWeight: 500, whiteSpace: 'nowrap', ...style }}>
      <span style={{ width: 6, height: 6, borderRadius: 999, background: palette.dot }} />
      {children}
    </span>);

};

const Chip = ({ children, active, onClick }) =>
<button onClick={onClick} style={{
  fontFamily: F.body, fontSize: 13, fontWeight: 500,
  padding: '7px 14px', borderRadius: 999, cursor: 'pointer',
  background: active ? C.ink900 : 'transparent',
  color: active ? C.cream : C.ink700,
  border: active ? '1px solid ' + C.ink900 : '1px solid ' + C.border,
  transition: 'all 140ms', whiteSpace: 'nowrap', flexShrink: 0
}}>{children}</button>;


const Btn = ({ children, kind = 'primary', size = 'md', onClick, style, full }) => {
  const sizes = {
    sm: { p: '8px 14px', fs: 13 },
    md: { p: '12px 20px', fs: 14 },
    lg: { p: '15px 22px', fs: 15 }
  }[size];
  const kinds = {
    primary: { bg: C.forest700, fg: C.cream, bd: 'transparent' },
    secondary: { bg: C.linen, fg: C.ink900, bd: C.bone },
    ghost: { bg: 'transparent', fg: C.forest700, bd: 'transparent' },
    dark: { bg: C.ink900, fg: C.sand300, bd: 'transparent' },
    danger: { bg: 'transparent', fg: C.danger, bd: C.danger }
  }[kind];
  return (
    <button onClick={onClick} style={{
      display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 8,
      width: full ? '100%' : 'auto', whiteSpace: 'nowrap',
      fontFamily: F.body, fontWeight: 500, fontSize: sizes.fs,
      padding: sizes.p, borderRadius: 999, cursor: 'pointer',
      background: kinds.bg, color: kinds.fg, border: '1px solid ' + kinds.bd,
      transition: 'all 140ms', ...style
    }}>{children}</button>);

};

const IconBtn = ({ name, size = 20, onClick, bg = 'transparent', color = C.ink800, style }) =>
<button onClick={onClick} style={{
  width: 40, height: 40, borderRadius: 999, border: 'none',
  background: bg, color, display: 'flex', alignItems: 'center', justifyContent: 'center',
  cursor: 'pointer', ...style
}}>
    <Icon name={name} size={size} />
  </button>;


const Card = ({ children, tone = 'paper', elev = 'sm', pad = 16, radius = 18, style, onClick }) => {
  const tones = {
    paper: { bg: C.paper, color: C.ink900, border: 'none' },
    cream: { bg: C.cream, color: C.ink900, border: 'none' },
    linen: { bg: C.linen, color: C.ink900, border: 'none' },
    forest: { bg: C.forest800, color: C.cream, border: 'none' },
    sand: { bg: C.sand300, color: C.ink900, border: 'none' },
    outline: { bg: C.paper, color: C.ink900, border: '1px solid ' + C.bone }
  }[tone];
  const elevs = {
    none: 'none',
    sm: '0 2px 6px rgba(46,38,24,.06), 0 1px 2px rgba(46,38,24,.04)',
    md: '0 8px 20px rgba(46,38,24,.08), 0 2px 4px rgba(46,38,24,.04)'
  };
  return (
    <div onClick={onClick} style={{
      background: tones.bg, color: tones.color, border: tones.border,
      borderRadius: radius, padding: pad, boxShadow: elevs[elev],
      cursor: onClick ? 'pointer' : 'default', ...style
    }}>{children}</div>);

};

const Section = ({ title, action, children, style }) =>
<div style={{ display: 'flex', flexDirection: 'column', gap: 10, ...style }}>
    {(title || action) &&
  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px' }}>
        {title && <Eyebrow>{title}</Eyebrow>}
        {action && <button style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500, whiteSpace: 'nowrap' }}>{action}</button>}
      </div>
  }
    {children}
  </div>;


const ListRow = ({ icon, iconTone = 'forest', title, sub, trail, trailSub, onClick }) => {
  const tones = {
    forest: { bg: C.forest50, fg: C.forest700 },
    sand: { bg: C.sand100, fg: '#6b4a25' },
    linen: { bg: C.linen, fg: C.ink700 },
    danger: { bg: C.dangerBg, fg: C.danger }
  }[iconTone];
  return (
    <div onClick={onClick} style={{
      display: 'grid', gridTemplateColumns: '40px 1fr auto', gap: 14, alignItems: 'center',
      padding: '12px 14px', cursor: onClick ? 'pointer' : 'default'
    }}>
      <div style={{ width: 40, height: 40, borderRadius: 12, background: tones.bg, color: tones.fg, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        {typeof icon === 'string' ? <Icon name={icon} size={20} /> : icon}
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: C.ink900, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</div>
        {sub && <div style={{ fontFamily: F.body, fontSize: 12, color: C.ink500, marginTop: 1 }}>{sub}</div>}
      </div>
      {trail &&
      <div style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
          <div style={{ fontFamily: F.mono, fontSize: 13, color: C.ink800, fontVariantNumeric: 'tabular-nums' }}>{trail}</div>
          {trailSub && <div style={{ fontFamily: F.body, fontSize: 11, color: C.ink500 }}>{trailSub}</div>}
        </div>
      }
    </div>);

};

// Bottom tab bar
const TabBar = ({ active, onChange }) => {
  const tabs = [
  { id: 'today', name: 'home', label: 'Today' },
  { id: 'inventory', name: 'beaker', label: 'Vials' },
  { id: 'log', name: 'plus', label: 'Log', primary: true },
  { id: 'insights', name: 'chart-bar', label: 'Trends' },
  { id: 'learn', name: 'book-open', label: 'Learn' }];

  return (
    <div style={{
      position: 'absolute', left: 0, right: 0, bottom: 0, zIndex: 40,
      paddingBottom: 30, paddingTop: 6, paddingLeft: 8, paddingRight: 8,
      background: 'linear-gradient(180deg, rgba(246,241,234,0) 0%, rgba(246,241,234,0.85) 40%, rgba(246,241,234,1) 100%)',
      backdropFilter: 'blur(8px)'
    }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5,1fr)', alignItems: 'end' }}>
        {tabs.map((t) => {
          if (t.primary) {
            return (
              <button key={t.id} onClick={() => onChange(t.id)} style={{
                justifySelf: 'center', width: 52, height: 52, borderRadius: 999,
                background: C.forest700, color: C.cream, border: 'none', cursor: 'pointer',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                boxShadow: '0 6px 16px rgba(45,95,63,.35)'
              }}>
                <Icon name="plus" size={24} strokeWidth={2} />
              </button>);

          }
          const on = active === t.id;
          return (
            <button key={t.id} onClick={() => onChange(t.id)} style={{
              background: 'none', border: 'none', cursor: 'pointer',
              display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2,
              padding: '6px 0', color: on ? C.forest700 : C.ink500
            }}>
              <Icon name={t.name} size={22} strokeWidth={on ? 1.8 : 1.5} />
              <span style={{ fontFamily: F.body, fontSize: 10, fontWeight: 500, letterSpacing: '.02em' }}>{t.label}</span>
            </button>);

        })}
      </div>
    </div>);

};

// App header — large serif title + avatar
const AppHeader = ({ greeting, name, onMenu, avatar = 'M' }) =>
<div style={{ padding: '8px 20px 18px', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 12 }}>
    <div style={{ minWidth: 0, flex: 1 }}>
      <Meta style={{ marginBottom: 4, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{greeting}</Meta>
      <Title size={32} style={{ whiteSpace: 'nowrap' }}>{name}</Title>
    </div>
    <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexShrink: 0 }}>
      <IconBtn name="bell" bg={C.linen} />
      <div style={{ width: 40, height: 40, borderRadius: 999, background: C.forest700, color: C.cream, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: F.display, fontStyle: 'italic', fontSize: 18 }}>{avatar}</div>
    </div>
  </div>;


// Tiny spark line for trend cards
const Spark = ({ data = [0.3, 0.5, 0.4, 0.6, 0.55, 0.7, 0.8], color = C.forest700, fill, width = 120, height = 36 }) => {
  const max = Math.max(...data),min = Math.min(...data);
  const pad = 2;
  const xs = data.map((_, i) => pad + i * (width - pad * 2) / (data.length - 1));
  const ys = data.map((v) => pad + (max - v) / (max - min || 1) * (height - pad * 2));
  const d = xs.map((x, i) => (i === 0 ? 'M' : 'L') + x + ' ' + ys[i]).join(' ');
  const dFill = d + ' L' + xs[xs.length - 1] + ' ' + (height - pad) + ' L' + pad + ' ' + (height - pad) + ' Z';
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`}>
      {fill && <path d={dFill} fill={fill} opacity="0.4" />}
      <path d={d} stroke={color} strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={xs[xs.length - 1]} cy={ys[ys.length - 1]} r="3" fill={color} />
    </svg>);

};

Object.assign(window, {
  C, F, Eyebrow, Title, Body, Meta, Num, Pill, Chip, Btn, IconBtn,
  Card, Section, ListRow, TabBar, AppHeader, Spark
});