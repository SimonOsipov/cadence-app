// Shared dashboard atoms — ported from shared.jsx:
// AppHeader, Spark, CycleRing, CadenceTabBar, Sheet (bottom sheet), Grabber, ScreenHeader.
import React, { useEffect, useRef } from 'react';
import { Animated, Modal, Pressable, Text, View, StyleProp, ViewStyle } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Circle, Line, Path } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, SHADOW } from '../theme';
import { Icon, IconName } from './Icon';
import { IconBtn, Meta, Press, Title } from './primitives';

// App header — meta greeting + large serif name + bell + avatar.
export function AppHeader({
  greeting,
  name,
  avatar = 'М',
  onAvatar,
  onBell,
}: {
  greeting: string;
  name: string;
  avatar?: string;
  onAvatar?: () => void;
  onBell?: () => void;
}) {
  return (
    <View
      style={{
        paddingTop: 8,
        paddingHorizontal: 20,
        paddingBottom: 18,
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'flex-end',
        gap: 12,
      }}
    >
      <View style={{ flex: 1, minWidth: 0 }}>
        <Meta numberOfLines={1} style={{ marginBottom: 4 }}>
          {greeting}
        </Meta>
        <Title size={32} numberOfLines={1}>
          {name}
        </Title>
      </View>
      <View style={{ flexDirection: 'row', gap: 8, alignItems: 'center' }}>
        <IconBtn name="bell" bg={C.linen} onPress={onBell} />
        <Press
          onPress={onAvatar}
          style={{
            width: 40,
            height: 40,
            borderRadius: 999,
            backgroundColor: C.forest700,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Text style={{ fontFamily: F.displayItalic, fontSize: 18, color: C.cream }}>
            {avatar}
          </Text>
        </Press>
      </View>
    </View>
  );
}

// Standard push-screen header: back chevron + eyebrow/title.
export function ScreenHeader({
  title,
  eyebrow,
  onBack,
  right,
}: {
  title: string;
  eyebrow?: string;
  onBack: () => void;
  right?: React.ReactNode;
}) {
  return (
    <View
      style={{
        flexDirection: 'row',
        alignItems: 'center',
        gap: 10,
        paddingHorizontal: 12,
        paddingTop: 6,
        paddingBottom: 10,
      }}
    >
      <IconBtn name="chevron-left" onPress={onBack} bg={C.linen} />
      <View style={{ flex: 1, minWidth: 0 }}>
        {eyebrow ? (
          <Text
            numberOfLines={1}
            style={{
              fontFamily: F.bodyMedium,
              fontSize: 11,
              letterSpacing: 11 * 0.14,
              textTransform: 'uppercase',
              color: pal.subtle,
              marginBottom: 2,
            }}
          >
            {eyebrow}
          </Text>
        ) : null}
        <Title size={24} numberOfLines={1}>
          {title}
        </Title>
      </View>
      {right}
    </View>
  );
}

// Tiny inline trend sparkline.
export function Spark({
  data = [0.3, 0.5, 0.4, 0.6, 0.55, 0.7, 0.8],
  color = C.forest700,
  fill,
  width = 120,
  height = 36,
}: {
  data?: number[];
  color?: string;
  fill?: string;
  width?: number;
  height?: number;
}) {
  const max = Math.max(...data);
  const min = Math.min(...data);
  const padPx = 2;
  const xs = data.map((_, i) => padPx + (i * (width - padPx * 2)) / (data.length - 1));
  const ys = data.map((v) => padPx + ((max - v) / (max - min || 1)) * (height - padPx * 2));
  const d = xs.map((x, i) => (i === 0 ? 'M' : 'L') + x + ' ' + ys[i]).join(' ');
  const dFill =
    d + ' L' + xs[xs.length - 1] + ' ' + (height - padPx) + ' L' + padPx + ' ' + (height - padPx) + ' Z';
  return (
    <Svg width={width} height={height} viewBox={`0 0 ${width} ${height}`}>
      {fill ? <Path d={dFill} fill={fill} opacity={0.4} /> : null}
      <Path d={d} stroke={color} strokeWidth={2} fill="none" strokeLinecap="round" strokeLinejoin="round" />
      <Circle cx={xs[xs.length - 1]} cy={ys[ys.length - 1]} r={3} fill={color} />
    </Svg>
  );
}

// Cycle ring — week N of 12, with week tick marks.
export function CycleRing({
  week = 4,
  total = 12,
  size = 132,
  stroke = 10,
  color = C.forest700,
}: {
  week?: number;
  total?: number;
  size?: number;
  stroke?: number;
  color?: string;
}) {
  const r = (size - stroke) / 2;
  const cx = size / 2;
  const cy = size / 2;
  const circ = 2 * Math.PI * r;
  const dash = circ * (week / total);
  return (
    <Svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      <Circle cx={cx} cy={cy} r={r} stroke={pal.bone} strokeWidth={stroke} fill="none" />
      <Circle
        cx={cx}
        cy={cy}
        r={r}
        stroke={color}
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
            stroke={pal.subtle}
            strokeWidth={1}
            opacity={0.5}
          />
        );
      })}
    </Svg>
  );
}

