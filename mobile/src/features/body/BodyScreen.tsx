// Cadence — Тело (Body metrics detail).
// Ported from design_handoff_cadence_app/body/body.jsx (BodyScreen).
// Composition ring · measurements with inline sparklines (tap for a fuller chart)
// · framed progress photos with a было/стало compare · working add flows.
import React, { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Circle, Path } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Eyebrow, Pill, Press, Title } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { Sheet } from '../../components/shared';
import { useAppState } from '../../state/AppState';
import { BODY, BodyPhoto, BodyState, MetricMeta } from './data';

const B = BODY;
const metaOf = (id: string): MetricMeta => B.metricMeta(id)!;

// ── Inline sparkline (auto-scaled) ────────────────────────────
function BodySpark({
  values,
  color,
  width = 64,
  height = 26,
  dot = true,
}: {
  values: number[];
  color: string;
  width?: number;
  height?: number;
  dot?: boolean;
}) {
  if (!values || values.length < 2) return <Svg width={width} height={height} />;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const pad = 3;
  const xOf = (i: number) => pad + (i / (values.length - 1)) * (width - pad * 2);
  const yOf = (v: number) => height - pad - ((v - min) / span) * (height - pad * 2);
  const d = values
    .map((v, i) => `${i ? 'L' : 'M'}${xOf(i).toFixed(1)} ${yOf(v).toFixed(1)}`)
    .join(' ');
  const lx = xOf(values.length - 1);
  const ly = yOf(values[values.length - 1]);
  return (
    <Svg width={width} height={height}>
      <Path d={d} fill="none" stroke={color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
      {dot ? <Circle cx={lx} cy={ly} r={2.6} fill={color} /> : null}
    </Svg>
  );
}

// ── Composition ring (fat vs lean) ────────────────────────────
function CompositionRing({ weight, bf }: { weight: number; bf: number }) {
  const lean = B.leanOf(weight, bf);
  const size = 150;
  const stroke = 14;
  const r = (size - stroke) / 2;
  const C0 = 2 * Math.PI * r;
  const frac = bf / 100;
  const fatLen = C0 * frac;
  return (
    <View style={{ flexDirection: 'row', gap: 18, alignItems: 'center' }}>
      <View style={{ width: size, height: size, flexShrink: 0 }}>
        <Svg width={size} height={size}>
          <Circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={C.forest700} strokeWidth={stroke} />
          <Circle
            cx={size / 2}
            cy={size / 2}
            r={r}
            fill="none"
            stroke={C.sand500}
            strokeWidth={stroke}
            strokeDasharray={`${fatLen} ${C0 - fatLen}`}
            strokeDashoffset={0}
            strokeLinecap="butt"
            transform={`rotate(-90 ${size / 2} ${size / 2})`}
          />
        </Svg>
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
            {B.fmt(weight, 1)}
          </Text>
          <Text style={{ fontFamily: F.displayItalic, fontSize: 15, color: pal.muted, marginTop: 2 }}>
            кг
          </Text>
        </View>
      </View>
      <View style={{ flex: 1, gap: 12 }}>
        {[
          { c: C.sand500, label: '% жира', v: B.fmt(bf, 1), u: '%' },
          { c: C.forest700, label: 'Сухая масса', v: B.fmt(lean, 1), u: 'кг' },
        ].map((row, i) => (
          <View key={i} style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
            <View style={{ width: 12, height: 12, borderRadius: 4, backgroundColor: row.c, flexShrink: 0 }} />
            <View style={{ minWidth: 0 }}>
              <Text style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>{row.label}</Text>
              <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 3 }}>
                <Text
                  style={{
                    fontFamily: F.monoMedium,
                    fontSize: 19,
                    color: pal.ink,
                    letterSpacing: 19 * -0.02,
                    fontVariant: ['tabular-nums'],
                  }}
                >
                  {row.v}
                </Text>
                <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>{row.u}</Text>
              </View>
            </View>
          </View>
        ))}
      </View>
    </View>
  );
}

