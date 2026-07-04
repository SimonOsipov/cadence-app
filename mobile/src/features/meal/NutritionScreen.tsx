// Cadence — Nutrition Today screen (RN port of meal/nutrition.jsx).
import React, { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import Svg, { Circle, G, Line, Rect, Text as SvgText } from 'react-native-svg';
import { C, F, pal } from '../../theme';
import { Eyebrow, IconBtn, Pill, Press } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { CadenceTabBar, TabId } from '../../components/shared';
import { useAppState } from '../../state/AppState';
import { LoggedMeal, MealTotals } from './data';

// ── Plural helpers (copy from prototype) ────────────────────────────

function mealsWord(n: number) {
  return n === 1 ? 'приём' : n < 5 ? 'приёма' : 'приёмов';
}
function itemsWord(n: number) {
  return n === 1 ? 'позиция' : n < 5 ? 'позиции' : 'позиций';
}

// ── NutritionRing — concentric arcs for kcal % and protein % ────────

function NutritionRing({
  totals,
  targets,
}: {
  totals: MealTotals;
  targets: { kcal: number; protein: number };
}) {
  const size = 120;
  const stroke = 10;
  const r = (size - stroke) / 2;
  const cx = size / 2;
  const cy = size / 2;
  const circ = 2 * Math.PI * r;
  const kcalPct = Math.min(1, totals.kcal / targets.kcal);
  const proteinPct = Math.min(1, totals.p / targets.protein);

  // Inner protein ring
  const innerR = r - (stroke + 4);
  const innerCirc = 2 * Math.PI * innerR;

  return (
    <Svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      {/* kcal track */}
      <Circle cx={cx} cy={cy} r={r} fill="none" stroke={pal.sunk} strokeWidth={stroke} />
      {/* kcal arc */}
      <Circle
        cx={cx}
        cy={cy}
        r={r}
        fill="none"
        stroke={C.sand500}
        strokeWidth={stroke}
        strokeLinecap="round"
        strokeDasharray={`${kcalPct * circ} ${circ}`}
        transform={`rotate(-90 ${cx} ${cy})`}
      />
      {/* protein track */}
      <Circle cx={cx} cy={cy} r={innerR} fill="none" stroke={pal.sunk} strokeWidth={6} />
      {/* protein arc */}
      <Circle
        cx={cx}
        cy={cy}
        r={innerR}
        fill="none"
        stroke={C.forest700}
        strokeWidth={6}
        strokeLinecap="round"
        strokeDasharray={`${proteinPct * innerCirc} ${innerCirc}`}
        transform={`rotate(-90 ${cx} ${cy})`}
      />
      <SvgText
        x={cx}
        y={cy - 4}
        textAnchor="middle"
        fontFamily={F.bodyMedium}
        fontSize={10}
        letterSpacing={10 * 0.06}
        fill={pal.subtle}
      >
        ККАЛ
      </SvgText>
      <SvgText
        x={cx}
        y={cy + 14}
        textAnchor="middle"
        fontFamily={F.monoMedium}
        fontSize={22}
        letterSpacing={22 * -0.02}
        fill={pal.ink}
      >
        {`${Math.round(kcalPct * 100)}%`}
      </SvgText>
    </Svg>
  );
}

// ── MacroBar — labelled progress bar with goal ──────────────────────

function MacroBar({
  label,
  v,
  goal,
  color,
  last,
}: {
  label: string;
  v: number;
  goal: number;
  color: string;
  last?: boolean;
}) {
  const pct = Math.min(1, v / Math.max(1, goal));
  return (
    <View style={{ marginBottom: last ? 0 : 8 }}>
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'baseline',
          marginBottom: 4,
        }}
      >
        <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.muted }}>{label}</Text>
        <Text
          style={{
            fontFamily: F.mono,
            fontSize: 11,
            color: pal.ink2,
            fontVariant: ['tabular-nums'],
          }}
        >
          {v}
          <Text style={{ color: pal.subtle }}>/{goal} г</Text>
        </Text>
      </View>
      <View
        style={{
          height: 5,
          backgroundColor: pal.sunk,
          borderRadius: 999,
          overflow: 'hidden',
        }}
      >
        <View
          style={{
            width: `${pct * 100}%`,
            height: '100%',
            backgroundColor: color,
            borderRadius: 999,
          }}
        />
      </View>
    </View>
  );
}

// ── MealCard — full meal row with items collapsed ───────────────────

