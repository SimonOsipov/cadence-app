// Cadence · Знания — education library landing.
// Ported from design_handoff_cadence_app/learn/learn.jsx (LibraryScreen).
import React, { useState } from 'react';
import { ScrollView, Text, TextInput, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, SHADOW } from '../../theme';
import { Eyebrow, Press, IconBtn } from '../../components/primitives';
import { Icon, IconName } from '../../components/Icon';
import { LEARN, LearnArticle, LearnCategory } from './data';

// Category tint → swatch (values from the prototype source).
export const TINTS: Record<string, { fg: string; soft: string }> = {
  forest: { fg: '#2d5f3f', soft: '#eaf0eb' },
  sand: { fg: '#6b4a25', soft: '#f3e8d6' },
  clay: { fg: '#9a5a3c', soft: '#f4e4d8' },
  slate: { fg: '#41566b', soft: '#e6ecf2' },
};
export function tintOf(catId: string) {
  const m = LEARN.catMeta(catId);
  return TINTS[(m && m.tint) || 'forest'];
}

// ── Compact typographic article row ───────────────────────────
export function ArticleRow({
  a,
  onOpen,
  last,
}: {
  a: LearnArticle;
  onOpen: (id: string) => void;
  last?: boolean;
}) {
  const m = LEARN.catMeta(a.cat)!;
  const tint = tintOf(a.cat);
  return (
    <Press
      onPress={() => onOpen(a.id)}
      style={{
        flexDirection: 'row',
        gap: 14,
        alignItems: 'flex-start',
        paddingVertical: 14,
        paddingHorizontal: 4,
        borderBottomWidth: last ? 0 : 1,
        borderBottomColor: pal.hairline,
      }}
    >
      <View
        style={{
          width: 44,
          height: 44,
          borderRadius: 12,
          backgroundColor: tint.soft,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={m.icon as IconName} size={20} color={tint.fg} />
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text
          style={{
            fontFamily: F.display,
            fontSize: 19,
            color: pal.ink,
            lineHeight: 19 * 1.12,
            letterSpacing: 19 * -0.012,
          }}
        >
          {a.title}
        </Text>
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
          {a.dek}
        </Text>
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 7 }}>
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 11, color: tint.fg }}>{m.label}</Text>
          <View
            style={{ width: 3, height: 3, borderRadius: 999, backgroundColor: pal.placeholder }}
          />
          <Text
            style={{
              fontFamily: F.mono,
              fontSize: 11,
              color: pal.subtle,
              fontVariant: ['tabular-nums'],
            }}
          >
            {a.readMin} мин
          </Text>
        </View>
      </View>
    </Press>
  );
}