// ── Framed progress-photo tile ────────────────────────────────
function PhotoTile({ photo, w = 96, h = 132 }: { photo: BodyPhoto; w?: number; h?: number }) {
  return (
    <View
      style={{
        flexShrink: 0,
        width: w,
        height: h,
        borderRadius: 14,
        overflow: 'hidden',
        borderWidth: 1,
        borderColor: pal.hairline,
      }}
    >
      <LinearGradient
        colors={[C.sand300, C.linen, C.bone]}
        locations={[0, 0.68, 1]}
        start={{ x: 0.1, y: 0 }}
        end={{ x: 0.9, y: 1 }}
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
        <Icon name="user" size={Math.round(h * 0.26)} color="rgba(107,74,37,.26)" />
      </View>
      <LinearGradient
        colors={['rgba(26,26,26,0)', 'rgba(26,26,26,.46)']}
        style={{
          position: 'absolute',
          left: 0,
          right: 0,
          bottom: 0,
          paddingTop: 16,
          paddingHorizontal: 9,
          paddingBottom: 8,
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
          {photo.week}
        </Text>
        {photo.weight != null ? (
          <Text
            style={{
              fontFamily: F.mono,
              fontSize: 12,
              color: C.cream,
              fontVariant: ['tabular-nums'],
            }}
          >
            {B.fmt(photo.weight, 1)} кг
          </Text>
        ) : null}
      </LinearGradient>
    </View>
  );
}

// ── Stepper used in the add sheet ─────────────────────────────
function MeasureStepper({
  meta,
  value,
  onChange,
}: {
  meta: MetricMeta;
  value: number;
  onChange: (v: number) => void;
}) {
  const step = meta.step ?? 1;
  const min = meta.min ?? 0;
  const max = meta.max ?? 999;
  const bump = (dir: number) => {
    const next = +(value + dir * step).toFixed(meta.dec);
    onChange(Math.max(min, Math.min(max, next)));
  };
  return (
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 14, justifyContent: 'center' }}>
      <Press
        onPress={() => bump(-1)}
        style={{
          width: 52,
          height: 52,
          borderRadius: 999,
          borderWidth: 1,
          borderColor: pal.border,
          backgroundColor: pal.paper,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Text style={{ fontFamily: F.body, fontSize: 24, lineHeight: 28, color: pal.ink }}>−</Text>
      </Press>
      <View style={{ minWidth: 132, alignItems: 'center' }}>
        <View style={{ flexDirection: 'row', alignItems: 'baseline', justifyContent: 'center', gap: 4 }}>
          <Text
            style={{
              fontFamily: F.monoMedium,
              fontSize: 44,
              color: pal.ink,
              letterSpacing: 44 * -0.03,
              lineHeight: 46,
              fontVariant: ['tabular-nums'],
            }}
          >
            {B.fmt(value, meta.dec)}
          </Text>
          {meta.unit ? (
            <Text style={{ fontFamily: F.displayItalic, fontSize: 18, color: pal.muted }}>
              {meta.unit}
            </Text>
          ) : null}
        </View>
      </View>
      <Press
        onPress={() => bump(1)}
        style={{
          width: 52,
          height: 52,
          borderRadius: 999,
          backgroundColor: C.forest700,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Text style={{ fontFamily: F.body, fontSize: 24, lineHeight: 28, color: C.cream }}>+</Text>
      </Press>
    </View>
  );
}

// ── Sheet header (serif title + round close) ──────────────────
function SheetHead({ title, onClose }: { title: React.ReactNode; onClose: () => void }) {
  return (
    <View
      style={{
        paddingTop: 6,
        paddingBottom: 8,
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}
    >
      <Title size={26}>{title}</Title>
      <Press
        onPress={onClose}
        style={{
          width: 34,
          height: 34,
          borderRadius: 999,
          backgroundColor: pal.sunk,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name="x-mark" size={17} color={pal.ink2} />
      </Press>
    </View>
  );
}

// ── Add measurement (and quick weight) ────────────────────────
function AddMeasureSheet({
  state,
  presetId,
  onClose,
  onSave,
}: {
  state: BodyState;
  presetId: string | null;
  onClose: () => void;
  onSave: (id: string, val: number) => void;
}) {
  const editable = B.METRICS.filter((m) => m.editable);
  const [id, setId] = useState(presetId || 'weight');
  const meta = metaOf(id);
  const [val, setVal] = useState(() => B.latest(B.histOf(state, id)));
  const pick = (nid: string) => {
    setId(nid);
    setVal(B.latest(B.histOf(state, nid)));
  };

  return (
    <Sheet open onClose={onClose}>
      <SheetHead
        onClose={onClose}
        title={
          <>
            Добавить{' '}
            <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>замер</Text>
          </>
        }
      />
      <View style={{ paddingTop: 14, paddingBottom: 8 }}>
        <Eyebrow color={pal.subtle} style={{ marginBottom: 10 }}>
          Что измеряем
        </Eyebrow>
        <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginBottom: 26 }}>
          {editable.map((m) => {
            const on = m.id === id;
            return (
              <Press
                key={m.id}
                onPress={() => pick(m.id)}
                style={{
                  paddingVertical: 8,
                  paddingHorizontal: 14,
                  borderRadius: 999,
                  backgroundColor: on ? C.forest700 : 'transparent',
                  borderWidth: 1,
                  borderColor: on ? C.forest700 : pal.border,
                }}
              >
                <Text
                  style={{
                    fontFamily: F.bodyMedium,
                    fontSize: 13,
                    color: on ? C.cream : pal.ink2,
                  }}
                >
                  {m.label}
                </Text>
              </Press>
            );
          })}
        </View>
        <View style={{ paddingBottom: 8 }}>
          <MeasureStepper meta={meta} value={val} onChange={setVal} />
        </View>
        <Text
          style={{
            textAlign: 'center',
            fontFamily: F.body,
            fontSize: 12,
            color: pal.subtle,
            marginTop: 14,
          }}
        >
          Сейчас в журнале · {B.fmt(B.latest(B.histOf(state, id)), meta.dec)} {meta.unit}
        </Text>
      </View>
      <View
        style={{
          marginHorizontal: -20,
          paddingHorizontal: 20,
          paddingTop: 10,
          borderTopWidth: 1,
          borderTopColor: pal.hairline,
        }}
      >
        <Press
          onPress={() => onSave(id, val)}
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
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 15, color: C.cream }}>Сохранить</Text>
          <Icon name="check" size={16} strokeWidth={2} color={C.cream} />
        </Press>
      </View>
    </Sheet>
  );
}

// ── Add photo ─────────────────────────────────────────────────
function AddPhotoSheet({ onClose, onCapture }: { onClose: () => void; onCapture: () => void }) {
  return (
    <Sheet open onClose={onClose}>
      <SheetHead
        onClose={onClose}
        title={
          <>
            Новый{' '}
            <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>снимок</Text>
          </>
        }
      />
      <View style={{ paddingTop: 14, paddingBottom: 8 }}>
        <View
          style={{
            width: '100%',
            height: 320,
            borderRadius: 18,
            overflow: 'hidden',
            borderWidth: 1,
            borderStyle: 'dashed',
            borderColor: pal.border,
            alignItems: 'center',
            justifyContent: 'center',
            gap: 10,
          }}
        >
          <LinearGradient
            colors={[C.sand300, C.linen, C.bone]}
            locations={[0, 0.7, 1]}
            start={{ x: 0.1, y: 0 }}
            end={{ x: 0.9, y: 1 }}
            style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0 }}
          />
          <Icon name="camera" size={40} color="#9a5a3c" />
          <Text style={{ fontFamily: F.body, fontSize: 13, color: '#7a4a06' }}>
            Анфас · ровный свет
          </Text>
        </View>
        <Text
          style={{
            fontFamily: F.body,
            fontSize: 12.5,
            color: pal.subtle,
            textAlign: 'center',
            marginTop: 14,
            lineHeight: 12.5 * 1.5,
          }}
        >
          Снимок добавится к этой неделе и встанет рядом с прошлыми.
        </Text>
      </View>
      <View style={{ paddingTop: 12 }}>
        <Press
          onPress={onCapture}
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
          <Icon name="camera" size={17} color={C.cream} />
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 15, color: C.cream }}>
            Сделать снимок
          </Text>
        </Press>
      </View>
    </Sheet>
  );
}

