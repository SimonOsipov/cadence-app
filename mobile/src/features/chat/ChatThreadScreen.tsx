// Cadence — Chat conversation screen.
// Ported from design_handoff_cadence_app/chat/chat-conversation.jsx (ChatConversation).
// Full-screen thread with date-grouped bubbles, inline data/photo cards,
// share chips above the input, and a "+" attach sheet.
import React, { useRef, useState } from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  Text,
  TextInput,
  View,
} from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';
import Svg, { Defs, RadialGradient, Rect, Stop } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Eyebrow, Press, Title, TitleEm } from '../../components/primitives';
import { Icon, IconName } from '../../components/Icon';
import { Sheet } from '../../components/shared';
import {
  CARE_TEAM,
  CHAT_MESSAGES,
  SHARE_OPTIONS,
  chatDayLabel,
  CareTeamMember,
  ChatDataCard,
  ChatMessage,
  ShareOption,
} from './data';

// Local message type — the conversation also carries photo messages.
interface Msg extends Omit<ChatMessage, 'kind'> {
  kind: 'text' | 'data' | 'photo';
  photo?: ChatDataCard;
}

interface DayGroup {
  day: number;
  messages: Msg[];
}

function groupByDay(messages: Msg[]): DayGroup[] {
  const groups: DayGroup[] = [];
  let cur: DayGroup | null = null;
  for (const m of messages) {
    if (!cur || cur.day !== m.day) {
      cur = { day: m.day, messages: [] };
      groups.push(cur);
    }
    cur.messages.push(m);
  }
  return groups;
}

