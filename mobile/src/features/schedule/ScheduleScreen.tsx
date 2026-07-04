// Cadence · График (Injection schedule) — calendar screen.
// Vertically-scrolling month grids across the 12-week cycle, dose dots per day,
// a titration callout, and a tappable day-detail sheet that hands off to Log dose.
import React, { useMemo, useRef, useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Icon } from '../../components/Icon';
import { Eyebrow, Pill, PillTone, Press } from '../../components/primitives';
import { Sheet } from '../../components/shared';
import { useAppState } from '../../state/AppState';
import * as S from './data';
import type { EventCtx } from './data';

// ── Status pill copy + tone ───────────────────────────────────
function statusMeta(status: string): { label: string; tone: PillTone } {
  return (
    (
      {
        done: { label: 'Записано', tone: 'forest' },
        pending: { label: 'Ждёт', tone: 'sand' },
        scheduled: { label: 'Запланировано', tone: 'neutral' },
        skipped: { label: 'Пропущено', tone: 'danger' },
        logged: { label: 'Записано', tone: 'forest' },
        open: { label: 'Открыто', tone: 'sand' },
      } as Record<string, { label: string; tone: PillTone }>
    )[status] || { label: '', tone: 'neutral' }
  );
}

const CELL_W = `${100 / 7}%` as const;

// ── One day cell in the month grid ────────────────────────────
function DayCell({
  date,
  ctx,
  onPick,
}: {
  date: Date | null;
  ctx: EventCtx;
  onPick: (d: Date) => void;
}) {
  if (!date) return <View style={{ width: CELL_W, aspectRatio: 1 / 1.05 }} />;
  const inCyc = S.inCycle(date);
  const isToday = S.sameDay(date, S.TODAY);
  const info = inCyc ? S.dotsForDate(date, ctx) : null;
  const r = S.rel(date);
  const dim = !inCyc;

  const ringColor = isToday ? C.forest700 : info && info.step ? C.sand500 : 'transparent';
  const anchorBg =
    info && info.injection && !isToday
      ? info.step
        ? 'rgba(212,165,116,.16)'
        : 'rgba(45,95,63,.07)'
      : 'transparent';

  return (
    <Press
      disabled={!inCyc}
      onPress={() => inCyc && onPick(date)}
      style={{
        width: CELL_W,
        aspectRatio: 1 / 1.05,
        alignItems: 'center',
        gap: 4,
        opacity: dim ? 0.32 : 1,
      }}
    >
      <View
        style={{
          width: 34,
          height: 34,
          borderRadius: 999,
          marginTop: 3,
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: isToday ? C.forest700 : anchorBg,
          borderWidth: 1.5,
          borderColor: !isToday && ringColor !== 'transparent' ? ringColor : 'transparent',
          ...(isToday
            ? {
                shadowColor: C.forest700,
                shadowOpacity: 0.35,
                shadowRadius: 8,
                shadowOffset: { width: 0, height: 2 },
                elevation: 4,
              }
            : null),
        }}
      >
        <Text
          style={{
            fontFamily: info && info.injection ? F.monoMedium : F.mono,
            fontSize: 14,
            fontVariant: ['tabular-nums'],
            color: isToday ? C.cream : info && info.injection ? pal.ink : pal.muted,
            letterSpacing: 14 * -0.02,
          }}
        >
          {date.getDate()}
        </Text>
      </View>
      {/* dots */}
      <View style={{ flexDirection: 'row', gap: 3, height: 6, alignItems: 'center' }}>
        {info &&
          info.cats.slice(0, 4).map((c) => (
            <View
              key={c}
              style={{
                width: 5,
                height: 5,
                borderRadius: 999,
                backgroundColor: S.CATS[c].dot,
                opacity: r === 'past' && c !== 'meal' ? 0.5 : 1,
              }}
            />
          ))}
      </View>
    </Press>
  );
}