// ── Measurement detail sheet (fuller chart + history) ─────────
function MeasureDetailSheet({
  state,
  metricId,
  onClose,
  onUpdate,
  onOpenTrend,
}: {
  state: BodyState;
  metricId: string;
  onClose: () => void;
  onUpdate: (id: string) => void;
  onOpenTrend: (id: string) => void;
}) {
  const meta = metaOf(metricId);
  const hist = B.histOf(state, metricId);
  const d = B.delta(hist);
  const W = 300;
  const H = 90;
  const min = Math.min(...hist);
  const max = Math.max(...hist);
  const span = max - min || 1;
  const pad = 8;
  const xOf = (i: number) => pad + (i / (hist.length - 1)) * (W - pad * 2);
  const yOf = (v: number) => H - pad - ((v - min) / span) * (H - pad * 2);
  const path = hist.map((v, i) => `${i ? 'L' : 'M'}${xOf(i).toFixed(1)} ${yOf(v).toFixed(1)}`).join(' ');

  return (
    <Sheet open onClose={onClose} contentStyle={{ maxHeight: '88%' }}>
      <SheetHead onClose={onClose} title={meta.label} />
      <ScrollView showsVerticalScrollIndicator={false} style={{ flexGrow: 0 }}>
        <View
          style={{
            flexDirection: 'row',
            alignItems: 'baseline',
            gap: 10,
            marginTop: 8,
            marginBottom: 14,
          }}
        >
          <Text
            style={{
              fontFamily: F.monoMedium,
              fontSize: 40,
              color: pal.ink,
              letterSpacing: 40 * -0.03,
              lineHeight: 42,
              fontVariant: ['tabular-nums'],
            }}
          >
            {B.fmt(B.latest(hist), meta.dec)}
          </Text>
          {meta.unit ? (
            <Text style={{ fontFamily: F.displayItalic, fontSize: 18, color: pal.muted }}>
              {meta.unit}
            </Text>
          ) : null}
          <View style={{ marginLeft: 'auto' }}>
            <Pill tone={d.down ? 'forest' : 'neutral'}>
              {B.fmtDelta(hist, meta.dec, meta.unit)} за курс
            </Pill>
          </View>
        </View>
        <View
          style={{
            backgroundColor: pal.paper,
            borderRadius: 16,
            borderWidth: 1,
            borderColor: pal.hairline,
            padding: 14,
            marginBottom: 14,
          }}
        >
          <View style={{ width: '100%', aspectRatio: W / H }}>
            <Svg width="100%" height="100%" viewBox={`0 0 ${W} ${H}`}>
              <Path d={path} fill="none" stroke={C.forest700} strokeWidth={2.5} strokeLinecap="round" strokeLinejoin="round" />
              {hist.map((v, i) => (
                <Circle
                  key={i}
                  cx={xOf(i)}
                  cy={yOf(v)}
                  r={i === hist.length - 1 ? 4 : 2.6}
                  fill={i === hist.length - 1 ? C.sand500 : C.paper}
                  stroke={C.forest700}
                  strokeWidth={2}
                />
              ))}
            </Svg>
          </View>
        </View>
        {/* history rows */}
        <View
          style={{
            backgroundColor: pal.paper,
            borderRadius: 16,
            borderWidth: 1,
            borderColor: pal.hairline,
            overflow: 'hidden',
          }}
        >
          {hist.map((v, i) => (
            <View
              key={i}
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'center',
                paddingVertical: 11,
                paddingHorizontal: 14,
                borderBottomWidth: i < hist.length - 1 ? 1 : 0,
                borderBottomColor: pal.hairline,
              }}
            >
              <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.muted }}>
                {B.WEEK_LABELS[i] || `Точка ${i + 1}`}
              </Text>
              <Text
                style={{
                  fontFamily: F.mono,
                  fontSize: 14,
                  color: pal.ink,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {B.fmt(v, meta.dec)} {meta.unit}
              </Text>
            </View>
          ))}
        </View>
        <View style={{ height: 8 }} />
      </ScrollView>
      <View
        style={{
          marginHorizontal: -20,
          paddingHorizontal: 20,
          paddingTop: 10,
          borderTopWidth: 1,
          borderTopColor: pal.hairline,
          flexDirection: 'row',
          gap: 10,
        }}
      >
        {meta.trendId ? (
          <Press
            onPress={() => onOpenTrend(meta.trendId!)}
            style={{
              flex: 1,
              paddingVertical: 13,
              paddingHorizontal: 16,
              borderRadius: 999,
              backgroundColor: 'transparent',
              borderWidth: 1,
              borderColor: pal.border,
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 6,
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink2 }}>
              Открыть тренд
            </Text>
            <Icon name="arrow-right" size={15} color={pal.ink2} />
          </Press>
        ) : null}
        {meta.editable ? (
          <Press
            onPress={() => onUpdate(metricId)}
            style={{
              flex: 1,
              paddingVertical: 13,
              paddingHorizontal: 16,
              borderRadius: 999,
              backgroundColor: C.forest700,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>Обновить</Text>
          </Press>
        ) : null}
      </View>
    </Sheet>
  );
}

