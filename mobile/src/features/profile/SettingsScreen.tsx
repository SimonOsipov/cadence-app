// Cadence — Настройки (Settings) sub-screen of the Profile.
// Ported from design_handoff_cadence_app/profile/profile.jsx (ProfileSettings).
// Also exports the small profile atoms shared with ProfileScreen:
// PrefToggle, SegControl, SectionHead, NavRow, GroupCard.
import React, { useEffect, useRef } from 'react';
import { Animated, Easing, Pressable, ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, SHADOW } from '../../theme';
import { Eyebrow, IconBtn, Press } from '../../components/primitives';
import { Icon, IconName } from '../../components/Icon';

// ── Brand toggle switch (forest when on) ────────────────────────────
export function PrefToggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  const anim = useRef(new Animated.Value(on ? 1 : 0)).current;
  useEffect(() => {
    Animated.timing(anim, {
      toValue: on ? 1 : 0,
      duration: 220,
      easing: Easing.bezier(0.34, 1.56, 0.64, 1),
      useNativeDriver: true,
    }).start();
  }, [on, anim]);
  return (
    <Pressable
      onPress={() => onChange(!on)}
      accessibilityRole="switch"
      accessibilityState={{ checked: on }}
      style={{
        width: 46,
        height: 28,
        borderRadius: 999,
        backgroundColor: on ? C.forest700 : pal.border,
        flexShrink: 0,
      }}
    >
      <Animated.View
        style={{
          position: 'absolute',
          top: 3,
          left: 3,
          width: 22,
          height: 22,
          borderRadius: 999,
          backgroundColor: C.paper,
          shadowColor: '#2e2618',
          shadowOpacity: 0.28,
          shadowRadius: 3,
          shadowOffset: { width: 0, height: 1 },
          elevation: 2,
          transform: [{ translateX: anim.interpolate({ inputRange: [0, 1], outputRange: [0, 18] }) }],
        }}
      />
    </Pressable>
  );
}

// ── Brand segmented control ─────────────────────────────────────────
export function SegControl({
  value,
  options,
  onChange,
}: {
  value: string;
  options: { id: string; label: string }[];
  onChange: (id: string) => void;
}) {
  return (
    <View
      style={{
        flexDirection: 'row',
        gap: 4,
        backgroundColor: pal.sunk,
        padding: 3,
        borderRadius: 999,
      }}
    >
      {options.map((o) => {
        const on = value === o.id;
        return (
          <Press
            key={o.id}
            onPress={() => onChange(o.id)}
            style={{
              flex: 1,
              paddingVertical: 8,
              paddingHorizontal: 10,
              borderRadius: 999,
              backgroundColor: on ? C.forest700 : 'transparent',
              alignItems: 'center',
            }}
          >
            <Text
              numberOfLines={1}
              style={{ fontFamily: F.bodyMedium, fontSize: 13, color: on ? C.cream : pal.muted }}
            >
              {o.label}
            </Text>
          </Press>
        );
      })}
    </View>
  );
}

// ── Section eyebrow + optional trailing action ──────────────────────
export function SectionHead({
  children,
  action,
  onAction,
}: {
  children: React.ReactNode;
  action?: string;
  onAction?: () => void;
}) {
  return (
    <View
      style={{
        flexDirection: 'row',
        justifyContent: 'space-between',
        alignItems: 'baseline',
        paddingHorizontal: 4,
        paddingBottom: 10,
      }}
    >
      <Eyebrow color={pal.subtle}>{children}</Eyebrow>
      {action ? (
        <Press onPress={onAction}>
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
            {action}
          </Text>
        </Press>
      ) : null}
    </View>
  );
}

// ── Card group container for stacked rows ───────────────────────────
export function GroupCard({ children }: { children: React.ReactNode }) {
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 18,
        borderWidth: 1,
        borderColor: pal.hairline,
        ...SHADOW.sm,
      }}
    >
      {children}
    </View>
  );
}