// ── A single month block ──────────────────────────────────────
function MonthBlock({
  m,
  ctx,
  onPick,
  onTodayLayout,
}: {
  m: S.CycleMonth;
  ctx: EventCtx;
  onPick: (d: Date) => void;
  onTodayLayout?: (y: number) => void;
}) {
  return (
    <View
      style={{ paddingHorizontal: 16, paddingBottom: 18 }}
      onLayout={
        m.isTodayMonth && onTodayLayout
          ? (e) => onTodayLayout(e.nativeEvent.layout.y)
          : undefined
      }
    >
      <View
        style={{
          flexDirection: 'row',
          alignItems: 'baseline',
          gap: 8,
          paddingHorizontal: 4,
          paddingTop: 4,
          paddingBottom: 8,
        }}
      >
        <Text
          style={{
            fontFamily: F.display,
            fontSize: 24,
            color: pal.ink,
            letterSpacing: 24 * -0.018,
          }}
        >
          {m.label}
        </Text>
        <Text style={{ fontFamily: F.mono, fontSize: 12, color: pal.placeholder }}>{m.year}</Text>
      </View>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', rowGap: 2 }}>
        {m.cells.map((d, i) => (
          <DayCell key={i} date={d} ctx={ctx} onPick={onPick} />
        ))}
      </View>
    </View>
  );
}