function DashedDivider() {
  return (
    <Svg width="100%" height={1}>
      <Line x1="0" y1="0.5" x2="1000" y2="0.5" stroke={pal.hairline} strokeWidth={1} strokeDasharray="4 4" />
    </Svg>
  );
}

function MealCard({ meal }: { meal: LoggedMeal }) {
  const [open, setOpen] = useState(false);
  const totals = meal.totals as MealTotals;
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 16,
        borderWidth: 1,
        borderColor: pal.hairline,
        overflow: 'hidden',
      }}
    >
      <Press
        onPress={() => setOpen((o) => !o)}
        style={{
          flexDirection: 'row',
          gap: 12,
          alignItems: 'center',
          paddingVertical: 12,
          paddingHorizontal: 14,
        }}
      >
        <View
          style={{
            width: 36,
            height: 36,
            borderRadius: 10,
            backgroundColor: C.sand100,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Text style={{ fontFamily: F.displayItalic, fontSize: 18, color: C.sand700 }}>
            {(meal.mealName || 'M').charAt(0).toUpperCase()}
          </Text>
        </View>
        <View style={{ flex: 1, minWidth: 0 }}>
          <Text
            numberOfLines={1}
            style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}
          >
            {meal.mealName}
          </Text>
          <Text
            numberOfLines={1}
            style={{
              fontFamily: F.body,
              fontSize: 11.5,
              color: pal.subtle,
              marginTop: 1,
              fontVariant: ['tabular-nums'],
            }}
          >
            {meal.time} · {meal.items.length} {itemsWord(meal.items.length)} ·{' '}
            {Math.round(totals.p)} г белка
          </Text>
        </View>
        <View style={{ alignItems: 'flex-end' }}>
          <Text
            style={{
              fontFamily: F.monoMedium,
              fontSize: 16,
              color: pal.ink,
              letterSpacing: 16 * -0.02,
              fontVariant: ['tabular-nums'],
              lineHeight: 16,
            }}
          >
            {totals.kcal}
          </Text>
          <Text style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle, marginTop: 2 }}>
            ккал
          </Text>
        </View>
        <Icon name={open ? 'chevron-up' : 'chevron-down'} size={16} color={pal.subtle} />
      </Press>
      {open ? (
        <View style={{ backgroundColor: pal.bg }}>
          <DashedDivider />
          <View style={{ paddingTop: 4, paddingHorizontal: 14, paddingBottom: 14 }}>
            {meal.items.map((it, i) => (
              <View
                key={it.id}
                style={{
                  flexDirection: 'row',
                  gap: 10,
                  alignItems: 'center',
                  paddingVertical: 8,
                  borderBottomWidth: i < meal.items.length - 1 ? 1 : 0,
                  borderBottomColor: pal.hairline,
                }}
              >
                <View style={{ flex: 1, minWidth: 0 }}>
                  <Text
                    numberOfLines={1}
                    style={{ fontFamily: F.bodyMedium, fontSize: 12.5, color: pal.ink2 }}
                  >
                    {it.name}
                  </Text>
                  <Text
                    numberOfLines={1}
                    style={{
                      fontFamily: F.mono,
                      fontSize: 10,
                      color: pal.subtle,
                      marginTop: 1,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    Белок {it.p}г · Углеводы {it.c}г · Жиры {it.f}г
                  </Text>
                </View>
                <Text
                  style={{
                    fontFamily: F.mono,
                    fontSize: 11,
                    color: pal.subtle,
                    fontVariant: ['tabular-nums'],
                  }}
                >
                  {it.grams} г
                </Text>
                <Text
                  style={{
                    fontFamily: F.mono,
                    fontSize: 12,
                    color: pal.ink2,
                    fontVariant: ['tabular-nums'],
                    minWidth: 36,
                    textAlign: 'right',
                  }}
                >
                  {it.kcal}
                </Text>
              </View>
            ))}
          </View>
        </View>
      ) : null}
    </View>
  );
}

// ── WeekBars — 7 bars, target line ──────────────────────────────────

