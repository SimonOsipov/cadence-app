// Cadence · Рецепты — library screen.
// Ported from design_handoff_cadence_app/recipe/recipe.jsx (RecipeLibrary).
import React, { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Eyebrow, IconBtn, Press } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { useAppState } from '../../state/AppState';
import { RECIPES, Recipe } from './data';

// Meal-type tints — values from the prototype's RECIPE_TYPE_TINT.
export const TYPE_TINT: Record<string, { soft: string; fg: string }> = {
  breakfast: { soft: C.sand100, fg: C.sand900 },
  lunch: { soft: C.forest50, fg: C.forest700 },
  dinner: { soft: '#e6ecf2', fg: '#41566b' },
  snack: { soft: '#f4e4d8', fg: '#9a5a3c' },
};

function FilterChip({ label, on, onPress }: { label: string; on: boolean; onPress: () => void }) {
  return (
    <Press
      onPress={onPress}
      style={{
        flexShrink: 0,
        paddingVertical: 7,
        paddingHorizontal: 13,
        borderRadius: 999,
        backgroundColor: on ? C.forest700 : 'transparent',
        borderWidth: 1,
        borderColor: on ? C.forest700 : pal.border,
      }}
    >
      <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: on ? C.cream : pal.ink2 }}>
        {label}
      </Text>
    </Press>
  );
}

// Typographic recipe row.
function RecipeRow({ r, onOpen, last }: { r: Recipe & { mine?: boolean }; onOpen: (id: string) => void; last: boolean }) {
  const ps = RECIPES.perServing(r);
  const tint = TYPE_TINT[r.mealType] || TYPE_TINT.lunch;
  return (
    <Press
      onPress={() => onOpen(r.id)}
      style={{
        paddingVertical: 14,
        paddingHorizontal: 4,
        borderBottomWidth: last ? 0 : 1,
        borderBottomColor: pal.hairline,
      }}
    >
      <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8, flexWrap: 'wrap' }}>
        <Text
          style={{
            fontFamily: F.display,
            fontSize: 20,
            color: pal.ink,
            lineHeight: 22,
            letterSpacing: 20 * -0.012,
          }}
        >
          {r.name}
        </Text>
        {r.mine ? (
          <View
            style={{
              backgroundColor: C.forest50,
              borderRadius: 999,
              paddingVertical: 2,
              paddingHorizontal: 8,
            }}
          >
            <Text style={{ fontFamily: F.bodySemibold, fontSize: 10, color: C.forest700 }}>
              Моё
            </Text>
          </View>
        ) : null}
      </View>
      <Text
        numberOfLines={2}
        style={{
          fontFamily: F.body,
          fontSize: 12.5,
          color: pal.subtle,
          marginTop: 3,
          lineHeight: 12.5 * 1.45,
        }}
      >
        {r.dek}
      </Text>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 9, flexWrap: 'wrap' }}>
        <View
          style={{
            backgroundColor: tint.soft,
            borderRadius: 999,
            paddingVertical: 3,
            paddingHorizontal: 9,
          }}
        >
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 11, color: tint.fg }}>
            {RECIPES.typeLabel(r.mealType)}
          </Text>
        </View>
        <Text
          style={{ fontFamily: F.mono, fontSize: 11.5, color: pal.ink2, fontVariant: ['tabular-nums'] }}
        >
          {RECIPES.round(ps.kcal)} ккал
        </Text>
        <View style={{ width: 3, height: 3, borderRadius: 999, backgroundColor: pal.placeholder }} />
        <Text
          style={{ fontFamily: F.mono, fontSize: 11.5, color: C.forest700, fontVariant: ['tabular-nums'] }}
        >
          {RECIPES.round(ps.p)} г белка
        </Text>
        <Text style={{ marginLeft: 'auto', fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>
          {RECIPES.totalTime(r)} мин
        </Text>
      </View>
    </Press>
  );
}

