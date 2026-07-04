// Confirm toast — bottom card with check tick after logging a meal. From prototype.jsx.
import React from 'react';
import { Text, View } from 'react-native';
import Svg, { Path } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, SHADOW } from '../theme';
import { useAppState } from '../state/AppState';

export function ConfirmToast() {
  const { confirmSheet, mealTargets } = useAppState();
  const insets = useSafeAreaInsets();
  if (!confirmSheet) return null;
  return (
    <View
      pointerEvents="auto"
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(20,44,31,.25)',
        justifyContent: 'flex-end',
        paddingHorizontal: 16,
        paddingBottom: Math.max(insets.bottom, 16) + 8,
      }}
    >
      <View
        style={{
        backgroundColor: pal.bg,
        borderRadius: 22,
        padding: 18,
        borderWidth: 1,
        borderColor: pal.hairline,
        flexDirection: 'row',
        alignItems: 'center',
        gap: 14,
        ...SHADOW.lg,
      }}
    >
      <View
        style={{
          width: 44,
          height: 44,
          borderRadius: 999,
          backgroundColor: C.forest700,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Svg width={20} height={20} viewBox="0 0 16 16" fill="none">
          <Path
            d="M3 8.5L6.5 12L13 4.5"
            stroke={C.cream}
            strokeWidth={2.2}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </Svg>
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text
          numberOfLines={1}
          style={{ fontFamily: F.display, fontSize: 18, color: pal.ink, marginBottom: 2 }}
        >
          {confirmSheet.mealName} · записано
        </Text>
        <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.muted }}>
          <Text style={{ fontFamily: F.monoMedium, color: pal.ink2 }}>
            {confirmSheet.kcal.toLocaleString('ru-RU')}
          </Text>
          <Text style={{ color: pal.subtle }}>
            {' '}/ {mealTargets.kcal.toLocaleString('ru-RU')} ккал сегодня
          </Text>
        </Text>
      </View>
      </View>
    </View>
  );
}
