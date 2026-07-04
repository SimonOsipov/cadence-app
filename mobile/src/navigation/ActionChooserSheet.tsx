// Action chooser — bottom sheet from the '+' tab. Ported from prototype.jsx.
import React from 'react';
import { Text, View } from 'react-native';
import { C, F, pal } from '../theme';
import { Icon, IconName } from '../components/Icon';
import { Eyebrow, Press } from '../components/primitives';
import { Sheet } from '../components/shared';
import { useAppState } from '../state/AppState';

function ActionOption({
  iconBg,
  iconFg,
  iconName,
  title,
  sub,
  onPress,
}: {
  iconBg: string;
  iconFg: string;
  iconName: IconName;
  title: string;
  sub: string;
  onPress: () => void;
}) {
  return (
    <Press
      onPress={onPress}
      style={{
        flexDirection: 'row',
        gap: 14,
        alignItems: 'center',
        padding: 14,
        backgroundColor: pal.paper,
        borderWidth: 1,
        borderColor: pal.hairline,
        borderRadius: 18,
      }}
    >
      <View
        style={{
          width: 52,
          height: 52,
          borderRadius: 14,
          backgroundColor: iconBg,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={iconName} size={24} color={iconFg} />
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text style={{ fontFamily: F.display, fontSize: 22, color: pal.ink, letterSpacing: -0.26 }}>
          {title}
        </Text>
        <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.muted, marginTop: 3 }}>
          {sub}
        </Text>
      </View>
      <Icon name="chevron-right" size={18} color={pal.subtle} />
    </Press>
  );
}

export function ActionChooserSheet({
  open,
  onClose,
  onPickDose,
  onPickMeal,
}: {
  open: boolean;
  onClose: () => void;
  onPickDose: () => void;
  onPickMeal: () => void;
}) {
  const { doseLogged, meals, mealTotals } = useAppState();
  const mealCount = meals.length;
  const mealKcal = mealTotals ? mealTotals.kcal : 0;
  return (
    <Sheet open={open} onClose={onClose}>
      <View style={{ marginBottom: 18 }}>
        <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
          Что записываем?
        </Eyebrow>
        <Text style={{ fontFamily: F.display, fontSize: 28, color: pal.ink, letterSpacing: -0.5 }}>
          Выберите{' '}
          <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>ритм</Text>.
        </Text>
      </View>
      <View style={{ gap: 10, marginBottom: 14 }}>
        <ActionOption
          iconBg={C.forest700}
          iconFg={C.cream}
          iconName="beaker"
          title="Записать дозу"
          sub={
            doseLogged
              ? 'Уже записано сегодня · открыть или поправить'
              : 'Семаглутид · 0,25 мг ждёт'
          }
          onPress={onPickDose}
        />
        <ActionOption
          iconBg={C.sand500}
          iconFg={C.ink900}
          iconName="cake"
          title="Записать приём пищи"
          sub={
            mealCount === 0
              ? 'Пока ничего сегодня · начнём ритм'
              : `${mealCount} ${mealCount === 1 ? 'приём' : mealCount < 5 ? 'приёма' : 'приёмов'} сегодня · ${mealKcal.toLocaleString('ru-RU')} ккал`
          }
          onPress={onPickMeal}
        />
      </View>
      <Press
        onPress={onClose}
        style={{
          paddingVertical: 13,
          borderRadius: 999,
          borderWidth: 1,
          borderColor: pal.border,
          alignItems: 'center',
        }}
      >
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.muted }}>Отмена</Text>
      </Press>
    </Sheet>
  );
}
