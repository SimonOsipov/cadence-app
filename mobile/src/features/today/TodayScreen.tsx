// Today — home screen. Ported from variants.jsx (V1Refined).
// Hero dose card, biomarker glance, wellbeing nudge, reorder hint,
// weekly protocol, meal hero + today's meals.
import React, { useCallback, useState } from 'react';
import { RefreshControl, ScrollView, Text, View, useWindowDimensions } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Circle, Defs, RadialGradient, Stop } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, SHADOW } from '../../theme';
import { Eyebrow, Num, Pill, Press, Title } from '../../components/primitives';
import { Icon, IconName } from '../../components/Icon';
import { CadenceTabBar, Spark, TabId } from '../../components/shared';
import { useAppState } from '../../state/AppState';
import { QuickFeelSheet } from '../journal/QuickFeelSheet';
import { BiomarkerSheet, BiomarkerSheetData } from './BiomarkerSheet';

// Headline biomarker glance — ported from the inline config in variants.jsx.
const WEIGHT_GLANCE: BiomarkerSheetData = {
  trendId: 'weight',
  eyebrow: 'Вес · за неделю',
  title: 'Вы плавно идёте вниз.',
  value: '110,0',
  unit: 'кг',
  delta: '↓ 0,6 кг',
  trend: 'down',
  series: [0.7, 0.65, 0.6, 0.55, 0.5, 0.48, 0.4],
  note: 'Килограмм в неделю — золотая середина: достаточно быстро, чтобы заметить, и медленно, чтобы удержать. Если ускорится — подскажем.',
};

const GUTTER = { paddingHorizontal: 16, paddingBottom: 14 } as const;