export function ChatThreadScreen({
  threadId,
  onBack,
  onBackToList,
}: {
  threadId: string;
  onBack: () => void;
  onBackToList: () => void;
}) {
  const insets = useSafeAreaInsets();
  const team = CARE_TEAM.find((t) => t.id === threadId) || CARE_TEAM[0];

  const [messages, setMessages] = useState<Msg[]>(CHAT_MESSAGES[threadId] || []);
  const [draft, setDraft] = useState('');
  const [attachSheetOpen, setAttachSheetOpen] = useState(false);

  const scrollRef = useRef<ScrollView>(null);

  const sendText = () => {
    const text = draft.trim();
    if (!text) return;
    setMessages((prev) => [
      ...prev,
      { id: `u-${Date.now()}`, from: 'me', kind: 'text', day: 0, time: 'сейчас', text },
    ]);
    setDraft('');
  };

  const insertShare = (opt: ShareOption) => {
    if (opt.id === 'photo') {
      setMessages((prev) => [
        ...prev,
        { id: `u-p-${Date.now()}`, from: 'me', kind: 'photo', day: 0, time: 'сейчас', photo: opt.data },
      ]);
    } else {
      setMessages((prev) => [
        ...prev,
        { id: `u-d-${Date.now()}`, from: 'me', kind: 'data', day: 0, time: 'сейчас', data: opt.data },
      ]);
    }
    setAttachSheetOpen(false);
  };

  const groups = groupByDay(messages);

  return (
    <KeyboardAvoidingView
      style={{ flex: 1, backgroundColor: pal.bg }}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}
    >
      {/* Top bar */}
      <View
        style={{
          paddingTop: insets.top + 6,
          paddingHorizontal: 12,
          paddingBottom: 8,
          flexDirection: 'row',
          alignItems: 'center',
          gap: 10,
          backgroundColor: pal.bg,
          borderBottomWidth: 1,
          borderBottomColor: pal.hairline,
        }}
      >
        <Press
          onPress={onBack}
          style={{
            width: 40,
            height: 40,
            borderRadius: 999,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name="chevron-left" size={22} color={pal.ink2} />
        </Press>
        <Press
          onPress={onBackToList}
          style={{ flex: 1, minWidth: 0, flexDirection: 'row', alignItems: 'center', gap: 10 }}
        >
          <View
            style={{
              width: 36,
              height: 36,
              borderRadius: 999,
              backgroundColor: team.avatarBg,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Text style={{ fontFamily: F.displayItalic, fontSize: 17, color: team.avatarFg }}>
              {team.initial}
            </Text>
          </View>
          <View style={{ flex: 1, minWidth: 0 }}>
            <Text
              numberOfLines={1}
              style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: pal.ink }}
            >
              {team.name}
            </Text>
            <Text
              numberOfLines={1}
              style={{ fontFamily: F.body, fontSize: 11.5, color: C.sand700 }}
            >
              {team.role}
            </Text>
          </View>
        </Press>
        <Press
          style={{
            width: 40,
            height: 40,
            borderRadius: 999,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name="ellipsis-horizontal" size={20} color={pal.ink2} />
        </Press>
      </View>

      {/* Message scroll area */}
      <ScrollView
        ref={scrollRef}
        style={{ flex: 1 }}
        contentContainerStyle={{
          flexGrow: 1,
          paddingTop: 16,
          paddingHorizontal: 14,
          paddingBottom: 10,
          gap: 4,
        }}
        showsVerticalScrollIndicator={false}
        onContentSizeChange={() => scrollRef.current?.scrollToEnd({ animated: false })}
      >
        {groups.length === 0 && (
          <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center' }}>
            <Text
              style={{
                textAlign: 'center',
                maxWidth: 240,
                color: pal.muted,
                fontFamily: F.body,
                fontSize: 13,
              }}
            >
              Начните разговор — напишите сообщение ниже.
            </Text>
          </View>
        )}
        {groups.map((g, gi) => (
          <React.Fragment key={gi}>
            <DateSeparator day={g.day} />
            {g.messages.map((m, mi) => {
              const prev = g.messages[mi - 1];
              const next = g.messages[mi + 1];
              const showAvatar = m.from === 'doctor' && (!next || next.from !== 'doctor');
              const isLastInRun = !next || next.from !== m.from;
              return (
                <MessageRow
                  key={m.id}
                  msg={m}
                  team={team}
                  showAvatar={showAvatar}
                  isLastInRun={isLastInRun}
                />
              );
            })}
          </React.Fragment>
        ))}
      </ScrollView>

      {/* Share chips row — quick data inserts */}
      <View style={{ borderTopWidth: 1, borderTopColor: pal.hairline }}>
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={{
            paddingVertical: 6,
            paddingHorizontal: 14,
            gap: 6,
          }}
        >
          {SHARE_OPTIONS.map((opt) => (
            <Press
              key={opt.id}
              onPress={() => insertShare(opt)}
              style={{
                paddingVertical: 7,
                paddingHorizontal: 12,
                borderRadius: 999,
                backgroundColor: pal.sunk,
                borderWidth: 1,
                borderColor: pal.hairline,
                flexDirection: 'row',
                alignItems: 'center',
                gap: 6,
              }}
            >
              <Icon name={opt.icon as IconName} size={14} color={C.forest700} />
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: pal.ink2 }}>
                {opt.label}
              </Text>
            </Press>
          ))}
        </ScrollView>
      </View>

      {/* Input bar */}
      <View
        style={{
          paddingTop: 8,
          paddingHorizontal: 12,
          paddingBottom: Math.max(insets.bottom, 16) + 8,
          flexDirection: 'row',
          alignItems: 'flex-end',
          gap: 8,
          backgroundColor: pal.bg,
        }}
      >
        <Press
          onPress={() => setAttachSheetOpen(true)}
          style={{
            width: 40,
            height: 40,
            borderRadius: 999,
            backgroundColor: pal.sunk,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name="plus" size={20} strokeWidth={1.7} color={pal.ink2} />
        </Press>
        <TextInput
          value={draft}
          onChangeText={setDraft}
          placeholder="Сообщение…"
          placeholderTextColor={pal.placeholder}
          multiline
          style={{
            flex: 1,
            paddingTop: 10,
            paddingBottom: 10,
            paddingHorizontal: 14,
            borderRadius: 20,
            backgroundColor: pal.paper,
            borderWidth: 1,
            borderColor: pal.border,
            fontFamily: F.body,
            fontSize: 14,
            color: pal.ink,
            lineHeight: 14 * 1.4,
            minHeight: 40,
            maxHeight: 96,
          }}
        />
        <Press
          onPress={sendText}
          disabled={!draft.trim()}
          style={{
            width: 40,
            height: 40,
            borderRadius: 999,
            backgroundColor: draft.trim() ? C.forest700 : pal.sunk,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name="paper-airplane" size={18} color={draft.trim() ? C.cream : pal.subtle} />
        </Press>
      </View>

      {/* Attach sheet */}
      <Sheet open={attachSheetOpen} onClose={() => setAttachSheetOpen(false)}>
        <View style={{ marginBottom: 14 }}>
          <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
            Прикрепить
          </Eyebrow>
          <Title size={24}>
            Что <TitleEm size={24}>отправляем</TitleEm>?
          </Title>
        </View>
        <View style={{ gap: 8 }}>
          {SHARE_OPTIONS.map((opt) => (
            <Press
              key={opt.id}
              onPress={() => insertShare(opt)}
              style={{
                flexDirection: 'row',
                alignItems: 'center',
                gap: 12,
                backgroundColor: pal.paper,
                borderWidth: 1,
                borderColor: pal.hairline,
                borderRadius: 14,
                padding: 12,
              }}
            >
              <View
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 12,
                  backgroundColor: opt.id === 'photo' ? C.sand100 : C.forest50,
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Icon
                  name={opt.icon as IconName}
                  size={20}
                  color={opt.id === 'photo' ? C.sand700 : C.forest700}
                />
              </View>
              <View style={{ flex: 1, minWidth: 0 }}>
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: pal.ink }}>
                  {opt.label}
                </Text>
                {opt.data && opt.data.title ? (
                  <Text
                    style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 2 }}
                  >
                    {opt.data.title}
                  </Text>
                ) : null}
              </View>
              <Icon name="arrow-right" size={16} color={pal.subtle} />
            </Press>
          ))}
        </View>
      </Sheet>
    </KeyboardAvoidingView>
  );
}

// ── DateSeparator ───────────────────────────────────────────────────

function DateSeparator({ day }: { day: number }) {
  return (
    <View
      style={{
        flexDirection: 'row',
        alignItems: 'center',
        gap: 10,
        paddingVertical: 8,
        paddingHorizontal: 4,
        marginVertical: 4,
      }}
    >
      <View style={{ flex: 1, height: 1, backgroundColor: pal.hairline }} />
      <Text
        style={{
          fontFamily: F.bodyMedium,
          fontSize: 11,
          letterSpacing: 11 * 0.06,
          textTransform: 'uppercase',
          color: pal.subtle,
        }}
      >
        {chatDayLabel(day)}
      </Text>
      <View style={{ flex: 1, height: 1, backgroundColor: pal.hairline }} />
    </View>
  );
}

// ── MessageRow ──────────────────────────────────────────────────────

function MessageRow({
  msg,
  team,
  showAvatar,
  isLastInRun,
}: {
  msg: Msg;
  team: CareTeamMember;
  showAvatar: boolean;
  isLastInRun: boolean;
}) {
  const isMe = msg.from === 'me';
  // iMessage-style corner pinching toward the avatar
  const radii = isMe
    ? { tl: 18, tr: 18, br: isLastInRun ? 4 : 18, bl: 18 }
    : { tl: 18, tr: 18, br: 18, bl: isLastInRun ? 4 : 18 };

  return (
    <View
      style={{
        flexDirection: 'row',
        gap: 8,
        marginBottom: isLastInRun ? 6 : 2,
        justifyContent: isMe ? 'flex-end' : 'flex-start',
      }}
    >
      {!isMe && (
        <View
          style={{
            width: 28,
            height: 28,
            borderRadius: 999,
            backgroundColor: showAvatar ? team.avatarBg : 'transparent',
            alignItems: 'center',
            justifyContent: 'center',
            alignSelf: 'flex-end',
            opacity: showAvatar ? 1 : 0,
          }}
        >
          <Text style={{ fontFamily: F.displayItalic, fontSize: 13, color: team.avatarFg }}>
            {team.initial}
          </Text>
        </View>
      )}
      <View style={{ maxWidth: '78%', alignItems: isMe ? 'flex-end' : 'flex-start' }}>
        {msg.kind === 'text' && (
          <View
            style={{
              backgroundColor: isMe ? C.forest700 : pal.paper,
              paddingVertical: 10,
              paddingHorizontal: 14,
              borderTopLeftRadius: radii.tl,
              borderTopRightRadius: radii.tr,
              borderBottomRightRadius: radii.br,
              borderBottomLeftRadius: radii.bl,
              ...(isMe ? null : { borderWidth: 1, borderColor: pal.hairline }),
            }}
          >
            <Text
              style={{
                fontFamily: F.body,
                fontSize: 14,
                lineHeight: 14 * 1.45,
                color: isMe ? C.cream : pal.ink,
              }}
            >
              {msg.text}
            </Text>
          </View>
        )}
        {msg.kind === 'data' && msg.data ? <DataCard data={msg.data} /> : null}
        {msg.kind === 'photo' && msg.photo ? <PhotoCard photo={msg.photo} /> : null}
        {isLastInRun && (
          <Text
            style={{
              fontFamily: F.mono,
              fontSize: 10,
              color: pal.subtle,
              fontVariant: ['tabular-nums'],
              marginTop: 4,
              paddingHorizontal: 4,
            }}
          >
            {msg.time}
          </Text>
        )}
      </View>
    </View>
  );
}

// ── DataCard — shared app data inside a message ─────────────────────

function DataCard({ data }: { data: ChatDataCard }) {
  const sandTone = data.kind === 'meal';
  const accent = sandTone ? C.sand700 : C.forest700;
  const accentBg = sandTone ? C.sand100 : C.forest50;

  // Cards always read as content, not as conversational text — keep paper-toned
  // on both sides of the conversation.
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderWidth: 1,
        borderColor: pal.hairline,
        borderRadius: 16,
        padding: 14,
        shadowColor: '#2e2618',
        shadowOpacity: 0.05,
        shadowRadius: 3,
        shadowOffset: { width: 0, height: 1 },
        elevation: 1,
        minWidth: 220,
        maxWidth: '100%',
      }}
    >
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6, marginBottom: 6 }}>
        <View
          style={{
            paddingVertical: 2,
            paddingHorizontal: 7,
            borderRadius: 999,
            backgroundColor: accentBg,
          }}
        >
          <Text
            style={{
              fontFamily: F.bodySemibold,
              fontSize: 9.5,
              letterSpacing: 9.5 * 0.08,
              textTransform: 'uppercase',
              color: accent,
            }}
          >
            Поделились
          </Text>
        </View>
        <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>{data.eyebrow}</Text>
      </View>
      <Text
        style={{
          fontFamily: F.display,
          fontSize: 22,
          lineHeight: 22 * 1.1,
          letterSpacing: 22 * -0.014,
          color: pal.ink,
          marginBottom: 4,
        }}
      >
        {data.title}
      </Text>
      <Text style={{ fontFamily: F.body, fontSize: 12.5, color: pal.ink2, marginBottom: 2 }}>
        {data.delta}
      </Text>
      <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>{data.meta}</Text>
    </View>
  );
}

