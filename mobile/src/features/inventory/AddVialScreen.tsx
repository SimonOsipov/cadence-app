// Cadence — Add a vial flow ("Новый флакон").
// Manual form: compound, dose, lot, expiration, location, open-now toggle,
// optional label photo. Single-screen modal wizard (slides up over Vials).
// Ported from design_handoff_cadence_app/inventory/add-vial.jsx.
import React from 'react';
import { Animated, Easing, ScrollView, Text, TextInput, View } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Path } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Eyebrow, Press, Title, TitleEm } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { COMPOUND_META } from './data';

const LOCATIONS = [
  'Холодильник, полка 1',
  'Холодильник, полка 2',
  'Холодильник, полка 3',
  'Морозильник',
  'Комната',
];

type PhotoState = 'pending' | 'attached' | null;

export function AddVialScreen({
  onCancel,
  onComplete,
}: {
  onCancel: () => void;
  onComplete: () => void;
}) {
  const insets = useSafeAreaInsets();
  const [compound, setCompound] = React.useState('sema');
  const [dose, setDose] = React.useState('0,25 мг');
  const [lot, setLot] = React.useState('');
  const [expires, setExpires] = React.useState(''); // free-form date, e.g. '14 авг'
  const [location, setLocation] = React.useState('Холодильник, полка 2');
  const [opened, setOpened] = React.useState(false); // also set as active?
  const [photo, setPhoto] = React.useState<PhotoState>(null);
  const photoTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  // Auto-fill dose from compound default.
  React.useEffect(() => {
    const meta = COMPOUND_META[compound];
    if (meta) setDose(meta.defaultDose);
  }, [compound]);

  React.useEffect(
    () => () => {
      if (photoTimer.current) clearTimeout(photoTimer.current);
    },
    [],
  );

  // Photo "capture" (mocked).
  const onPhotoTap = () => {
    if (photo === 'attached') {
      setPhoto(null);
      return;
    }
    setPhoto('pending');
    photoTimer.current = setTimeout(() => setPhoto('attached'), 900);
  };

  const canSave = lot.trim().length > 0 && expires.trim().length > 0;

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <ScrollView
        showsVerticalScrollIndicator={false}
        keyboardShouldPersistTaps="handled"
        contentContainerStyle={{ paddingTop: insets.top + 6, paddingBottom: 110 }}
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
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.muted }}>
            Новый флакон
          </Text>
          <View style={{ width: 40 }} />
        </View>

        {/* Hero */}
        <View style={{ paddingTop: 4, paddingHorizontal: 24, paddingBottom: 18 }}>
          <Eyebrow color={C.sand700} style={{ marginBottom: 6 }}>
            В аптечку
          </Eyebrow>
          <Title size={32}>
            Что <TitleEm size={32} color={C.sand700}>прибыло?</TitleEm>
          </Title>
        </View>

        {/* Compound chooser */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <FieldLabel>Препарат</FieldLabel>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {Object.entries(COMPOUND_META).map(([id, meta]) => {
              const sel = compound === id;
              return (
                <Press
                  key={id}
                  onPress={() => setCompound(id)}
                  style={{
                    width: '48.7%',
                    paddingVertical: 12,
                    paddingHorizontal: 14,
                    borderRadius: 14,
                    backgroundColor: sel ? pal.paper : 'transparent',
                    borderWidth: 1,
                    borderColor: sel ? C.forest700 : pal.border,
                    flexDirection: 'row',
                    alignItems: 'center',
                    gap: 10,
                  }}
                >
                  <View
                    style={{
                      width: 18,
                      height: 18,
                      borderRadius: 999,
                      borderWidth: 1.5,
                      borderColor: sel ? C.forest700 : pal.border,
                      backgroundColor: sel ? C.forest700 : 'transparent',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0,
                    }}
                  >
                    {sel ? (
                      <Svg width={10} height={10} viewBox="0 0 16 16" fill="none">
                        <Path
                          d="M3 8.5L6.5 12L13 4.5"
                          stroke={C.cream}
                          strokeWidth={2.5}
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      </Svg>
                    ) : null}
                  </View>
                  <Text
                    numberOfLines={1}
                    style={{
                      fontFamily: F.bodyMedium,
                      fontSize: 13.5,
                      color: sel ? pal.ink : pal.ink2,
                      flexShrink: 1,
                    }}
                  >
                    {meta.name}
                  </Text>
                </Press>
              );
            })}
          </View>
        </View>

        {/* Dose */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <FieldLabel>Дозировка</FieldLabel>
          <Field value={dose} onChange={setDose} placeholder="0,25 мг" />
        </View>

        {/* Lot + expires row */}
        <View
          style={{ paddingHorizontal: 16, paddingBottom: 14, flexDirection: 'row', gap: 10 }}
        >
          <View style={{ flex: 1 }}>
            <FieldLabel>Лот</FieldLabel>
            <Field value={lot} onChange={setLot} placeholder="A24-0312" mono />
          </View>
          <View style={{ flex: 1 }}>
            <FieldLabel>До какого</FieldLabel>
            <Field value={expires} onChange={setExpires} placeholder="14 сен" />
          </View>
        </View>

        {/* Location */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <FieldLabel>Хранение</FieldLabel>
          <View style={{ flexDirection: 'row', gap: 6, flexWrap: 'wrap' }}>
            {LOCATIONS.map((opt) => {
              const on = location === opt;
              return (
                <Press
                  key={opt}
                  onPress={() => setLocation(opt)}
                  style={{
                    paddingVertical: 8,
                    paddingHorizontal: 12,
                    borderRadius: 999,
                    backgroundColor: on ? C.forest700 : 'transparent',
                    borderWidth: 1,
                    borderColor: on ? C.forest700 : pal.border,
                  }}
                >
                  <Text
                    style={{
                      fontFamily: F.bodyMedium,
                      fontSize: 12,
                      color: on ? C.cream : pal.ink2,
                    }}
                  >
                    {opt}
                  </Text>
                </Press>
              );
            })}
          </View>
        </View>

        {/* Open now toggle */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <Press
            onPress={() => setOpened(!opened)}
            style={{
              backgroundColor: pal.paper,
              borderWidth: 1,
              borderColor: pal.hairline,
              borderRadius: 14,
              padding: 14,
              flexDirection: 'row',
              alignItems: 'center',
              gap: 12,
            }}
          >
            <View style={{ flex: 1, minWidth: 0 }}>
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: pal.ink }}>
                Открыть сразу
              </Text>
              <Text
                style={{
                  fontFamily: F.body,
                  fontSize: 11.5,
                  lineHeight: 11.5 * 1.4,
                  color: pal.subtle,
                  marginTop: 2,
                }}
              >
                Сделаем флакон активным и начнём отсчёт сроков.
              </Text>
            </View>
            <View
              style={{
                width: 44,
                height: 26,
                borderRadius: 999,
                backgroundColor: opened ? C.forest700 : pal.border,
                padding: 2,
                flexShrink: 0,
              }}
            >
              <View
                style={{
                  width: 22,
                  height: 22,
                  borderRadius: 999,
                  backgroundColor: C.cream,
                  transform: [{ translateX: opened ? 18 : 0 }],
                  shadowColor: '#000',
                  shadowOpacity: 0.18,
                  shadowRadius: 3,
                  shadowOffset: { width: 0, height: 1 },
                  elevation: 2,
                }}
              />
            </View>
          </Press>
        </View>

        {/* Photo */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <Press
            onPress={onPhotoTap}
            style={{
              borderRadius: 14,
              backgroundColor: photo === 'attached' ? pal.sunk : 'transparent',
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
                backgroundColor: photo === 'attached' ? C.forest700 : pal.sunk,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              {photo === 'pending' ? (
                <Spinner />
              ) : photo === 'attached' ? (
                <Svg width={20} height={20} viewBox="0 0 16 16" fill="none">
                  <Path
                    d="M3 8.5L6.5 12L13 4.5"
                    stroke={C.cream}
                    strokeWidth={2}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </Svg>
              ) : (
                <Icon name="camera" size={20} color={pal.muted} />
              )}
            </View>
            <View style={{ flex: 1, minWidth: 0 }}>
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.ink }}>
                {photo === 'attached'
                  ? 'Фото добавлено'
                  : photo === 'pending'
                    ? 'Снимаем…'
                    : 'Снимок этикетки'}
              </Text>
              <Text
                style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}
              >
                {photo === 'attached' ? 'Нажмите, чтобы убрать' : 'По желанию'}
              </Text>
            </View>
          </Press>
        </View>
      </ScrollView>

      {/* Sticky Save button */}
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
          onPress={onComplete}
          style={{
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
            <>
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>
                Добавить в аптечку
              </Text>
              <Text style={{ fontFamily: F.displayItalic, fontSize: 14, color: C.cream }}>
                →
              </Text>
            </>
          ) : (
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.subtle }}>
              Заполните лот и срок
            </Text>
          )}
        </Press>
      </LinearGradient>
    </View>
  );
}

