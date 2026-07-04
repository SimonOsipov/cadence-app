// Cadence · Log Dose — wizard atoms ported from log-dose/shared-log.jsx:
// BodyDiagram, DoseStepper, SyringeBar, VialPicker, MoodSlider, ChipsRow,
// PhotoSlot, WizardChrome.
import React, { useEffect, useRef } from 'react';
import { Animated, Easing, ScrollView, Text, View } from 'react-native';
import Svg, { Circle, Ellipse, G, Line, Path } from 'react-native-svg';
import { LinearGradient } from 'expo-linear-gradient';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Icon } from '../../components/Icon';
import { Body, Chip, Eyebrow, Press } from '../../components/primitives';
import {
  ALL_ZONES,
  LogState,
  LogVial,
  SIDE_EFFECTS,
  VIALS,
  ZONES_BACK,
  ZONES_FRONT,
  fmtDose,
  zoneLabel,
} from './data';

type Update = (patch: Partial<LogState>) => void;

// Simple fade-in keyed by step — the prototype's slide-r/fade-up transitions.
export function FadeIn({ k, children }: { k: React.Key; children: React.ReactNode }) {
  const op = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    op.setValue(0);
    Animated.timing(op, { toValue: 1, duration: 220, useNativeDriver: true }).start();
  }, [k, op]);
  return <Animated.View style={{ opacity: op }}>{children}</Animated.View>;
}