// ── PhotoCard — image placeholder ───────────────────────────────────

const PHOTO_W = 200;
const PHOTO_H = PHOTO_W * 0.75; // 4:3

function PhotoCard({ photo }: { photo: ChatDataCard }) {
  return (
    <View
      style={{
        width: PHOTO_W,
        maxWidth: '100%',
        borderRadius: 16,
        overflow: 'hidden',
        borderWidth: 1,
        borderColor: pal.hairline,
        backgroundColor: pal.paper,
      }}
    >
      <View style={{ width: '100%', height: PHOTO_H }}>
        <LinearGradient
          colors={[C.sand500, C.sand700]}
          start={{ x: 0, y: 0 }}
          end={{ x: 1, y: 1 }}
          style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0 }}
        />
        <Svg
          width="100%"
          height="100%"
          viewBox={`0 0 ${PHOTO_W} ${PHOTO_H}`}
          style={{ position: 'absolute', top: 0, left: 0 }}
        >
          <Defs>
            <RadialGradient id="photoGlow" cx="30%" cy="40%" r="50%">
              <Stop offset="0%" stopColor="#fff0d2" stopOpacity={0.35} />
              <Stop offset="100%" stopColor="#fff0d2" stopOpacity={0} />
            </RadialGradient>
          </Defs>
          <Rect x={0} y={0} width={PHOTO_W} height={PHOTO_H} fill="url(#photoGlow)" />
        </Svg>
        <View
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name="camera" size={28} color={C.cream} />
        </View>
      </View>
      <Text
        style={{
          paddingVertical: 8,
          paddingHorizontal: 12,
          fontFamily: F.body,
          fontSize: 12,
          color: pal.ink2,
        }}
      >
        {photo.caption}
      </Text>
    </View>
  );
}
