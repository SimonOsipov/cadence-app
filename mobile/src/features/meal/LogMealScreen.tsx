// Cadence — Meal-log modal, ported from meal/meal-flow.jsx (MealLogScreen).
// Segmented mode (Текст / Фото / Голос), input, parsed-items review, Save footer.
import React, { useEffect, useRef, useState } from 'react';
import {
  Animated,
  Easing,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  Text,
  TextInput,
  View,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Circle, Path } from 'react-native-svg';
import { C, F, pal, SHADOW } from '../../theme';
import { Eyebrow, Meta, Press } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import {
  MEAL_TARGETS,
  MealItem,
  MealTotals,
  SAMPLE_PARSES,
  SampleParse,
  pickSampleForInput,
  rescaleItem,
  totalMeal,
} from './data';

// Carb macro dot color — from the prototype's ParsedItem/MacroStrip.
const CARB = '#a5773d';

type Stage = 'idle' | 'capturing' | 'parsing' | 'parsed';
type Mode = 'chat' | 'photo' | 'voice';

const MODES: { id: Mode; icon: 'paper-airplane' | 'camera' | 'microphone'; label: string }[] = [
  { id: 'chat', icon: 'paper-airplane', label: 'Текст' },
  { id: 'photo', icon: 'camera', label: 'Фото' },
  { id: 'voice', icon: 'microphone', label: 'Голос' },
];