export function RecipesScreen({
  onBack,
  onOpen,
  onCreate,
}: {
  onBack: () => void;
  onOpen: (id: string) => void;
  onCreate: () => void;
}) {
  const insets = useSafeAreaInsets();
  const { allRecipes, mealTotals } = useAppState();
  const [tag, setTag] = useState<string | null>(null);
  const [mealType, setMealType] = useState<string | null>(null);

  const { pick, rem } = RECIPES.suggest(allRecipes, mealTotals);
  const pickPs = RECIPES.perServing(pick);
  const list = RECIPES.filter(allRecipes, { tag, mealType });
  const mine = list.filter((r: any) => r.mine);
  const rest = list.filter((r: any) => !r.mine);

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Top bar */}
      <View
        style={{
          paddingTop: insets.top + 8,
          paddingHorizontal: 16,
          paddingBottom: 10,
          flexDirection: 'row',
          alignItems: 'center',
          gap: 10,
          backgroundColor: pal.bg,
          borderBottomWidth: 1,
          borderBottomColor: pal.hairline,
        }}
      >
        <IconBtn name="chevron-left" onPress={onBack} bg={pal.sunk} color={pal.ink2} />
        <Text style={{ fontFamily: F.bodySemibold, fontSize: 15, color: pal.ink }}>Рецепты</Text>
        <Press
          onPress={onCreate}
          style={{
            marginLeft: 'auto',
            height: 40,
            paddingHorizontal: 14,
            borderRadius: 999,
            backgroundColor: C.forest700,
            flexDirection: 'row',
            alignItems: 'center',
            gap: 6,
          }}
        >
          <Icon name="plus" size={16} strokeWidth={2} color={C.cream} />
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: C.cream }}>Создать</Text>
        </Press>
      </View>

      <ScrollView contentContainerStyle={{ paddingBottom: insets.bottom + 40 }}>
        {/* Personalized featured */}
        <View style={{ paddingTop: 8, paddingHorizontal: 16, paddingBottom: 14 }}>
          <Press
            onPress={() => onOpen(pick.id)}
            style={{
              borderRadius: 24,
              overflow: 'hidden',
              backgroundColor: C.forest800,
              shadowColor: C.forest900,
              shadowOpacity: 0.18,
              shadowRadius: 24,
              shadowOffset: { width: 0, height: 8 },
              elevation: 8,
            }}
          >
            <View
              style={{
                height: 150,
                backgroundColor: C.linen,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name="camera" color={C.ink400} />
            </View>
            <View style={{ padding: 18 }}>
              <View
                style={{
                  alignSelf: 'flex-start',
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: 6,
                  marginBottom: 10,
                  backgroundColor: 'rgba(212,165,116,.2)',
                  borderRadius: 999,
                  paddingVertical: 5,
                  paddingHorizontal: 11,
                }}
              >
                <Icon name="sparkles" size={13} color={C.sand300} />
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 11, color: C.sand300 }}>
                  Для вас сегодня
                </Text>
              </View>
              <Text
                style={{
                  fontFamily: F.display,
                  fontSize: 27,
                  color: C.cream,
                  lineHeight: 27 * 1.06,
                  letterSpacing: 27 * -0.018,
                }}
              >
                {pick.name}
              </Text>
              <Text
                style={{
                  fontFamily: F.body,
                  fontSize: 13,
                  color: 'rgba(246,241,234,.74)',
                  lineHeight: 13 * 1.5,
                  marginTop: 8,
                }}
              >
                Осталось{' '}
                <Text style={{ fontFamily: F.mono, color: C.sand300, fontVariant: ['tabular-nums'] }}>
                  {RECIPES.round(rem.p)} г
                </Text>{' '}
                белка сегодня — порция закроет{' '}
                <Text style={{ fontFamily: F.mono, color: C.sand300, fontVariant: ['tabular-nums'] }}>
                  {RECIPES.round(pickPs.p)} г
                </Text>
                .
              </Text>
              <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 14 }}>
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: C.sand300 }}>
                  Открыть рецепт
                </Text>
                <Icon name="arrow-right" size={15} color={C.sand300} />
                <Text
                  style={{
                    marginLeft: 'auto',
                    fontFamily: F.mono,
                    fontSize: 11,
                    color: 'rgba(246,241,234,.55)',
                    fontVariant: ['tabular-nums'],
                  }}
                >
                  {RECIPES.round(pickPs.kcal)} ккал · {RECIPES.totalTime(pick)} мин
                </Text>
              </View>
            </View>
          </Press>
        </View>

        {/* Filters */}
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={{ gap: 8, paddingHorizontal: 16, paddingBottom: 10 }}
        >
          <FilterChip label="Все" on={!mealType} onPress={() => setMealType(null)} />
          {RECIPES.MEAL_TYPES.map((t) => (
            <FilterChip
              key={t.id}
              label={t.label}
              on={mealType === t.id}
              onPress={() => setMealType(mealType === t.id ? null : t.id)}
            />
          ))}
        </ScrollView>
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={{ gap: 8, paddingHorizontal: 16, paddingBottom: 14 }}
        >
          <FilterChip label="Любые" on={!tag} onPress={() => setTag(null)} />
          {RECIPES.TAGS.map((t) => (
            <FilterChip
              key={t.id}
              label={t.label}
              on={tag === t.id}
              onPress={() => setTag(tag === t.id ? null : t.id)}
            />
          ))}
        </ScrollView>

        {/* Mine */}
        {mine.length > 0 ? (
          <View style={{ paddingHorizontal: 16, paddingBottom: 8 }}>
            <Eyebrow color={pal.subtle} style={{ paddingHorizontal: 4, paddingBottom: 4 }}>
              Мои рецепты
            </Eyebrow>
            {mine.map((r: any, i: number) => (
              <RecipeRow key={r.id} r={r} onOpen={onOpen} last={i === mine.length - 1} />
            ))}
          </View>
        ) : null}

        {/* Library */}
        <View style={{ paddingHorizontal: 16 }}>
          <Eyebrow color={pal.subtle} style={{ paddingHorizontal: 4, paddingBottom: 4 }}>
            {mine.length ? 'Готовые рецепты' : 'Все рецепты'}
          </Eyebrow>
          {rest.length === 0 ? (
            <Text
              style={{
                fontFamily: F.body,
                fontSize: 13,
                color: pal.subtle,
                paddingVertical: 12,
                paddingHorizontal: 4,
              }}
            >
              Под фильтр ничего не подошло.
            </Text>
          ) : (
            rest.map((r: any, i: number) => (
              <RecipeRow key={r.id} r={r} onOpen={onOpen} last={i === rest.length - 1} />
            ))
          )}
        </View>
      </ScrollView>
    </View>
  );
}