// Bottom tab bar — Сегодня / Аптечка / Записать(+) / Тренды / Питание.
export type TabId = 'today' | 'inventory' | 'log' | 'insights' | 'nutrition';

const TABS: { id: TabId; name: IconName; label: string; primary?: boolean }[] = [
  { id: 'today', name: 'home', label: 'Сегодня' },
  { id: 'inventory', name: 'beaker', label: 'Аптечка' },
  { id: 'log', name: 'plus', label: 'Записать', primary: true },
  { id: 'insights', name: 'chart-bar', label: 'Тренды' },
  { id: 'nutrition', name: 'cake', label: 'Питание' },
];

export function CadenceTabBar({
  active,
  onChange,
}: {
  active: TabId | null;
  onChange: (id: TabId) => void;
}) {
  const insets = useSafeAreaInsets();
  return (
    <LinearGradient
      colors={['rgba(246,241,234,0)', 'rgba(246,241,234,0.85)', 'rgba(246,241,234,1)']}
      locations={[0, 0.4, 1]}
      style={{
        position: 'absolute',
        left: 0,
        right: 0,
        bottom: 0,
        paddingBottom: Math.max(insets.bottom, 16),
        paddingTop: 8,
        paddingHorizontal: 8,
      }}
    >
      <View style={{ flexDirection: 'row', alignItems: 'flex-end' }}>
        {TABS.map((t) => {
          if (t.primary) {
            return (
              <View key={t.id} style={{ flex: 1, alignItems: 'center' }}>
                <Press
                  onPress={() => onChange(t.id)}
                  style={{
                    width: 52,
                    height: 52,
                    borderRadius: 999,
                    backgroundColor: C.forest700,
                    alignItems: 'center',
                    justifyContent: 'center',
                    shadowColor: C.forest700,
                    shadowOpacity: 0.35,
                    shadowRadius: 16,
                    shadowOffset: { width: 0, height: 6 },
                    elevation: 8,
                  }}
                >
                  <Icon name="plus" size={24} strokeWidth={2} color={C.cream} />
                </Press>
              </View>
            );
          }
          const on = active === t.id;
          const color = on ? C.forest700 : pal.subtle;
          return (
            <Press
              key={t.id}
              onPress={() => onChange(t.id)}
              style={{ flex: 1, alignItems: 'center', gap: 2, paddingVertical: 6 }}
            >
              <Icon name={t.name} size={22} strokeWidth={on ? 1.8 : 1.5} color={color} />
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 10, color }}>{t.label}</Text>
            </Press>
          );
        })}
      </View>
    </LinearGradient>
  );
}

// Sheet grabber bar.
export function Grabber() {
  return (
    <View
      style={{
        width: 38,
        height: 4,
        borderRadius: 999,
        backgroundColor: pal.border,
        alignSelf: 'center',
        marginBottom: 16,
      }}
    />
  );
}

// Bottom sheet — forest-tinted scrim + slide-up panel, top radius 28.
export function Sheet({
  open,
  onClose,
  children,
  scrim = pal.glassDark,
  contentStyle,
}: {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
  scrim?: string;
  contentStyle?: StyleProp<ViewStyle>;
}) {
  const insets = useSafeAreaInsets();
  const slide = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    if (open) {
      slide.setValue(0);
      Animated.timing(slide, { toValue: 1, duration: 380, useNativeDriver: true }).start();
    }
  }, [open, slide]);
  if (!open) return null;
  return (
    <Modal transparent visible animationType="fade" onRequestClose={onClose}>
      <View style={{ flex: 1, justifyContent: 'flex-end' }}>
        <Pressable
          onPress={onClose}
          style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: scrim }}
        />
        <Animated.View
          style={[
            {
              backgroundColor: pal.bg,
              borderTopLeftRadius: 28,
              borderTopRightRadius: 28,
              paddingTop: 12,
              paddingHorizontal: 20,
              paddingBottom: Math.max(insets.bottom, 20) + 12,
              ...SHADOW.lg,
              transform: [
                {
                  translateY: slide.interpolate({ inputRange: [0, 1], outputRange: [400, 0] }),
                },
              ],
            },
            contentStyle,
          ]}
        >
          <Grabber />
          {children}
        </Animated.View>
      </View>
    </Modal>
  );
}