export function LogMealScreen({
  onCancel,
  onComplete,
}: {
  onCancel: () => void;
  onComplete: (meal: any) => void;
}) {
  const insets = useSafeAreaInsets();
  const [mode, setMode] = useState<Mode>('chat');
  const [stage, setStage] = useState<Stage>('idle');
  const [transcript, setTranscript] = useState('');
  const [items, setItems] = useState<MealItem[]>([]);
  const [mealName, setMealName] = useState('');
  const [editingGramsId, setEditingGramsId] = useState<string | null>(null);
  const [sampleIdx, setSampleIdx] = useState(0);
  const [chatText, setChatText] = useState('');

  // Reset on mode change
  const switchMode = (m: Mode) => {
    if (m === mode) return;
    setMode(m);
    setStage('idle');
    setTranscript('');
    setItems([]);
    setChatText('');
  };

  // Common "I've got a parsed sample" handler
  const acceptSample = (sample: SampleParse) => {
    setMealName(sample.mealName);
    setTranscript(sample.transcript);
    setItems(sample.items.map((it) => ({ ...it })));
    setStage('parsed');
  };

  // Run a fake parse — show 'parsing' for a beat, then settle
  const runParse = (text: string) => {
    setStage('parsing');
    const sample = pickSampleForInput(text);
    setTimeout(() => acceptSample(sample), 720);
  };

  const totals = totalMeal(items);
  const canSave = stage === 'parsed' && items.length > 0;
  const posWord =
    items.length === 1 ? 'позиция' : items.length < 5 ? 'позиции' : 'позиций';

  return (
    <KeyboardAvoidingView
      style={{ flex: 1 }}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
    >
      <View style={{ flex: 1, backgroundColor: pal.bg }}>
        <ScrollView
          style={{ flex: 1 }}
          contentContainerStyle={{ paddingTop: insets.top + 6, paddingBottom: 130 }}
          keyboardShouldPersistTaps="handled"
        >
          {/* Top bar */}
          <View
            style={{
              paddingVertical: 8,
              paddingHorizontal: 16,
              flexDirection: 'row',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}
          >
            <Press
              onPress={onCancel}
              style={{
                width: 40,
                height: 40,
                borderRadius: 999,
                backgroundColor: pal.sunk,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name="x-mark" size={18} color={pal.ink2} />
            </Press>
            <Text
              style={{
                fontFamily: F.mono,
                fontSize: 12,
                color: pal.subtle,
                fontVariant: ['tabular-nums'],
              }}
            >
              08:42 · вс 24 мая
            </Text>
            <View style={{ width: 40 }} />
          </View>

          {/* Hero title */}
          <View style={{ paddingTop: 4, paddingHorizontal: 24, paddingBottom: 18 }}>
            <Eyebrow color={C.sand700} style={{ marginBottom: 6 }}>
              Запись приёма
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
              Что вы{' '}
              <Text style={{ fontFamily: F.displayItalic, color: C.sand700 }}>ели?</Text>
            </Text>
          </View>

          {/* Segmented mode picker */}
          <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
            <View
              style={{
                flexDirection: 'row',
                backgroundColor: pal.sunk,
                borderRadius: 12,
                padding: 3,
                gap: 2,
              }}
            >
              {MODES.map((m) => {
                const on = mode === m.id;
                return (
                  <Pressable
                    key={m.id}
                    onPress={() => switchMode(m.id)}
                    style={{
                      flex: 1,
                      paddingVertical: 10,
                      borderRadius: 9,
                      backgroundColor: on ? pal.paper : 'transparent',
                      flexDirection: 'row',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: 6,
                      ...(on ? SHADOW.xs : null),
                    }}
                  >
                    <MealModeIcon name={m.icon} color={on ? pal.ink : pal.muted} />
                    <Text
                      style={{
                        fontFamily: F.bodyMedium,
                        fontSize: 13,
                        color: on ? pal.ink : pal.muted,
                      }}
                    >
                      {m.label}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
          </View>

          {/* Mode-specific input */}
          <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
            {mode === 'chat' && (
              <ChatInput
                text={chatText}
                setText={setChatText}
                stage={stage}
                onParse={(txt) => runParse(txt)}
                sampleIdx={sampleIdx}
                onCycleSample={() => {
                  const next = (sampleIdx + 1) % SAMPLE_PARSES.length;
                  setSampleIdx(next);
                  setChatText(SAMPLE_PARSES[next].transcript);
                }}
              />
            )}
            {mode === 'photo' && (
              <PhotoInput setStage={setStage} onParse={(samp) => acceptSample(samp)} />
            )}
            {mode === 'voice' && (
              <VoiceInput setStage={setStage} onParse={(samp) => acceptSample(samp)} />
            )}
          </View>

          {/* Parsed transcript chip (after parse) */}
          {stage === 'parsed' && !!transcript && (
            <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
              <View
                style={{
                  backgroundColor: pal.sunk,
                  borderRadius: 14,
                  paddingVertical: 10,
                  paddingHorizontal: 12,
                  flexDirection: 'row',
                  alignItems: 'flex-start',
                  gap: 10,
                  borderWidth: 1,
                  borderColor: pal.hairline,
                }}
              >
                <Icon name="sparkles" size={16} color={C.sand700} />
                <View style={{ minWidth: 0, flex: 1 }}>
                  <Text
                    style={{
                      fontFamily: F.body,
                      fontSize: 11,
                      color: pal.subtle,
                      marginBottom: 2,
                    }}
                  >
                    Услышали
                  </Text>
                  <Text
                    style={{
                      fontFamily: F.body,
                      fontSize: 12.5,
                      color: pal.ink2,
                      lineHeight: 12.5 * 1.4,
                    }}
                  >
                    {transcript}
                  </Text>
                </View>
              </View>
            </View>
          )}

          {/* Parsed items list */}
          {stage === 'parsed' && (
            <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
              <View
                style={{
                  flexDirection: 'row',
                  justifyContent: 'space-between',
                  alignItems: 'baseline',
                  paddingTop: 4,
                  paddingHorizontal: 4,
                  paddingBottom: 10,
                }}
              >
                <Eyebrow color={pal.subtle}>
                  {mealName} · {items.length} {posWord}
                </Eyebrow>
                <Text
                  style={{
                    fontFamily: F.mono,
                    fontSize: 11,
                    color: pal.subtle,
                    fontVariant: ['tabular-nums'],
                  }}
                >
                  {totals.kcal} ккал
                </Text>
              </View>
              <View style={{ gap: 8 }}>
                {items.map((it) => (
                  <ParsedItem
                    key={it.id}
                    item={it}
                    editing={editingGramsId === it.id}
                    onStartEdit={() =>
                      setEditingGramsId(editingGramsId === it.id ? null : it.id)
                    }
                    onChangeGrams={(g) => {
                      setItems(items.map((x) => (x.id === it.id ? rescaleItem(x, g) : x)));
                    }}
                    onDelete={() => setItems(items.filter((x) => x.id !== it.id))}
                  />
                ))}
              </View>

              {/* Totals strip */}
              <View style={{ marginTop: 12 }}>
                <MacroStrip totals={totals} />
              </View>
            </View>
          )}

          {/* Parsing skeleton */}
          {stage === 'parsing' && (
            <View style={{ paddingHorizontal: 16 }}>
              <View
                style={{
                  padding: 16,
                  borderRadius: 14,
                  backgroundColor: pal.sunk,
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: 10,
                  borderWidth: 1,
                  borderColor: pal.hairline,
                }}
              >
                <Spinner />
                <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.muted }}>
                  Разбираем, что вы ели…
                </Text>
              </View>
            </View>
          )}
        </ScrollView>

        {/* Footer Save button — sticks to bottom */}
        <LinearGradient
          colors={['rgba(246,241,234,0)', pal.bg, pal.bg]}
          locations={[0, 0.4, 1]}
          style={{
            position: 'absolute',
            left: 0,
            right: 0,
            bottom: 0,
            paddingTop: 14,
            paddingHorizontal: 16,
            paddingBottom: Math.max(28, insets.bottom + 14),
          }}
        >
          <Press
            disabled={!canSave}
            onPress={() =>
              onComplete({
                mealName,
                items,
                totals,
                time: '08:42',
              })
            }
            style={{
              width: '100%',
              paddingVertical: 15,
              paddingHorizontal: 20,
              borderRadius: 999,
              backgroundColor: canSave ? C.forest700 : pal.sunk,
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 8,
            }}
          >
            {canSave ? (
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>
                Сохранить ·{' '}
                <Text style={{ fontFamily: F.mono, fontVariant: ['tabular-nums'] }}>
                  {totals.kcal}
                </Text>{' '}
                ккал
              </Text>
            ) : (
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.subtle }}>
                Добавьте что-нибудь
              </Text>
            )}
          </Press>
        </LinearGradient>
      </View>
    </KeyboardAvoidingView>
  );
}

// ────────────────────────────────────────────────────────────────────
// MODE: CHAT
// ────────────────────────────────────────────────────────────────────

function ChatInput({
  text,
  setText,
  stage,
  onParse,
  sampleIdx,
  onCycleSample,
}: {
  text: string;
  setText: (t: string) => void;
  stage: Stage;
  onParse: (txt: string) => void;
  sampleIdx: number;
  onCycleSample: () => void;
}) {
  const ph = SAMPLE_PARSES[sampleIdx].placeholder;
  const hasText = text.trim().length > 0;
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 18,
        padding: 14,
        borderWidth: 1,
        borderColor: pal.hairline,
        ...SHADOW.xs,
      }}
    >
      <TextInput
        value={text}
        onChangeText={setText}
        placeholder={ph}
        placeholderTextColor={pal.placeholder}
        multiline
        style={{
          minHeight: 66,
          textAlignVertical: 'top',
          fontFamily: F.body,
          fontSize: 15,
          color: pal.ink,
          lineHeight: 15 * 1.5,
          padding: 0,
        }}
      />
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginTop: 10,
          gap: 8,
        }}
      >
        <Press
          onPress={onCycleSample}
          style={{
            paddingVertical: 8,
            paddingHorizontal: 12,
            borderRadius: 999,
            backgroundColor: 'transparent',
            borderWidth: 1,
            borderColor: pal.border,
            flexDirection: 'row',
            alignItems: 'center',
            gap: 5,
          }}
        >
          <Icon name="sparkles" size={14} color={pal.muted} />
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: pal.muted }}>
            Пример
          </Text>
        </Press>
        <Press
          onPress={() => onParse(text || SAMPLE_PARSES[0].transcript)}
          disabled={stage === 'parsing'}
          style={{
            paddingVertical: 8,
            paddingHorizontal: 16,
            borderRadius: 999,
            backgroundColor: hasText ? C.sand500 : pal.sunk,
            flexDirection: 'row',
            alignItems: 'center',
            gap: 6,
          }}
        >
          <Text
            style={{
              fontFamily: F.bodyMedium,
              fontSize: 13,
              color: hasText ? C.ink900 : pal.subtle,
            }}
          >
            Разобрать
          </Text>
          <Text
            style={{
              fontFamily: F.displayItalic,
              fontSize: 13,
              color: hasText ? C.ink900 : pal.subtle,
            }}
          >
            →
          </Text>
        </Press>
      </View>
    </View>
  );
}