// ─────────────────────────────────────────────────────────────
// Body diagram — front/back silhouette with tappable zones
// ─────────────────────────────────────────────────────────────
export function BodyDiagram({
  state,
  update,
  size = 220,
}: {
  state: LogState;
  update: Update;
  size?: number;
}) {
  const zones = state.view === 'back' ? ZONES_BACK : ZONES_FRONT;
  const stroke = pal.border;
  const fill = pal.sunk;
  const torsoD =
    state.view === 'back'
      ? 'M55 78 Q56 70 66 70 L134 70 Q144 70 145 78 L145 180 Q145 192 134 194 L66 194 Q55 192 55 180 Z'
      : 'M55 78 Q56 70 66 70 L134 70 Q144 70 145 78 L145 175 Q145 188 134 190 L66 190 Q55 188 55 175 Z';

  return (
    <View style={{ alignItems: 'center', gap: 10 }}>
      {/* Front / back toggle */}
      <View
        style={{
          flexDirection: 'row',
          backgroundColor: pal.sunk,
          padding: 3,
          borderRadius: 999,
          gap: 2,
        }}
      >
        {(
          [
            { id: 'front', label: 'Спереди' },
            { id: 'back', label: 'Сзади' },
          ] as const
        ).map((t) => {
          const on = state.view === t.id;
          return (
            <Press
              key={t.id}
              onPress={() => update({ view: t.id })}
              style={{
                paddingVertical: 6,
                paddingHorizontal: 14,
                borderRadius: 999,
                backgroundColor: on ? pal.paper : 'transparent',
                ...(on
                  ? {
                      shadowColor: '#2e2618',
                      shadowOpacity: 0.1,
                      shadowRadius: 3,
                      shadowOffset: { width: 0, height: 1 },
                      elevation: 1,
                    }
                  : null),
              }}
            >
              <Text
                style={{
                  fontFamily: F.bodyMedium,
                  fontSize: 12,
                  color: on ? pal.ink : pal.muted,
                }}
              >
                {t.label}
              </Text>
            </Press>
          );
        })}
      </View>

      <Svg viewBox="0 0 200 340" width={size} height={size * (340 / 200)}>
        {/* Silhouette */}
        <G opacity={0.9}>
          <Ellipse cx={100} cy={38} rx={22} ry={24} fill={fill} stroke={stroke} strokeWidth={1.2} />
          <Path
            d="M92 60 L92 72 Q92 76 96 76 L104 76 Q108 76 108 72 L108 60 Z"
            fill={fill}
            stroke={stroke}
            strokeWidth={1.2}
          />
          <Path d={torsoD} fill={fill} stroke={stroke} strokeWidth={1.2} />
          <Path
            d="M40 80 Q34 84 34 96 L34 196 Q34 204 41 206 L48 206 Q55 204 55 196 L55 90 Q55 80 47 78 Z"
            fill={fill}
            stroke={stroke}
            strokeWidth={1.2}
          />
          <Path
            d="M160 80 Q166 84 166 96 L166 196 Q166 204 159 206 L152 206 Q145 204 145 196 L145 90 Q145 80 153 78 Z"
            fill={fill}
            stroke={stroke}
            strokeWidth={1.2}
          />
          <Path
            d="M68 190 L94 190 Q98 190 98 198 L98 320 Q98 328 91 328 L75 328 Q68 328 68 320 Z"
            fill={fill}
            stroke={stroke}
            strokeWidth={1.2}
          />
          <Path
            d="M132 190 L106 190 Q102 190 102 198 L102 320 Q102 328 109 328 L125 328 Q132 328 132 320 Z"
            fill={fill}
            stroke={stroke}
            strokeWidth={1.2}
          />
        </G>

        {/* Last-used dots */}
        {state.lastUsed.map((uid) => {
          const z = ALL_ZONES.find((a) => a.id === uid);
          if (!z) return null;
          const inView = zones.some((zz) => zz.id === uid);
          if (!inView) return null;
          return <Circle key={'lu-' + uid} cx={z.cx} cy={z.cy} r={3} fill={pal.muted} opacity={0.6} />;
        })}

        {/* Zone targets */}
        {zones.map((z) => {
          const isSel = state.site === z.id;
          const isSug = state.suggested === z.id && !state.site;
          return (
            <G key={z.id} onPress={() => update({ site: z.id })}>
              {isSug && (
                <Circle
                  cx={z.cx}
                  cy={z.cy}
                  r={15}
                  fill="none"
                  stroke={C.sand500}
                  strokeWidth={1.5}
                  strokeDasharray="3 3"
                />
              )}
              <Circle
                cx={z.cx}
                cy={z.cy}
                r={isSel ? 12 : 10}
                fill={isSel ? C.forest700 : 'rgba(246,241,234,0.92)'}
                stroke={isSel ? C.forest700 : pal.border}
                strokeWidth={isSel ? 0 : 1.5}
              />
              {isSel && (
                <Path
                  d={`M${z.cx - 4} ${z.cy} L${z.cx - 1} ${z.cy + 3} L${z.cx + 5} ${z.cy - 3}`}
                  stroke={C.cream}
                  strokeWidth={2}
                  fill="none"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              )}
              {/* Larger invisible hit target */}
              <Circle cx={z.cx} cy={z.cy} r={18} fill="transparent" />
            </G>
          );
        })}
      </Svg>

      <View style={{ minHeight: 32, alignItems: 'center' }}>
        {state.site ? (
          <FadeIn k={state.site}>
            <View style={{ alignItems: 'center' }}>
              <Text
                style={{
                  fontFamily: F.display,
                  fontSize: 17,
                  color: pal.ink,
                  letterSpacing: 17 * -0.012,
                }}
              >
                {zoneLabel(state.site)}
              </Text>
              <Text
                style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}
              >
                Нажмите другую зону, чтобы изменить
              </Text>
            </View>
          </FadeIn>
        ) : (
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
            <View
              style={{ width: 8, height: 8, borderRadius: 999, backgroundColor: C.sand500 }}
            />
            <Text
              style={{
                fontFamily: F.body,
                fontSize: 12,
                color: pal.subtle,
                fontStyle: 'italic',
              }}
            >
              Предложено:{' '}
              <Text style={{ color: pal.ink2, fontStyle: 'normal' }}>
                {zoneLabel(state.suggested)}
              </Text>{' '}
              — следующее в ротации
            </Text>
          </View>
        )}
      </View>
    </View>
  );
}

