// Cadence · Рецепт — detail screen with macros, ingredients, steps and "add to day".
// Ported from design_handoff_cadence_app/recipe/recipe.jsx (RecipeDetail).
import React, { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { LinearGradient } from 'expo-linear-gradient';
import { C, F, pal } from '../../theme';
import { Eyebrow, Press } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { useAppState } from '../../state/AppState';
import { RECIPES, Recipe, Macros } from './data';
import { TYPE_TINT } from './RecipesScreen';

const MACRO_C: Record<string, string> = { p: C.forest700, c: C.sand500, f: C.sand700 };

// Macro split bar (by kcal contribution).
function RecipeMacroBar({ macro }: { macro: Macros }) {
  const kp = macro.p * 4;
  const kc = macro.c * 4;
  const kf = macro.f * 9;
  const tot = kp + kc + kf || 1;
  const seg: [string, number][] = [
    ['p', kp],
    ['c', kc],
    ['f', kf],
  ];
  const legend: [keyof Macros, string][] = [
    ['p', 'Белок'],
    ['c', 'Углев'],
    ['f', 'Жиры'],
  ];
  return (
    <View>
      <View
        style={{
          flexDirection: 'row',
          height: 8,
          borderRadius: 999,
          overflow: 'hidden',
          backgroundColor: pal.sunk,
        }}
      >
        {seg.map(([k, v]) => (
          <View key={k} style={{ flex: v / tot, backgroundColor: MACRO_C[k] }} />
        ))}
      </View>
      <View style={{ flexDirection: 'row', gap: 16, marginTop: 9 }}>
        {legend.map(([k, lbl]) => (
          <View key={k} style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
            <View style={{ width: 8, height: 8, borderRadius: 3, backgroundColor: MACRO_C[k] }} />
            <Text style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>{lbl}</Text>
            <Text
              style={{
                fontFamily: F.mono,
                fontSize: 12.5,
                color: pal.ink2,
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

export function RecipeDetailScreen({
  recipeId,
  onBack,
  onAddToDay,
}: {
  recipeId: string;
  onBack: () => void;
  onAddToDay: (recipe: any, portions: number) => void;
}) {
  const insets = useSafeAreaInsets();
  const { allRecipes } = useAppState();
  const [mode, setMode] = useState<'serving' | 'whole'>('serving');
  const [portions, setPortions] = useState(1);

  const recipe: (Recipe & { mine?: boolean }) | undefined = allRecipes.find(
    (r: any) => r.id === recipeId,
  );

  if (!recipe) {
    return (
      <View style={{ flex: 1, backgroundColor: pal.bg, paddingTop: insets.top + 6 }}>
        <View style={{ paddingHorizontal: 12 }}>
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
        </View>
        <Text
          style={{
            fontFamily: F.body,
            fontSize: 13,
            color: pal.subtle,
            paddingHorizontal: 20,
            paddingTop: 16,
          }}
        >
          Рецепт не найден.
        </Text>
      </View>
    );
  }

  const tint = TYPE_TINT[recipe.mealType] || TYPE_TINT.lunch;
  const whole = RECIPES.recipeTotals(recipe);
  const ps = RECIPES.perServing(recipe);
  const macro = mode === 'serving' ? ps : whole;

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <ScrollView contentContainerStyle={{ paddingBottom: insets.bottom + 110 }}>
        {/* Hero */}
        <View
          style={{
            backgroundColor: tint.soft,
            paddingTop: insets.top + 52,
            paddingHorizontal: 20,
            paddingBottom: 24,
          }}
        >
          <Text
            style={{
              fontFamily: F.bodySemibold,
              fontSize: 11,
              letterSpacing: 11 * 0.1,
              textTransform: 'uppercase',
              color: tint.fg,
              marginBottom: 12,
            }}
          >
            {RECIPES.typeLabel(recipe.mealType)}
            {recipe.mine ? ' · моё' : ''}
          </Text>
          <Text
            style={{
              fontFamily: F.display,
              fontSize: 34,
              color: C.ink900,
              lineHeight: 34 * 1.06,
              letterSpacing: 34 * -0.02,
            }}
          >
            {recipe.name}
          </Text>
          <Text
            style={{
              fontFamily: F.body,
              fontSize: 14.5,
              color: C.ink600,
              lineHeight: 14.5 * 1.5,
              marginTop: 10,
            }}
          >
            {recipe.dek}
          </Text>
          <View
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              gap: 16,
              marginTop: 16,
              flexWrap: 'wrap',
            }}
          >
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
              <Icon name="clock" size={15} color={tint.fg} />
              <Text style={{ fontFamily: F.body, fontSize: 12.5, color: tint.fg }}>
                {RECIPES.totalTime(recipe)} мин
              </Text>
            </View>
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
              <Icon name="cake" size={15} color={tint.fg} />
              <Text style={{ fontFamily: F.body, fontSize: 12.5, color: tint.fg }}>
                {recipe.servings} порц.
              </Text>
            </View>
            {(recipe.tags || []).map((t) => (
              <View
                key={t}
                style={{
                  backgroundColor: 'rgba(255,255,255,.5)',
                  borderRadius: 999,
                  paddingVertical: 3,
                  paddingHorizontal: 10,
                }}
              >
                <Text style={{ fontFamily: F.body, fontSize: 11.5, color: tint.fg }}>
                  {RECIPES.tagLabel(t)}
                </Text>
              </View>
            ))}
          </View>
        </View>

        {/* Macros */}
        <View style={{ paddingTop: 16, paddingHorizontal: 16, paddingBottom: 6 }}>
          <View
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
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 14,
              }}
            >
              <Eyebrow color={pal.subtle}>Макросы</Eyebrow>
              <View
                style={{
                  flexDirection: 'row',
                  gap: 3,
                  backgroundColor: pal.sunk,
                  padding: 3,
                  borderRadius: 999,
                }}
              >
                {(
                  [
                    ['serving', 'На порцию'],
                    ['whole', 'Всё'],
                  ] as const
                ).map(([id, lbl]) => (
                  <Press
                    key={id}
                    onPress={() => setMode(id)}
                    style={{
                      paddingVertical: 6,
                      paddingHorizontal: 12,
                      borderRadius: 999,
                      backgroundColor: mode === id ? C.forest700 : 'transparent',
                    }}
                  >
                    <Text
                      style={{
                        fontFamily: F.bodyMedium,
                        fontSize: 12,
                        color: mode === id ? C.cream : pal.muted,
                      }}
                    >
                      {lbl}
                    </Text>
                  </Press>
                ))}
              </View>
            </View>
            <View
              style={{ flexDirection: 'row', alignItems: 'baseline', gap: 6, marginBottom: 14 }}
            >
              <Text
                style={{
                  fontFamily: F.monoMedium,
                  fontSize: 34,
                  color: pal.ink,
                  letterSpacing: 34 * -0.03,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {RECIPES.round(macro.kcal)}
              </Text>
              <Text style={{ fontFamily: F.displayItalic, fontSize: 16, color: pal.muted }}>
                ккал
              </Text>
              <Text
                style={{ marginLeft: 'auto', fontFamily: F.body, fontSize: 12, color: pal.subtle }}
              >
                {mode === 'serving' ? '1 порция' : `${recipe.servings} порц.`}
              </Text>
            </View>
            <RecipeMacroBar macro={macro} />
          </View>
        </View>

        {/* Ingredients */}
        <View style={{ paddingTop: 10, paddingHorizontal: 16, paddingBottom: 6 }}>
          <Eyebrow color={pal.subtle} style={{ paddingHorizontal: 4, paddingBottom: 8 }}>
            Ингредиенты · {recipe.servings} порц.
          </Eyebrow>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
              overflow: 'hidden',
            }}
          >
            {recipe.ingredients.map((it, i) => {
              const m = RECIPES.ingMacros(it.id, it.g);
              const meta = RECIPES.ingMeta(it.id);
              return (
                <View
                  key={i}
                  style={{
                    flexDirection: 'row',
                    alignItems: 'center',
                    gap: 12,
                    paddingVertical: 12,
                    paddingHorizontal: 14,
                    borderBottomWidth: i < recipe.ingredients.length - 1 ? 1 : 0,
                    borderBottomColor: pal.hairline,
                  }}
                >
                  <Text style={{ flex: 1, fontFamily: F.body, fontSize: 14, color: pal.ink }}>
                    {meta ? meta.name : it.id}
                  </Text>
                  <Text
                    style={{
                      fontFamily: F.mono,
                      fontSize: 13,
                      color: pal.ink2,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {it.g} г
                  </Text>
                  <Text
                    style={{
                      fontFamily: F.mono,
                      fontSize: 12,
                      color: pal.subtle,
                      minWidth: 54,
                      textAlign: 'right',
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {RECIPES.round(m.kcal)} ккал
                  </Text>
                </View>
              );
            })}
          </View>
        </View>

        {/* Steps */}
        <View style={{ paddingTop: 12, paddingHorizontal: 16, paddingBottom: 6 }}>
          <Eyebrow color={pal.subtle} style={{ paddingHorizontal: 4, paddingBottom: 10 }}>
            Приготовление
          </Eyebrow>
          <View style={{ gap: 14 }}>
            {recipe.steps.map((s, i) => (
              <View key={i} style={{ flexDirection: 'row', gap: 12, alignItems: 'flex-start' }}>
                <View
                  style={{
                    width: 30,
                    height: 30,
                    borderRadius: 999,
                    backgroundColor: C.forest50,
                    alignItems: 'center',
                    justifyContent: 'center',
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
                    {i + 1}
                  </Text>
                </View>
                <Text
                  style={{
                    flex: 1,
                    fontFamily: F.body,
                    fontSize: 14.5,
                    color: pal.ink2,
                    lineHeight: 14.5 * 1.55,
                    paddingTop: 3,
                  }}
                >
                  {s}
                </Text>
              </View>
            ))}
          </View>
        </View>
      </ScrollView>

      {/* Floating back button */}
      <View style={{ position: 'absolute', top: insets.top + 6, left: 12 }}>
        <Press
          onPress={onBack}
          style={{
            width: 40,
            height: 40,
            borderRadius: 999,
            backgroundColor: pal.bg,
            alignItems: 'center',
            justifyContent: 'center',
            shadowColor: '#2e2618',
            shadowOpacity: 0.16,
            shadowRadius: 8,
            shadowOffset: { width: 0, height: 2 },
            elevation: 4,
          }}
        >
          <Icon name="chevron-left" size={20} color={pal.ink2} />
        </Press>
      </View>

      {/* Add to day */}
      <LinearGradient
        colors={['rgba(246,241,234,0)', pal.bg]}
        locations={[0, 0.28]}
        style={{
          position: 'absolute',
          left: 0,
          right: 0,
          bottom: 0,
          paddingTop: 12,
          paddingHorizontal: 16,
          paddingBottom: Math.max(28, insets.bottom + 12),
        }}
      >
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
            <Press
              onPress={() => setPortions((p) => Math.max(1, p - 1))}
              style={{
                width: 38,
                height: 38,
                borderRadius: 999,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Text style={{ fontFamily: F.body, fontSize: 20, color: pal.ink }}>−</Text>
            </Press>
            <Text
              style={{
                minWidth: 34,
                textAlign: 'center',
                fontFamily: F.mono,
                fontSize: 16,
                color: pal.ink,
                fontVariant: ['tabular-nums'],
              }}
            >
              {portions}
            </Text>
            <Press
              onPress={() => setPortions((p) => Math.min(6, p + 1))}
              style={{
                width: 38,
                height: 38,
                borderRadius: 999,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Text style={{ fontFamily: F.body, fontSize: 20, color: pal.ink }}>+</Text>
            </Press>
          </View>
          <Press
            onPress={() => onAddToDay(recipe, portions)}
            style={{
              flex: 1,
              paddingVertical: 15,
              paddingHorizontal: 18,
              borderRadius: 999,
              backgroundColor: C.forest700,
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 8,
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 15, color: C.cream }}>
              Добавить в день
            </Text>
            <Icon name="plus" size={16} strokeWidth={2} color={C.cream} />
          </Press>
        </View>
      </LinearGradient>
    </View>
  );
}