function WeekBars({ values, target }: { values: number[]; target: number }) {
  const labels = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Сег'];
  const max = Math.max(...values, target) * 1.05;
  const W = 320;
  const H = 90;
  const padL = 8;
  const padR = 8;
  const padT = 4;
  const padB = 18;
  const bw = (W - padL - padR) / values.length;
  const yFor = (v: number) => padT + ((max - v) / max) * (H - padT - padB);

  return (
    <View style={{ width: '100%', aspectRatio: W / H }}>
      <Svg width="100%" height="100%" viewBox={`0 0 ${W} ${H}`}>
        {/* target line */}
        <Line
          x1={padL}
          y1={yFor(target)}
          x2={W - padR}
          y2={yFor(target)}
          stroke={C.sand700}
          strokeOpacity={0.4}
          strokeWidth={1}
          strokeDasharray="2 3"
        />
        <SvgText
          x={W - padR}
          y={yFor(target) - 4}
          textAnchor="end"
          fontFamily={F.mono}
          fontSize={9}
          fill={C.sand700}
          opacity={0.75}
        >
          {`цель ${target.toLocaleString()}`}
        </SvgText>
        {/* bars */}
        {values.map((v, i) => {
          const isToday = i === values.length - 1;
          const y = yFor(v);
          const h = H - padB - y;
          const x = padL + i * bw + 3;
          const w = bw - 6;
          return (
            <G key={i}>
              <Rect
                x={x}
                y={y}
                width={w}
                height={Math.max(0, h)}
                rx={3}
                fill={isToday ? C.forest700 : C.sand500}
                opacity={isToday ? 1 : 0.7}
              />
              <SvgText
                x={x + w / 2}
                y={H - 4}
                textAnchor="middle"
                fontFamily={isToday ? F.bodyMedium : F.body}
                fontSize={9}
                fill={isToday ? pal.ink : pal.subtle}
              >
                {labels[i]}
              </SvgText>
            </G>
          );
        })}
      </Svg>
    </View>
  );
}

// ── NutritionScreen ─────────────────────────────────────────────────