// ── A tappable list row inside a card group ─────────────────────────
export function NavRow({
  iconName,
  iconTone = 'forest',
  title,
  sub,
  trail,
  onPress,
  last,
}: {
  iconName: IconName;
  iconTone?: 'forest' | 'sand' | 'linen' | 'danger';
  title: string;
  sub?: string;
  trail?: string;
  onPress?: () => void;
  last?: boolean;
}) {
  const tones = {
    forest: { bg: C.forest50, fg: C.forest700 },
    sand: { bg: C.sand100, fg: C.sand900 },
    linen: { bg: pal.sunk, fg: pal.ink2 },
    danger: { bg: C.dangerBg, fg: C.danger },
  }[iconTone];
  const content = (
    <View
      style={{
        flexDirection: 'row',
        gap: 14,
        alignItems: 'center',
        paddingVertical: 13,
        paddingHorizontal: 14,
        borderBottomWidth: last ? 0 : 1,
        borderBottomColor: pal.hairline,
      }}
    >
      <View
        style={{
          width: 40,
          height: 40,
          borderRadius: 12,
          backgroundColor: tones.bg,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={iconName} size={20} color={tones.fg} />
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text
          numberOfLines={1}
          style={{
            fontFamily: F.bodyMedium,
            fontSize: 14,
            color: iconTone === 'danger' ? C.danger : pal.ink,
          }}
        >
          {title}
        </Text>
        {sub ? (
          <Text
            numberOfLines={1}
            style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}
          >
            {sub}
          </Text>
        ) : null}
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
        {trail ? (
          <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.subtle }}>{trail}</Text>
        ) : null}
        {onPress ? <Icon name="chevron-right" size={16} color={pal.placeholder} /> : null}
      </View>
    </View>
  );
  if (onPress) return <Press onPress={onPress}>{content}</Press>;
  return content;
}

// ── Toggle row inside a group card ──────────────────────────────────
function ToggleRow({
  title,
  sub,
  on,
  onChange,
  last,
}: {
  title: string;
  sub?: string;
  on: boolean;
  onChange: (v: boolean) => void;
  last?: boolean;
}) {
  return (
    <View
      style={{
        flexDirection: 'row',
        alignItems: 'center',
        gap: 14,
        padding: 14,
        borderBottomWidth: last ? 0 : 1,
        borderBottomColor: pal.hairline,
      }}
    >
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: pal.ink }}>{title}</Text>
        {sub ? (
          <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}>
            {sub}
          </Text>
        ) : null}
      </View>
      <PrefToggle on={on} onChange={onChange} />
    </View>
  );
}

