// Cadence · Знания — long-form serif article reader.
// Ported from design_handoff_cadence_app/learn/learn.jsx (ArticleReader).
import React from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Eyebrow, Press } from '../../components/primitives';
import { Icon, IconName } from '../../components/Icon';
import { LEARN, LearnBlock } from './data';
import { ArticleRow, tintOf } from './LearnScreen';

// Emulates CSS margin collapsing from the prototype: spacing between blocks is the
// max of the previous block's bottom margin and the current block's top margin.
function spacingFor(prev: LearnBlock['t'], cur: LearnBlock['t']): number {
  if (cur === 'h') return 28;
  if (cur === 'quote') return 28;
  if (cur === 'key') return 26;
  // p
  if (prev === 'quote') return 28;
  if (prev === 'key') return 26;
  return 18; // after p or h
}

function Block({ b, mt }: { b: LearnBlock; mt: number }) {
  if (b.t === 'p') {
    return (
      <Text
        style={{
          fontFamily: F.body,
          fontSize: 16,
          color: pal.ink2,
          lineHeight: 16 * 1.62,
          marginTop: mt,
        }}
      >
        {b.x}
      </Text>
    );
  }
  if (b.t === 'h') {
    return (
      <Text
        style={{
          fontFamily: F.display,
          fontSize: 25,
          color: pal.ink,
          lineHeight: 25 * 1.12,
          letterSpacing: 25 * -0.016,
          marginTop: mt,
        }}
      >
        {b.x}
      </Text>
    );
  }
  if (b.t === 'quote') {
    return (
      <View
        style={{
          marginTop: mt,
          paddingLeft: 18,
          borderLeftWidth: 3,
          borderLeftColor: C.sand500,
        }}
      >
        <Text
          style={{
            fontFamily: F.displayItalic,
            fontSize: 24,
            color: C.forest700,
            lineHeight: 24 * 1.28,
            letterSpacing: 24 * -0.01,
          }}
        >
          {b.x}
        </Text>
      </View>
    );
  }
  if (b.t === 'key') {
    return (
      <View
        style={{
          marginTop: mt,
          backgroundColor: C.forest800,
          borderRadius: 20,
          padding: 20,
          shadowColor: C.forest900,
          shadowOpacity: 0.18,
          shadowRadius: 24,
          shadowOffset: { width: 0, height: 8 },
          elevation: 8,
        }}
      >
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 9, marginBottom: 14 }}>
          <View
            style={{
              width: 30,
              height: 30,
              borderRadius: 9,
              backgroundColor: 'rgba(212,165,116,.2)',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Icon name="sparkles" size={16} color={C.sand300} />
          </View>
          <Text
            style={{
              fontFamily: F.bodySemibold,
              fontSize: 11,
              letterSpacing: 11 * 0.12,
              textTransform: 'uppercase',
              color: C.sand300,
            }}
          >
            {b.title}
          </Text>
        </View>
        <View style={{ gap: 11 }}>
          {b.items.map((it, j) => (
            <View key={j} style={{ flexDirection: 'row', gap: 10, alignItems: 'flex-start' }}>
              <View style={{ width: 20, marginTop: 2 }}>
                <Icon name="check" size={16} strokeWidth={2} color={C.sand300} />
              </View>
              <Text
                style={{
                  flex: 1,
                  fontFamily: F.body,
                  fontSize: 14.5,
                  color: 'rgba(246,241,234,.92)',
                  lineHeight: 14.5 * 1.45,
                }}
              >
                {it}
              </Text>
            </View>
          ))}
        </View>
      </View>
    );
  }
  return null;
}

export function ArticleScreen({
  articleId,
  onBack,
  onOpenArticle,
}: {
  articleId: string;
  onBack: () => void;
  onOpenArticle: (id: string) => void;
}) {
  const insets = useSafeAreaInsets();
  const a = LEARN.byId(articleId);
  if (!a) return null;
  const m = LEARN.catMeta(a.cat)!;
  const tint = tintOf(a.cat);
  const keepReading = LEARN.ARTICLES.filter((x) => x.id !== a.id).slice(0, 3);

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Floating back */}
      <View style={{ position: 'absolute', top: insets.top + 6, left: 12, zIndex: 30 }}>
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

      <ScrollView
        style={{ flex: 1 }}
        contentContainerStyle={{ paddingBottom: 48 + insets.bottom }}
        showsVerticalScrollIndicator={false}
      >
        {/* Editorial hero — tinted band */}
        <View
          style={{
            backgroundColor: tint.soft,
            paddingTop: insets.top + 60,
            paddingHorizontal: 20,
            paddingBottom: 26,
          }}
        >
          <View
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              gap: 7,
              alignSelf: 'flex-start',
              marginBottom: 14,
            }}
          >
            <Icon name={m.icon as IconName} size={14} color={tint.fg} />
            <Text
              style={{
                fontFamily: F.bodySemibold,
                fontSize: 11,
                letterSpacing: 11 * 0.1,
                textTransform: 'uppercase',
                color: tint.fg,
              }}
            >
              {a.eyebrow || m.label}
            </Text>
          </View>
          <Text
            style={{
              fontFamily: F.display,
              fontSize: 36,
              color: C.ink900,
              lineHeight: 36 * 1.06,
              letterSpacing: 36 * -0.02,
            }}
          >
            {a.title}
          </Text>
          <Text
            style={{
              fontFamily: F.body,
              fontSize: 15,
              color: C.ink600,
              lineHeight: 15 * 1.5,
              marginTop: 12,
            }}
          >
            {a.dek}
          </Text>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 16 }}>
            <Text
              style={{
                fontFamily: F.mono,
                fontSize: 12,
                color: tint.fg,
                fontVariant: ['tabular-nums'],
              }}
            >
              {a.readMin} мин чтения
            </Text>
          </View>
        </View>

        {/* Body */}
        <View style={{ paddingTop: 6, paddingHorizontal: 22, paddingBottom: 8 }}>
          {a.blocks.map((b, i) => (
            <Block key={i} b={b} mt={i === 0 ? 0 : spacingFor(a.blocks[i - 1].t, b.t)} />
          ))}
        </View>

        {/* End rule */}
        <Text
          style={{
            textAlign: 'center',
            color: pal.placeholder,
            fontFamily: F.displayItalic,
            fontSize: 22,
            paddingTop: 8,
            paddingBottom: 20,
          }}
        >
          —
        </Text>

        {/* Keep reading */}
        <View style={{ paddingHorizontal: 16 }}>
          <Eyebrow color={pal.subtle} style={{ paddingHorizontal: 4, paddingBottom: 6 }}>
            Читать дальше
          </Eyebrow>
          <View
            style={{
              backgroundColor: pal.paper,
              borderRadius: 18,
              borderWidth: 1,
              borderColor: pal.hairline,
              overflow: 'hidden',
              paddingHorizontal: 14,
            }}
          >
            {keepReading.map((x, i) => (
              <ArticleRow
                key={x.id}
                a={x}
                onOpen={onOpenArticle}
                last={i === keepReading.length - 1}
              />
            ))}
          </View>
        </View>
      </ScrollView>
    </View>
  );
}