export function TodayScreen({
  onLogDose,
  onPlusTap,
  onOpenTrends,
  onOpenVials,
  onOpenChat,
  onOpenProfile,
  onOpenSchedule,
  onOpenLearn,
  onOpenJournal,
  onLogMeal,
  onOpenNutrition,
  onOpenRecipes,
  onOpenTrend,
  onChangeTab,
}: {
  onLogDose: () => void;
  onPlusTap: () => void;
  onOpenTrends: () => void;
  onOpenVials: () => void;
  onOpenChat: () => void;
  onOpenProfile: () => void;
  onOpenSchedule: () => void;
  onOpenLearn: () => void;
  onOpenJournal: () => void;
  onLogMeal: () => void;
  onOpenNutrition: () => void;
  onOpenRecipes: () => void;
  onOpenTrend: (id: string) => void;
  onChangeTab: (id: TabId) => void;
}) {
  void onPlusTap;
  void onOpenTrends;
  const insets = useSafeAreaInsets();
  const { width } = useWindowDimensions();
  const app = useAppState();
  const {
    doseLogged,
    setDoseLogged,
    lastDoseSite,
    meals,
    mealTotals,
    mealTargets,
    mealHeroSuggestion,
    reorderHint,
    saveQuickFeel,
  } = app;

  const [sheet, setSheet] = useState<BiomarkerSheetData | null>(null);
  const [quickFeelOpen, setQuickFeelOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    setTimeout(() => setRefreshing(false), 900);
  }, []);

  const heroAction = () => {
    if (!doseLogged) {
      onLogDose();
      return;
    }
    setDoseLogged(!doseLogged);
  };

  // Glance spark fills half the card: screen − gutters − card pad − column gap.
  const glanceSparkW = (width - 32 - 32 - 14) / 2;

  const protocolRows: {
    icon: IconName;
    tone: { bg: string; fg: string };
    title: string;
    sub: string;
    trail: string;
    trailSub: string;
  }[] = [
    {
      icon: 'check-circle',
      tone: { bg: pal.forestPill.bg, fg: pal.forestPill.fg },
      title: 'Семаглутид · 0,25 мг',
      sub: 'Воскресенье 07:00 · еженедельно',
      trail: 'Сегодня',
      trailSub: doseLogged ? 'записано' : 'ждёт',
    },
    {
      icon: 'beaker',
      tone: { bg: C.sand100, fg: C.sand900 },
      title: 'BPC-157 · 250 мкг',
      sub: 'Ежедневно · подкожно',
      trail: '2× в день',
      trailSub: 'восстановление',
    },
    {
      icon: 'moon',
      tone: { bg: pal.sunk, fg: pal.ink2 },
      title: 'Глицин + магний',
      sub: 'На ночь · за 30 мин до сна',
      trail: '21:30',
      trailSub: 'вечером',
    },
  ];

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Header — fixed above the scroll region */}
      <View style={{ paddingTop: insets.top + 6, backgroundColor: pal.bg }}>
        <View style={{ paddingHorizontal: 20, paddingTop: 12, paddingBottom: 14 }}>
          <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginBottom: 8 }}>
            Воскресенье, утро · 4-я неделя
          </Text>
          <View
            style={{
              flexDirection: 'row',
              justifyContent: 'space-between',
              alignItems: 'flex-end',
              gap: 12,
            }}
          >
            <Text
              numberOfLines={1}
              style={{
                flex: 1,
                minWidth: 0,
                fontFamily: F.display,
                fontSize: 32,
                lineHeight: 32 * 1.05,
                letterSpacing: 32 * -0.018,
                color: pal.ink,
              }}
            >
              Марина
            </Text>
            <View style={{ flexDirection: 'row', gap: 8 }}>
              <Press
                onPress={onOpenChat}
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 999,
                  backgroundColor: C.forest50,
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Icon name="chat-bubble" size={20} color={C.forest700} />
                {/* Tiny presence dot to hint "doctor is online" */}
                <View
                  style={{
                    position: 'absolute',
                    top: 8,
                    right: 9,
                    width: 7,
                    height: 7,
                    borderRadius: 999,
                    backgroundColor: C.forest700,
                    borderWidth: 1.5,
                    borderColor: C.forest50,
                  }}
                />
              </Press>
              <Press
                onPress={onOpenSchedule}
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 999,
                  backgroundColor: pal.sunk,
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Icon name="calendar" size={20} color={pal.ink2} />
              </Press>
              <Press
                onPress={onOpenLearn}
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 999,
                  backgroundColor: pal.sunk,
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Icon name="book-open" size={20} color={pal.ink2} />
              </Press>
              <Press
                onPress={onOpenProfile}
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 999,
                  backgroundColor: C.forest700,
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Text style={{ fontFamily: F.displayItalic, fontSize: 18, color: C.cream }}>
                  М
                </Text>
              </Press>
            </View>
          </View>
        </View>
      </View>

      <ScrollView
        showsVerticalScrollIndicator={false}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={pal.subtle} />
        }
        contentContainerStyle={{ paddingBottom: 130 }}
      >
        {/* Hero dose card */}
        <View style={GUTTER}>
          <View
            style={{
              backgroundColor: pal.forestBg,
              borderRadius: 24,
              padding: 22,
              shadowColor: C.forest900,
              shadowOpacity: 0.18,
              shadowRadius: 24,
              shadowOffset: { width: 0, height: 8 },
              elevation: 8,
            }}
          >
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'flex-start',
                marginBottom: 12,
              }}
            >
              <View
                style={{
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: 6,
                  paddingVertical: 4,
                  paddingHorizontal: 10,
                  borderRadius: 999,
                  backgroundColor: 'rgba(246,241,234,.12)',
                }}
              >
                <View
                  style={{ width: 6, height: 6, borderRadius: 999, backgroundColor: C.sand500 }}
                />
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 11, color: C.sand300 }}>
                  Сегодня утром
                </Text>
              </View>
              <Text
                style={{
                  fontFamily: F.mono,
                  fontSize: 11,
                  color: 'rgba(246,241,234,.6)',
                  fontVariant: ['tabular-nums'],
                }}
              >
                06:42
              </Text>
            </View>
            <Text
              style={{
                fontFamily: F.display,
                fontSize: 36,
                lineHeight: 36 * 1.04,
                letterSpacing: 36 * -0.018,
                color: C.cream,
                marginBottom: 10,
              }}
            >
              Семаглутид{'\n'}
              <Text style={{ fontFamily: F.displayItalic, color: C.sand300 }}>0,25 мг</Text>
            </Text>
            <Text
              style={{
                fontFamily: F.body,
                fontSize: 13,
                color: 'rgba(246,241,234,.75)',
                lineHeight: 13 * 1.5,
                marginBottom: 16,
              }}
            >
              {doseLogged
                ? `Записано · ${(lastDoseSite || 'правый живот').toLowerCase()}, ротация.`
                : 'Недельная инъекция, запланирована на сегодня.'}
            </Text>
            <Press
              onPress={heroAction}
              style={{
                flexDirection: 'row',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 8,
                paddingVertical: 14,
                paddingHorizontal: 20,
                borderRadius: 999,
                backgroundColor: doseLogged ? 'rgba(246,241,234,.12)' : C.sand500,
              }}
            >
              {doseLogged ? (
                <>
                  <Icon name="check" size={16} strokeWidth={2} color={C.cream} />
                  <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>
                    Открыть детали
                  </Text>
                </>
              ) : (
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.ink900 }}>
                  Записать →
                </Text>
              )}
            </Press>
            {doseLogged && (
              <Press
                onPress={() => setQuickFeelOpen(true)}
                style={{
                  flexDirection: 'row',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 7,
                  marginTop: 10,
                  paddingVertical: 11,
                  paddingHorizontal: 16,
                  borderRadius: 999,
                  borderWidth: 1,
                  borderColor: 'rgba(246,241,234,.2)',
                }}
              >
                <Icon name="heart" size={15} color={C.sand300} />
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: C.sand300 }}>
                  Как перенесли дозу? Отметить
                </Text>
              </Press>
            )}
          </View>
        </View>

        {/* Headline biomarker — full width with chart on the right half */}
        <View style={GUTTER}>
          <Press
            onPress={() => setSheet(WEIGHT_GLANCE)}
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              padding: 16,
              borderWidth: 1,
              borderColor: pal.hairline,
              ...SHADOW.sm,
              flexDirection: 'row',
              alignItems: 'center',
              gap: 14,
            }}
          >
            <View style={{ flex: 1 }}>
              <Eyebrow color={pal.subtle} style={{ marginBottom: 10 }}>
                Вес · 7 дней
              </Eyebrow>
              <Num value="110,0" unit="кг" size={34} color={pal.ink} unitColor={pal.muted} />
              <View style={{ marginTop: 10 }}>
                <Pill tone="forest">↓ 0,6 кг</Pill>
              </View>
            </View>
            <View style={{ flex: 1, gap: 4 }}>
              <Spark
                data={[0.7, 0.65, 0.6, 0.55, 0.5, 0.48, 0.4]}
                color={C.forest700}
                fill={C.forest50}
                width={glanceSparkW}
                height={70}
              />
              <View
                style={{
                  flexDirection: 'row',
                  justifyContent: 'space-between',
                  paddingTop: 2,
                }}
              >
                {['Пн', 'Ср', 'Пт', 'Сег'].map((d) => (
                  <Text
                    key={d}
                    style={{
                      fontFamily: F.mono,
                      fontSize: 10,
                      color: pal.subtle,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {d}
                  </Text>
                ))}
              </View>
            </View>
          </Press>
        </View>

        {/* Wellbeing nudge — opens the side-effects journal */}
        <View style={GUTTER}>
          <Press
            onPress={onOpenJournal}
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              gap: 12,
              backgroundColor: pal.paper,
              borderWidth: 1,
              borderColor: pal.hairline,
              borderRadius: 18,
              padding: 14,
              ...SHADOW.sm,
            }}
          >
            <View
              style={{
                width: 44,
                height: 44,
                borderRadius: 12,
                backgroundColor: C.sand100,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name="heart" size={22} color="#9a5a3c" />
            </View>
            <View style={{ flex: 1, minWidth: 0 }}>
              <Text
                style={{
                  fontFamily: F.display,
                  fontSize: 20,
                  lineHeight: 20 * 1.12,
                  letterSpacing: 20 * -0.012,
                  color: pal.ink,
                }}
              >
                Как вы себя чувствуете?
              </Text>
              <Text
                style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}
              >
                Дневник самочувствия · сегодня не отмечено
              </Text>
            </View>
            <Icon name="chevron-right" size={18} color={pal.subtle} />
          </Press>
        </View>

        {/* Reorder reminder — surfaced when inventory has a low-stock compound */}
        {reorderHint && (
          <View style={GUTTER}>
            <Press
              onPress={onOpenVials}
              style={{
                flexDirection: 'row',
                alignItems: 'center',
                gap: 12,
                backgroundColor: C.warningBg,
                borderWidth: 1,
                borderColor: 'rgba(194,120,10,.18)',
                borderRadius: 16,
                padding: 14,
              }}
            >
              <View
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 12,
                  backgroundColor: 'rgba(194,120,10,.18)',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Icon name="information-circle" size={20} color={C.warning} />
              </View>
              <View style={{ flex: 1, minWidth: 0 }}>
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: '#7a4a06' }}>
                  {reorderHint.meta.name} закончится через ~{reorderHint.weeksLeft}{' '}
                  {reorderHint.weeksLeft === 1
                    ? 'неделю'
                    : reorderHint.weeksLeft < 5
                      ? 'недели'
                      : 'недель'}
                </Text>
                <Text
                  style={{
                    fontFamily: F.body,
                    fontSize: 11.5,
                    color: '#7a4a06',
                    opacity: 0.7,
                    marginTop: 1,
                  }}
                >
                  Запасного флакона нет
                </Text>
              </View>
              <View style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}>
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.warning }}>
                  В аптечку
                </Text>
                <Icon name="arrow-right" size={14} color={C.warning} />
              </View>
            </Press>
          </View>
        )}

        {/* This week's protocol */}
        <View style={GUTTER}>
          <View
            style={{
              flexDirection: 'row',
              justifyContent: 'space-between',
              alignItems: 'baseline',
              paddingHorizontal: 4,
              paddingBottom: 10,
            }}
          >
            <Eyebrow color={pal.subtle}>Протокол этой недели</Eyebrow>
            <Press onPress={onOpenSchedule}>
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
                Весь график
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
            {protocolRows.map((r, i) => (
              <React.Fragment key={r.title}>
                <View
                  style={{
                    flexDirection: 'row',
                    alignItems: 'center',
                    gap: 14,
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
                    <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}>
                      {r.title}
                    </Text>
                    <Text
                      style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}
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
                {i < protocolRows.length - 1 && (
                  <View style={{ height: 1, backgroundColor: pal.hairline, marginLeft: 68 }} />
                )}
              </React.Fragment>
            ))}
          </View>
        </View>

        {/* Sand meal hero — second floating hero, calling for the next meal */}
        <View style={GUTTER}>
          <MealHero
            totals={mealTotals}
            targets={mealTargets}
            now={mealHeroSuggestion?.now || '08:42'}
            suggestion={mealHeroSuggestion}
            onLogMeal={onLogMeal}
            onOpenRecipes={onOpenRecipes}
          />
        </View>

        {/* Today's meals card — replaces the old fuel strip */}
        <View style={GUTTER}>
          <TodayMeals
            meals={meals}
            totals={mealTotals}
            targets={mealTargets}
            onOpenNutrition={onOpenNutrition}
          />
        </View>
      </ScrollView>

      <CadenceTabBar active="today" onChange={onChangeTab} />

      <BiomarkerSheet
        open={!!sheet}
        biomarker={sheet}
        onClose={() => setSheet(null)}
        onOpenTrend={onOpenTrend}
      />

      <QuickFeelSheet
        open={quickFeelOpen}
        onCancel={() => setQuickFeelOpen(false)}
        onSave={(entry: any) => {
          saveQuickFeel(entry);
          setQuickFeelOpen(false);
          onOpenJournal();
        }}
      />
    </View>
  );
}

