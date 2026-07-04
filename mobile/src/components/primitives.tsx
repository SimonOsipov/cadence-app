// Cadence brand primitives — ported from design-system/cadence-components.jsx.
// Eyebrow, Title, Body, Meta, Num, Pill, Chip, Btn, IconBtn, Card, Section, ListRow, Press.
import React from 'react';
import {
  Pressable,
  PressableProps,
  StyleProp,
  Text,
  TextStyle,
  View,
  ViewStyle,
} from 'react-native';
import { C, F, SHADOW } from '../theme';
import { Icon, IconName } from './Icon';

// Press — scale-to-0.98 press feedback (the prototype's `.press` class).
export function Press({
  style,
  children,
  disabled,
  ...rest
}: PressableProps & { style?: StyleProp<ViewStyle>; children?: React.ReactNode }) {
  return (
    <Pressable
      disabled={disabled}
      {...rest}
      style={({ pressed }) => [style, pressed && { transform: [{ scale: 0.98 }] }]}
    >
      {children}
    </Pressable>
  );
}

export function Eyebrow({
  children,
  color,
  style,
}: {
  children: React.ReactNode;
  color?: string;
  style?: StyleProp<TextStyle>;
}) {
  return (
    <Text
      numberOfLines={1}
      style={[
        {
          fontFamily: F.bodyMedium,
          fontSize: 11,
          letterSpacing: 11 * 0.14,
          textTransform: 'uppercase',
          color: color || C.ink500,
        },
        style,
      ]}
    >
      {children}
    </Text>
  );
}

export function Title({
  children,
  size = 28,
  italic,
  color,
  style,
  numberOfLines,
}: {
  children: React.ReactNode;
  size?: number;
  italic?: boolean;
  color?: string;
  style?: StyleProp<TextStyle>;
  numberOfLines?: number;
}) {
  return (
    <Text
      numberOfLines={numberOfLines}
      style={[
        {
          fontFamily: italic ? F.displayItalic : F.display,
          fontSize: size,
          lineHeight: size * 1.08,
          letterSpacing: size * -0.018,
          color: color || C.ink900,
        },
        style,
      ]}
    >
      {children}
    </Text>
  );
}

// Inline serif-italic emphasis inside a Title (e.g. "Выберите ритм.")
export function TitleEm({
  children,
  size = 28,
  color,
}: {
  children: React.ReactNode;
  size?: number;
  color?: string;
}) {
  return (
    <Text style={{ fontFamily: F.displayItalic, fontSize: size, color: color || C.forest700 }}>
      {children}
    </Text>
  );
}

export function Body({
  children,
  size = 14,
  color,
  style,
  numberOfLines,
}: {
  children: React.ReactNode;
  size?: number;
  color?: string;
  style?: StyleProp<TextStyle>;
  numberOfLines?: number;
}) {
  return (
    <Text
      numberOfLines={numberOfLines}
      style={[
        { fontFamily: F.body, fontSize: size, lineHeight: size * 1.5, color: color || C.ink800 },
        style,
      ]}
    >
      {children}
    </Text>
  );
}

export function Meta({
  children,
  style,
  color,
  numberOfLines,
}: {
  children: React.ReactNode;
  style?: StyleProp<TextStyle>;
  color?: string;
  numberOfLines?: number;
}) {
  return (
    <Text
      numberOfLines={numberOfLines}
      style={[{ fontFamily: F.body, fontSize: 12, color: color || C.ink500 }, style]}
    >
      {children}
    </Text>
  );
}

// Mono tabular value with serif-italic unit (unit sized 0.36×).
export function Num({
  value,
  unit,
  size = 44,
  color,
  unitColor,
  style,
}: {
  value: React.ReactNode;
  unit?: string;
  size?: number;
  color?: string;
  unitColor?: string;
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <View style={[{ flexDirection: 'row', alignItems: 'baseline', gap: 4 }, style]}>
      <Text
        style={{
          fontFamily: F.monoMedium,
          fontSize: size,
          letterSpacing: size * -0.03,
          color: color || C.ink900,
          fontVariant: ['tabular-nums'],
          lineHeight: size * 1.05,
        }}
      >
        {value}
      </Text>
      {unit ? (
        <Text
          style={{
            fontFamily: F.displayItalic,
            fontSize: size * 0.36,
            color: unitColor || C.ink600,
          }}
        >
          {unit}
        </Text>
      ) : null}
    </View>
  );
}

