// Cadence — Trend detail screen (single biomarker drill-down).
// Ported from design_handoff_cadence_app/trends/trends.jsx (TrendDetail, cream header
// variant with protocol annotations on — the prototype shell's configuration).
import React from 'react';
import { ScrollView, Text, View } from 'react-native';
import Svg, { Line, Rect } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, SHADOW } from '../../theme';
import { Eyebrow, IconBtn, Press } from '../../components/primitives';
import { useAppState } from '../../state/AppState';
import { ScrubChart } from './ScrubChart';
import {
  TODAY_DAY,
  TREND_DATA,
  Timeframe,
  fmtValue,
  formatDeltaPill,
  seriesStats,
} from './data';

const TIMEFRAMES: { id: Timeframe; short: string }[] = [
  { id: '7d', short: '7 дней' },
  { id: '4w', short: '4 недели' },
  { id: '3m', short: '3 месяца' },
  { id: 'cycle', short: 'Весь цикл' },
];

function TimeframeChips({
  value,
  onChange,
}: {
  value: Timeframe;
  onChange: (tf: Timeframe) => void;
}) {
  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      contentContainerStyle={{ flexDirection: 'row', gap: 6, paddingHorizontal: 16 }}
      style={{ marginBottom: 14 }}
    >
      {TIMEFRAMES.map((tf) => {
        const on = value === tf.id;
        return (
          <Press
            key={tf.id}
            onPress={() => onChange(tf.id)}
            style={{
              paddingVertical: 8,
              paddingHorizontal: 14,
              borderRadius: 999,
              backgroundColor: on ? C.forest700 : pal.paper,
              borderWidth: 1,
              borderColor: on ? 'transparent' : pal.border,
            }}
          >
            <Text
              numberOfLines={1}
              style={{
                fontFamily: F.bodyMedium,
                fontSize: 13,
                color: on ? C.cream : pal.muted,
              }}
            >
              {tf.short}
            </Text>
          </Press>
        );
      })}
    </ScrollView>
  );
}

// Tiny correlation bar — bipolar -1..+1
function CorrelationBar({ strength }: { strength: number }) {
  const W = 56,
    H = 6;
  const mid = W / 2;
  const mag = Math.min(1, Math.abs(strength));
  const w = mag * (W / 2 - 1);
  const x = strength >= 0 ? mid : mid - w;
  const color = strength >= 0 ? C.forest700 : C.sand700;
  return (
    <Svg width={W} height={H} viewBox={`0 0 ${W} ${H}`}>
      <Rect x={0} y={H / 2 - 0.5} width={W} height={1} fill={pal.hairline} />
      <Line x1={mid} y1={0} x2={mid} y2={H} stroke={pal.border} strokeWidth={1} />
      <Rect x={x} y={1} width={w} height={H - 2} rx={(H - 2) / 2} fill={color} />
    </Svg>
  );
}

function dayLabel(day: number): string {
  const ago = TODAY_DAY - day;
  if (ago < 1) return 'Сегодня';
  if (ago === 1) return 'Вчера';
  if (ago < 7) return `${ago} дн назад`;
  const w = Math.round(ago / 7);
  return `${w} нед назад`;
}