// ── MealHero — sand-warm CTA (ported from meal/meal-today.jsx) ──────────────
function MealHero({
  totals,
  targets,
  now,
  suggestion,
  onLogMeal,
  onOpenRecipes,
}: {
  totals: { kcal: number; p: number };
  targets: { kcal: number; protein: number };
  now: string;
  suggestion: { eyebrow: string; title: string; meta: string };
  onLogMeal: () => void;
  onOpenRecipes: () => void;
}) {
  const kcalLeft = Math.max(0, targets.kcal - totals.kcal);
  const proteinLeft = Math.max(0, targets.protein - Math.round(totals.p));
  const accentText = '#3a2a14';
  const sub = 'rgba(26,26,26,.62)';

  const words = suggestion.title.split(' ');
  const titleEl =
    words.length === 1 ? (
      <Text style={{ fontFamily: F.displayItalic, color: accentText }}>{suggestion.title}</Text>
    ) : (
      <>
        {words.slice(0, -1).join(' ')}
        {'\n'}
        <Text style={{ fontFamily: F.displayItalic, color: accentText }}>
          {words[words.length - 1]}
        </Text>
      </>
    );

  return (
    <View
      style={{
        backgroundColor: C.sand500,
        borderRadius: 24,
        padding: 22,
        overflow: 'hidden',
        shadowColor: '#b8874c',
        shadowOpacity: 0.22,
        shadowRadius: 22,
        shadowOffset: { width: 0, height: 8 },
        elevation: 8,
      }}
    >
      {/* soft sun highlight */}
      <View style={{ position: 'absolute', top: -32, right: -32 }} pointerEvents="none">
        <Svg width={140} height={140}>
          <Defs>
            <RadialGradient id="sun" cx="50%" cy="50%" r="50%">
              <Stop offset="0" stopColor="#fff0d2" stopOpacity={0.55} />
              <Stop offset="0.7" stopColor={C.sand500} stopOpacity={0} />
            </RadialGradient>
          </Defs>
          <Circle cx={70} cy={70} r={70} fill="url(#sun)" />
        </Svg>
      </View>

      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          marginBottom: 12,
          gap: 12,
        }}
      >
        <View
          style={{
            paddingVertical: 4,
            paddingHorizontal: 10,
            borderRadius: 999,
            backgroundColor: 'rgba(26,26,26,.12)',
          }}
        >
          <Text
            style={{
              fontFamily: F.bodyMedium,
              fontSize: 11,
              color: accentText,
              letterSpacing: 11 * 0.02,
            }}
          >
            {suggestion.eyebrow}
          </Text>
        </View>
        <Text
          style={{
            fontFamily: F.mono,
            fontSize: 11,
            color: sub,
            fontVariant: ['tabular-nums'],
          }}
        >
          {now}
        </Text>
      </View>

      <Text
        style={{
          fontFamily: F.display,
          fontSize: 36,
          lineHeight: 36 * 1.04,
          letterSpacing: 36 * -0.018,
          color: C.ink900,
          marginBottom: 10,
        }}
      >
        {titleEl}
      </Text>

      <Text
        style={{
          fontFamily: F.body,
          fontSize: 13,
          color: 'rgba(26,26,26,.72)',
          lineHeight: 13 * 1.5,
          marginBottom: 16,
        }}
      >
        <Text style={{ fontFamily: F.monoMedium, fontVariant: ['tabular-nums'] }}>
          {kcalLeft.toLocaleString()}
        </Text>{' '}
        ккал ·{' '}
        <Text style={{ fontFamily: F.monoMedium, fontVariant: ['tabular-nums'] }}>
          {proteinLeft}
        </Text>{' '}
        г белка осталось. {suggestion.meta}
      </Text>

      <Press
        onPress={onLogMeal}
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 8,
          paddingVertical: 14,
          paddingHorizontal: 20,
          borderRadius: 999,
          backgroundColor: C.forest700,
        }}
      >
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>
          Записать приём пищи
        </Text>
        <Text style={{ fontFamily: F.displayItalic, fontSize: 14, color: C.cream }}>→</Text>
      </Press>
      <Press
        onPress={onOpenRecipes}
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 7,
          marginTop: 8,
          paddingVertical: 11,
          paddingHorizontal: 16,
          borderRadius: 999,
          backgroundColor: 'rgba(26,26,26,.08)',
        }}
      >
        <Icon name="book-open" size={15} color={accentText} />
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: accentText }}>
          Из рецепта
        </Text>
      </Press>
    </View>
  );
}

