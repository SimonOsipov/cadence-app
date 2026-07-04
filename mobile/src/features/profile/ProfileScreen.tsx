// Cadence — Профиль (Profile) screen + navigable Settings sub-screen.
// Ported from design_handoff_cadence_app/profile/profile.jsx (ProfileScreen).
// Centerpiece is the week 4-of-12 cycle ring on a forest hero card.
import React, { useEffect, useRef, useState } from 'react';
import {
  Animated,
  Easing,
  ScrollView,
  Text,
  useWindowDimensions,
  View,
} from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Circle, Line } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { IconBtn, Pill, Press } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { CARE_TEAM } from '../chat/data';
import { GroupCard, NavRow, SectionHead, SettingsScreen } from './SettingsScreen';

// ── Persona facts (single source so copy stays consistent) ──────────
const PROFILE = {
  name: 'Марина',
  full: 'Марина Волкова',
  email: 'marina.volkova@gmail.com',
  since: 'В Cadence с марта 2026',
  goalPill: 'Лёгкость к лету',
  cycle: { week: 4, total: 12, compound: 'Семаглутид', dose: '0,25 мг', endLabel: 'до 20 июля' },
  journey: { start: 118.0, now: 110.0, target: 102.0, unit: 'кг' },
  stats: [
    { id: 'streak', value: '8', unit: 'нед', label: 'Серия доз' },
    { id: 'doses', value: '11', unit: '', label: 'Доз записано' },
    { id: 'adhere', value: '96', unit: '%', label: 'Дисциплина' },
  ],
  photos: [
    { week: 'Нед 1', w: '118,0' },
    { week: 'Нед 2', w: '115,4' },
    { week: 'Нед 4', w: '110,0' },
  ],
  measures: [
    { id: 'waist', label: 'Талия', value: '92', unit: 'см', delta: '↓ 4' },
    { id: 'hip', label: 'Бёдра', value: '108', unit: 'см', delta: '↓ 3' },
    { id: 'bmi', label: 'ИМТ', value: '31,2', unit: '', delta: '↓ 2,1' },
  ],
};

// ── Cycle ring recolored for the forest hero card ────────────────────
// The shared CycleRing draws its track/ticks in light-surface tones; the
// prototype overrides them to faint cream on forest-800, so we draw the
// same geometry locally with the hero palette (rgba of C.cream / C.sand500).
function HeroRing({
  week,
  total,
  size = 132,
  stroke = 9,
}: {
  week: number;
  total: number;
  size?: number;
  stroke?: number;
}) {
  const r = (size - stroke) / 2;
  const cx = size / 2;
  const cy = size / 2;
  const circ = 2 * Math.PI * r;
  const dash = circ * (week / total);
  return (
    <Svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      <Circle cx={cx} cy={cy} r={r} stroke="rgba(246,241,234,.14)" strokeWidth={stroke} fill="none" />
      <Circle
        cx={cx}
        cy={cy}
        r={r}
        stroke={C.sand500}
        strokeWidth={stroke}
        fill="none"
        strokeDasharray={`${dash} ${circ - dash}`}
        strokeDashoffset={circ / 4}
        strokeLinecap="round"
        transform={`rotate(-90 ${cx} ${cy})`}
      />
      {Array.from({ length: total }).map((_, i) => {
        const a = (i / total) * Math.PI * 2 - Math.PI / 2;
        const inner = r - stroke / 2 - 4;
        const outer = r - stroke / 2 - 1;
        return (
          <Line
            key={i}
            x1={cx + Math.cos(a) * inner}
            y1={cy + Math.sin(a) * inner}
            x2={cx + Math.cos(a) * outer}
            y2={cy + Math.sin(a) * outer}
            stroke="rgba(246,241,234,.4)"
            strokeWidth={1}
            opacity={0.5}
          />
        );
      })}
    </Svg>
  );
}