// ─────────────────────────────────────────────────────────────
// Dose stepper — big mono number + - / +
// ─────────────────────────────────────────────────────────────
export function DoseStepper({
  value,
  onChange,
  step = 0.05,
  unit = 'мг',
}: {
  value: string;
  onChange: (v: string) => void;
  step?: number;
  unit?: string;
}) {
  const v = parseFloat(value);
  const num = isNaN(v) ? 0 : v;
  const fmt = (n: number) =>
    unit === 'мкг' ? String(Math.round(n)) : n.toFixed(2).replace(/\.?0+$/, '') || '0';

  const SZ = { num: 72, btn: 52, btnIcon: 22 };

  const bump = (delta: number) => {
    const next = Math.max(0, +(num + delta).toFixed(3));
    onChange(fmt(next));
  };

  const roundBtn = {
    width: SZ.btn,
    height: SZ.btn,
    borderRadius: 999,
    backgroundColor: pal.paper,
    borderWidth: 1,
    borderColor: pal.border,
    alignItems: 'center' as const,
    justifyContent: 'center' as const,
  };

  return (
    <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 18 }}>
      <Press onPress={() => bump(-step)} style={roundBtn}>
        <Svg width={SZ.btnIcon} height={2} viewBox="0 0 22 2">
          <Line x1={0} y1={1} x2={22} y2={1} stroke={pal.ink2} strokeWidth={2} strokeLinecap="round" />
        </Svg>
      </Press>
      <View
        style={{
          flexDirection: 'row',
          alignItems: 'baseline',
          gap: 6,
          minWidth: 140,
          justifyContent: 'center',
        }}
      >
        <Text
          style={{
            fontFamily: F.monoMedium,
            fontSize: SZ.num,
            color: pal.ink,
            letterSpacing: SZ.num * -0.04,
            fontVariant: ['tabular-nums'],
            lineHeight: SZ.num,
          }}
        >
          {fmtDose(fmt(num))}
        </Text>
        <Text
          style={{ fontFamily: F.displayItalic, fontSize: SZ.num * 0.35, color: pal.muted }}
        >
          {unit}
        </Text>
      </View>
      <Press onPress={() => bump(+step)} style={roundBtn}>
        <Icon name="plus" size={SZ.btnIcon} color={pal.ink2} />
      </Press>
    </View>
  );
}

// ─────────────────────────────────────────────────────────────
// Syringe bar — horizontal pen showing dose on a 100-unit scale
// ─────────────────────────────────────────────────────────────
export function SyringeBar({ fill = 25, max = 100 }: { fill?: number; max?: number }) {
  const pct = Math.min(100, Math.max(0, (fill / max) * 100));
  return (
    <View>
      <View
        style={{
          position: 'relative',
          height: 22,
          backgroundColor: pal.sunk,
          borderRadius: 999,
          borderWidth: 1,
          borderColor: pal.border,
          overflow: 'hidden',
        }}
      >
        {/* fill */}
        <View
          style={{
            position: 'absolute',
            top: 0,
            bottom: 0,
            left: 0,
            width: `${pct}%`,
            backgroundColor: C.sand500,
          }}
        />
        {/* tick marks */}
        {[10, 20, 30, 40, 50, 60, 70, 80, 90].map((t) => (
          <View
            key={t}
            style={{
              position: 'absolute',
              top: 4,
              bottom: 4,
              left: `${t}%`,
              width: 1,
              backgroundColor: pal.border,
              opacity: t % 50 === 0 ? 0.6 : 0.25,
            }}
          />
        ))}
        {/* needle */}
        <View
          style={{
            position: 'absolute',
            right: 0,
            top: '50%',
            marginTop: -1,
            width: 18,
            height: 2,
            backgroundColor: pal.border,
          }}
        />
      </View>
      <View
        style={{ flexDirection: 'row', justifyContent: 'space-between', marginTop: 6 }}
      >
        <Text style={monoMicro}>0u</Text>
        <Text style={[monoMicro, { color: pal.ink2 }]}>{Math.round(fill)} ед. в шприце</Text>
        <Text style={monoMicro}>{max}u</Text>
      </View>
    </View>
  );
}