export function TrendDetailScreen({
  biomarkerId,
  onBack,
}: {
  biomarkerId: string;
  onBack: () => void;
}) {
  const insets = useSafeAreaInsets();
  const { timeframe: tfRaw, setTimeframe, activeBiomarker, setActiveBiomarker } = useAppState();
  const timeframe = tfRaw as Timeframe;

  React.useEffect(() => {
    if (biomarkerId !== activeBiomarker) setActiveBiomarker(biomarkerId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const biom = TREND_DATA[activeBiomarker] || TREND_DATA.weight;
  const seriesAtTf = biom.series[timeframe];
  const last = seriesAtTf.values[seriesAtTf.values.length - 1];
  const delta = formatDeltaPill(seriesAtTf, biom);
  const stats = seriesStats(seriesAtTf, biom);
  const baselineDiff = last - biom.baseline;
  const baselineDelta =
    (baselineDiff > 0 ? '↑ ' : '↓ ') +
    fmtValue(Math.abs(baselineDiff), biom.decimals) +
    ' ' +
    (biom.unit.startsWith('/') ? '' : biom.unit);

  const deltaPrefix = delta.match(/^[↑↓]/)?.[0] || '';
  const deltaValue = delta.replace(/^[↑↓]\s*/, '');

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Top bar — fixed above the scroll region so back is always reachable */}
      <View
        style={{
          paddingTop: insets.top + 14,
          paddingHorizontal: 16,
          paddingBottom: 10,
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'center',
          backgroundColor: pal.bg,
        }}
      >
        <IconBtn name="chevron-left" size={20} onPress={onBack} bg={pal.sunk} color={pal.ink2} />
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.muted }}>
          {biom.label}
        </Text>
        <IconBtn name="ellipsis-horizontal" size={20} bg={pal.sunk} color={pal.ink2} />
      </View>

      <ScrollView
        showsVerticalScrollIndicator={false}
        contentContainerStyle={{
          paddingBottom: 40 + insets.bottom,
        }}
      >
        {/* Hero */}
        <View style={{ paddingTop: 6, paddingHorizontal: 24, paddingBottom: 22, marginBottom: 4 }}>
          <Eyebrow color={pal.subtle} style={{ marginBottom: 8 }}>
            {biom.eyebrow}
          </Eyebrow>

          <Text
            style={{
              fontFamily: F.display,
              fontSize: 36,
              lineHeight: 36 * 1.04,
              letterSpacing: 36 * -0.018,
              color: pal.ink,
              marginBottom: 18,
            }}
          >
            {biom.coach.lead}{' '}
            <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>
              {biom.coach.emph}
            </Text>
          </Text>

          <View
            style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8, marginBottom: 10 }}
          >
            <Text
              style={{
                fontFamily: F.monoMedium,
                fontSize: 64,
                fontVariant: ['tabular-nums'],
                color: pal.ink,
                letterSpacing: 64 * -0.03,
                lineHeight: 64,
              }}
            >
              {fmtValue(last, biom.decimals)}
            </Text>
            <Text style={{ fontFamily: F.displayItalic, fontSize: 22, color: pal.muted }}>
              {biom.unit}
            </Text>
          </View>

          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <View
              style={{
                paddingVertical: 3,
                paddingHorizontal: 10,
                borderRadius: 999,
                backgroundColor: C.forest50,
              }}
            >
              <Text
                numberOfLines={1}
                style={{ fontFamily: F.bodyMedium, fontSize: 11, color: C.forest800 }}
              >
                {delta}
              </Text>
            </View>
            <Text numberOfLines={1} style={{ fontFamily: F.body, fontSize: 12, color: pal.muted }}>
              за{' '}
              {timeframe === '7d'
                ? 'неделю'
                : timeframe === '4w'
                  ? 'месяц'
                  : timeframe === '3m'
                    ? '3 месяца'
                    : 'цикл'}
            </Text>
            {timeframe !== 'cycle' && (
              <>
                <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.muted, opacity: 0.4 }}>
                  ·
                </Text>
                <Text
                  numberOfLines={1}
                  style={{ fontFamily: F.body, fontSize: 12, color: pal.muted }}
                >
                  {baselineDelta} к старту
                </Text>
              </>
            )}
          </View>
        </View>

        {/* Timeframe chips */}
        <TimeframeChips value={timeframe} onChange={setTimeframe} />

        {/* Chart card */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              padding: 16,
              borderWidth: 1,
              borderColor: pal.hairline,
              ...SHADOW.sm,
            }}
          >
            <ScrubChart
              data={seriesAtTf}
              accent={biom.accent}
              biom={biom}
              annotations
              showDoseRow
              height={220}
            />
            <View
              style={{ marginTop: 8, flexDirection: 'row', alignItems: 'center', gap: 6 }}
            >
              <Svg width={18} height={2} style={{ opacity: 0.6 }}>
                <Line x1={0} y1={1} x2={18} y2={1} stroke={C.sand700} strokeWidth={1} strokeDasharray="3 3" />
              </Svg>
              <Text
                style={{
                  flex: 1,
                  fontFamily: F.body,
                  fontSize: 11.5,
                  lineHeight: 11.5 * 1.4,
                  color: pal.subtle,
                }}
              >
                Пунктир — изменения протокола. Тяните по графику, чтобы посмотреть любой день.
              </Text>
            </View>
          </View>
        </View>

        {/* Stat strip */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
              padding: 14,
              flexDirection: 'row',
            }}
          >
            {[
              { lbl: 'Сред.', v: stats.avg, prefix: '' },
              { lbl: 'Мин', v: stats.min, prefix: '' },
              { lbl: 'Макс', v: stats.max, prefix: '' },
              { lbl: 'Δ', v: deltaValue, prefix: deltaPrefix },
            ].map((s, i) => (
              <View
                key={i}
                style={{
                  flex: 1,
                  borderLeftWidth: i === 0 ? 0 : 1,
                  borderLeftColor: pal.hairline,
                  paddingLeft: i === 0 ? 0 : 12,
                }}
              >
                <Eyebrow color={pal.subtle} style={{ fontSize: 10, marginBottom: 4 }}>
                  {s.lbl}
                </Eyebrow>
                <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 2 }}>
                  {s.prefix ? (
                    <Text
                      style={{
                        fontFamily: F.mono,
                        fontSize: 14,
                        color: C.forest700,
                        fontVariant: ['tabular-nums'],
                      }}
                    >
                      {s.prefix}
                    </Text>
                  ) : null}
                  <Text
                    numberOfLines={1}
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 18,
                      color: pal.ink,
                      letterSpacing: 18 * -0.02,
                      fontVariant: ['tabular-nums'],
                      lineHeight: 18,
                    }}
                  >
                    {s.v}
                  </Text>
                </View>
              </View>
            ))}
          </View>
        </View>

        {/* Narrative + Correlations card */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              padding: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
            }}
          >
            <Eyebrow color={pal.subtle} style={{ marginBottom: 8 }}>
              Что связано
            </Eyebrow>
            <Text
              style={{
                fontFamily: F.display,
                fontSize: 19,
                color: pal.ink,
                lineHeight: 19 * 1.25,
                letterSpacing: 19 * -0.012,
                marginBottom: 14,
              }}
            >
              {biom.narrative}
            </Text>

            <View style={{ gap: 10 }}>
              {biom.correlations.map((c, i) => (
                <Press
                  key={i}
                  onPress={() => setActiveBiomarker(c.withId)}
                  style={{
                    flexDirection: 'row',
                    gap: 10,
                    alignItems: 'center',
                    paddingVertical: 10,
                    paddingHorizontal: 12,
                    backgroundColor: pal.sunk,
                    borderRadius: 12,
                  }}
                >
                  <View style={{ flex: 1, minWidth: 0 }}>
                    <View
                      style={{
                        flexDirection: 'row',
                        alignItems: 'center',
                        gap: 6,
                        marginBottom: 4,
                      }}
                    >
                      <Text
                        numberOfLines={1}
                        style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.ink }}
                      >
                        {biom.label}
                      </Text>
                      <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.subtle }}>
                        ↔
                      </Text>
                      <Text
                        numberOfLines={1}
                        style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.ink }}
                      >
                        {c.label}
                      </Text>
                    </View>
                    <Text
                      style={{
                        fontFamily: F.body,
                        fontSize: 12,
                        color: pal.muted,
                        lineHeight: 12 * 1.4,
                      }}
                    >
                      {c.note}
                    </Text>
                  </View>
                  <View style={{ alignItems: 'flex-end' }}>
                    <CorrelationBar strength={c.strength} />
                    <Text
                      style={{
                        marginTop: 4,
                        fontFamily: F.mono,
                        fontSize: 10,
                        color: pal.subtle,
                        fontVariant: ['tabular-nums'],
                      }}
                    >
                      r = {c.strength > 0 ? '+' : ''}
                      {c.strength.toFixed(2)}
                    </Text>
                  </View>
                </Press>
              ))}
            </View>
          </View>
        </View>

        {/* Recent entries */}
        <View style={{ paddingHorizontal: 16 }}>
          <View
            style={{
              flexDirection: 'row',
              justifyContent: 'space-between',
              alignItems: 'baseline',
              paddingHorizontal: 4,
              paddingBottom: 10,
            }}
          >
            <Eyebrow color={pal.subtle}>Последние записи</Eyebrow>
            <Press>
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
                Все
              </Text>
            </Press>
          </View>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
            }}
          >
            {biom.recent.map((r, i, arr) => (
              <React.Fragment key={i}>
                <View
                  style={{
                    flexDirection: 'row',
                    gap: 12,
                    alignItems: 'center',
                    paddingVertical: 12,
                    paddingHorizontal: 16,
                  }}
                >
                  <View style={{ flex: 1, minWidth: 0 }}>
                    <Text
                      numberOfLines={1}
                      style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.ink }}
                    >
                      {dayLabel(r.day)}
                    </Text>
                    {r.note ? (
                      <Text
                        numberOfLines={1}
                        style={{
                          fontFamily: F.body,
                          fontSize: 11.5,
                          color: pal.subtle,
                          marginTop: 1,
                        }}
                      >
                        {r.note}
                      </Text>
                    ) : null}
                  </View>
                  <View style={{ flexDirection: 'row', alignItems: 'baseline' }}>
                    <Text
                      style={{
                        fontFamily: F.monoMedium,
                        fontSize: 16,
                        color: pal.ink,
                        fontVariant: ['tabular-nums'],
                        letterSpacing: 16 * -0.02,
                      }}
                    >
                      {fmtValue(r.value, biom.decimals)}
                    </Text>
                    <Text
                      style={{
                        marginLeft: 3,
                        fontFamily: F.body,
                        fontSize: 11,
                        color: pal.subtle,
                      }}
                    >
                      {biom.unit}
                    </Text>
                  </View>
                </View>
                {i < arr.length - 1 && (
                  <View style={{ height: 1, backgroundColor: pal.hairline, marginLeft: 16 }} />
                )}
              </React.Fragment>
            ))}
          </View>
        </View>
      </ScrollView>
    </View>
  );
}