// ── Cycle ring hero — the emotional centerpiece ─────────────────────
function CycleHero({ onOpenSchedule }: { onOpenSchedule: () => void }) {
  const { week, total, compound, dose, endLabel } = PROFILE.cycle;
  return (
    <View
      style={{
        backgroundColor: C.forest800,
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
          alignItems: 'center',
          marginBottom: 16,
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
          {endLabel}
        </Text>
      </View>

      <View style={{ flexDirection: 'row', gap: 18, alignItems: 'center' }}>
        {/* Ring with center week label */}
        <View style={{ width: 132, height: 132, flexShrink: 0 }}>
          <HeroRing week={week} total={total} size={132} stroke={9} />
          <View
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 2 }}>
              <Text
                style={{
                  fontFamily: F.monoMedium,
                  fontSize: 36,
                  color: C.cream,
                  lineHeight: 38,
                  letterSpacing: 36 * -0.03,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {week}
              </Text>
              <Text style={{ fontFamily: F.displayItalic, fontSize: 18, color: C.sand300 }}>
                /{total}
              </Text>
            </View>
            <Text
              style={{
                fontFamily: F.bodyMedium,
                fontSize: 10,
                letterSpacing: 10 * 0.08,
                textTransform: 'uppercase',
                color: 'rgba(246,241,234,.55)',
                marginTop: 3,
              }}
            >
              неделя
            </Text>
          </View>
        </View>

        {/* Copy */}
        <View style={{ flex: 1, minWidth: 0 }}>
          <Text
            style={{
              fontFamily: F.display,
              fontSize: 28,
              color: C.cream,
              lineHeight: 28 * 1.04,
              letterSpacing: 28 * -0.018,
            }}
          >
            Четвёртая неделя — вы{' '}
            <Text style={{ fontFamily: F.displayItalic, color: C.sand300 }}>в ритме</Text>.
          </Text>
          <Text
            style={{
              fontFamily: F.body,
              fontSize: 13,
              color: 'rgba(246,241,234,.72)',
              lineHeight: 13 * 1.5,
              marginTop: 8,
            }}
          >
            {compound} · {dose} · еженедельно
          </Text>
        </View>
      </View>

      <Press
        onPress={onOpenSchedule}
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 8,
          marginTop: 18,
          paddingVertical: 12,
          paddingHorizontal: 18,
          borderRadius: 999,
          backgroundColor: 'rgba(246,241,234,.12)',
        }}
      >
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: C.cream }}>
          Открыть курс целиком
        </Text>
        <Icon name="arrow-right" size={15} color={C.cream} />
      </Press>
    </View>
  );
}

// ── Weight journey: start → now → target with progress track ────────
function JourneyCard({ onOpenTrend }: { onOpenTrend: (id: string) => void }) {
  const { start, now, target, unit } = PROFILE.journey;
  const pct = Math.max(0, Math.min(1, (start - now) / (start - target)));
  const fmt = (n: number) => n.toFixed(1).replace('.', ',');
  const marks: { k: string; v: number; accent?: boolean }[] = [
    { k: 'Старт', v: start },
    { k: 'Сейчас', v: now, accent: true },
    { k: 'Цель', v: target },
  ];
  return (
    <Press
      onPress={() => onOpenTrend('weight')}
      style={{
        backgroundColor: pal.paper,
        borderRadius: 18,
        padding: 16,
        borderWidth: 1,
        borderColor: pal.hairline,
        shadowColor: '#2e2618',
        shadowOpacity: 0.05,
        shadowRadius: 6,
        shadowOffset: { width: 0, height: 2 },
        elevation: 2,
      }}
    >
      <SectionHead action="Тренд →" onAction={() => onOpenTrend('weight')}>
        Путь к цели
      </SectionHead>

      <Text
        style={{
          fontFamily: F.display,
          fontSize: 22,
          color: pal.ink,
          lineHeight: 22 * 1.1,
          letterSpacing: 22 * -0.012,
          paddingTop: 2,
          paddingHorizontal: 4,
          paddingBottom: 16,
        }}
      >
        Восемь килограммов позади —{' '}
        <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>половина пути</Text>.
      </Text>

      {/* Track */}
      <View style={{ paddingHorizontal: 4 }}>
        <View
          style={{ height: 8, borderRadius: 999, backgroundColor: pal.sunk, marginBottom: 12 }}
        >
          <View
            style={{
              position: 'absolute',
              left: 0,
              top: 0,
              bottom: 0,
              width: `${pct * 100}%`,
              borderRadius: 999,
              backgroundColor: C.forest700,
            }}
          />
          <View
            style={{
              position: 'absolute',
              top: -4,
              left: `${pct * 100}%`,
              marginLeft: -8,
              width: 16,
              height: 16,
              borderRadius: 999,
              backgroundColor: C.sand500,
              borderWidth: 3,
              borderColor: pal.paper,
              shadowColor: '#2e2618',
              shadowOpacity: 0.25,
              shadowRadius: 4,
              shadowOffset: { width: 0, height: 1 },
              elevation: 2,
            }}
          />
        </View>
        <View style={{ flexDirection: 'row', justifyContent: 'space-between' }}>
          {marks.map((s, i) => (
            <View
              key={s.k}
              style={{ alignItems: i === 0 ? 'flex-start' : i === 2 ? 'flex-end' : 'center' }}
            >
              <Text
                style={{
                  fontFamily: F.bodyMedium,
                  fontSize: 10,
                  letterSpacing: 10 * 0.1,
                  textTransform: 'uppercase',
                  color: pal.subtle,
                  marginBottom: 3,
                }}
              >
                {s.k}
              </Text>
              <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 3 }}>
                <Text
                  style={{
                    fontFamily: F.monoMedium,
                    fontSize: s.accent ? 20 : 16,
                    color: s.accent ? pal.ink : pal.muted,
                    fontVariant: ['tabular-nums'],
                    letterSpacing: (s.accent ? 20 : 16) * -0.02,
                  }}
                >
                  {fmt(s.v)}
                </Text>
                <Text style={{ fontFamily: F.displayItalic, fontSize: 11, color: pal.subtle }}>
                  {unit}
                </Text>
              </View>
            </View>
          ))}
        </View>
      </View>
    </Press>
  );
}