const monoMicro = {
  fontFamily: F.mono,
  fontSize: 10,
  color: pal.subtle,
  fontVariant: ['tabular-nums'] as ('tabular-nums')[],
};

// ─────────────────────────────────────────────────────────────
// Radio circle shared by compound rows and vial picker
// ─────────────────────────────────────────────────────────────
export function RadioDot({ selected }: { selected: boolean }) {
  return (
    <View
      style={{
        width: 28,
        height: 28,
        borderRadius: 999,
        borderWidth: 2,
        borderColor: selected ? C.forest700 : pal.border,
        backgroundColor: selected ? C.forest700 : 'transparent',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
      }}
    >
      {selected && <Icon name="check" size={12} strokeWidth={2.2} color={C.cream} />}
    </View>
  );
}

// ─────────────────────────────────────────────────────────────
// Vial picker — segmented list with dose-remaining bar
// ─────────────────────────────────────────────────────────────
export function VialPicker({
  state,
  update,
  compoundFilter,
}: {
  state: LogState;
  update: Update;
  compoundFilter?: string;
}) {
  const vials: LogVial[] = compoundFilter
    ? VIALS.filter((v) => v.compound === compoundFilter)
    : VIALS;
  return (
    <View style={{ gap: 6 }}>
      {vials.map((v) => {
        const sel = state.vialId === v.id;
        const pct = Math.round((v.remaining / v.total) * 100);
        return (
          <Press
            key={v.id}
            onPress={() => update({ vialId: v.id })}
            style={{
              paddingVertical: 12,
              paddingHorizontal: 14,
              borderRadius: 14,
              backgroundColor: sel ? pal.paper : 'transparent',
              borderWidth: 1,
              borderColor: sel ? C.forest700 : pal.border,
              flexDirection: 'row',
              gap: 12,
              alignItems: 'center',
            }}
          >
            <RadioDot selected={sel} />
            <View style={{ flex: 1, minWidth: 0 }}>
              <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 6 }}>
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.ink }}>
                  Флакон · {v.dose}
                </Text>
                {v.active && !v.warn && (
                  <Text style={{ fontFamily: F.bodyMedium, fontSize: 10, color: C.forest700 }}>
                    активен
                  </Text>
                )}
                {v.warn && (
                  <Text style={{ fontFamily: F.bodyMedium, fontSize: 10, color: C.warning }}>
                    до {v.expires}
                  </Text>
                )}
              </View>
              <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 4 }}>
                <View
                  style={{
                    flex: 1,
                    height: 4,
                    backgroundColor: pal.bone,
                    borderRadius: 999,
                    overflow: 'hidden',
                    maxWidth: 120,
                  }}
                >
                  <View
                    style={{
                      height: '100%',
                      width: `${pct}%`,
                      backgroundColor: v.warn ? C.warning : C.forest600,
                    }}
                  />
                </View>
                <Text style={monoMicro}>
                  {v.remaining}/{v.total} доз
                </Text>
              </View>
            </View>
            {v.opened !== '—' && (
              <Text numberOfLines={1} style={[monoMicro, { flexShrink: 0 }]}>
                открыт {v.opened}
              </Text>
            )}
          </Press>
        );
      })}
    </View>
  );
}

// ─────────────────────────────────────────────────────────────
// Mood slider — 5 dots
// ─────────────────────────────────────────────────────────────
export function MoodSlider({ value, onChange }: { value: number; onChange: (n: number) => void }) {
  const labels = ['Никак', 'Слабо', 'Ровно', 'Хорошо', 'Светло'];
  return (
    <View>
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 8,
        }}
      >
        {[1, 2, 3, 4, 5].map((n) => {
          const on = value === n;
          return (
            <Press
              key={n}
              onPress={() => onChange(n)}
              style={{
                width: 34,
                height: 34,
                borderRadius: 999,
                backgroundColor: on ? C.forest700 : 'transparent',
                borderWidth: 1.5,
                borderColor: on ? C.forest700 : pal.border,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Text
                style={{
                  fontFamily: F.monoMedium,
                  fontSize: 11,
                  color: on ? C.cream : pal.muted,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {n}
              </Text>
            </Press>
          );
        })}
      </View>
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          paddingHorizontal: 4,
        }}
      >
        <Text style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle }}>Никак</Text>
        <Text
          style={{ fontFamily: F.body, fontSize: 10, color: pal.ink2, fontStyle: 'italic' }}
        >
          {labels[value - 1]}
        </Text>
        <Text style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle }}>Светло</Text>
      </View>
    </View>
  );
}

