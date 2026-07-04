// Cadence — scrubbable line chart with optional protocol annotations.
// Designed for the Trend Detail screen. SVG viewBox: 0..W × 0..H, scales fluidly.
//
// Props:
//   data         — { values: number[], days: number[], spanLabel: string }
//   accent       — 'forest' | 'sand'
//   pal          — palette from getPalette()
//   biom         — biomarker meta (for unit + decimals + label)
//   annotations  — bool, show PROTOCOL_EVENTS dashed lines + labels
//   showDoseRow  — bool, render the DOSE_SPANS pill row below the axis
//   height       — chart height (default 220)
//   onScrub      — optional (index, value) callback (for sibling stat cards, if wanted)

function ScrubChart({ data, accent, pal, biom, annotations, showDoseRow, height, onScrub }) {
  const W = 340;
  const H = height || 220;
  const padL = 8, padR = 8, padT = 28, padB = 24;
  const values = data.values;
  const days = data.days;
  const n = values.length;
  const minV = Math.min(...values);
  const maxV = Math.max(...values);
  const range = maxV - minV || 1;
  // Slight headroom so peaks don't kiss the top
  const yMin = minV - range * 0.12;
  const yMax = maxV + range * 0.12;
  const yRange = yMax - yMin || 1;

  const x = (i) => padL + (i * (W - padL - padR)) / (n - 1);
  const y = (v) => padT + ((yMax - v) / yRange) * (H - padT - padB);

  const xs = values.map((_, i) => x(i));
  const ys = values.map(v => y(v));

  const lineD = xs.map((xi, i) => (i === 0 ? 'M' : 'L') + xi + ' ' + ys[i]).join(' ');
  const fillD =
    lineD +
    ' L' + xs[n - 1] + ' ' + (H - padB) +
    ' L' + xs[0]     + ' ' + (H - padB) + ' Z';

  // Accent colors
  const isSand = accent === 'sand';
  const stroke = isSand ? '#a5773d' : C.forest700;
  const fillTop = isSand ? 'rgba(212,165,116,0.32)' : 'rgba(45,95,63,0.18)';
  const fillBot = isSand ? 'rgba(212,165,116,0.0)'  : 'rgba(45,95,63,0.0)';
  const dotFill = stroke;

  // Annotation positions
  const firstDay = days[0];
  const lastDay = days[days.length - 1];
  const annsInRange = annotations
    ? PROTOCOL_EVENTS.filter(e => e.day >= firstDay && e.day <= lastDay)
    : [];
  const annX = (day) => {
    const frac = (day - firstDay) / (lastDay - firstDay);
    return padL + frac * (W - padL - padR);
  };

  // Dose-span x positions
  const doseSpansInRange = showDoseRow
    ? DOSE_SPANS
        .map(s => ({
          ...s,
          a: Math.max(s.fromDay, firstDay),
          b: Math.min(s.toDay, lastDay),
        }))
        .filter(s => s.b - s.a >= (lastDay - firstDay) * 0.04)
    : [];

  // ── Scrub state ────────────────────────────────────────────────
  const wrapRef = React.useRef(null);
  // Default scrub index = last point (current reading)
  const [scrubI, setScrubI] = React.useState(n - 1);
  const [active, setActive] = React.useState(false);

  // Clamp during render so a stale index from a longer previous series
  // can't index past the end of a shorter new one. (useEffect reset alone
  // is too late — it runs after the first render with the new data.)
  const safeScrubI = Math.min(Math.max(scrubI, 0), n - 1);

  // Reset to "current" when the data changes (timeframe / biomarker switch).
  React.useEffect(() => {
    setScrubI(n - 1);
    setActive(false);
  }, [data]);

  const onPoint = (clientX) => {
    const el = wrapRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    // Map clientX → SVG x-coordinate (SVG is scaled to fit container)
    const px = ((clientX - rect.left) / rect.width) * W;
    // Find nearest index
    let best = 0, bestDx = Infinity;
    for (let i = 0; i < n; i++) {
      const dx = Math.abs(px - xs[i]);
      if (dx < bestDx) { bestDx = dx; best = i; }
    }
    if (best !== scrubI) setScrubI(best);
    if (onScrub) onScrub(best, values[best]);
  };

  const handleDown = (e) => {
    setActive(true);
    if (e.touches) onPoint(e.touches[0].clientX);
    else onPoint(e.clientX);
  };
  const handleMove = (e) => {
    if (!active) return;
    if (e.touches) onPoint(e.touches[0].clientX);
    else onPoint(e.clientX);
  };
  const handleUp = () => setActive(false);

  // Tooltip text
  const scrubDay = days[safeScrubI];
  const scrubValue = values[safeScrubI];
  const dayLabel = formatScrubDay(scrubDay, biom);
  // For 7d the scrub label should read like Mon/Tue; for longer ranges, "Wk 6" / "Day 42"
  const isCurrent = safeScrubI === n - 1;

  // Tooltip placement — keep within chart x-bounds
  const tipX = Math.min(Math.max(xs[safeScrubI], 64), W - 64);

  // ── Y-axis ticks (3 — min, mid, max in human terms) ──
  const yTicks = [yMax - yRange * 0.15, (yMin + yMax) / 2, yMin + yRange * 0.15];

  return (
    <div>
      {/* Stable header row above the chart — date label + scrub value.
          Lives outside the pointer-event wrapper so it can never collide with
          annotation labels at the top of the chart. */}
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end',
        padding: '0 2px 8px', gap: 12,
      }}>
        <div>
          <div style={{
            fontFamily: 'var(--font-body)', fontSize: 10, fontWeight: 500,
            letterSpacing: '.06em', textTransform: 'uppercase',
            color: pal.subtle, marginBottom: 2,
          }}>{isCurrent ? 'Сегодня' : dayLabel}</div>
          <div style={{
            fontFamily: 'var(--font-body)', fontSize: 11.5, color: pal.muted,
          }}>
            {active ? 'смотрим день' : isCurrent ? 'текущее значение' : 'нажмите, чтобы посмотреть другие дни'}
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
          <span style={{
            fontFamily: 'var(--font-mono)', fontSize: 26, fontWeight: 500,
            fontVariantNumeric: 'tabular-nums',
            color: pal.ink, letterSpacing: '-0.025em', lineHeight: 1,
          }}>{fmtValue(scrubValue, biom.decimals)}</span>
          <span style={{
            fontFamily: 'var(--font-display)', fontStyle: 'italic',
            fontSize: 14, color: pal.muted,
          }}>{biom.unit}</span>
        </div>
      </div>

      <div
        ref={wrapRef}
        onMouseDown={handleDown}
        onMouseMove={handleMove}
        onMouseUp={handleUp}
        onMouseLeave={handleUp}
        onTouchStart={handleDown}
        onTouchMove={handleMove}
        onTouchEnd={handleUp}
        onTouchCancel={handleUp}
        style={{
          position: 'relative',
          touchAction: 'none',
          userSelect: 'none',
          cursor: 'crosshair',
        }}
      >
      <svg viewBox={`0 0 ${W} ${H}`} style={{ width: '100%', height: 'auto', display: 'block' }}>
        <defs>
          <linearGradient id={`grad-${accent}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%"  stopColor={fillTop} />
            <stop offset="100%" stopColor={fillBot} />
          </linearGradient>
        </defs>

        {/* Horizontal hairlines */}
        {yTicks.map((tv, i) => (
          <line key={i}
            x1={padL} x2={W - padR}
            y1={y(tv)} y2={y(tv)}
            stroke={pal.hairline} strokeWidth="1" strokeDasharray="1 4"
          />
        ))}

        {/* Annotation dashed verticals — drawn under the line */}
        {annsInRange.map((e, i) => {
          const ex = annX(e.day);
          return (
            <g key={i}>
              <line x1={ex} x2={ex} y1={padT - 12} y2={H - padB}
                stroke={C.sand700} strokeOpacity="0.45"
                strokeWidth="1" strokeDasharray="2 3"
              />
            </g>
          );
        })}

        {/* Filled area */}
        <path d={fillD} fill={`url(#grad-${accent})`} />

        {/* Line */}
        <path d={lineD} stroke={stroke} strokeWidth="2.25" fill="none"
              strokeLinecap="round" strokeLinejoin="round" />

        {/* Annotation tiny dots on the line */}
        {annsInRange.map((e, i) => {
          const ex = annX(e.day);
          // interpolate y at that day
          const frac = (e.day - firstDay) / (lastDay - firstDay);
          const fi = frac * (n - 1);
          const lo = Math.floor(fi), hi = Math.ceil(fi);
          const yv = lo === hi ? ys[lo] : ys[lo] + (ys[hi] - ys[lo]) * (fi - lo);
          return (
            <circle key={`ann-dot-${i}`} cx={ex} cy={yv} r="3.5"
              fill={pal.bg} stroke={C.sand700} strokeWidth="1.5" />
          );
        })}

        {/* Scrubber vertical line */}
        <line
          x1={xs[safeScrubI]} x2={xs[safeScrubI]}
          y1={padT - 8} y2={H - padB}
          stroke={stroke} strokeOpacity={active ? 0.7 : 0.25}
          strokeWidth="1"
        />

        {/* Scrub dot */}
        <circle cx={xs[safeScrubI]} cy={ys[safeScrubI]} r={isCurrent ? 5 : 4.5}
          fill={pal.bg} stroke={stroke} strokeWidth="2" />
        {isCurrent && (
          <circle cx={xs[safeScrubI]} cy={ys[safeScrubI]} r="2.5" fill={dotFill} />
        )}

        {/* X-axis day labels (subtle, at four positions) */}
        {[0, Math.floor((n - 1) / 3), Math.floor(2 * (n - 1) / 3), n - 1].map((i, idx) => (
          <text key={`xl-${idx}`}
            x={xs[i]} y={H - padB + 14}
            textAnchor={idx === 0 ? 'start' : idx === 3 ? 'end' : 'middle'}
            fontFamily="JetBrains Mono, ui-monospace, monospace"
            fontSize="9"
            fill={pal.subtle}
            opacity="0.9"
          >
            {axisLabel(days[i], i === n - 1)}
          </text>
        ))}
      </svg>

      {/* Annotation labels (HTML, so they can wrap nicely) */}
      {annsInRange.map((e, i) => {
        const fracX = annX(e.day) / W;
        const leftPct = fracX * 100;
        // Stagger label vertically when annotations are close together
        const tooClose = annsInRange.some((o, j) => j < i && Math.abs(annX(o.day) - annX(e.day)) < 56);
        // Bias the label anchor toward the chart center near the edges,
        // so they don't get clipped on the right side of the card.
        const anchor = leftPct > 88 ? 'right' : leftPct < 12 ? 'left' : 'center';
        return (
          <div key={`ann-l-${i}`} style={{
            position: 'absolute',
            top: tooClose ? 14 : 0,
            left: anchor === 'right' ? 'auto' : anchor === 'left' ? '0' : `${leftPct}%`,
            right: anchor === 'right' ? '0' : 'auto',
            transform: anchor === 'center' ? 'translateX(-50%)' : 'none',
            display: 'flex', alignItems: 'center', gap: 4,
            background: pal.bg,
            border: `1px solid ${pal.hairline}`,
            padding: '2px 6px', borderRadius: 6,
            fontFamily: 'var(--font-body)', fontSize: 9.5, fontWeight: 500,
            color: C.sand700,
            whiteSpace: 'nowrap',
            pointerEvents: 'none',
          }}>
            <span style={{ width: 4, height: 4, borderRadius: 999, background: C.sand500 }} />
            {e.short}
          </div>
        );
      })}
      </div>

      {/* Dose-span pill row, sits below the chart (separate div, flow layout) */}
      {showDoseRow && doseSpansInRange.length > 0 && (
        <div style={{
          position: 'relative', height: 22,
          marginTop: 8,
        }}>
          {doseSpansInRange.map((s, i) => {
            const aFrac = (s.a - firstDay) / (lastDay - firstDay);
            const bFrac = (s.b - firstDay) / (lastDay - firstDay);
            const leftPct = aFrac * 100;
            const widthPct = (bFrac - aFrac) * 100;
            const isLast = i === doseSpansInRange.length - 1;
            return (
              <div key={i} style={{
                position: 'absolute',
                left: `calc(${leftPct}% + 2px)`,
                width: `calc(${widthPct}% - 4px)`,
                height: 20,
                background: isLast ? C.forest50 : pal.sunk,
                color: isLast ? C.forest800 : pal.muted,
                border: `1px solid ${isLast ? 'rgba(45,95,63,.18)' : pal.hairline}`,
                borderRadius: 6,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontFamily: 'var(--font-mono)', fontSize: 10,
                fontVariantNumeric: 'tabular-nums',
                letterSpacing: '-0.01em',
                overflow: 'hidden',
                whiteSpace: 'nowrap',
              }}>{s.dose}</div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// Date labels — short tag based on day index from today
function axisLabel(day, isLast) {
  if (isLast) return 'сегодня';
  const ago = TODAY_DAY - day;
  if (ago < 7) {
    const dows = ['Вс','Пн','Вт','Ср','Чт','Пт','Сб'];
    // assume today is Sunday for the mock
    const today = 0;
    return dows[(today - ago + 70) % 7];
  }
  if (ago < 28) {
    const w = Math.round(ago / 7);
    return `${w} нед`;
  }
  const w = Math.round(ago / 7);
  return `нед ${12 - w}`;
}

function formatScrubDay(day, biom) {
  const ago = TODAY_DAY - day;
  if (ago < 1) return 'Сегодня';
  if (ago < 7) {
    const dows = ['Вс','Пн','Вт','Ср','Чт','Пт','Сб'];
    return `${dows[(0 - Math.round(ago) + 70) % 7]} · ${Math.round(ago)} дн назад`;
  }
  const w = ago / 7;
  if (w < 13) return `неделя ${Math.max(1, 12 - Math.round(w) + 1)}`;
  return `${Math.round(ago)} дн назад`;
}

Object.assign(window, { ScrubChart });