// ── Day-detail bottom sheet ───────────────────────────────────
function DaySheet({
  date,
  ctx,
  onClose,
  onLogDose,
}: {
  date: Date | null;
  ctx: EventCtx;
  onClose: () => void;
  onLogDose: () => void;
}) {
  if (!date) return null;
  const evs = S.eventsForDate(date, ctx);
  const week = S.weekOfCycle(date);
  const step = S.isTitrationDay(date);
  const r = S.rel(date);
  const wd = S.WD_FULL[date.getDay()];
  const pendingSema = evs.find((e) => e.id === 'sema' && e.status === 'pending');

  return (
    <Sheet open onClose={onClose} contentStyle={{ paddingHorizontal: 0, maxHeight: '82%' }}>
      {/* Header */}
      <View
        style={{
          paddingTop: 6,
          paddingHorizontal: 20,
          paddingBottom: 14,
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          gap: 12,
        }}
      >
        <View style={{ minWidth: 0, flex: 1 }}>
          <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
            {r === 'today' ? 'Сегодня' : r === 'past' ? 'Прошедший день' : 'Впереди'}
          </Eyebrow>
          <Text
            style={{
              fontFamily: F.display,
              fontSize: 27,
              color: pal.ink,
              lineHeight: 27 * 1.04,
              letterSpacing: 27 * -0.018,
            }}
          >
            {wd},{' '}
            <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>
              {S.longDate(date)}
            </Text>
          </Text>
          <View style={{ marginTop: 8 }}>
            <Pill tone="neutral">{week}-я неделя курса</Pill>
          </View>
        </View>
        <Press
          onPress={onClose}
          style={{
            width: 36,
            height: 36,
            borderRadius: 999,
            backgroundColor: pal.sunk,
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          <Icon name="x-mark" size={18} color={pal.ink2} />
        </Press>
      </View>

      {/* Titration callout */}
      {step ? (
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <View
            style={{
              flexDirection: 'row',
              gap: 12,
              alignItems: 'center',
              backgroundColor: C.sand100,
              borderWidth: 1,
              borderColor: 'rgba(184,137,90,.28)',
              borderRadius: 16,
              padding: 14,
            }}
          >
            <View
              style={{
                width: 40,
                height: 40,
                borderRadius: 12,
                backgroundColor: C.sand500,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name="arrow-trending-up" size={20} color={C.ink900} />
            </View>
            <View style={{ minWidth: 0, flex: 1 }}>
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: C.sand900 }}>
                Шаг титрации
              </Text>
              <Text
                style={{
                  fontFamily: F.body,
                  fontSize: 12,
                  color: C.sand900,
                  opacity: 0.78,
                  marginTop: 1,
                }}
              >
                Доза растёт: {step.from} мг →{' '}
                <Text style={{ fontFamily: F.bodySemibold }}>{step.to} мг</Text>
              </Text>
            </View>
          </View>
        </View>
      ) : null}

      {/* Events */}
      <ScrollView
        style={{ flexGrow: 0, flexShrink: 1 }}
        contentContainerStyle={{ paddingHorizontal: 16, paddingBottom: 8 }}
      >
        <View
          style={{
            backgroundColor: pal.paper,
            borderRadius: 18,
            borderWidth: 1,
            borderColor: pal.hairline,
            overflow: 'hidden',
          }}
        >
          {evs.map((e, i) => {
            const cat = S.CATS[e.cat];
            const sm = statusMeta(e.status);
            const tappable =
              e.loggable && (e.status === 'pending' || e.status === 'scheduled') && r !== 'past';
            return (
              <Press
                key={e.id}
                disabled={!tappable}
                onPress={tappable ? onLogDose : undefined}
                style={{
                  flexDirection: 'row',
                  gap: 13,
                  alignItems: 'center',
                  paddingVertical: 13,
                  paddingHorizontal: 14,
                  borderBottomWidth: i < evs.length - 1 ? 1 : 0,
                  borderBottomColor: pal.hairline,
                }}
              >
                <View
                  style={{
                    width: 44,
                    height: 44,
                    borderRadius: 12,
                    backgroundColor: cat.soft,
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <Icon name={cat.icon as any} size={21} color={cat.fg} />
                </View>
                <View style={{ minWidth: 0, flex: 1 }}>
                  <View
                    style={{
                      flexDirection: 'row',
                      alignItems: 'baseline',
                      gap: 7,
                      flexWrap: 'wrap',
                    }}
                  >
                    <Text style={{ fontFamily: F.bodyMedium, fontSize: 14.5, color: pal.ink }}>
                      {e.title}
                    </Text>
                    {e.dose ? (
                      <Text
                        style={{
                          fontFamily: F.mono,
                          fontSize: 12.5,
                          color: pal.ink2,
                          fontVariant: ['tabular-nums'],
                        }}
                      >
                        {e.dose}
                      </Text>
                    ) : null}
                  </View>
                  <Text
                    style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}
                  >
                    {e.time ? (
                      <Text style={{ fontFamily: F.mono, color: pal.muted }}>{e.time}</Text>
                    ) : null}
                    {e.time && e.sub ? ' · ' : ''}
                    {e.sub}
                  </Text>
                </View>
                <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
                  {sm.label ? <Pill tone={sm.tone}>{sm.label}</Pill> : null}
                  {tappable ? (
                    <Icon name="chevron-right" size={15} color={pal.placeholder} />
                  ) : null}
                </View>
              </Press>
            );
          })}
        </View>
      </ScrollView>

      {/* Primary CTA — today with a queued dose */}
      <View style={{ paddingTop: 12, paddingHorizontal: 16 }}>
        {pendingSema ? (
          <Press
            onPress={onLogDose}
            style={{
              paddingVertical: 15,
              paddingHorizontal: 20,
              borderRadius: 999,
              backgroundColor: C.forest700,
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 8,
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 15, color: C.cream }}>
              Записать дозу
            </Text>
            <Icon name="arrow-right" size={16} color={C.cream} />
          </Press>
        ) : (
          <Press
            onPress={onClose}
            style={{
              paddingVertical: 14,
              paddingHorizontal: 20,
              borderRadius: 999,
              borderWidth: 1,
              borderColor: pal.border,
              alignItems: 'center',
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.muted }}>
              Закрыть
            </Text>
          </Press>
        )}
      </View>
    </Sheet>
  );
}

// ════════════════════════════════════════════════════════════════
// Main schedule screen
// ════════════════════════════════════════════════════════════════
export function ScheduleScreen({
  onBack,
  onLogDose,
}: {
  onBack: () => void;
  onLogDose: () => void;
}) {
  const insets = useSafeAreaInsets();
  const { doseLogged, meals, mealTotals } = useAppState();
  const ctx: EventCtx = {
    doseLogged,
    todayMeals: meals.length,
    todayKcal: mealTotals?.kcal ?? 0,
  };
  const [pick, setPick] = useState<Date | null>(null);
  const scrollRef = useRef<ScrollView>(null);
  const todayY = useRef(0);
  const didInitScroll = useRef(false);

  const months = useMemo(() => S.monthsInCycle(), []);
  const week = S.weekOfCycle(S.TODAY);
  const pct = Math.max(0, Math.min(1, (week - 1 + 0.5) / S.CYCLE_WEEKS));
  const titr = S.nextTitration();
  const dose = S.semaDose(week);

  const jumpToToday = () => {
    scrollRef.current?.scrollTo({ y: Math.max(0, todayY.current - 12), animated: true });
  };

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Top bar */}
      <View
        style={{
          paddingTop: insets.top + 6,
          backgroundColor: pal.bg,
          borderBottomWidth: 1,
          borderBottomColor: pal.hairline,
          zIndex: 20,
        }}
      >
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
          <Press
            onPress={onBack}
            style={{
              width: 40,
              height: 40,
              borderRadius: 999,
              backgroundColor: pal.sunk,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Icon name="chevron-left" size={20} color={pal.ink2} />
          </Press>
          <Text style={{ fontFamily: F.bodySemibold, fontSize: 15, color: pal.ink }}>График</Text>
          <Press
            onPress={jumpToToday}
            style={{
              height: 40,
              paddingHorizontal: 14,
              borderRadius: 999,
              borderWidth: 1,
              borderColor: pal.border,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: C.forest700 }}>
              Сегодня
            </Text>
          </Press>
        </View>

        {/* Weekday header */}
        <View
          style={{
            flexDirection: 'row',
            paddingTop: 2,
            paddingHorizontal: 16,
            paddingBottom: 8,
          }}
        >
          {S.WD_SHORT.map((w, i) => (
            <Text
              key={w}
              style={{
                width: CELL_W,
                textAlign: 'center',
                fontFamily: F.bodyMedium,
                fontSize: 11,
                letterSpacing: 11 * 0.06,
                color: i === 6 ? C.forest700 : pal.subtle,
                textTransform: 'uppercase',
              }}
            >
              {w}
            </Text>
          ))}
        </View>
      </View>

      {/* Scroll body */}
      <ScrollView
        ref={scrollRef}
        style={{ flex: 1 }}
        contentContainerStyle={{ paddingBottom: Math.max(insets.bottom, 16) + 24 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Cycle band + titration callout */}
        <View style={{ paddingTop: 6, paddingHorizontal: 16, paddingBottom: 16 }}>
          <View
            style={{
              backgroundColor: C.forest800,
              borderRadius: 22,
              padding: 18,
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
                alignItems: 'baseline',
                marginBottom: 12,
              }}
            >
              <Text
                style={{
                  fontFamily: F.bodyMedium,
                  fontSize: 11,
                  letterSpacing: 11 * 0.14,
                  textTransform: 'uppercase',
                  color: C.sand300,
                }}
              >
                Текущий курс
              </Text>
              <Text
                style={{
                  fontFamily: F.mono,
                  fontSize: 11,
                  color: 'rgba(246,241,234,.6)',
                  fontVariant: ['tabular-nums'],
                }}
              >
                неделя {week} из {S.CYCLE_WEEKS}
              </Text>
            </View>
            <Text
              style={{
                fontFamily: F.display,
                fontSize: 26,
                lineHeight: 26 * 1.06,
                letterSpacing: 26 * -0.018,
                color: C.cream,
                marginBottom: 14,
              }}
            >
              Семаглутид ·{' '}
              <Text style={{ fontFamily: F.displayItalic, color: C.sand300 }}>
                {dose.value} мг
              </Text>{' '}
              еженедельно
            </Text>
            {/* progress track */}
            <View
              style={{
                height: 6,
                borderRadius: 999,
                backgroundColor: 'rgba(246,241,234,.16)',
                marginBottom: 8,
              }}
            >
              <View
                style={{
                  position: 'absolute',
                  left: 0,
                  top: 0,
                  bottom: 0,
                  width: `${pct * 100}%`,
                  borderRadius: 999,
                  backgroundColor: C.sand500,
                }}
              />
              <View
                style={{
                  position: 'absolute',
                  top: -3,
                  left: `${pct * 100}%`,
                  marginLeft: -6,
                  width: 12,
                  height: 12,
                  borderRadius: 999,
                  backgroundColor: C.cream,
                  borderWidth: 2,
                  borderColor: C.forest800,
                }}
              />
            </View>
            <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
              <Text style={{ fontFamily: F.body, fontSize: 10.5, color: 'rgba(246,241,234,.55)' }}>
                10 мая · старт
              </Text>
              <Text style={{ fontFamily: F.body, fontSize: 10.5, color: 'rgba(246,241,234,.55)' }}>
                26 июля · финиш
              </Text>
            </View>

            {titr ? (
              <View
                style={{
                  marginTop: 16,
                  paddingTop: 14,
                  borderTopWidth: 1,
                  borderTopColor: 'rgba(246,241,234,.14)',
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: 12,
                }}
              >
                <View
                  style={{
                    width: 38,
                    height: 38,
                    borderRadius: 11,
                    backgroundColor: 'rgba(212,165,116,.2)',
                    alignItems: 'center',
                    justifyContent: 'center',
                    flexShrink: 0,
                  }}
                >
                  <Icon name="arrow-trending-up" size={19} color={C.sand300} />
                </View>
                <View style={{ minWidth: 0, flex: 1 }}>
                  <Text style={{ fontFamily: F.body, fontSize: 13, color: C.cream }}>
                    Следующий шаг —{' '}
                    <Text style={{ fontFamily: F.bodySemibold, color: C.sand300 }}>
                      {titr.to} мг
                    </Text>{' '}
                    с {titr.week}-й недели
                  </Text>
                  <Text
                    style={{
                      fontFamily: F.body,
                      fontSize: 11.5,
                      color: 'rgba(246,241,234,.6)',
                      marginTop: 1,
                    }}
                  >
                    Через {S.daysBetween(S.TODAY, titr.date)} дн. · {S.longDate(titr.date)}
                  </Text>
                </View>
              </View>
            ) : null}
          </View>
        </View>

        {/* Legend */}
        <View
          style={{
            flexDirection: 'row',
            flexWrap: 'wrap',
            rowGap: 8,
            columnGap: 16,
            paddingHorizontal: 20,
            paddingBottom: 16,
          }}
        >
          {(['inj', 'supp', 'weigh', 'meal'] as const).map((c) => (
            <View key={c} style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
              <View
                style={{ width: 7, height: 7, borderRadius: 999, backgroundColor: S.CATS[c].dot }}
              />
              <Text style={{ fontFamily: F.body, fontSize: 11.5, color: pal.muted }}>
                {S.CATS[c].label}
              </Text>
            </View>
          ))}
        </View>

        {/* Months */}
        {months.map((m) => (
          <MonthBlock
            key={m.key}
            m={m}
            ctx={ctx}
            onPick={setPick}
            onTodayLayout={
              m.isTodayMonth
                ? (y) => {
                    todayY.current = y;
                    if (!didInitScroll.current) {
                      didInitScroll.current = true;
                      scrollRef.current?.scrollTo({ y: Math.max(0, y - 12), animated: false });
                    }
                  }
                : undefined
            }
          />
        ))}

        <Text
          style={{
            textAlign: 'center',
            fontFamily: F.mono,
            fontSize: 11,
            color: pal.placeholder,
            paddingTop: 6,
            paddingBottom: 8,
            letterSpacing: 11 * 0.04,
          }}
        >
          Курс · 12 недель
        </Text>
      </ScrollView>

      <DaySheet
        date={pick}
        ctx={ctx}
        onClose={() => setPick(null)}
        onLogDose={() => {
          setPick(null);
          onLogDose();
        }}
      />
    </View>
  );
}
