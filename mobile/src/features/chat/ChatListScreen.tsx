// Cadence — Chat landing (care-team thread list).
// Ported from design_handoff_cadence_app/chat/chat-landing.jsx (ChatLanding).
import React from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Eyebrow, Press, Title, TitleEm } from '../../components/primitives';
import { Icon } from '../../components/Icon';
import { CARE_TEAM } from './data';

export function ChatListScreen({
  onBack,
  onOpenThread,
}: {
  onBack: () => void;
  onOpenThread: (id: string) => void;
}) {
  const insets = useSafeAreaInsets();
  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <ScrollView
        contentContainerStyle={{ paddingTop: insets.top + 6, paddingBottom: 30 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Top bar */}
        <View
          style={{
            paddingTop: 8,
            paddingHorizontal: 16,
            paddingBottom: 10,
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
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
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.muted }}>
            Команда
          </Text>
          <View style={{ width: 40 }} />
        </View>

        {/* Hero */}
        <View style={{ paddingTop: 4, paddingHorizontal: 24, paddingBottom: 20 }}>
          <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
            Связь с командой
          </Eyebrow>
          <Title size={32}>
            Кому <TitleEm size={32}>напишем</TitleEm>?
          </Title>
        </View>

        {/* Thread list */}
        <View style={{ paddingHorizontal: 16, gap: 10 }}>
          {CARE_TEAM.map((t) => (
            <Press
              key={t.id}
              onPress={() => onOpenThread(t.id)}
              style={{
                flexDirection: 'row',
                alignItems: 'center',
                gap: 14,
                backgroundColor: pal.paper,
                borderWidth: 1,
                borderColor: pal.hairline,
                borderRadius: 18,
                padding: 14,
                shadowColor: '#2e2618',
                shadowOpacity: 0.04,
                shadowRadius: 6,
                shadowOffset: { width: 0, height: 2 },
                elevation: 1,
              }}
            >
              <View
                style={{
                  width: 52,
                  height: 52,
                  borderRadius: 999,
                  backgroundColor: t.avatarBg,
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Text style={{ fontFamily: F.displayItalic, fontSize: 22, color: t.avatarFg }}>
                  {t.initial}
                </Text>
              </View>
              <View style={{ flex: 1, minWidth: 0 }}>
                <View
                  style={{
                    flexDirection: 'row',
                    justifyContent: 'space-between',
                    alignItems: 'baseline',
                    gap: 8,
                    marginBottom: 4,
                  }}
                >
                  <Text
                    numberOfLines={1}
                    style={{
                      flexShrink: 1,
                      fontFamily: F.bodyMedium,
                      fontSize: 14,
                      color: pal.ink,
                    }}
                  >
                    {t.name}
                  </Text>
                  <Text
                    style={{
                      fontFamily: F.mono,
                      fontSize: 10.5,
                      color: pal.subtle,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {t.preview.time}
                  </Text>
                </View>
                <Text
                  style={{
                    fontFamily: F.bodyMedium,
                    fontSize: 11.5,
                    color: C.sand700,
                    marginBottom: 4,
                  }}
                >
                  {t.role}
                </Text>
                <Text
                  numberOfLines={1}
                  style={{
                    fontFamily: F.body,
                    fontSize: 12.5,
                    color: pal.muted,
                    lineHeight: 12.5 * 1.4,
                  }}
                >
                  {t.preview.text}
                </Text>
              </View>
              {t.preview.unread > 0 ? (
                <View
                  style={{
                    width: 22,
                    height: 22,
                    borderRadius: 999,
                    backgroundColor: C.forest700,
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <Text
                    style={{
                      fontFamily: F.monoMedium,
                      fontSize: 11,
                      color: C.cream,
                      fontVariant: ['tabular-nums'],
                    }}
                  >
                    {t.preview.unread}
                  </Text>
                </View>
              ) : (
                <Icon name="chevron-right" size={16} color={pal.subtle} />
              )}
            </Press>
          ))}
        </View>
      </ScrollView>
    </View>
  );
}