// ── FieldLabel — uppercase eyebrow for form fields ──────────────────

function FieldLabel({ children }: { children: React.ReactNode }) {
  return (
    <Eyebrow color={pal.subtle} style={{ marginBottom: 8, paddingHorizontal: 4 }}>
      {children}
    </Eyebrow>
  );
}

// ── Field — simple paper-card input ─────────────────────────────────

function Field({
  value,
  onChange,
  placeholder,
  mono,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  mono?: boolean;
}) {
  return (
    <TextInput
      value={value}
      onChangeText={onChange}
      placeholder={placeholder}
      placeholderTextColor={pal.placeholder}
      style={{
        paddingVertical: 12,
        paddingHorizontal: 14,
        backgroundColor: pal.paper,
        borderWidth: 1,
        borderColor: pal.border,
        borderRadius: 12,
        fontFamily: mono ? F.mono : F.body,
        fontSize: 14,
        color: pal.ink,
        ...(mono ? { fontVariant: ['tabular-nums'] as const, letterSpacing: -0.14 } : null),
      }}
    />
  );
}

// ── Spinner — photo-capture pending state ───────────────────────────

function Spinner() {
  const spin = React.useRef(new Animated.Value(0)).current;
  React.useEffect(() => {
    const loop = Animated.loop(
      Animated.timing(spin, {
        toValue: 1,
        duration: 700,
        easing: Easing.linear,
        useNativeDriver: true,
      }),
    );
    loop.start();
    return () => loop.stop();
  }, [spin]);
  const rotate = spin.interpolate({ inputRange: [0, 1], outputRange: ['0deg', '360deg'] });
  return (
    <Animated.View
      style={{
        width: 18,
        height: 18,
        borderRadius: 999,
        borderWidth: 2,
        borderColor: pal.muted,
        borderTopColor: 'transparent',
        transform: [{ rotate }],
      }}
    />
  );
}