// ════════════════════════════════════════════════════════════════════
// Settings sub-screen — navigable, working toggles & segments.
// ════════════════════════════════════════════════════════════════════
export function SettingsScreen({ onBack }: { onBack: () => void }) {
  const insets = useSafeAreaInsets();
  const [doseReminders, setDoseReminders] = React.useState(true);
  const [leadTime, setLeadTime] = React.useState('30');
  const [mealReminders, setMealReminders] = React.useState(false);
  const [units, setUnits] = React.useState('kg');
  const [timeFmt, setTimeFmt] = React.useState('24');
  const [weeklyReport, setWeeklyReport] = React.useState(true);
  const [teamMessages, setTeamMessages] = React.useState(true);
  const [reorderAlerts, setReorderAlerts] = React.useState(true);
  const [faceId, setFaceId] = React.useState(true);

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <ScrollView
        contentContainerStyle={{
          paddingTop: insets.top + 6,
          paddingBottom: insets.bottom + 48,
        }}
        showsVerticalScrollIndicator={false}
      >
        {/* Top bar */}
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
          <IconBtn name="chevron-left" bg={pal.sunk} color={pal.ink2} onPress={onBack} />
          <Text style={{ fontFamily: F.bodySemibold, fontSize: 15, color: pal.ink }}>
            Настройки
          </Text>
        </View>

        {/* Header */}
        <View style={{ paddingTop: 6, paddingHorizontal: 24, paddingBottom: 18 }}>
          <Text
            style={{
              fontFamily: F.display,
              fontSize: 34,
              color: pal.ink,
              lineHeight: 34,
              letterSpacing: 34 * -0.018,
            }}
          >
            Ваш <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>ритм</Text>, как
            удобно.
          </Text>
        </View>

        {/* Reminders */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
          <SectionHead>Напоминания</SectionHead>
          <GroupCard>
            <ToggleRow
              title="Напоминания о дозах"
              sub="Подскажем перед каждой инъекцией"
              on={doseReminders}
              onChange={setDoseReminders}
            />
            <View
              pointerEvents={doseReminders ? 'auto' : 'none'}
              style={{
                padding: 14,
                borderBottomWidth: 1,
                borderBottomColor: pal.hairline,
                opacity: doseReminders ? 1 : 0.4,
              }}
            >
              <Text
                style={{
                  fontFamily: F.bodyMedium,
                  fontSize: 14,
                  color: pal.ink,
                  marginBottom: 10,
                }}
              >
                За сколько напоминать
              </Text>
              <SegControl
                value={leadTime}
                onChange={setLeadTime}
                options={[
                  { id: '15', label: '15 мин' },
                  { id: '30', label: '30 мин' },
                  { id: '60', label: '1 час' },
                ]}
              />
            </View>
            <ToggleRow
              title="Напоминания о приёме пищи"
              sub="Тихий нудж, если день без записей"
              on={mealReminders}
              onChange={setMealReminders}
              last
            />
          </GroupCard>
        </View>

        {/* Units */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
          <SectionHead>Единицы измерения</SectionHead>
          <GroupCard>
            <View style={{ padding: 14, borderBottomWidth: 1, borderBottomColor: pal.hairline }}>
              <Text
                style={{
                  fontFamily: F.bodyMedium,
                  fontSize: 14,
                  color: pal.ink,
                  marginBottom: 10,
                }}
              >
                Вес
              </Text>
              <SegControl
                value={units}
                onChange={setUnits}
                options={[
                  { id: 'kg', label: 'Килограммы' },
                  { id: 'lb', label: 'Фунты' },
                ]}
              />
            </View>
            <View style={{ padding: 14 }}>
              <Text
                style={{
                  fontFamily: F.bodyMedium,
                  fontSize: 14,
                  color: pal.ink,
                  marginBottom: 10,
                }}
              >
                Время
              </Text>
              <SegControl
                value={timeFmt}
                onChange={setTimeFmt}
                options={[
                  { id: '24', label: '24 часа' },
                  { id: '12', label: 'AM · PM' },
                ]}
              />
            </View>
          </GroupCard>
        </View>

        {/* Notifications */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
          <SectionHead>Уведомления</SectionHead>
          <GroupCard>
            <ToggleRow
              title="Недельный отчёт"
              sub="Сводка трендов по воскресеньям"
              on={weeklyReport}
              onChange={setWeeklyReport}
            />
            <ToggleRow
              title="Сообщения от команды"
              sub="Доктор, диетолог, медсестра"
              on={teamMessages}
              onChange={setTeamMessages}
            />
            <ToggleRow
              title="Заказ флаконов"
              sub="Когда запас подходит к концу"
              on={reorderAlerts}
              onChange={setReorderAlerts}
              last
            />
          </GroupCard>
        </View>

        {/* Privacy */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
          <SectionHead>Приватность и данные</SectionHead>
          <GroupCard>
            <ToggleRow
              title="Face ID для входа"
              sub="Блокировать приложение при выходе"
              on={faceId}
              onChange={setFaceId}
            />
            <NavRow
              iconName="document-text"
              iconTone="linen"
              title="Экспорт данных"
              sub="CSV · все записи курса"
              onPress={() => {}}
            />
            <NavRow iconName="trash" iconTone="danger" title="Удалить аккаунт" onPress={() => {}} last />
          </GroupCard>
        </View>
      </ScrollView>
    </View>
  );
}
