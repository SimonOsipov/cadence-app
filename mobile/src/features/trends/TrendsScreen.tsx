// Cadence — Trends landing screen (TrendsLanding in trends/trends.jsx).
// Overview of all 7 biomarkers; tapping a card opens TrendDetail.
import React from 'react';
import { ScrollView, Text, View, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, SHADOW } from '../../theme';
import { Eyebrow, Pill, Press } from '../../components/primitives';
import { useAppState } from '../../state/AppState';
import { Icon } from '../../components/Icon';
import { CadenceTabBar, Spark, TabId } from '../../components/shared';
import {
  TREND_DATA,
  TREND_ORDER,
  Timeframe,
  fmtValue,
  formatDeltaPill,
  isProgress,
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
      contentContainerStyle={{ gap: 6, paddingHorizontal: 16, paddingBottom: 14 }}
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

// Spark colors for the landing biomarker cards (prototype MiniSpark).
function sparkColors(accent: string): { stroke: string; fill: string } {
  // Spark applies 0.4 opacity to its fill path — pre-divide the target alpha.
  if (accent === 'sand') {
    return { stroke: C.sand700, fill: 'rgba(212,165,116,0.5)' };
  }
  return { stroke: C.forest700, fill: 'rgba(45,95,63,0.35)' };
}

const NOTABLE_SHIFTS = [
  {
    icon: 'arrow-trending-up' as const,
    tone: { bg: C.forest50, fg: C.forest800 },
    title: 'HRV вырос на 17 мс',
    sub: 'С момента старта BPC-157',
    trail: '+31%',
    trailSub: 'к старту',
  },
  {
    icon: 'moon' as const,
    tone: { bg: C.sand100, fg: C.sand900 },
    title: 'Глубокий сон +24 мин',
    sub: 'В среднем за ночь',
    trail: '1ч 38м',
    trailSub: 'цель на ночь',
  },
  {
    icon: 'fire' as const,
    tone: { bg: C.forest50, fg: C.forest800 },
    title: 'Метаболизм стабилен',
    sub: 'Не падает на повышении дозы',
    trail: '1 720',
    trailSub: 'ккал / день',
  },
];

// Link-out row card (Самочувствие / Состав тела).
function LinkRowCard({
  icon,
  iconBg,
  iconColor,
  title,
  sub,
  onPress,
}: {
  icon: 'heart' | 'scale';
  iconBg: string;
  iconColor: string;
  title: string;
  sub: string;
  onPress?: () => void;
}) {
  return (
    <Press
      onPress={onPress}
      style={{
        flexDirection: 'row',
        alignItems: 'center',
        gap: 14,
        backgroundColor: pal.paper,
        borderWidth: 1,
        borderColor: pal.hairline,
        borderRadius: 18,
        padding: 14,
      }}
    >
      <View
        style={{
          width: 44,
          height: 44,
          borderRadius: 12,
          backgroundColor: iconBg,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={icon} size={22} color={iconColor} />
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text
          numberOfLines={1}
          style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}
        >
          {title}
        </Text>
        <Text
          numberOfLines={1}
          style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}
        >
          {sub}
        </Text>
      </View>
      <Icon name="chevron-right" size={18} color={pal.subtle} />
    </Press>
  );
}

export function TrendsScreen({
  onBack,
  onOpenDetail,
  onOpenJournal,
  onOpenBody,
  onChangeTab,
}: {
  onBack: () => void;
  onOpenDetail: (id: string) => void;
  onOpenJournal: () => void;
  onOpenBody: () => void;
  onChangeTab: (id: TabId) => void;
}) {
  const insets = useSafeAreaInsets();
  const { width } = useWindowDimensions();
  const { timeframe: tfRaw, setTimeframe, activeBiomarker } = useAppState();
  const timeframe = tfRaw as Timeframe;

  // Hero biomarker — featured card; the rest go in the 2-up grid.
  const hero = TREND_DATA[activeBiomarker] || TREND_DATA.weight;
  const heroSeries = hero.series[timeframe];
  const heroDelta = formatDeltaPill(heroSeries, hero);
  const heroSpark = sparkColors(hero.accent);
  const heroSparkW = width - 32 - 36; // gutters 16×2, card padding 18×2

  const others = TREND_ORDER.filter((id) => id !== hero.id);
  const cardW = (width - 32 - 10) / 2; // gutters + 10 gap
  const gridSparkW = cardW - 28; // card padding 14×2

  const tfLabel = TIMEFRAMES.find((t) => t.id === timeframe)!.short.toLowerCase();

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <ScrollView
        showsVerticalScrollIndicator={false}
        contentContainerStyle={{ paddingTop: insets.top + 6, paddingBottom: 130 }}
      >
        {/* Header */}
        <View
          style={{
            paddingTop: 12,
            paddingHorizontal: 20,
            paddingBottom: 14,
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'flex-end',
            gap: 12,
          }}
        >
          <View style={{ minWidth: 0, flex: 1 }}>
            <Text
              style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginBottom: 4 }}
            >
              12-я неделя · {tfLabel}
            </Text>
            <Text
              style={{
                fontFamily: F.display,
                fontSize: 36,
                lineHeight: 36,
                letterSpacing: 36 * -0.018,
                color: pal.ink,
              }}
            >
              Ваш{' '}
              <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>ритм</Text>
            </Text>
          </View>
          <View
            style={{
              width: 40,
              height: 40,
              borderRadius: 999,
              backgroundColor: C.forest700,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Text style={{ fontFamily: F.displayItalic, fontSize: 18, color: C.cream }}>M</Text>
          </View>
        </View>

        <TimeframeChips value={timeframe} onChange={setTimeframe} />

        {/* Hero biomarker — featured card with full mini chart */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <Press
            onPress={() => onOpenDetail(hero.id)}
            style={{
              backgroundColor: pal.paper,
              borderRadius: 20,
              padding: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
              ...SHADOW.sm,
            }}
          >
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'flex-start',
                marginBottom: 12,
                gap: 12,
              }}
            >
              <View>
                <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
                  {hero.eyebrow}
                </Eyebrow>
                <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 6 }}>
                  <Text
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 36,
                      fontVariant: ['tabular-nums'],
                      color: pal.ink,
                      letterSpacing: 36 * -0.03,
                      lineHeight: 36,
                    }}
                  >
                    {fmtValue(
                      hero.series.cycle.values[hero.series.cycle.values.length - 1],
                      hero.decimals
                    )}
                  </Text>
                  <Text
                    style={{ fontFamily: F.displayItalic, fontSize: 18, color: pal.muted }}
                  >
                    {hero.unit}
                  </Text>
                </View>
              </View>
              <View style={{ alignItems: 'flex-end' }}>
                <Pill tone="forest">{heroDelta}</Pill>
                <Text
                  style={{
                    marginTop: 6,
                    fontFamily: F.body,
                    fontSize: 11,
                    color: pal.subtle,
                  }}
                >
                  к {timeframe === 'cycle' ? 'старту цикла' : 'началу недели'}
                </Text>
              </View>
            </View>

            <View style={{ marginBottom: 10 }}>
              <Spark
                data={heroSeries.values}
                color={heroSpark.stroke}
                fill={heroSpark.fill}
                width={heroSparkW}
                height={60}
              />
            </View>

            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}
            >
              <Text style={{ fontFamily: F.body, fontSize: 12.5, color: pal.muted }}>
                Открыть детали
              </Text>
              <Icon name="arrow-right" size={16} color={C.forest700} />
            </View>
          </Press>
        </View>

        {/* Other biomarkers — 2-up grid */}
        <View
          style={{
            paddingHorizontal: 16,
            paddingBottom: 14,
            flexDirection: 'row',
            flexWrap: 'wrap',
            gap: 10,
          }}
        >
          {others.map((id) => {
            const b = TREND_DATA[id];
            const s = b.series[timeframe];
            const last = s.values[s.values.length - 1];
            const delta = formatDeltaPill(s, b);
            const prog = isProgress(s, b);
            const spark = sparkColors(b.accent);
            return (
              <Press
                key={id}
                onPress={() => onOpenDetail(id)}
                style={{
                  width: cardW,
                  backgroundColor: pal.paper,
                  borderRadius: 18,
                  padding: 14,
                  borderWidth: 1,
                  borderColor: pal.hairline,
                  ...SHADOW.xs,
                }}
              >
                <View style={{ marginBottom: 8 }}>
                  <Eyebrow color={pal.subtle}>{b.label}</Eyebrow>
                </View>
                <View
                  style={{
                    flexDirection: 'row',
                    alignItems: 'baseline',
                    gap: 4,
                    marginBottom: 8,
                  }}
                >
                  <Text
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 22,
                      fontVariant: ['tabular-nums'],
                      color: pal.ink,
                      letterSpacing: 22 * -0.025,
                      lineHeight: 22,
                    }}
                  >
                    {fmtValue(last, b.decimals)}
                  </Text>
                  <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>
                    {b.unit}
                  </Text>
                </View>
                <View style={{ marginBottom: 8 }}>
                  <Spark
                    data={s.values}
                    color={spark.stroke}
                    fill={spark.fill}
                    width={gridSparkW}
                    height={28}
                  />
                </View>
                <Pill tone={prog === false ? 'neutral' : 'forest'}>{delta}</Pill>
              </Press>
            );
          })}
        </View>

        {/* Notable shifts — narrative section */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View
            style={{
              flexDirection: 'row',
              justifyContent: 'space-between',
              alignItems: 'baseline',
              paddingHorizontal: 4,
              paddingBottom: 10,
            }}
          >
            <Eyebrow color={pal.subtle}>Заметные сдвиги</Eyebrow>
            <Text
              style={{
                fontFamily: F.mono,
                fontSize: 10,
                color: pal.subtle,
                fontVariant: ['tabular-nums'],
              }}
            >
              НЕД 12 / 12
            </Text>
          </View>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
            }}
          >
            {NOTABLE_SHIFTS.map((r, i) => (
              <React.Fragment key={i}>
                <View
                  style={{
                    flexDirection: 'row',
                    gap: 14,
                    alignItems: 'center',
                    paddingVertical: 12,
                    paddingHorizontal: 14,
                  }}
                >
                  <View
                    style={{
                      width: 40,
                      height: 40,
                      borderRadius: 12,
                      backgroundColor: r.tone.bg,
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <Icon name={r.icon} size={20} color={r.tone.fg} />
                  </View>
                  <View style={{ flex: 1, minWidth: 0 }}>
                    <Text
                      numberOfLines={1}
                      style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}
                    >
                      {r.title}
                    </Text>
                    <Text
                      numberOfLines={1}
                      style={{
                        fontFamily: F.body,
                        fontSize: 12,
                        color: pal.subtle,
                        marginTop: 1,
                      }}
                    >
                      {r.sub}
                    </Text>
                  </View>
                  <View style={{ alignItems: 'flex-end' }}>
                    <Text
                      style={{
                        fontFamily: F.mono,
                        fontSize: 13,
                        color: pal.ink2,
                        fontVariant: ['tabular-nums'],
                      }}
                    >
                      {r.trail}
                    </Text>
                    <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>
                      {r.trailSub}
                    </Text>
                  </View>
                </View>
                {i < NOTABLE_SHIFTS.length - 1 ? (
                  <View style={{ height: 1, backgroundColor: pal.hairline, marginLeft: 68 }} />
                ) : null}
              </React.Fragment>
            ))}
          </View>
        </View>

        {/* Самочувствие — opens the side-effects journal */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View style={{ paddingHorizontal: 4, paddingBottom: 10 }}>
            <Eyebrow color={pal.subtle}>Самочувствие</Eyebrow>
          </View>
          <LinkRowCard
            icon="heart"
            iconBg={C.sand100}
            iconColor={C.sand900}
            title="Дневник самочувствия"
            sub="Настроение, побочные, заметки по курсу"
            onPress={onOpenJournal}
          />
        </View>

        {/* Тело — opens the body-metrics screen */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View style={{ paddingHorizontal: 4, paddingBottom: 10 }}>
            <Eyebrow color={pal.subtle}>Состав тела</Eyebrow>
          </View>
          <LinkRowCard
            icon="scale"
            iconBg={C.forest50}
            iconColor={C.forest700}
            title="Тело · замеры и фото"
            sub="Состав, талия, бёдра, снимки"
            onPress={onOpenBody}
          />
        </View>
      </ScrollView>

      <CadenceTabBar active="insights" onChange={onChangeTab} />
    </View>
  );
}