// ─────────────────────────────────────────────────────────────
// Side-effect chips
// ─────────────────────────────────────────────────────────────
export function ChipsRow({
  value,
  onChange,
}: {
  value: string[];
  onChange: (next: string[]) => void;
}) {
  const toggle = (id: string) => {
    const has = value.includes(id);
    let next: string[];
    if (id === 'none') next = has ? [] : ['none'];
    else next = has ? value.filter((v) => v !== id) : [...value.filter((v) => v !== 'none'), id];
    onChange(next);
  };
  return (
    <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6 }}>
      {SIDE_EFFECTS.map((i) => (
        <Chip key={i.id} active={value.includes(i.id)} onPress={() => toggle(i.id)}>
          {i.label}
        </Chip>
      ))}
    </View>
  );
}

// ─────────────────────────────────────────────────────────────
// Photo slot — simulated capture
// ─────────────────────────────────────────────────────────────
function Spinner() {
  const spin = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    Animated.loop(
      Animated.timing(spin, {
        toValue: 1,
        duration: 700,
        easing: Easing.linear,
        useNativeDriver: true,
      })
    ).start();
  }, [spin]);
  return (
    <Animated.View
      style={{
        width: 18,
        height: 18,
        borderRadius: 999,
        borderWidth: 2,
        borderColor: pal.muted,
        borderTopColor: 'transparent',
        transform: [
          { rotate: spin.interpolate({ inputRange: [0, 1], outputRange: ['0deg', '360deg'] }) },
        ],
      }}
    />
  );
}

export function PhotoSlot({ state, update }: { state: LogState; update: Update }) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);
  const onTap = () => {
    if (state.photo === 'attached') {
      update({ photo: null });
      return;
    }
    update({ photo: 'pending' });
    timer.current = setTimeout(() => update({ photo: 'attached' }), 900);
  };
  const attached = state.photo === 'attached';
  return (
    <Press
      onPress={onTap}
      style={{
        borderRadius: 14,
        backgroundColor: attached ? pal.sunk : 'transparent',
        borderWidth: 1.5,
        borderStyle: 'dashed',
        borderColor: pal.border,
        padding: 14,
        flexDirection: 'row',
        alignItems: 'center',
        gap: 12,
      }}
    >
      <View
        style={{
          width: 44,
          height: 44,
          borderRadius: 12,
          flexShrink: 0,
          backgroundColor: attached ? C.forest700 : pal.sunk,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {state.photo === 'pending' ? (
          <Spinner />
        ) : attached ? (
          <Icon name="check" size={20} strokeWidth={2} color={C.cream} />
        ) : (
          <Icon name="camera" size={20} color={pal.muted} />
        )}
      </View>
      <View style={{ flex: 1 }}>
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.ink }}>
          {attached ? 'Фото добавлено' : state.photo === 'pending' ? 'Снимаем…' : 'Добавить фото'}
        </Text>
        <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}>
          {attached ? 'Нажмите, чтобы убрать' : 'Место или флакон · по желанию'}
        </Text>
      </View>
    </Press>
  );
}

