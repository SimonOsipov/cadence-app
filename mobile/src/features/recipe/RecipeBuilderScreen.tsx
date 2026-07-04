// Cadence · Рецепты — Builder (новый рецепт) + ingredient picker.
// Ported from design_handoff_cadence_app/recipe/recipe-builder.jsx.
import React, { useState } from 'react';
import { ScrollView, Text, TextInput, View } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Eyebrow, Press } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { Sheet } from '../../components/shared';
import {
  RECIPES,
  Recipe,
  RecipeIngredient,
  Macros,
} from './data';

const MACRO_C = { p: C.forest700, c: C.sand500, f: C.sand700 } as const;

// ── Macro split bar (by kcal contribution) — dark-card variant ──
function MacroBarDark({ macro }: { macro: Macros }) {
  const kp = macro.p * 4;
  const kc = macro.c * 4;
  const kf = macro.f * 9;
  const tot = kp + kc + kf || 1;
  const seg: [keyof typeof MACRO_C, number][] = [
    ['p', kp],
    ['c', kc],
    ['f', kf],
  ];
  return (
    <View>
      <View
        style={{
          flexDirection: 'row',
          height: 8,
          borderRadius: 999,
          overflow: 'hidden',
          backgroundColor: 'rgba(246,241,234,.14)',
        }}
      >
        {seg.map(([k, v]) => (
          <View key={k} style={{ width: `${(v / tot) * 100}%`, backgroundColor: MACRO_C[k] }} />
        ))}
      </View>
      <View style={{ flexDirection: 'row', gap: 16, marginTop: 9 }}>
        {(
          [
            ['p', 'Белок'],
            ['c', 'Углев'],
            ['f', 'Жиры'],
          ] as [keyof typeof MACRO_C, string][]
        ).map(([k, lbl]) => (
          <View key={k} style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
            <View style={{ width: 8, height: 8, borderRadius: 3, backgroundColor: MACRO_C[k] }} />
            <Text style={{ fontFamily: F.body, fontSize: 11.5, color: 'rgba(246,241,234,.6)' }}>
              {lbl}
            </Text>
            <Text
              style={{
                fontFamily: F.mono,
                fontSize: 12.5,
                color: C.cream,
                fontVariant: ['tabular-nums'],
              }}
            >
              {RECIPES.round(macro[k])} г
            </Text>
          </View>
        ))}
      </View>
    </View>
  );
}