// ────────────────────────────────────────────────────────────────────
// MODE: PHOTO
// ────────────────────────────────────────────────────────────────────

function PhotoInput({
  setStage,
  onParse,
}: {
  setStage: (s: Stage) => void;
  onParse: (samp: SampleParse) => void;
}) {
  const [captured, setCaptured] = useState(false);

  const onCapture = () => {
    setCaptured(true);
    setStage('parsing');
    setTimeout(() => onParse(SAMPLE_PARSES[1]), 1100); // photo always parses lunch bowl
  };

  if (!captured) {
    return (
      <Press
        onPress={onCapture}
        style={{
          width: '100%',
          aspectRatio: 4 / 3,
          backgroundColor: pal.sunk,
          borderWidth: 1.5,
          borderStyle: 'dashed',
          borderColor: pal.border,
          borderRadius: 18,
          alignItems: 'center',
          justifyContent: 'center',
          gap: 10,
        }}
      >
        <View
          style={{
            width: 56,
            height: 56,
            borderRadius: 999,
            backgroundColor: C.sand300,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name="camera" size={26} color={C.sand700} />
        </View>
        <Text
          style={{
            fontFamily: F.display,
            fontSize: 20,
            color: pal.ink,
            letterSpacing: 20 * -0.012,
          }}
        >
          Снимок{' '}
          <Text style={{ fontFamily: F.displayItalic, color: C.sand700 }}>тарелки</Text>
        </Text>
        <Text
          style={{
            fontFamily: F.body,
            fontSize: 12,
            color: pal.subtle,
            textAlign: 'center',
            maxWidth: 240,
          }}
        >
          Нажмите — мы распознаем, что на ней
        </Text>
      </Press>
    );
  }

  // Captured: show a soft sand placeholder
  return (
    <LinearGradient
      colors={[C.sand500, C.sand700]}
      start={{ x: 0, y: 0 }}
      end={{ x: 1, y: 1 }}
      style={{
        width: '100%',
        aspectRatio: 16 / 9,
        borderRadius: 18,
        overflow: 'hidden',
        borderWidth: 1,
        borderColor: pal.hairline,
        alignItems: 'center',
        justifyContent: 'center',
        gap: 8,
      }}
    >
      <Icon name="camera" size={28} color={C.cream} />
      <Text
        style={{
          fontFamily: F.body,
          fontSize: 12,
          color: C.cream,
          opacity: 0.85,
          letterSpacing: 12 * 0.04,
        }}
      >
        ФОТО СДЕЛАНО
      </Text>
    </LinearGradient>
  );
}

// ────────────────────────────────────────────────────────────────────
// MODE: VOICE
// ────────────────────────────────────────────────────────────────────

function VoiceInput({
  setStage,
  onParse,
}: {
  setStage: (s: Stage) => void;
  onParse: (samp: SampleParse) => void;
}) {
  const [holding, setHolding] = useState(false);
  const [held, setHeld] = useState(false); // has been released once

  const onStart = () => {
    if (held) return;
    setHolding(true);
    setStage('capturing');
  };
  const onEnd = () => {
    if (!holding) return;
    setHolding(false);
    setHeld(true);
    setStage('parsing');
    setTimeout(() => onParse(SAMPLE_PARSES[2]), 1100);
  };

  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 18,
        padding: 22,
        borderWidth: 1,
        borderColor: pal.hairline,
        alignItems: 'center',
        gap: 14,
      }}
    >
      <Text
        style={{ fontFamily: F.body, fontSize: 12.5, color: pal.muted, textAlign: 'center' }}
      >
        {held
          ? 'Поняли — секунду.'
          : holding
            ? 'Слушаем… отпустите, когда закончите.'
            : 'Зажмите кнопку и говорите.'}
      </Text>

      {/* Halo wrapper — emulates the 14px soft ring while holding */}
      <View style={{ width: 124, height: 124, alignItems: 'center', justifyContent: 'center' }}>
        {holding && (
          <View
            style={{
              position: 'absolute',
              width: 124,
              height: 124,
              borderRadius: 999,
              backgroundColor: 'rgba(45,95,63,.12)',
            }}
          />
        )}
        <Pressable
          onPressIn={onStart}
          onPressOut={onEnd}
          disabled={held}
          style={{
            width: 96,
            height: 96,
            borderRadius: 999,
            backgroundColor: holding ? C.forest800 : held ? pal.sunk : C.sand500,
            alignItems: 'center',
            justifyContent: 'center',
            ...(holding
              ? null
              : {
                  shadowColor: C.sand500,
                  shadowOpacity: 0.36,
                  shadowRadius: 14,
                  shadowOffset: { width: 0, height: 4 },
                  elevation: 6,
                }),
          }}
        >
          <MicIcon size={36} color={holding ? C.cream : held ? pal.subtle : C.ink900} />
        </Pressable>
      </View>

      {/* Waveform bars */}
      <View
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 4,
          height: 24,
        }}
      >
        {Array.from({ length: 9 }).map((_, i) => (
          <View
            key={i}
            style={{
              width: 3,
              borderRadius: 2,
              backgroundColor: holding ? C.forest700 : pal.bone,
              height: holding ? 8 + Math.abs(Math.sin((i + 1) * 1.3)) * 16 : 6,
            }}
          />
        ))}
      </View>
    </View>
  );
}

