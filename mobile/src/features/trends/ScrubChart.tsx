// Cadence — scrubbable line chart with optional protocol annotations.
// Ported from design_handoff_cadence_app/trends/scrub-chart.jsx.
// Touch-scrub via PanResponder; SVG viewBox 0..W × 0..H scales to container width.
import React from 'react';
import { LayoutChangeEvent, PanResponder, Text, View } from 'react-native';
import Svg, {
  Circle,
  Defs,
  Line,
  LinearGradient,
  Path,
  Stop,
  Text as SvgText,
} from 'react-native-svg';
import { C, F, pal } from '../../theme';
import {
  DOSE_SPANS,
  PROTOCOL_EVENTS,
  TODAY_DAY,
  TimeframeSeries,
  TrendBiomarker,
  fmtValue,
} from './data';

// Date labels — short tag based on day index from today
function axisLabel(day: number, isLast: boolean): string {
  if (isLast) return 'сегодня';
  const ago = TODAY_DAY - day;
  if (ago < 7) {
    const dows = ['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'];
    // assume today is Sunday for the mock
    const today = 0;
    return dows[(today - Math.round(ago) + 70) % 7];
  }
  if (ago < 28) {
    const w = Math.round(ago / 7);
    return `${w} нед`;
  }
  const w = Math.round(ago / 7);
  return `нед ${12 - w}`;
}

function formatScrubDay(day: number): string {
  const ago = TODAY_DAY - day;
  if (ago < 1) return 'Сегодня';
  if (ago < 7) {
    const dows = ['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'];
    return `${dows[(0 - Math.round(ago) + 70) % 7]} · ${Math.round(ago)} дн назад`;
  }
  const w = ago / 7;
  if (w < 13) return `неделя ${Math.max(1, 12 - Math.round(w) + 1)}`;
  return `${Math.round(ago)} дн назад`;
}

// Annotation label — measures itself so it can center on its anchor x,
// clamped to the chart bounds (matches the prototype's edge anchoring).
function AnnLabel({
  leftPx,
  containerW,
  top,
  short,
}: {
  leftPx: number;
  containerW: number;
  top: number;
  short: string;
}) {
  const [w, setW] = React.useState(0);
  const left = Math.min(Math.max(leftPx - w / 2, 0), Math.max(containerW - w, 0));
  return (
    <View
      pointerEvents="none"
      onLayout={(e: LayoutChangeEvent) => setW(e.nativeEvent.layout.width)}
      style={{
        position: 'absolute',
        top,
        left,
        opacity: w === 0 ? 0 : 1,
        flexDirection: 'row',
        alignItems: 'center',
        gap: 4,
        backgroundColor: pal.bg,
        borderWidth: 1,
        borderColor: pal.hairline,
        paddingVertical: 2,
        paddingHorizontal: 6,
        borderRadius: 6,
      }}
    >
      <View style={{ width: 4, height: 4, borderRadius: 999, backgroundColor: C.sand500 }} />
      <Text
        numberOfLines={1}
        style={{ fontFamily: F.bodyMedium, fontSize: 9.5, color: C.sand700 }}
      >
        {short}
      </Text>
    </View>
  );
}