// ── Personalized horizontal card ──────────────────────────────
function ForYouCard({ a, onOpen }: { a: LearnArticle; onOpen: (id: string) => void }) {
  const m = LEARN.catMeta(a.cat)!;
  const tint = tintOf(a.cat);
  return (
    <Press
      onPress={() => onOpen(a.id)}
      style={[
        {
          width: 220,
          backgroundColor: pal.paper,
          borderWidth: 1,
          borderColor: pal.hairline,
          borderRadius: 18,
          padding: 16,
          gap: 10,
          minHeight: 168,
        },
        SHADOW.sm,
      ]}
    >
      <View
        style={{
          width: 40,
          height: 40,
          borderRadius: 11,
          backgroundColor: tint.soft,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={m.icon as IconName} size={20} color={tint.fg} />
      </View>
      <Text
        style={{
          fontFamily: F.display,
          fontSize: 20,
          color: pal.ink,
          lineHeight: 20 * 1.1,
          letterSpacing: 20 * -0.012,
          flex: 1,
        }}
      >
        {a.title}
      </Text>
      <Text
        style={{
          fontFamily: F.mono,
          fontSize: 11,
          color: pal.subtle,
          fontVariant: ['tabular-nums'],
        }}
      >
        {a.readMin} мин · {m.short}
      </Text>
    </Press>
  );
}

// ── Topic grid card ───────────────────────────────────────────
function TopicCard({ c, onPress }: { c: LearnCategory; onPress: () => void }) {
  const tint = TINTS[c.tint];
  const n = LEARN.byCat(c.id).length;
  return (
    <Press
      onPress={onPress}
      style={[
        {
          width: '48%',
          flexGrow: 1,
          alignItems: 'flex-start',
          gap: 10,
          backgroundColor: pal.paper,
          borderWidth: 1,
          borderColor: pal.hairline,
          borderRadius: 16,
          padding: 14,
        },
        SHADOW.sm,
      ]}
    >
      <View
        style={{
          width: 40,
          height: 40,
          borderRadius: 11,
          backgroundColor: tint.soft,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={c.icon as IconName} size={20} color={tint.fg} />
      </View>
      <Text
        style={{
          fontFamily: F.bodySemibold,
          fontSize: 14,
          color: pal.ink,
          lineHeight: 14 * 1.2,
        }}
      >
        {c.label}
      </Text>
      <Text
        style={{
          fontFamily: F.mono,
          fontSize: 11,
          color: pal.subtle,
          fontVariant: ['tabular-nums'],
        }}
      >
        {n} {n === 1 ? 'материал' : n < 5 ? 'материала' : 'материалов'}
      </Text>
    </Press>
  );
}

// ════════════════════════════════════════════════════════════════
// Library landing
// ════════════════════════════════════════════════════════════════
export function LearnScreen({
  onBack,
  onOpenArticle,
}: {
  onBack: () => void;
  onOpenArticle: (id: string) => void;
}) {
  const insets = useSafeAreaInsets();
  const [q, setQ] = useState('');
  const [cat, setCat] = useState<string | null>(null);
  const feat = LEARN.featured()!;
  const forYou = LEARN.FOR_YOU.map(LEARN.byId).filter(
    (a): a is LearnArticle => !!a && a.id !== feat.id,
  );
  const results = q.trim() ? LEARN.search(q) : null;
  const catArticles = cat ? LEARN.byCat(cat) : null;

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Top bar */}
      <View
        style={{
          paddingTop: insets.top + 6,
          backgroundColor: pal.bg,
          borderBottomWidth: 1,
          borderBottomColor: pal.hairline,
          zIndex: 20,
        }}
      >
        <View
          style={{
            paddingTop: 8,
            paddingHorizontal: 16,
            paddingBottom: 6,
            flexDirection: 'row',
            alignItems: 'center',
            gap: 10,
          }}
        >
          <IconBtn name="chevron-left" onPress={onBack} bg={pal.sunk} color={pal.ink2} />
          <Text style={{ fontFamily: F.bodySemibold, fontSize: 15, color: pal.ink }}>Знания</Text>
        </View>
        {/* Search */}
        <View style={{ paddingTop: 4, paddingHorizontal: 16, paddingBottom: 12 }}>
          <View
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              gap: 10,
              backgroundColor: pal.sunk,
              borderRadius: 14,
              paddingVertical: 11,
              paddingHorizontal: 14,
            }}
          >
            <Icon name="magnifying-glass" size={18} color={pal.subtle} />
            <TextInput
              value={q}
              onChangeText={setQ}
              placeholder="Поиск по библиотеке"
              placeholderTextColor={pal.placeholder}
              style={{
                flex: 1,
                padding: 0,
                fontFamily: F.body,
                fontSize: 14,
                color: pal.ink,
              }}
            />
            {q ? (
              <Press onPress={() => setQ('')} style={{ padding: 0 }}>
                <Icon name="x-mark" size={16} color={pal.subtle} />
              </Press>
            ) : null}
          </View>
        </View>
      </View>

      {/* Body */}
      <ScrollView
        style={{ flex: 1 }}
        contentContainerStyle={{ paddingBottom: 40 + insets.bottom }}
        showsVerticalScrollIndicator={false}
      >
        {results ? (
          /* SEARCH RESULTS */
          <View style={{ paddingTop: 6, paddingHorizontal: 16 }}>
            <Eyebrow
              color={pal.subtle}
              style={{ paddingTop: 6, paddingHorizontal: 4, paddingBottom: 8 }}
            >
              {results.length ? `Найдено · ${results.length}` : 'Ничего не найдено'}
            </Eyebrow>
            {results.length === 0 ? (
              <Text
                style={{
                  fontFamily: F.body,
                  fontSize: 13.5,
                  color: pal.subtle,
                  paddingVertical: 8,
                  paddingHorizontal: 4,
                  lineHeight: 13.5 * 1.5,
                }}
              >
                Попробуйте «семаглутид», «тошнота» или «белок».
              </Text>
            ) : (
              results.map((a, i) => (
                <ArticleRow
                  key={a.id}
                  a={a}
                  onOpen={onOpenArticle}
                  last={i === results.length - 1}
                />
              ))
            )}
          </View>
        ) : cat && catArticles ? (
          /* CATEGORY FILTER */
          <View style={{ paddingTop: 6, paddingHorizontal: 16 }}>
            <Press
              onPress={() => setCat(null)}
              style={{
                flexDirection: 'row',
                alignItems: 'center',
                gap: 6,
                alignSelf: 'flex-start',
                marginBottom: 6,
                backgroundColor: pal.sunk,
                borderRadius: 999,
                paddingVertical: 7,
                paddingHorizontal: 14,
              }}
            >
              <Icon name="chevron-left" size={14} color={pal.ink2} />
              <Text style={{ fontFamily: F.bodyMedium, fontSize: 12.5, color: pal.ink2 }}>
                Все темы
              </Text>
            </Press>
            <Text
              style={{
                fontFamily: F.display,
                fontSize: 30,
                color: pal.ink,
                letterSpacing: 30 * -0.018,
                paddingTop: 6,
                paddingHorizontal: 4,
                paddingBottom: 12,
              }}
            >
              {LEARN.catMeta(cat)!.label}
            </Text>
            {catArticles.map((a, i) => (
              <ArticleRow
                key={a.id}
                a={a}
                onOpen={onOpenArticle}
                last={i === catArticles.length - 1}
              />
            ))}
          </View>
        ) : (
          /* BROWSE */
          <>
            {/* Lead */}
            <View style={{ paddingTop: 8, paddingHorizontal: 20, paddingBottom: 14 }}>
              <Text
                style={{
                  fontFamily: F.display,
                  fontSize: 32,
                  color: pal.ink,
                  lineHeight: 32 * 1.04,
                  letterSpacing: 32 * -0.018,
                }}
              >
                Понимать свой{' '}
                <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>ритм</Text>.
              </Text>
            </View>

            {/* Featured (personalized, photo) */}
            <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
              <Press
                onPress={() => onOpenArticle(feat.id)}
                style={{
                  borderRadius: 24,
                  backgroundColor: C.forest800,
                  shadowColor: C.forest900,
                  shadowOpacity: 0.18,
                  shadowRadius: 24,
                  shadowOffset: { width: 0, height: 8 },
                  elevation: 8,
                }}
              >
                <View style={{ borderRadius: 24, overflow: 'hidden' }}>
                  <View
                    style={{
                      width: '100%',
                      height: 156,
                      backgroundColor: C.linen,
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <Icon name="camera" color={C.ink400} />
                  </View>
                  <View style={{ padding: 20 }}>
                    <View
                      style={{
                        flexDirection: 'row',
                        alignItems: 'center',
                        gap: 6,
                        alignSelf: 'flex-start',
                        marginBottom: 10,
                        backgroundColor: 'rgba(212,165,116,.2)',
                        borderRadius: 999,
                        paddingVertical: 5,
                        paddingHorizontal: 11,
                      }}
                    >
                      <Icon name="sparkles" size={13} color={C.sand300} />
                      <Text
                        style={{
                          fontFamily: F.bodyMedium,
                          fontSize: 11,
                          letterSpacing: 11 * 0.04,
                          color: C.sand300,
                        }}
                      >
                        {feat.eyebrow}
                      </Text>
                    </View>
                    <Text
                      style={{
                        fontFamily: F.display,
                        fontSize: 28,
                        color: C.cream,
                        lineHeight: 28 * 1.06,
                        letterSpacing: 28 * -0.018,
                      }}
                    >
                      {feat.title}
                    </Text>
                    <Text
                      style={{
                        fontFamily: F.body,
                        fontSize: 13,
                        color: 'rgba(246,241,234,.74)',
                        lineHeight: 13 * 1.5,
                        marginTop: 9,
                      }}
                    >
                      {feat.dek}
                    </Text>
                    <View
                      style={{
                        flexDirection: 'row',
                        alignItems: 'center',
                        gap: 8,
                        marginTop: 16,
                      }}
                    >
                      <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: C.sand300 }}>
                        Читать
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
                        {feat.readMin} мин
                      </Text>
                    </View>
                  </View>
                </View>
              </Press>
            </View>

            {/* For you rail */}
            <View style={{ paddingBottom: 18 }}>
              <View style={{ paddingHorizontal: 20, paddingBottom: 10 }}>
                <Eyebrow color={pal.subtle}>Готовимся к 0,5 мг</Eyebrow>
              </View>
              <ScrollView
                horizontal
                showsHorizontalScrollIndicator={false}
                contentContainerStyle={{ gap: 12, paddingHorizontal: 16, paddingBottom: 4 }}
              >
                {forYou.map((a) => (
                  <ForYouCard key={a.id} a={a} onOpen={onOpenArticle} />
                ))}
              </ScrollView>
            </View>

            {/* Topic grid */}
            <View style={{ paddingHorizontal: 16, paddingBottom: 18 }}>
              <Eyebrow color={pal.subtle} style={{ paddingHorizontal: 4, paddingBottom: 10 }}>
                Темы
              </Eyebrow>
              <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 10 }}>
                {LEARN.CATEGORIES.map((c) => (
                  <TopicCard key={c.id} c={c} onPress={() => setCat(c.id)} />
                ))}
              </View>
            </View>

            {/* All articles */}
            <View style={{ paddingHorizontal: 16 }}>
              <Eyebrow color={pal.subtle} style={{ paddingHorizontal: 4, paddingBottom: 6 }}>
                Все материалы
              </Eyebrow>
              {LEARN.ARTICLES.map((a, i) => (
                <ArticleRow
                  key={a.id}
                  a={a}
                  onOpen={onOpenArticle}
                  last={i === LEARN.ARTICLES.length - 1}
                />
              ))}
            </View>
          </>
        )}
      </ScrollView>
    </View>
  );
}