// ────────────────────────────────────────────────────────────────────
// PARSED ITEM ROW
// ────────────────────────────────────────────────────────────────────

function MacroBadge({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <View style={{ flexDirection: 'row', alignItems: 'center', gap: 3 }}>
      <View style={{ width: 5, height: 5, borderRadius: 999, backgroundColor: color }} />
      <Text
        style={{
          fontFamily: F.mono,
          fontSize: 10.5,
          color: pal.muted,
          fontVariant: ['tabular-nums'],
        }}
      >
        {label} <Text style={{ color: pal.ink2 }}>{value}</Text>
      </Text>
    </View>
  );
}

function ParsedItem({
  item,
  editing,
  onStartEdit,
  onChangeGrams,
  onDelete,
}: {
  item: MealItem;
  editing: boolean;
  onStartEdit: () => void;
  onChangeGrams: (g: number) => void;
  onDelete: () => void;
}) {
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 14,
        paddingVertical: 12,
        paddingHorizontal: 14,
        borderWidth: 1,
        borderColor: pal.hairline,
      }}
    >
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
        <View style={{ minWidth: 0, flex: 1 }}>
          <Text
            numberOfLines={1}
            style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}
          >
            {item.name}
          </Text>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 4 }}>
            <MacroBadge label="Белок" value={item.p + 'г'} color={C.forest700} />
            <MacroBadge label="Углеводы" value={item.c + 'г'} color={CARB} />
            <MacroBadge label="Жиры" value={item.f + 'г'} color={C.sand700} />
          </View>
        </View>
        <Press
          onPress={onStartEdit}
          style={{
            paddingVertical: 4,
            paddingHorizontal: 10,
            borderRadius: 999,
            backgroundColor: editing ? C.forest700 : pal.sunk,
          }}
        >
          <Text
            numberOfLines={1}
            style={{
              fontFamily: F.mono,
              fontSize: 12,
              color: editing ? C.cream : pal.ink2,
              fontVariant: ['tabular-nums'],
              letterSpacing: 12 * -0.01,
            }}
          >
            {item.grams} г
          </Text>
        </Press>
        <View style={{ alignItems: 'flex-end' }}>
          <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 2 }}>
            <Text
              style={{
                fontFamily: F.monoMedium,
                fontSize: 16,
                color: pal.ink,
                fontVariant: ['tabular-nums'],
                letterSpacing: 16 * -0.02,
              }}
            >
                          </Text>
            <Text style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle }}>ккал</Text>
          </View>
          <Press onPress={onDelete} style={{ marginTop: 2, padding: 4 }}>
            <Icon name="trash" size={14} color={pal.subtle} />
          </Press>
        </View>
      </View>

      {/* Inline grams stepper */}
      {editing && (
        <View
          style={{
            marginTop: 12,
            paddingTop: 12,
            borderTopWidth: 1,
            borderStyle: 'dashed',
            borderTopColor: pal.hairline,
            flexDirection: 'row',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
          }}
        >
          <Meta color={pal.subtle}>Поправить порцию</Meta>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
            <Press
              onPress={() => onChangeGrams(Math.max(5, item.grams - 10))}
              style={{
                width: 32,
                height: 32,
                borderRadius: 999,
                backgroundColor: pal.sunk,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Text style={{ fontFamily: F.body, fontSize: 18, color: pal.ink }}>−</Text>
            </Press>
            <Text
              style={{
                fontFamily: F.monoMedium,
                fontSize: 18,
                fontVariant: ['tabular-nums'],
                color: pal.ink,
                minWidth: 56,
                textAlign: 'center',
              }}
            >
              {item.grams} г
            </Text>
            <Press
              onPress={() => onChangeGrams(item.grams + 10)}
              style={{
                width: 32,
                height: 32,
                borderRadius: 999,
                backgroundColor: C.sand500,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Text style={{ fontFamily: F.body, fontSize: 18, color: C.ink900 }}>+</Text>
            </Press>
          </View>
        </View>
      )}
    </View>
  );
}

// ────────────────────────────────────────────────────────────────────
// MACRO TOTALS STRIP
// ────────────────────────────────────────────────────────────────────

function MacroStrip({ totals }: { totals: MealTotals }) {
  const t = MEAL_TARGETS;
  const rows = [
    { lbl: 'ккал', v: totals.kcal, goal: t.kcal, color: C.forest700 },
    { lbl: 'белок', v: Math.round(totals.p), goal: t.protein, color: C.forest700 },
    { lbl: 'углев', v: Math.round(totals.c), goal: t.carbs, color: CARB },
    { lbl: 'жиры', v: Math.round(totals.f), goal: t.fat, color: C.sand700 },
  ];
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 14,
        padding: 12,
        borderWidth: 1,
        borderColor: pal.hairline,
        flexDirection: 'row',
        gap: 10,
      }}
    >
      {rows.map((r, i) => (
        <View
          key={r.lbl}
          style={{
            flex: 1,
            borderLeftWidth: i === 0 ? 0 : 1,
            borderLeftColor: pal.hairline,
            paddingLeft: i === 0 ? 0 : 10,
          }}
        >
          <Eyebrow
            color={pal.subtle}
            style={{ marginBottom: 4, fontSize: 10, letterSpacing: 10 * 0.14 }}
          >
            {r.lbl}
          </Eyebrow>
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
            {r.v}
          </Text>
          <Text style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle, marginTop: 2 }}>
            / {r.goal}
            {r.lbl === 'ккал' ? '' : ' г'}
          </Text>
        </View>
      ))}
    </View>
  );
}