// ── Three adherence stat tiles ──────────────────────────────────────
function StatTiles() {
  return (
    <View style={{ flexDirection: 'row', gap: 10 }}>
      {PROFILE.stats.map((s) => (
        <View
          key={s.id}
          style={{
            flex: 1,
            backgroundColor: pal.paper,
            borderRadius: 16,
            paddingVertical: 14,
            paddingHorizontal: 12,
            borderWidth: 1,
            borderColor: pal.hairline,
            shadowColor: '#2e2618',
            shadowOpacity: 0.05,
            shadowRadius: 6,
            shadowOffset: { width: 0, height: 2 },
            elevation: 2,
            alignItems: 'center',
          }}
        >
          <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 2 }}>
            <Text
              style={{
                fontFamily: F.monoMedium,
                fontSize: 26,
                color: pal.ink,
                letterSpacing: 26 * -0.03,
                fontVariant: ['tabular-nums'],
              }}
            >
              {s.value}
            </Text>
            {s.unit ? (
              <Text style={{ fontFamily: F.displayItalic, fontSize: 13, color: pal.muted }}>
                {s.unit}
              </Text>
            ) : null}
          </View>
          <Text
            style={{
              fontFamily: F.body,
              fontSize: 11,
              color: pal.subtle,
              marginTop: 5,
              lineHeight: 11 * 1.2,
              textAlign: 'center',
            }}
          >
            {s.label}
          </Text>
        </View>
      ))}
    </View>
  );
}

// ── Body metrics & progress photos ──────────────────────────────────
function BodyCard({ onOpenBody }: { onOpenBody: () => void }) {
  return (
    <Press
      onPress={onOpenBody}
      style={{
        backgroundColor: pal.paper,
        borderRadius: 18,
        padding: 16,
        borderWidth: 1,
        borderColor: pal.hairline,
        shadowColor: '#2e2618',
        shadowOpacity: 0.05,
        shadowRadius: 6,
        shadowOffset: { width: 0, height: 2 },
        elevation: 2,
      }}
    >
      <SectionHead action="Открыть →" onAction={onOpenBody}>
        Тело · снимки
      </SectionHead>

      {/* Photo strip */}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        style={{ marginHorizontal: -4 }}
        contentContainerStyle={{ gap: 10, paddingHorizontal: 4, paddingBottom: 4 }}
      >
        {PROFILE.photos.map((p, i) => (
          <View
            key={i}
            style={{
              width: 96,
              height: 132,
              borderRadius: 14,
              overflow: 'hidden',
              borderWidth: 1,
              borderColor: pal.hairline,
            }}
          >
            <LinearGradient
              colors={[C.sand300, C.linen, C.bone]}
              locations={[0, 0.7, 1]}
              start={{ x: 0.37, y: 0 }}
              end={{ x: 0.63, y: 1 }}
              style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0 }}
            />
            <View
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name="camera" size={26} color="rgba(107,74,37,.28)" />
            </View>
            <LinearGradient
              colors={['rgba(26,26,26,0)', 'rgba(26,26,26,.42)']}
              style={{
                position: 'absolute',
                left: 0,
                right: 0,
                bottom: 0,
                paddingTop: 14,
                paddingHorizontal: 8,
                paddingBottom: 7,
              }}
            >
              <Text
                style={{
                  fontFamily: F.bodySemibold,
                  fontSize: 10,
                  letterSpacing: 10 * 0.06,
                  textTransform: 'uppercase',
                  color: C.cream,
                }}
              >
                {p.week}
              </Text>
              <Text
                style={{
                  fontFamily: F.mono,
                  fontSize: 12,
                  color: C.cream,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {p.w} кг
              </Text>
            </LinearGradient>
          </View>
        ))}
        {/* Add tile */}
        <Press
          onPress={onOpenBody}
          style={{
            width: 96,
            height: 132,
            borderRadius: 14,
            borderWidth: 1.5,
            borderStyle: 'dashed',
            borderColor: pal.border,
            alignItems: 'center',
            justifyContent: 'center',
            gap: 6,
          }}
        >
          <Icon name="plus" size={22} color={pal.muted} />
          <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.muted }}>Снимок</Text>
        </Press>
      </ScrollView>

      {/* Measurements */}
      <View style={{ flexDirection: 'row', gap: 10, marginTop: 14 }}>
        {PROFILE.measures.map((m) => (
          <View
            key={m.id}
            style={{
              flex: 1,
              backgroundColor: pal.sunk,
              borderRadius: 12,
              paddingVertical: 10,
              paddingHorizontal: 12,
            }}
          >
            <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginBottom: 4 }}>
              {m.label}
            </Text>
            <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 3 }}>
              <Text
                style={{
                  fontFamily: F.monoMedium,
                  fontSize: 17,
                  color: pal.ink,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {m.value}
              </Text>
              {m.unit ? (
                <Text style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle }}>
                  {m.unit}
                </Text>
              ) : null}
            </View>
            <Text style={{ fontFamily: F.body, fontSize: 11, color: C.forest700, marginTop: 2 }}>
              {m.delta}
            </Text>
          </View>
        ))}
      </View>
    </Press>
  );
}