// ════════════════════════════════════════════════════════════════
// Body screen
// ════════════════════════════════════════════════════════════════
export function BodyScreen({
  onBack,
  onOpenTrend,
}: {
  onBack: () => void;
  onOpenTrend: (id: string) => void;
}) {
  const insets = useSafeAreaInsets();
  const { bodyState, addBodyMeasure, addBodyPhoto } = useAppState();
  const state = bodyState as BodyState;

  const [addPreset, setAddPreset] = useState<string | null>(null); // metric id or null
  const [addOpen, setAddOpen] = useState(false);
  const [detail, setDetail] = useState<string | null>(null); // metric id
  const [photoOpen, setPhotoOpen] = useState(false);

  const weight = B.latest(state.hist.weight);
  const bf = B.latest(state.hist.bodyfat);
  const toTarget = weight - B.TARGET.weight;
  const photos = state.photos;
  const firstPhoto = photos[0];
  const lastPhoto = photos[photos.length - 1];
  const wDelta =
    lastPhoto.weight != null && firstPhoto.weight != null
      ? lastPhoto.weight - firstPhoto.weight
      : null;

  const openAdd = (preset: string | null) => {
    setAddPreset(preset || null);
    setAddOpen(true);
  };
  const measureRows = ['waist', 'hip', 'chest', 'bmi'];

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
            alignItems: 'center',
            gap: 10,
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
          <Text style={{ fontFamily: F.bodySemibold, fontSize: 15, color: pal.ink }}>Тело</Text>
        </View>
      </View>

      <ScrollView
        showsVerticalScrollIndicator={false}
        contentContainerStyle={{ paddingBottom: 96 + insets.bottom }}
      >
        {/* Composition */}
        <View style={{ paddingTop: 6, paddingHorizontal: 16, paddingBottom: 14 }}>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              padding: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
              shadowColor: '#2e2618',
              shadowOpacity: 0.05,
              shadowRadius: 6,
              shadowOffset: { width: 0, height: 2 },
              elevation: 2,
            }}
          >
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'baseline',
                marginBottom: 16,
              }}
            >
              <Eyebrow color={pal.subtle}>Состав тела</Eyebrow>
              <Press
                onPress={() => onOpenTrend('weight')}
                style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}
              >
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
                  Вес · тренд
                </Text>
                <Icon name="arrow-right" size={13} color={C.forest700} />
              </Press>
            </View>
            <CompositionRing weight={weight} bf={bf} />
            <View
              style={{
                flexDirection: 'row',
                alignItems: 'center',
                gap: 8,
                marginTop: 16,
                paddingTop: 14,
                borderTopWidth: 1,
                borderTopColor: pal.hairline,
              }}
            >
              <Icon name="scale" size={16} color={pal.subtle} />
              <Text style={{ flex: 1, fontFamily: F.body, fontSize: 12.5, color: pal.muted }}>
                Цель{' '}
                <Text style={{ fontFamily: F.mono, color: pal.ink2, fontVariant: ['tabular-nums'] }}>
                  {B.fmt(B.TARGET.weight, 1)} кг
                </Text>{' '}
                · осталось{' '}
                <Text style={{ fontFamily: F.mono, color: C.forest700, fontVariant: ['tabular-nums'] }}>
                  {B.fmt(toTarget, 1)} кг
                </Text>
              </Text>
              <Press
                onPress={() => openAdd('weight')}
                style={{
                  marginLeft: 'auto',
                  paddingVertical: 8,
                  paddingHorizontal: 14,
                  borderRadius: 999,
                  borderWidth: 1,
                  borderColor: pal.border,
                  backgroundColor: 'transparent',
                }}
              >
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 12.5, color: pal.ink2 }}>
                  Обновить вес
                </Text>
              </Press>
            </View>
          </View>
        </View>

        {/* Measurements */}
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
            <Eyebrow color={pal.subtle}>Замеры</Eyebrow>
            <Press
              onPress={() => openAdd(null)}
              style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}
            >
              <Icon name="plus" size={13} strokeWidth={2} color={C.forest700} />
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
                Добавить
              </Text>
            </Press>
          </View>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
              overflow: 'hidden',
            }}
          >
            {measureRows.map((mid, i) => {
              const meta = metaOf(mid);
              const hist = B.histOf(state, mid);
              const d = B.delta(hist);
              return (
                <Press
                  key={mid}
                  onPress={() => setDetail(mid)}
                  style={{
                    flexDirection: 'row',
                    gap: 14,
                    alignItems: 'center',
                    paddingVertical: 13,
                    paddingHorizontal: 14,
                    borderBottomWidth: i < measureRows.length - 1 ? 1 : 0,
                    borderBottomColor: pal.hairline,
                  }}
                >
                  <View style={{ flex: 1, minWidth: 0 }}>
                    <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}>
                      {meta.label}
                    </Text>
                    <View
                      style={{ flexDirection: 'row', alignItems: 'baseline', gap: 4, marginTop: 2 }}
                    >
                      <Text
                        style={{
                          fontFamily: F.monoMedium,
                          fontSize: 18,
                          color: pal.ink,
                          letterSpacing: 18 * -0.02,
                          fontVariant: ['tabular-nums'],
                        }}
                      >
                        {B.fmt(B.latest(hist), meta.dec)}
                      </Text>
                      {meta.unit ? (
                        <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>
                          {meta.unit}
                        </Text>
                      ) : null}
                      <Text
                        style={{
                          fontFamily: F.body,
                          fontSize: 11.5,
                          color: d.down ? C.forest700 : pal.subtle,
                          marginLeft: 4,
                        }}
                      >
                        {B.fmtDelta(hist, meta.dec)}
                      </Text>
                    </View>
                  </View>
                  <BodySpark values={hist} color={C.forest700} width={60} height={26} />
                  <Icon name="chevron-right" size={16} color={pal.placeholder} />
                </Press>
              );
            })}
          </View>
        </View>

        {/* Before / after */}
        {wDelta != null ? (
          <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
            <View style={{ paddingHorizontal: 4, paddingBottom: 10 }}>
              <Eyebrow color={pal.subtle}>До и после</Eyebrow>
            </View>
            <View
              style={{
                backgroundColor: pal.paper,
                borderRadius: 18,
                borderWidth: 1,
                borderColor: pal.hairline,
                padding: 16,
                shadowColor: '#2e2618',
                shadowOpacity: 0.05,
                shadowRadius: 6,
                shadowOffset: { width: 0, height: 2 },
                elevation: 2,
              }}
            >
              <View style={{ flexDirection: 'row', gap: 12, alignItems: 'center' }}>
                <View style={{ flex: 1, alignItems: 'center', gap: 8 }}>
                  <PhotoTile photo={firstPhoto} w={104} h={142} />
                  <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>
                    было · {firstPhoto.week}
                  </Text>
                </View>
                <View style={{ alignItems: 'center' }}>
                  <Text
                    style={{
                      fontFamily: F.displayItalic,
                      fontSize: 30,
                      color: C.forest700,
                      lineHeight: 32,
                    }}
                  >
                    −{B.fmt(Math.abs(wDelta), 1)}
                  </Text>
                  <Text
                    style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}
                  >
                    кг
                  </Text>
                </View>
                <View style={{ flex: 1, alignItems: 'center', gap: 8 }}>
                  <PhotoTile photo={lastPhoto} w={104} h={142} />
                  <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>
                    стало · {lastPhoto.week}
                  </Text>
                </View>
              </View>
            </View>
          </View>
        ) : null}

        {/* Gallery */}
        <View style={{ paddingBottom: 8 }}>
          <View style={{ paddingHorizontal: 20, paddingBottom: 10 }}>
            <Eyebrow color={pal.subtle}>Все снимки</Eyebrow>
          </View>
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ gap: 10, paddingHorizontal: 16, paddingBottom: 4 }}
          >
            {photos.map((p) => (
              <PhotoTile key={p.id} photo={p} />
            ))}
            <Press
              onPress={() => setPhotoOpen(true)}
              style={{
                width: 96,
                height: 132,
                borderRadius: 14,
                backgroundColor: 'transparent',
                borderWidth: 1.5,
                borderStyle: 'dashed',
                borderColor: pal.border,
                alignItems: 'center',
                justifyContent: 'center',
                gap: 6,
              }}
            >
              <Icon name="camera" size={22} color={pal.muted} />
              <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.muted }}>Снимок</Text>
            </Press>
          </ScrollView>
        </View>
      </ScrollView>

      {/* Sheets */}
      {addOpen ? (
        <AddMeasureSheet
          state={state}
          presetId={addPreset}
          onClose={() => setAddOpen(false)}
          onSave={(id, val) => {
            addBodyMeasure(id, val);
            setAddOpen(false);
          }}
        />
      ) : null}
      {photoOpen ? (
        <AddPhotoSheet
          onClose={() => setPhotoOpen(false)}
          onCapture={() => {
            addBodyPhoto();
            setPhotoOpen(false);
          }}
        />
      ) : null}
      {detail != null ? (
        <MeasureDetailSheet
          state={state}
          metricId={detail}
          onClose={() => setDetail(null)}
          onUpdate={(id) => {
            setDetail(null);
            openAdd(id);
          }}
          onOpenTrend={(tid) => {
            setDetail(null);
            onOpenTrend(tid);
          }}
        />
      ) : null}
    </View>
  );
}