// ────────────────────────────────────────────────────────────────────
// SMALL BITS
// ────────────────────────────────────────────────────────────────────

// Parsing spinner — ring with a sand arc, rotating.
function Spinner() {
  const a = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    const loop = Animated.loop(
      Animated.timing(a, {
        toValue: 1,
        duration: 800,
        easing: Easing.linear,
        useNativeDriver: true,
      })
    );
    loop.start();
    return () => loop.stop();
  }, [a]);
  const rotate = a.interpolate({ inputRange: [0, 1], outputRange: ['0deg', '360deg'] });
  return (
    <Animated.View style={{ width: 14, height: 14, transform: [{ rotate }] }}>
      <Svg width={14} height={14} viewBox="0 0 14 14" fill="none">
        <Circle cx={7} cy={7} r={5.5} stroke={pal.bone} strokeWidth={2} />
        <Path
          d="M 7 1.5 A 5.5 5.5 0 0 1 12.5 7"
          stroke={C.sand700}
          strokeWidth={2}
          strokeLinecap="round"
        />
      </Svg>
    </Animated.View>
  );
}

// Microphone glyph — not in the Heroicons subset; ported from MealModeIcon.
function MicIcon({ size, color }: { size: number; color: string }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Path
        d="M12 18.75v3M12 18.75a6 6 0 0 0 6-6V9a6 6 0 1 0-12 0v3.75a6 6 0 0 0 6 6Z M12 4.5v8.25"
        stroke={color}
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Svg>
  );
}

function MealModeIcon({
  name,
  color,
}: {
  name: 'paper-airplane' | 'camera' | 'microphone';
  color: string;
}) {
  if (name === 'microphone') return <MicIcon size={14} color={color} />;
  return <Icon name={name} size={14} color={color} />;
}