// ════════════════════════════════════════════════════════════════════
// Main profile screen.
// ════════════════════════════════════════════════════════════════════
export function ProfileScreen({
  onBack,
  onOpenChat,
  onOpenTrend,
  onOpenSchedule,
  onOpenJournal,
  onOpenBody,
}: {
  onBack: () => void;
  onOpenChat: (threadId?: string) => void;
  onOpenTrend: (id: string) => void;
  onOpenSchedule: () => void;
  onOpenJournal: () => void;
  onOpenBody: () => void;
}) {
  const insets = useSafeAreaInsets();
  const { width } = useWindowDimensions();
  const [settingsOpen, setSettingsOpen] = useState(false);
  const slide = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    Animated.timing(slide, {
      toValue: settingsOpen ? 1 : 0,
      duration: 360,
      easing: Easing.bezier(0.22, 1, 0.36, 1),
      useNativeDriver: true,
    }).start();
  }, [settingsOpen, slide]);

  const cardShadow = {
    shadowColor: '#2e2618',
    shadowOpacity: 0.05,
    shadowRadius: 6,
    shadowOffset: { width: 0, height: 2 },
    elevation: 2,
  } as const;

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Top bar — fixed outside the scroll region */}
      <View style={{ paddingTop: insets.top + 6, backgroundColor: pal.bg, zIndex: 5 }}>
        <View
          style={{
            paddingTop: 8,
            paddingHorizontal: 16,
            paddingBottom: 6,
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <IconBtn name="chevron-left" bg={pal.sunk} color={pal.ink2} onPress={onBack} />
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.muted }}>Профиль</Text>
          <IconBtn name="cog" bg={pal.sunk} color={pal.ink2} onPress={() => setSettingsOpen(true)} />
        </View>
      </View>

      <ScrollView
        contentContainerStyle={{ paddingBottom: insets.bottom + 48 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Identity */}
        <View
          style={{
            paddingTop: 10,
            paddingHorizontal: 20,
            paddingBottom: 18,
            flexDirection: 'row',
            alignItems: 'center',
            gap: 16,
          }}
        >
          <View
            style={{
              width: 68,
              height: 68,
              borderRadius: 999,
              backgroundColor: C.forest700,
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
              shadowColor: C.forest700,
              shadowOpacity: 0.28,
              shadowRadius: 14,
              shadowOffset: { width: 0, height: 4 },
              elevation: 6,
            }}
          >
            <Text style={{ fontFamily: F.displayItalic, fontSize: 32, color: C.cream }}>М</Text>
          </View>
          <View style={{ flex: 1, minWidth: 0 }}>
            <Text
              numberOfLines={1}
              style={{
                fontFamily: F.display,
                fontSize: 30,
                color: pal.ink,
                lineHeight: 30 * 1.02,
                letterSpacing: 30 * -0.018,
              }}
            >
              {PROFILE.full}
            </Text>
            <Text style={{ fontFamily: F.body, fontSize: 12.5, color: pal.subtle, marginTop: 3 }}>
              {PROFILE.since}
            </Text>
            <View style={{ marginTop: 8 }}>
              <Pill tone="sand">Цель · {PROFILE.goalPill}</Pill>
            </View>
          </View>
        </View>

        {/* Cycle ring hero */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
          <CycleHero onOpenSchedule={onOpenSchedule} />
        </View>

        {/* Journey */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
          <JourneyCard onOpenTrend={onOpenTrend} />
        </View>

        {/* Stats */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
          <StatTiles />
        </View>

        {/* Wellbeing journal */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
          <Press
            onPress={onOpenJournal}
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              gap: 14,
              backgroundColor: pal.paper,
              borderWidth: 1,
              borderColor: pal.hairline,
              borderRadius: 18,
              padding: 14,
              ...cardShadow,
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
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}>
                Дневник самочувствия
              </Text>
              <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>
                Настроение и побочные по курсу
              </Text>
            </View>
            <Icon name="chevron-right" size={18} color={pal.placeholder} />
          </Press>
        </View>

        {/* Body & photos */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
          <BodyCard onOpenBody={onOpenBody} />
        </View>

        {/* Care team */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
          <SectionHead>Ваша команда</SectionHead>
          <GroupCard>
            {CARE_TEAM.map((m, i) => (
              <Press
                key={m.id}
                onPress={() => onOpenChat(m.id)}
                style={{
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: 14,
                  paddingVertical: 12,
                  paddingHorizontal: 14,
                  borderBottomWidth: i < CARE_TEAM.length - 1 ? 1 : 0,
                  borderBottomColor: pal.hairline,
                }}
              >
                <View
                  style={{
                    width: 44,
                    height: 44,
                    borderRadius: 999,
                    backgroundColor: m.avatarBg,
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <Text style={{ fontFamily: F.displayItalic, fontSize: 19, color: m.avatarFg }}>
                    {m.initial}
                  </Text>
                </View>
                <View style={{ flex: 1, minWidth: 0 }}>
                  <Text
                    numberOfLines={1}
                    style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}
                  >
                    {m.name}
                  </Text>
                  <Text
                    style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}
                  >
                    {m.role}
                  </Text>
                </View>
                <Icon name="chat-bubble" size={18} color={C.forest700} />
              </Press>
            ))}
          </GroupCard>
        </View>

        {/* Settings list */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
          <SectionHead>Настройки</SectionHead>
          <GroupCard>
            <NavRow
              iconName="bell"
              iconTone="forest"
              title="Напоминания"
              sub="Дозы · приёмы пищи"
              onPress={() => setSettingsOpen(true)}
            />
            <NavRow
              iconName="scale"
              iconTone="sand"
              title="Единицы измерения"
              trail="кг · 24 ч"
              onPress={() => setSettingsOpen(true)}
            />
            <NavRow
              iconName="chat-bubble"
              iconTone="linen"
              title="Уведомления"
              onPress={() => setSettingsOpen(true)}
            />
            <NavRow
              iconName="document-text"
              iconTone="linen"
              title="Приватность и данные"
              onPress={() => setSettingsOpen(true)}
              last
            />
          </GroupCard>
        </View>

        {/* Log out */}
        <View style={{ paddingTop: 6, paddingHorizontal: 16, paddingBottom: 4 }}>
          <Press
            onPress={() => {}}
            style={{
              paddingVertical: 14,
              borderRadius: 999,
              borderWidth: 1,
              borderColor: pal.border,
              alignItems: 'center',
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.danger }}>
              Выйти из аккаунта
            </Text>
          </Press>
        </View>

        <Text
          style={{
            textAlign: 'center',
            fontFamily: F.mono,
            fontSize: 11,
            color: pal.placeholder,
            paddingTop: 14,
            paddingBottom: 4,
            letterSpacing: 11 * 0.04,
            fontVariant: ['tabular-nums'],
          }}
        >
          Cadence 2.4.0
        </Text>
      </ScrollView>

      {/* Settings sub-screen — slides over the profile */}
      <Animated.View
        pointerEvents={settingsOpen ? 'auto' : 'none'}
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 10,
          backgroundColor: pal.bg,
          transform: [
            { translateX: slide.interpolate({ inputRange: [0, 1], outputRange: [width, 0] }) },
          ],
        }}
      >
        <SettingsScreen onBack={() => setSettingsOpen(false)} />
      </Animated.View>
    </View>
  );
}