export function NutritionScreen({
  onBack,
  onLogMeal,
  onOpenRecipes,
  onChangeTab,
}: {
  onBack: () => void;
  onLogMeal: () => void;
  onOpenRecipes: () => void;
  onChangeTab: (id: TabId) => void;
}) {
  const insets = useSafeAreaInsets();
  const { meals, mealTotals, mealTargets } = useAppState();
  const totals = mealTotals as MealTotals;
  const t = mealTargets;

  // 7-day kcal history (cosmetic; mocked, with today computed from real totals).
  const weekHistory = [1685, 1742, 1610, 1820, 1455, 1690, totals.kcal];
  const weekProteinHistory = [128, 142, 118, 145, 102, 138, Math.round(totals.p)];
  const proteinAvg = Math.round(
    weekProteinHistory.reduce((a, b) => a + b) / weekProteinHistory.length,
  );

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <ScrollView
        contentContainerStyle={{ paddingTop: insets.top + 6, paddingBottom: 130 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Top bar */}
        <View
          style={{
            paddingTop: 8,
            paddingHorizontal: 16,
            paddingBottom: 10,
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <IconBtn name="chevron-left" size={20} onPress={onBack} bg={pal.sunk} color={pal.ink2} />
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.muted }}>
            Питание
          </Text>
          <IconBtn name="ellipsis-horizontal" size={20} bg={pal.sunk} color={pal.ink2} />
        </View>

        {/* Hero */}
        <View style={{ paddingTop: 6, paddingHorizontal: 24, paddingBottom: 18 }}>
          <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
            Тарелка сегодня
          </Eyebrow>
          <Text
            style={{
              fontFamily: F.display,
              fontSize: 32,
              color: pal.ink,
              lineHeight: 32 * 1.05,
              letterSpacing: 32 * -0.018,
            }}
          >
            {meals.length === 0 ? (
              <>
                Пока ничего — начнём, когда{' '}
                <Text style={{ fontFamily: F.displayItalic, color: C.sand700 }}>
                  будете готовы
                </Text>
                .
              </>
            ) : (
              <>
                {meals.length} {mealsWord(meals.length)}.
              </>
            )}
          </Text>
        </View>

        {/* Ring + macros block */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 20,
              padding: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
              shadowColor: '#2e2618',
              shadowOpacity: 0.05,
              shadowRadius: 8,
              shadowOffset: { width: 0, height: 2 },
              elevation: 2,
              flexDirection: 'row',
              gap: 18,
              alignItems: 'center',
            }}
          >
            <NutritionRing totals={totals} targets={t} />
            <View style={{ flex: 1, minWidth: 0 }}>
              <Eyebrow color={pal.subtle} style={{ marginBottom: 4, fontSize: 10 }}>
                ккал сегодня
              </Eyebrow>
              <View
                style={{
                  flexDirection: 'row',
                  alignItems: 'baseline',
                  gap: 4,
                  marginBottom: 14,
                }}
              >
                <Text
                  style={{
                    fontFamily: F.monoMedium,
                    fontSize: 32,
                    fontVariant: ['tabular-nums'],
                    color: pal.ink,
                    letterSpacing: 32 * -0.03,
                    lineHeight: 32,
                  }}
                >
                  {totals.kcal.toLocaleString()}
                </Text>
                <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle }}>
                  / {t.kcal.toLocaleString()}
                </Text>
              </View>

              <MacroBar label="белок" v={Math.round(totals.p)} goal={t.protein} color={C.forest700} />
              <MacroBar label="углеводы" v={Math.round(totals.c)} goal={t.carbs} color="#a5773d" />
              <MacroBar label="жиры" v={Math.round(totals.f)} goal={t.fat} color={C.sand700} last />
            </View>
          </View>
        </View>

        {/* Meals timeline */}
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
            <Eyebrow color={pal.subtle}>Приёмы сегодня</Eyebrow>
            <Text
              style={{
                fontFamily: F.mono,
                fontSize: 10.5,
                color: pal.subtle,
                fontVariant: ['tabular-nums'],
              }}
            >
              {meals.length} из 4
            </Text>
          </View>

          {meals.length === 0 ? (
            <View
              style={{
                backgroundColor: pal.paper,
                borderRadius: 18,
                padding: 18,
                borderWidth: 1,
                borderColor: pal.hairline,
              }}
            >
              <Text
                style={{
                  fontFamily: F.body,
                  fontSize: 13,
                  color: pal.muted,
                  lineHeight: 13 * 1.5,
                }}
              >
                Сегодня пока ничего.{' '}
                <Text
                  onPress={onLogMeal}
                  style={{ fontFamily: F.bodyMedium, color: C.forest700 }}
                >
                  Запишите первый приём
                </Text>
                , чтобы начать ритм.
              </Text>
            </View>
          ) : (
            <View style={{ gap: 10 }}>
              {meals.map((m: LoggedMeal) => (
                <MealCard key={m.id} meal={m} />
              ))}
            </View>
          )}
        </View>

        {/* Week trend */}
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
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'baseline',
                marginBottom: 12,
              }}
            >
              <Eyebrow color={pal.subtle}>Прошлая неделя</Eyebrow>
              <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>
                ккал · по дням
              </Text>
            </View>
            <WeekBars values={weekHistory} target={t.kcal} />
            <View
              style={{
                marginTop: 14,
                paddingTop: 12,
                borderTopWidth: 1,
                borderTopColor: pal.hairline,
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}
            >
              <View>
                <Eyebrow color={pal.subtle} style={{ marginBottom: 2, fontSize: 10 }}>
                  Белок · средн.
                </Eyebrow>
                <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 3 }}>
                  <Text
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 18,
                      color: pal.ink,
                      letterSpacing: 18 * -0.02,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {proteinAvg}
                  </Text>
                  <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>
                    г / день
                  </Text>
                </View>
              </View>
              <Pill tone="forest">↑ 6 г к прошлой</Pill>
            </View>
          </View>
        </View>

        {/* Рецепты — opens the recipe library/builder */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View style={{ paddingHorizontal: 4, paddingBottom: 10 }}>
            <Eyebrow color={pal.subtle}>Рецепты</Eyebrow>
          </View>
          <Press
            onPress={onOpenRecipes}
            style={{
              flexDirection: 'row',
              gap: 14,
              alignItems: 'center',
              backgroundColor: pal.paper,
              borderWidth: 1,
              borderColor: pal.hairline,
              borderRadius: 18,
              padding: 14,
              shadowColor: '#2e2618',
              shadowOpacity: 0.05,
              shadowRadius: 6,
              shadowOffset: { width: 0, height: 2 },
              elevation: 2,
            }}
          >
            <View
              style={{
                width: 44,
                height: 44,
                borderRadius: 12,
                backgroundColor: C.forest50,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name="book-open" size={22} color={C.forest700} />
            </View>
            <View style={{ flex: 1, minWidth: 0 }}>
              <Text
                numberOfLines={1}
                style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}
              >
                Рецепты и конструктор
              </Text>
              <Text
                numberOfLines={1}
                style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}
              >
                Белковые блюда · соберите своё
              </Text>
            </View>
            <Icon name="chevron-right" size={18} color={pal.subtle} />
          </Press>
        </View>
      </ScrollView>

      {/* Tab bar */}
      <CadenceTabBar active="nutrition" onChange={onChangeTab} />
    </View>
  );
}