const PILL_TONES = {
  forest: { bg: C.forest50, fg: C.forest800, dot: C.forest700 },
  sand: { bg: C.sand100, fg: C.sand900, dot: C.sand700 },
  warning: { bg: C.warningBg, fg: '#7a4a06', dot: C.warning },
  danger: { bg: C.dangerBg, fg: '#6b2818', dot: C.danger },
  neutral: { bg: C.linen, fg: C.ink800, dot: C.ink500 },
  dark: { bg: C.ink900, fg: C.sand300, dot: C.sand500 },
} as const;
export type PillTone = keyof typeof PILL_TONES;

export function Pill({
  children,
  tone = 'forest',
  style,
}: {
  children: React.ReactNode;
  tone?: PillTone;
  style?: StyleProp<ViewStyle>;
}) {
  const p = PILL_TONES[tone];
  return (
    <View
      style={[
        {
          flexDirection: 'row',
          alignItems: 'center',
          gap: 6,
          paddingVertical: 4,
          paddingHorizontal: 10,
          borderRadius: 999,
          backgroundColor: p.bg,
          alignSelf: 'flex-start',
        },
        style,
      ]}
    >
      <View style={{ width: 6, height: 6, borderRadius: 999, backgroundColor: p.dot }} />
      <Text numberOfLines={1} style={{ fontFamily: F.bodyMedium, fontSize: 11, color: p.fg }}>
        {children}
      </Text>
    </View>
  );
}

export function Chip({
  children,
  active,
  onPress,
  style,
}: {
  children: React.ReactNode;
  active?: boolean;
  onPress?: () => void;
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <Press
      onPress={onPress}
      style={[
        {
          paddingVertical: 7,
          paddingHorizontal: 14,
          borderRadius: 999,
          backgroundColor: active ? C.ink900 : 'transparent',
          borderWidth: 1,
          borderColor: active ? C.ink900 : C.border,
        },
        style,
      ]}
    >
      <Text
        numberOfLines={1}
        style={{ fontFamily: F.bodyMedium, fontSize: 13, color: active ? C.cream : C.ink700 }}
      >
        {children}
      </Text>
    </Press>
  );
}

const BTN_SIZES = {
  sm: { pv: 8, ph: 14, fs: 13 },
  md: { pv: 12, ph: 20, fs: 14 },
  lg: { pv: 15, ph: 22, fs: 15 },
} as const;
const BTN_KINDS = {
  primary: { bg: C.forest700, fg: C.cream, bd: 'transparent' },
  secondary: { bg: C.linen, fg: C.ink900, bd: C.bone },
  ghost: { bg: 'transparent', fg: C.forest700, bd: 'transparent' },
  dark: { bg: C.ink900, fg: C.sand300, bd: 'transparent' },
  danger: { bg: 'transparent', fg: C.danger, bd: C.danger },
} as const;

export function Btn({
  children,
  kind = 'primary',
  size = 'md',
  onPress,
  full,
  icon,
  style,
}: {
  children: React.ReactNode;
  kind?: keyof typeof BTN_KINDS;
  size?: keyof typeof BTN_SIZES;
  onPress?: () => void;
  full?: boolean;
  icon?: IconName;
  style?: StyleProp<ViewStyle>;
}) {
  const s = BTN_SIZES[size];
  const k = BTN_KINDS[kind];
  return (
    <Press
      onPress={onPress}
      style={[
        {
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 8,
          alignSelf: full ? 'stretch' : 'flex-start',
          paddingVertical: s.pv,
          paddingHorizontal: s.ph,
          borderRadius: 999,
          backgroundColor: k.bg,
          borderWidth: 1,
          borderColor: k.bd,
        },
        style,
      ]}
    >
      {icon ? <Icon name={icon} size={s.fs + 3} color={k.fg} /> : null}
      <Text numberOfLines={1} style={{ fontFamily: F.bodyMedium, fontSize: s.fs, color: k.fg }}>
        {children}
      </Text>
    </Press>
  );
}