// Round stepper glyph button (− / +).
function StepBtn({
  glyph,
  onPress,
  size = 34,
  fontSize = 18,
  bg = 'transparent',
  color = pal.ink,
  borderColor,
}: {
  glyph: string;
  onPress: () => void;
  size?: number;
  fontSize?: number;
  bg?: string;
  color?: string;
  borderColor?: string;
}) {
  return (
    <Press
      onPress={onPress}
      style={{
        width: size,
        height: size,
        borderRadius: 999,
        backgroundColor: bg,
        borderWidth: borderColor ? 1 : 0,
        borderColor,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Text style={{ fontFamily: F.body, fontSize, color, lineHeight: fontSize * 1.15 }}>
        {glyph}
      </Text>
    </Press>
  );
}

// ── Ingredient picker sheet content ──────────────────────────
function IngredientPicker({
  onClose,
  onAdd,
}: {
  onClose: () => void;
  onAdd: (id: string, g: number) => void;
}) {
  const insets = useSafeAreaInsets();
  const [q, setQ] = useState('');
  const [sel, setSel] = useState<string | null>(null);
  const [grams, setGrams] = useState(100);
  const list = RECIPES.ING.filter(i => i.name.toLowerCase().includes(q.trim().toLowerCase()));

  const pick = (id: string) => {
    setSel(id);
    setGrams(100);
  };
  const selMeta = sel ? RECIPES.ingMeta(sel) : null;
  const m = sel ? RECIPES.ingMacros(sel, grams) : null;

  return (
    <View style={{ flex: 1 }}>
      {/* header */}
      <View
        style={{
          paddingTop: 4,
          paddingHorizontal: 20,
          paddingBottom: 10,
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'center',
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
          Ингредиент
        </Text>
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
      {/* search */}
      <View style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
        <View
          style={{
            flexDirection: 'row',
            alignItems: 'center',
            gap: 10,
            backgroundColor: pal.sunk,
            borderRadius: 14,
            paddingVertical: 11,
            paddingHorizontal: 14,
          }}
        >
          <Icon name="magnifying-glass" size={18} color={pal.subtle} />
          <TextInput
            value={q}
            onChangeText={setQ}
            placeholder="Найти продукт"
            placeholderTextColor={pal.placeholder}
            style={{
              flex: 1,
              padding: 0,
              fontFamily: F.body,
              fontSize: 14,
              color: pal.ink,
            }}
          />
        </View>
      </View>
      {/* list */}
      <ScrollView
        style={{ flex: 1, paddingHorizontal: 16 }}
        keyboardShouldPersistTaps="handled"
      >
        {list.map(i => {
          const on = sel === i.id;
          return (
            <Press
              key={i.id}
              onPress={() => pick(i.id)}
              style={{
                flexDirection: 'row',
                gap: 10,
                alignItems: 'center',
                paddingVertical: 12,
                paddingHorizontal: 12,
                borderRadius: 12,
                marginBottom: 4,
                backgroundColor: on ? C.forest50 : 'transparent',
              }}
            >
              <View style={{ flex: 1 }}>
                <Text
                  style={{
                    fontFamily: on ? F.bodySemibold : F.body,
                    fontSize: 14,
                    color: on ? C.forest800 : pal.ink,
                  }}
                >
                  {i.name}
                </Text>
                <Text
                  style={{
                    fontFamily: F.mono,
                    fontSize: 11,
                    color: pal.subtle,
                    marginTop: 1,
                    fontVariant: ['tabular-nums'],
                  }}
                >
                  {i.kcal} ккал · {i.p} г белка / 100 г
                </Text>
              </View>
              {on && <Icon name="check-circle" size={20} color={C.forest700} />}
            </Press>
          );
        })}
        {list.length === 0 && (
          <Text
            style={{
              fontFamily: F.body,
              fontSize: 13,
              color: pal.subtle,
              paddingVertical: 12,
              paddingHorizontal: 4,
            }}
          >
            Ничего не найдено.
          </Text>
        )}
      </ScrollView>
      {/* footer grams + add */}
      {sel && selMeta && m && (
        <View
          style={{
            paddingHorizontal: 16,
            paddingTop: 12,
            paddingBottom: Math.max(insets.bottom, 26),
            borderTopWidth: 1,
            borderTopColor: pal.hairline,
          }}
        >
          <View
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'space-between',
              marginBottom: 12,
            }}
          >
            <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.muted }}>
              {selMeta.name}
            </Text>
            <Text
              style={{
                fontFamily: F.mono,
                fontSize: 13,
                color: C.forest700,
                fontVariant: ['tabular-nums'],
              }}
            >
              {RECIPES.round(m.kcal)} ккал · {RECIPES.round1(m.p)} г белка
            </Text>
          </View>
          <View style={{ flexDirection: 'row', gap: 10, alignItems: 'center' }}>
            <View
              style={{
                flexDirection: 'row',
                alignItems: 'center',
                backgroundColor: pal.paper,
                borderWidth: 1,
                borderColor: pal.border,
                borderRadius: 999,
                padding: 3,
              }}
            >
              <StepBtn
                glyph="−"
                fontSize={20}
                size={40}
                onPress={() => setGrams(g => Math.max(5, g - 10))}
              />
              <Text
                style={{
                  minWidth: 64,
                  textAlign: 'center',
                  fontFamily: F.mono,
                  fontSize: 16,
                  color: pal.ink,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {grams} г
              </Text>
              <StepBtn
                glyph="+"
                fontSize={20}
                size={40}
                onPress={() => setGrams(g => Math.min(600, g + 10))}
              />
            </View>
            <Press
              onPress={() => onAdd(sel, grams)}
              style={{
                flex: 1,
                paddingVertical: 14,
                borderRadius: 999,
                backgroundColor: C.forest700,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>
                Добавить
              </Text>
            </Press>
          </View>
        </View>
      )}
    </View>
  );
}

// ════════════════════════════════════════════════════════════════
// Builder screen
// ════════════════════════════════════════════════════════════════
export function RecipeBuilderScreen({
  onCancel,
  onSave,
}: {
  onCancel: () => void;
  onSave: (recipe: any) => void;
}) {
  const insets = useSafeAreaInsets();
  const [name, setName] = useState('');
  const [mealType, setMealType] = useState('lunch');
  const [tags, setTags] = useState<string[]>([]);
  const [servings, setServings] = useState(2);
  const [time, setTime] = useState(20);
  const [ings, setIngs] = useState<RecipeIngredient[]>([]);
  const [steps, setSteps] = useState<string[]>(['']);
  const [pickerOpen, setPickerOpen] = useState(false);

  const draft = { ingredients: ings, servings } as Recipe;
  const ps = RECIPES.perServing(draft);
  const canSave = !!name.trim() && ings.length > 0;

  const toggleTag = (id: string) =>
    setTags(t => (t.includes(id) ? t.filter(x => x !== id) : [...t, id]));
  const setGrams = (idx: number, g: number) =>
    setIngs(arr => arr.map((it, i) => (i === idx ? { ...it, g: Math.max(5, g) } : it)));
  const removeIng = (idx: number) => setIngs(arr => arr.filter((_, i) => i !== idx));
  const setStep = (idx: number, v: string) =>
    setSteps(arr => arr.map((s, i) => (i === idx ? v : s)));
  const addStep = () => setSteps(arr => [...arr, '']);
  const removeStep = (idx: number) =>
    setSteps(arr => (arr.length > 1 ? arr.filter((_, i) => i !== idx) : arr));

  const save = () => {
    if (!canSave) return;
    onSave({
      id: `u-${Date.now()}`,
      name: name.trim(),
      mealType,
      tags,
      servings,
      prepMin: time,
      cookMin: 0,
      photo: false,
      mine: true,
      dek: `${RECIPES.round(ps.p)} г белка · ${RECIPES.round(ps.kcal)} ккал на порцию.`,
      ingredients: ings.map(it => ({ id: it.id, g: it.g })),
      steps: steps.map(s => s.trim()).filter(Boolean),
    });
  };

  const sectionHead = (label: string, action: string, onAction: () => void) => (
    <View
      style={{
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'baseline',
        paddingHorizontal: 4,
        paddingBottom: 10,
      }}
    >
      <Eyebrow color={pal.subtle}>{label}</Eyebrow>
      <Press
        onPress={onAction}
        style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}
      >
        <Icon name="plus" size={13} strokeWidth={2} color={C.forest700} />
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
          {action}
        </Text>
      </Press>
    </View>
  );

  const steppers: {
    lbl: string;
    val: number;
    set: React.Dispatch<React.SetStateAction<number>>;
    min: number;
    max: number;
    step: number;
  }[] = [
    { lbl: 'Порций', val: servings, set: setServings, min: 1, max: 8, step: 1 },
    { lbl: 'Время, мин', val: time, set: setTime, min: 5, max: 120, step: 5 },
  ];

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
            <Icon name="x-mark" size={20} color={pal.ink2} />
          </Press>
          <Text style={{ fontFamily: F.bodySemibold, fontSize: 15, color: pal.ink }}>
            Новый рецепт
          </Text>
        </View>
      </View>

      <ScrollView
        style={{ flex: 1 }}
        contentContainerStyle={{ paddingBottom: 140 }}
        keyboardShouldPersistTaps="handled"
      >
        {/* Name */}
        <View style={{ paddingTop: 8, paddingHorizontal: 16, paddingBottom: 18 }}>
          <TextInput
            value={name}
            onChangeText={setName}
            placeholder="Название рецепта"
            placeholderTextColor={pal.placeholder}
            style={{
              borderBottomWidth: 1,
              borderBottomColor: pal.border,
              fontFamily: F.display,
              fontSize: 28,
              color: pal.ink,
              letterSpacing: 28 * -0.018,
              paddingHorizontal: 4,
              paddingTop: 0,
              paddingBottom: 10,
            }}
          />
        </View>

        {/* Meal type + tags */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
          <Eyebrow
            color={pal.subtle}
            style={{ paddingHorizontal: 4, paddingBottom: 10 }}
          >
            Тип приёма
          </Eyebrow>
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ gap: 8, paddingBottom: 4 }}
          >
            {RECIPES.MEAL_TYPES.map(t => {
              const on = mealType === t.id;
              return (
                <Press
                  key={t.id}
                  onPress={() => setMealType(t.id)}
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
                    {t.label}
                  </Text>
                </Press>
              );
            })}
          </ScrollView>
          <View style={{ height: 10 }} />
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {RECIPES.TAGS.map(t => {
              const on = tags.includes(t.id);
              return (
                <Press
                  key={t.id}
                  onPress={() => toggleTag(t.id)}
                  style={{
                    paddingVertical: 8,
                    paddingHorizontal: 14,
                    borderRadius: 999,
                    backgroundColor: on ? C.sand500 : 'transparent',
                    borderWidth: 1,
                    borderColor: on ? C.sand500 : pal.border,
                  }}
                >
                  <Text
                    style={{
                      fontFamily: F.bodyMedium,
                      fontSize: 13,
                      color: on ? C.ink900 : pal.ink2,
                    }}
                  >
                    {t.label}
                  </Text>
                </Press>
              );
            })}
          </View>
        </View>

        {/* Servings + time */}
        <View
          style={{
            paddingHorizontal: 16,
            paddingBottom: 18,
            flexDirection: 'row',
            gap: 10,
          }}
        >
          {steppers.map((row, i) => (
            <View
              key={i}
              style={{
                flex: 1,
                backgroundColor: pal.paper,
                borderWidth: 1,
                borderColor: pal.hairline,
                borderRadius: 16,
                paddingVertical: 12,
                paddingHorizontal: 14,
              }}
            >
              <Text
                style={{
                  fontFamily: F.body,
                  fontSize: 11.5,
                  color: pal.subtle,
                  marginBottom: 8,
                }}
              >
                {row.lbl}
              </Text>
              <View
                style={{
                  flexDirection: 'row',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                }}
              >
                <StepBtn
                  glyph="−"
                  borderColor={pal.border}
                  onPress={() => row.set(v => Math.max(row.min, v - row.step))}
                />
                <Text
                  style={{
                    fontFamily: F.mono,
                    fontSize: 20,
                    color: pal.ink,
                    fontVariant: ['tabular-nums'],
                  }}
                >
                  {row.val}
                </Text>
                <StepBtn
                  glyph="+"
                  bg={C.forest700}
                  color={C.cream}
                  onPress={() => row.set(v => Math.min(row.max, v + row.step))}
                />
              </View>
            </View>
          ))}
        </View>

        {/* Ingredients */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
          {sectionHead('Ингредиенты', 'Добавить', () => setPickerOpen(true))}
          {ings.length === 0 ? (
            <Press
              onPress={() => setPickerOpen(true)}
              style={{
                padding: 20,
                borderRadius: 16,
                borderWidth: 1.5,
                borderStyle: 'dashed',
                borderColor: pal.border,
                flexDirection: 'row',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 8,
              }}
            >
              <Icon name="plus" size={18} color={pal.subtle} />
              <Text style={{ fontFamily: F.body, fontSize: 13.5, color: pal.subtle }}>
                Добавьте первый ингредиент
              </Text>
            </Press>
          ) : (
            <View
              style={{
                backgroundColor: pal.paper,
                borderRadius: 16,
                borderWidth: 1,
                borderColor: pal.hairline,
                overflow: 'hidden',
              }}
            >
              {ings.map((it, idx) => {
                const meta = RECIPES.ingMeta(it.id);
                const m = RECIPES.ingMacros(it.id, it.g);
                return (
                  <View
                    key={idx}
                    style={{
                      flexDirection: 'row',
                      gap: 10,
                      alignItems: 'center',
                      paddingVertical: 10,
                      paddingHorizontal: 12,
                      borderBottomWidth: idx < ings.length - 1 ? 1 : 0,
                      borderBottomColor: pal.hairline,
                    }}
                  >
                    <View style={{ flex: 1, minWidth: 0 }}>
                      <Text
                        numberOfLines={1}
                        style={{ fontFamily: F.body, fontSize: 13.5, color: pal.ink }}
                      >
                        {meta ? meta.name : it.id}
                      </Text>
                      <Text
                        style={{
                          fontFamily: F.mono,
                          fontSize: 11,
                          color: pal.subtle,
                          fontVariant: ['tabular-nums'],
                        }}
                      >
                        {RECIPES.round(m.kcal)} ккал · {RECIPES.round1(m.p)} б
                      </Text>
                    </View>
                    <View
                      style={{
                        flexDirection: 'row',
                        alignItems: 'center',
                        backgroundColor: pal.sunk,
                        borderRadius: 999,
                        padding: 2,
                      }}
                    >
                      <StepBtn
                        glyph="−"
                        size={28}
                        fontSize={16}
                        color={pal.ink2}
                        onPress={() => setGrams(idx, it.g - 10)}
                      />
                      <Text
                        style={{
                          minWidth: 44,
                          textAlign: 'center',
                          fontFamily: F.mono,
                          fontSize: 12.5,
                          color: pal.ink,
                          fontVariant: ['tabular-nums'],
                        }}
                      >
                        {it.g} г
                      </Text>
                      <StepBtn
                        glyph="+"
                        size={28}
                        fontSize={16}
                        color={pal.ink2}
                        onPress={() => setGrams(idx, it.g + 10)}
                      />
                    </View>
                    <Press
                      onPress={() => removeIng(idx)}
                      style={{
                        width: 30,
                        height: 30,
                        borderRadius: 999,
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >
                      <Icon name="x-mark" size={15} color={pal.placeholder} />
                    </Press>
                  </View>
                );
              })}
            </View>
          )}

          {/* live macro total */}
          {ings.length > 0 && (
            <View
              style={{
                marginTop: 10,
                backgroundColor: C.forest800,
                borderRadius: 16,
                padding: 14,
              }}
            >
              <View
                style={{
                  flexDirection: 'row',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 10,
                }}
              >
                <Text
                  style={{
                    fontFamily: F.bodySemibold,
                    fontSize: 11,
                    letterSpacing: 11 * 0.1,
                    textTransform: 'uppercase',
                    color: C.sand300,
                  }}
                >
                  На порцию
                </Text>
                <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 4 }}>
                  <Text
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 22,
                      color: C.cream,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {RECIPES.round(ps.kcal)}
                  </Text>
                  <Text
                    style={{ fontFamily: F.displayItalic, fontSize: 14, color: C.sand300 }}
                  >
                    ккал
                  </Text>
                </View>
              </View>
              <MacroBarDark macro={ps} />
            </View>
          )}
        </View>

        {/* Steps */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 8 }}>
          {sectionHead('Приготовление', 'Шаг', addStep)}
          <View style={{ gap: 10 }}>
            {steps.map((s, idx) => (
              <View
                key={idx}
                style={{ flexDirection: 'row', gap: 10, alignItems: 'flex-start' }}
              >
                <View
                  style={{
                    width: 30,
                    height: 30,
                    borderRadius: 999,
                    backgroundColor: C.forest50,
                    alignItems: 'center',
                    justifyContent: 'center',
                    marginTop: 4,
                  }}
                >
                  <Text
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 14,
                      color: C.forest700,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {idx + 1}
                  </Text>
                </View>
                <TextInput
                  value={s}
                  onChangeText={v => setStep(idx, v)}
                  multiline
                  placeholder="Опишите шаг"
                  placeholderTextColor={pal.placeholder}
                  style={{
                    flex: 1,
                    minHeight: 62,
                    borderWidth: 1,
                    borderColor: pal.border,
                    borderRadius: 12,
                    paddingVertical: 10,
                    paddingHorizontal: 12,
                    backgroundColor: pal.paper,
                    fontFamily: F.body,
                    fontSize: 14,
                    color: pal.ink,
                    lineHeight: 14 * 1.45,
                    textAlignVertical: 'top',
                  }}
                />
                {steps.length > 1 && (
                  <Press
                    onPress={() => removeStep(idx)}
                    style={{
                      width: 30,
                      height: 30,
                      borderRadius: 999,
                      marginTop: 6,
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <Icon name="x-mark" size={15} color={pal.placeholder} />
                  </Press>
                )}
              </View>
            ))}
          </View>
        </View>
      </ScrollView>

      {/* Save */}
      <View style={{ position: 'absolute', left: 0, right: 0, bottom: 0 }}>
        <LinearGradient
          colors={['rgba(246,241,234,0)', C.cream]}
          locations={[0, 0.3]}
          style={{
            paddingTop: 12,
            paddingHorizontal: 16,
            paddingBottom: Math.max(insets.bottom, 28),
          }}
        >
          <Press
            onPress={save}
            disabled={!canSave}
            style={{
              paddingVertical: 15,
              paddingHorizontal: 20,
              borderRadius: 999,
              backgroundColor: C.forest700,
              opacity: canSave ? 1 : 0.4,
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 8,
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 15, color: C.cream }}>
              Сохранить рецепт
            </Text>
            <Icon name="check" size={16} strokeWidth={2} color={C.cream} />
          </Press>
        </LinearGradient>
      </View>

      <Sheet
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        contentStyle={{ height: '78%', paddingHorizontal: 0, paddingBottom: 0 }}
      >
        <IngredientPicker
          onClose={() => setPickerOpen(false)}
          onAdd={(id, g) => {
            setIngs(arr => [...arr, { id, g }]);
            setPickerOpen(false);
          }}
        />
      </Sheet>
    </View>
  );
}