export function ScrubChart({
  data,
  accent,
  biom,
  annotations,
  showDoseRow,
  height,
  onScrub,
}: {
  data: TimeframeSeries;
  accent: string;
  biom: TrendBiomarker;
  annotations?: boolean;
  showDoseRow?: boolean;
  height?: number;
  onScrub?: (index: number, value: number) => void;
}) {
  const W = 340;
  const H = height || 220;
  const padL = 8,
    padR = 8,
    padT = 28,
    padB = 24;
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

  const x = (i: number) => padL + (i * (W - padL - padR)) / (n - 1);
  const y = (v: number) => padT + ((yMax - v) / yRange) * (H - padT - padB);

  const xs = values.map((_, i) => x(i));
  const ys = values.map((v) => y(v));

  const lineD = xs.map((xi, i) => (i === 0 ? 'M' : 'L') + xi + ' ' + ys[i]).join(' ');
  const fillD =
    lineD + ' L' + xs[n - 1] + ' ' + (H - padB) + ' L' + xs[0] + ' ' + (H - padB) + ' Z';

  // Accent colors
  const isSand = accent === 'sand';
  const stroke = isSand ? '#a5773d' : C.forest700;
  const fillTop = isSand ? 'rgba(212,165,116,0.32)' : 'rgba(45,95,63,0.18)';
  const fillBot = isSand ? 'rgba(212,165,116,0.0)' : 'rgba(45,95,63,0.0)';
  const dotFill = stroke;

  // Annotation positions
  const firstDay = days[0];
  const lastDay = days[days.length - 1];
  const annsInRange = annotations
    ? PROTOCOL_EVENTS.filter((e) => e.day >= firstDay && e.day <= lastDay)
    : [];
  const annX = (day: number) => {
    const frac = (day - firstDay) / (lastDay - firstDay);
    return padL + frac * (W - padL - padR);
  };

  // Dose-span x positions
  const doseSpansInRange = showDoseRow
    ? DOSE_SPANS.map((s) => ({
        ...s,
        a: Math.max(s.fromDay, firstDay),
        b: Math.min(s.toDay, lastDay),
      })).filter((s) => s.b - s.a >= (lastDay - firstDay) * 0.04)
    : [];

  // ── Scrub state ────────────────────────────────────────────────
  // Default scrub index = last point (current reading)
  const [scrubI, setScrubI] = React.useState(n - 1);
  const [active, setActive] = React.useState(false);
  const [chartW, setChartW] = React.useState(0);

  // Clamp during render so a stale index from a longer previous series
  // can't index past the end of a shorter new one.
  const safeScrubI = Math.min(Math.max(scrubI, 0), n - 1);

  // Reset to "current" when the data changes (timeframe / biomarker switch).
  React.useEffect(() => {
    setScrubI(n - 1);
    setActive(false);
  }, [data]);

  // Latest geometry for the (stable) PanResponder callbacks.
  const geo = React.useRef({ xs, n, values, chartW, onScrub });
  geo.current = { xs, n, values, chartW, onScrub };

  const onPoint = (localX: number) => {
    const g = geo.current;
    if (g.chartW <= 0) return;
    // Map touch x → SVG x-coordinate (SVG is scaled to fit container)
    const px = (localX / g.chartW) * W;
    // Find nearest index
    let best = 0,
      bestDx = Infinity;
    for (let i = 0; i < g.n; i++) {
      const dx = Math.abs(px - g.xs[i]);
      if (dx < bestDx) {
        bestDx = dx;
        best = i;
      }
    }
    setScrubI(best);
    if (g.onScrub) g.onScrub(best, g.values[best]);
  };

  const panResponder = React.useRef(
    PanResponder.create({
      onStartShouldSetPanResponder: () => true,
      onMoveShouldSetPanResponder: () => true,
      onPanResponderTerminationRequest: () => false,
      onShouldBlockNativeResponder: () => true,
      onPanResponderGrant: (evt) => {
        setActive(true);
        onPoint(evt.nativeEvent.locationX);
      },
      onPanResponderMove: (evt) => {
        onPoint(evt.nativeEvent.locationX);
      },
      onPanResponderRelease: () => setActive(false),
      onPanResponderTerminate: () => setActive(false),
    })
  ).current;

  // Tooltip text
  const scrubDay = days[safeScrubI];
  const scrubValue = values[safeScrubI];
  const dayLabel = formatScrubDay(scrubDay);
  const isCurrent = safeScrubI === n - 1;

  // ── Y-axis ticks (3 — min, mid, max in human terms) ──
  const yTicks = [yMax - yRange * 0.15, (yMin + yMax) / 2, yMin + yRange * 0.15];

  const svgH = chartW > 0 ? (chartW * H) / W : H;
  const toPx = (svgX: number) => (chartW > 0 ? (svgX / W) * chartW : 0);

  return (
    <View>
      {/* Stable header row above the chart — date label + scrub value. */}
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'flex-end',
          paddingHorizontal: 2,
          paddingBottom: 8,
          gap: 12,
        }}
      >
        <View style={{ flex: 1, minWidth: 0 }}>
          <Text
            numberOfLines={1}
            style={{
              fontFamily: F.bodyMedium,
              fontSize: 10,
              letterSpacing: 10 * 0.06,
              textTransform: 'uppercase',
              color: pal.subtle,
              marginBottom: 2,
            }}
          >
            {isCurrent ? 'Сегодня' : dayLabel}
          </Text>
          <Text
            numberOfLines={1}
            style={{ fontFamily: F.body, fontSize: 11.5, color: pal.muted }}
          >
            {active
              ? 'смотрим день'
              : isCurrent
                ? 'текущее значение'
                : 'нажмите, чтобы посмотреть другие дни'}
          </Text>
        </View>
        <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 4 }}>
          <Text
            style={{
              fontFamily: F.monoMedium,
              fontSize: 26,
              fontVariant: ['tabular-nums'],
              color: pal.ink,
              letterSpacing: 26 * -0.025,
              lineHeight: 26,
            }}
          >
            {fmtValue(scrubValue, biom.decimals)}
          </Text>
          <Text style={{ fontFamily: F.displayItalic, fontSize: 14, color: pal.muted }}>
            {biom.unit}
          </Text>
        </View>
      </View>

      <View
        onLayout={(e: LayoutChangeEvent) => setChartW(e.nativeEvent.layout.width)}
        {...panResponder.panHandlers}
      >
        <Svg width="100%" height={svgH} viewBox={`0 0 ${W} ${H}`}>
          <Defs>
            <LinearGradient id={`grad-${accent}`} x1="0" x2="0" y1="0" y2="1">
              <Stop offset="0%" stopColor={fillTop} />
              <Stop offset="100%" stopColor={fillBot} />
            </LinearGradient>
          </Defs>

          {/* Horizontal hairlines */}
          {yTicks.map((tv, i) => (
            <Line
              key={i}
              x1={padL}
              x2={W - padR}
              y1={y(tv)}
              y2={y(tv)}
              stroke={pal.hairline}
              strokeWidth={1}
              strokeDasharray="1 4"
            />
          ))}

          {/* Annotation dashed verticals — drawn under the line */}
          {annsInRange.map((e, i) => {
            const ex = annX(e.day);
            return (
              <Line
                key={`ann-${i}`}
                x1={ex}
                x2={ex}
                y1={padT - 12}
                y2={H - padB}
                stroke={C.sand700}
                strokeOpacity={0.45}
                strokeWidth={1}
                strokeDasharray="2 3"
              />
            );
          })}

          {/* Filled area */}
          <Path d={fillD} fill={`url(#grad-${accent})`} />

          {/* Line */}
          <Path
            d={lineD}
            stroke={stroke}
            strokeWidth={2.25}
            fill="none"
            strokeLinecap="round"
            strokeLinejoin="round"
          />

          {/* Annotation tiny dots on the line */}
          {annsInRange.map((e, i) => {
            const ex = annX(e.day);
            // interpolate y at that day
            const frac = (e.day - firstDay) / (lastDay - firstDay);
            const fi = frac * (n - 1);
            const lo = Math.floor(fi),
              hi = Math.ceil(fi);
            const yv = lo === hi ? ys[lo] : ys[lo] + (ys[hi] - ys[lo]) * (fi - lo);
            return (
              <Circle
                key={`ann-dot-${i}`}
                cx={ex}
                cy={yv}
                r={3.5}
                fill={pal.bg}
                stroke={C.sand700}
                strokeWidth={1.5}
              />
            );
          })}

          {/* Scrubber vertical line */}
          <Line
            x1={xs[safeScrubI]}
            x2={xs[safeScrubI]}
            y1={padT - 8}
            y2={H - padB}
            stroke={stroke}
            strokeOpacity={active ? 0.7 : 0.25}
            strokeWidth={1}
          />

          {/* Scrub dot */}
          <Circle
            cx={xs[safeScrubI]}
            cy={ys[safeScrubI]}
            r={isCurrent ? 5 : 4.5}
            fill={pal.bg}
            stroke={stroke}
            strokeWidth={2}
          />
          {isCurrent && (
            <Circle cx={xs[safeScrubI]} cy={ys[safeScrubI]} r={2.5} fill={dotFill} />
          )}

          {/* X-axis day labels (subtle, at four positions) */}
          {[0, Math.floor((n - 1) / 3), Math.floor((2 * (n - 1)) / 3), n - 1].map((i, idx) => (
            <SvgText
              key={`xl-${idx}`}
              x={xs[i]}
              y={H - padB + 14}
              textAnchor={idx === 0 ? 'start' : idx === 3 ? 'end' : 'middle'}
              fontFamily={F.mono}
              fontSize={9}
              fill={pal.subtle}
              opacity={0.9}
            >
              {axisLabel(days[i], i === n - 1)}
            </SvgText>
          ))}
        </Svg>

        {/* Annotation labels (overlaid, so they can sit above the plot) */}
        {chartW > 0 &&
          annsInRange.map((e, i) => {
            // Stagger label vertically when annotations are close together
            const tooClose = annsInRange.some(
              (o, j) => j < i && Math.abs(annX(o.day) - annX(e.day)) < 56
            );
            return (
              <AnnLabel
                key={`ann-l-${i}`}
                leftPx={toPx(annX(e.day))}
                containerW={chartW}
                top={tooClose ? 14 : 0}
                short={e.short}
              />
            );
          })}
      </View>

      {/* Dose-span pill row, sits below the chart */}
      {showDoseRow && doseSpansInRange.length > 0 && chartW > 0 && (
        <View style={{ position: 'relative', height: 22, marginTop: 8 }}>
          {doseSpansInRange.map((s, i) => {
            const aFrac = (s.a - firstDay) / (lastDay - firstDay);
            const bFrac = (s.b - firstDay) / (lastDay - firstDay);
            const isLast = i === doseSpansInRange.length - 1;
            return (
              <View
                key={i}
                style={{
                  position: 'absolute',
                  left: aFrac * chartW + 2,
                  width: (bFrac - aFrac) * chartW - 4,
                  height: 20,
                  backgroundColor: isLast ? C.forest50 : pal.sunk,
                  borderWidth: 1,
                  borderColor: isLast ? 'rgba(45,95,63,.18)' : pal.hairline,
                  borderRadius: 6,
                  alignItems: 'center',
                  justifyContent: 'center',
                  overflow: 'hidden',
                }}
              >
                <Text
                  numberOfLines={1}
                  style={{
                    fontFamily: F.mono,
                    fontSize: 10,
                    fontVariant: ['tabular-nums'],
                    letterSpacing: 10 * -0.01,
                    color: isLast ? C.forest800 : pal.muted,
                  }}
                >
                  {s.dose}
                </Text>
              </View>
            );
          })}
        </View>
      )}
    </View>
  );
}