export function IconBtn({
  name,
  size = 20,
  onPress,
  bg = 'transparent',
  color = C.ink800,
  style,
}: {
  name: IconName;
  size?: number;
  onPress?: () => void;
  bg?: string;
  color?: string;
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <Press
      onPress={onPress}
      style={[
        {
          width: 40,
          height: 40,
          borderRadius: 999,
          backgroundColor: bg,
          alignItems: 'center',
          justifyContent: 'center',
        },
        style,
      ]}
    >
      <Icon name={name} size={size} color={color} />
    </Press>
  );
}

const CARD_TONES = {
  paper: { bg: C.paper, border: undefined },
  cream: { bg: C.cream, border: undefined },
  linen: { bg: C.linen, border: undefined },
  forest: { bg: C.forest800, border: undefined },
  sand: { bg: C.sand300, border: undefined },
  outline: { bg: C.paper, border: C.bone },
} as const;
export type CardTone = keyof typeof CARD_TONES;

export function Card({
  children,
  tone = 'paper',
  elev = 'sm',
  pad = 16,
  radius = 18,
  style,
  onPress,
}: {
  children: React.ReactNode;
  tone?: CardTone;
  elev?: 'none' | 'sm' | 'md';
  pad?: number;
  radius?: number;
  style?: StyleProp<ViewStyle>;
  onPress?: () => void;
}) {
  const t = CARD_TONES[tone];
  const base: ViewStyle = {
    backgroundColor: t.bg,
    borderRadius: radius,
    padding: pad,
    ...(t.border ? { borderWidth: 1, borderColor: t.border } : null),
    ...(elev !== 'none' ? SHADOW[elev] : null),
  };
  if (onPress) {
    return (
      <Press onPress={onPress} style={[base, style]}>
        {children}
      </Press>
    );
  }
  return <View style={[base, style]}>{children}</View>;
}

export function Section({
  title,
  action,
  onAction,
  children,
  style,
}: {
  title?: string;
  action?: string;
  onAction?: () => void;
  children: React.ReactNode;
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <View style={[{ gap: 10 }, style]}>
      {title || action ? (
        <View
          style={{
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'baseline',
            paddingHorizontal: 4,
          }}
        >
          {title ? <Eyebrow>{title}</Eyebrow> : <View />}
          {action ? (
            <Press onPress={onAction}>
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
                {action}
              </Text>
            </Press>
          ) : null}
        </View>
      ) : null}
      {children}
    </View>
  );
}

const ROW_TONES = {
  forest: { bg: C.forest50, fg: C.forest700 },
  sand: { bg: C.sand100, fg: C.sand900 },
  linen: { bg: C.linen, fg: C.ink700 },
  danger: { bg: C.dangerBg, fg: C.danger },
} as const;

export function ListRow({
  icon,
  iconTone = 'forest',
  title,
  sub,
  trail,
  trailSub,
  onPress,
}: {
  icon: IconName | React.ReactNode;
  iconTone?: keyof typeof ROW_TONES;
  title: string;
  sub?: string;
  trail?: string;
  trailSub?: string;
  onPress?: () => void;
}) {
  const t = ROW_TONES[iconTone];
  const content = (
    <View
      style={{
        flexDirection: 'row',
        gap: 14,
        alignItems: 'center',
        paddingVertical: 12,
        paddingHorizontal: 14,
      }}
    >
      <View
        style={{
          width: 40,
          height: 40,
          borderRadius: 12,
          backgroundColor: t.bg,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {typeof icon === 'string' ? <Icon name={icon as IconName} size={20} color={t.fg} /> : icon}
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text
          numberOfLines={1}
          style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.ink900 }}
        >
          {title}
        </Text>
        {sub ? (
          <Text
            numberOfLines={1}
            style={{ fontFamily: F.body, fontSize: 12, color: C.ink500, marginTop: 1 }}
          >
            {sub}
          </Text>
        ) : null}
      </View>
      {trail ? (
        <View style={{ alignItems: 'flex-end' }}>
          <Text
            style={{
              fontFamily: F.mono,
              fontSize: 13,
              color: C.ink800,
              fontVariant: ['tabular-nums'],
            }}
          >
            {trail}
          </Text>
          {trailSub ? (
            <Text style={{ fontFamily: F.body, fontSize: 11, color: C.ink500 }}>{trailSub}</Text>
          ) : null}
        </View>
      ) : null}
    </View>
  );
  if (onPress) return <Press onPress={onPress}>{content}</Press>;
  return content;
}