// ── TodayMeals — paper card replacing the fuel strip ─────────────────────────
function TodayMeals({
  meals,
  totals,
  targets,
  onOpenNutrition,
}: {
  meals: any[];
  totals: { kcal: number; p: number; c: number; f: number };
  targets: { kcal: number; protein: number; carbs: number; fat: number };
  onOpenNutrition: () => void;
}) {
  const empty = meals.length === 0;
  const kcalPct = Math.min(1, totals.kcal / targets.kcal);
  const shown = meals.slice(-3);

  return (
    <Press
      onPress={onOpenNutrition}
      style={{
        backgroundColor: pal.paper,
        borderRadius: 18,
        padding: 16,
        borderWidth: 1,
        borderColor: pal.hairline,
      }}
    >
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'baseline',
          marginBottom: 10,
        }}
      >
        <Eyebrow color={pal.subtle}>Приёмы сегодня</Eyebrow>
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}>
          <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle }}>Питание</Text>
          <Icon name="arrow-right" size={12} color={pal.subtle} />
        </View>
      </View>

      {/* Totals row */}
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'baseline',
          marginBottom: 8,
          gap: 12,
        }}
      >
        <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 4 }}>
          <Text
            style={{
              fontFamily: F.monoMedium,
              fontSize: 30,
              color: pal.ink,
              letterSpacing: 30 * -0.03,
              lineHeight: 30,
              fontVariant: ['tabular-nums'],
            }}
          >
            {totals.kcal.toLocaleString()}
          </Text>
          <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle }}>
            / {targets.kcal.toLocaleString()} ккал
          </Text>
        </View>
        <View style={{ flexDirection: 'row', gap: 10 }}>
          {(
            [
              [Math.round(totals.p), targets.protein, 'P'],
              [Math.round(totals.c), targets.carbs, 'C'],
              [Math.round(totals.f), targets.fat, 'F'],
            ] as const
          ).map(([val, tgt, letter]) => (
            <Text
              key={letter}
              style={{ fontFamily: F.mono, fontSize: 10.5, fontVariant: ['tabular-nums'] }}
            >
              <Text style={{ color: pal.ink2 }}>{val}</Text>
              <Text style={{ color: pal.subtle }}>
                /{tgt}
                {letter}
              </Text>
            </Text>
          ))}
        </View>
      </View>

      {/* kcal progress bar */}
      <View
        style={{
          height: 6,
          backgroundColor: pal.sunk,
          borderRadius: 999,
          overflow: 'hidden',
          marginBottom: 14,
        }}
      >
        <LinearGradient
          colors={[C.sand500, C.forest700]}
          start={{ x: 0, y: 0 }}
          end={{ x: 1, y: 0 }}
          style={{ width: `${kcalPct * 100}%`, height: '100%', borderRadius: 999 }}
        />
      </View>

      {empty ? (
        <Text
          style={{
            paddingTop: 10,
            paddingHorizontal: 4,
            paddingBottom: 4,
            fontFamily: F.body,
            fontSize: 13,
            color: pal.muted,
            lineHeight: 13 * 1.5,
          }}
        >
          Сегодня пока ничего — первый приём приземлится здесь.
        </Text>
      ) : (
        <View>
          {shown.map((m, i) => (
            <React.Fragment key={m.id}>
              <View
                style={{
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: 12,
                  paddingVertical: 8,
                }}
              >
                <View
                  style={{
                    width: 32,
                    height: 32,
                    borderRadius: 10,
                    backgroundColor: C.sand100,
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <Text style={{ fontFamily: F.display, fontSize: 16, color: C.sand700 }}>
                    {(m.mealName || 'M').charAt(0).toUpperCase()}
                  </Text>
                </View>
                <View style={{ flex: 1, minWidth: 0 }}>
                  <Text
                    numberOfLines={1}
                    style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: pal.ink }}
                  >
                    {m.mealName}
                  </Text>
                  <Text
                    style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 1 }}
                  >
                    {m.time} · {m.items.length}{' '}
                    {m.items.length === 1
                      ? 'позиция'
                      : m.items.length < 5
                        ? 'позиции'
                        : 'позиций'}
                  </Text>
                </View>
                <Text style={{ textAlign: 'right' }}>
                  <Text
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 14,
                      color: pal.ink,
                      letterSpacing: 14 * -0.02,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {m.totals?.kcal}
                  </Text>
                  <Text style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle }}>
                    {' '}
                    ккал
                  </Text>
                </Text>
              </View>
              {i < shown.length - 1 && (
                <View style={{ height: 1, backgroundColor: pal.hairline, marginLeft: 44 }} />
              )}
            </React.Fragment>
          ))}
        </View>
      )}
    </Press>
  );
}