// ─────────────────────────────────────────────────────────────
// Wizard chrome — header w/ Cancel · step counter, progress bar,
// step heading, scrollable body, gradient footer w/ prev+next
// ─────────────────────────────────────────────────────────────
export function WizardChrome({
  step,
  total,
  onCancel,
  onPrev,
  onNext,
  nextLabel = 'Дальше',
  nextDisabled,
  eyebrow,
  title,
  sub,
  children,
}: {
  step: number;
  total: number;
  onCancel: () => void;
  onPrev: () => void;
  onNext: () => void;
  nextLabel?: string;
  nextDisabled?: boolean;
  eyebrow?: string;
  title?: React.ReactNode;
  sub?: string | null;
  children: React.ReactNode;
}) {
  const insets = useSafeAreaInsets();
  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Header */}
      <View
        style={{
          paddingTop: insets.top + 6,
          paddingHorizontal: 18,
          paddingBottom: 6,
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <Press onPress={onCancel}>
          <Text style={{ fontFamily: F.body, fontSize: 14, color: pal.muted }}>Отмена</Text>
        </Press>
        <Text
          style={{
            fontFamily: F.mono,
            fontSize: 11,
            color: pal.subtle,
            fontVariant: ['tabular-nums'],
          }}
        >
          Шаг {step} из {total}
        </Text>
        <View style={{ width: 50 }} />
      </View>

      {/* Progress bar */}
      <View style={{ paddingHorizontal: 18, paddingTop: 8, flexDirection: 'row', gap: 4 }}>
        {Array.from({ length: total }).map((_, i) => (
          <View
            key={i}
            style={{
              flex: 1,
              height: 3,
              borderRadius: 999,
              backgroundColor: i < step ? C.forest700 : pal.bone,
            }}
          />
        ))}
      </View>

      {/* Step heading */}
      {(eyebrow || title) ? (
        <FadeIn k={'head-' + step}>
          <View style={{ paddingTop: 28, paddingHorizontal: 24, paddingBottom: 18 }}>
            {eyebrow ? <Eyebrow style={{ marginBottom: 8 }}>{eyebrow}</Eyebrow> : null}
            {title}
            {sub ? (
              <Body size={13} color={pal.muted} style={{ marginTop: 8, maxWidth: 320 }}>
                {sub}
              </Body>
            ) : null}
          </View>
        </FadeIn>
      ) : null}

      {/* Step body */}
      <ScrollView
        style={{ flex: 1 }}
        contentContainerStyle={{ paddingHorizontal: 18, paddingBottom: 140 }}
        showsVerticalScrollIndicator={false}
      >
        <FadeIn k={'body-' + step}>{children}</FadeIn>
      </ScrollView>

      {/* Footer */}
      <LinearGradient
        colors={['rgba(246,241,234,0)', pal.bg]}
        locations={[0, 0.3]}
        style={{
          position: 'absolute',
          left: 0,
          right: 0,
          bottom: 0,
          paddingTop: 12,
          paddingHorizontal: 18,
          paddingBottom: Math.max(insets.bottom, 22) + 12,
        }}
      >
        <View style={{ flexDirection: 'row', gap: 8 }}>
          {step > 1 && (
            <Press
              onPress={onPrev}
              style={{
                paddingVertical: 14,
                paddingHorizontal: 18,
                borderRadius: 999,
                backgroundColor: pal.sunk,
                flexDirection: 'row',
                alignItems: 'center',
                gap: 4,
              }}
            >
              <Icon name="chevron-left" size={16} color={pal.ink} />
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}>
                Назад
              </Text>
            </Press>
          )}
          <Press
            onPress={onNext}
            disabled={nextDisabled}
            style={{
              flex: 1,
              paddingVertical: 14,
              paddingHorizontal: 20,
              borderRadius: 999,
              backgroundColor: nextDisabled ? pal.sunk : C.forest700,
              opacity: nextDisabled ? 0.65 : 1,
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 6,
            }}
          >
            <Text
              style={{
                fontFamily: F.bodyMedium,
                fontSize: 14,
                color: nextDisabled ? pal.subtle : C.cream,
              }}
            >
              {nextLabel}
            </Text>
            {nextLabel !== 'Сохранить дозу' && (
              <Icon
                name="arrow-right"
                size={16}
                color={nextDisabled ? pal.subtle : C.cream}
              />
            )}
          </Press>
        </View>
      </LinearGradient>
    </View>
  );
}
