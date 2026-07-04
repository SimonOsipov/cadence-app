// Cadence · Самочувствие — wellbeing journal.
// Ported from journal/journal.jsx (JournalScreen). Two modes:
// Дневник (timeline, tap to expand a note) · Динамика (mood line + heatmap).
import React, { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import Svg, { Circle, Line, Path, Rect, Text as SvgText } from 'react-native-svg';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal } from '../../theme';
import { Icon } from '../../components/Icon';
import { Eyebrow, IconBtn, Press } from '../../components/primitives';
import { useAppState } from '../../state/AppState';
import { JOURNAL, JournalEntry, MoodScale } from './data';
import { QuickFeelSheet } from './QuickFeelSheet';

const J = JOURNAL;

// ── Mood trend line (full 12-week axis, data in elapsed portion) ──
function MoodLine({ entries }: { entries: JournalEntry[] }) {
  const [w, setW] = useState(0);
  const W = 320;
  const H = 150;
  const padL = 10;
  const padR = 12;
  const padT = 14;
  const padB = 22;
  const xOf = (day: number) => padL + (day / 84) * (W - padL - padR);
  const yOf = (m: number) => H - padB - ((m - 1) / 4) * (H - padT - padB);
  const pts = J.moodPoints(entries);
  const today = J.TODAY_DAY;
  const titr = J.TITRATION_DAY;
  const line = pts
    .map((p, i) => `${i ? 'L' : 'M'}${xOf(p.day).toFixed(1)} ${yOf(p.mood).toFixed(1)}`)
    .join(' ');

  return (
    <View onLayout={(e) => setW(e.nativeEvent.layout.width)}>
      {w > 0 ? (
        <Svg width={w} height={(w * H) / W} viewBox={`0 0 ${W} ${H}`}>
          {/* future shade */}
          <Rect
            x={xOf(today)}
            y={padT}
            width={W - padR - xOf(today)}
            height={H - padT - padB}
            fill={C.sand100}
            opacity={0.4}
          />
          {/* mood gridlines */}
          {[1, 2, 3, 4, 5].map((m) => (
            <Line
              key={m}
              x1={padL}
              x2={W - padR}
              y1={yOf(m)}
              y2={yOf(m)}
              stroke={pal.hairline}
              strokeWidth={1}
            />
          ))}
          {/* titration marker */}
          <Line
            x1={xOf(titr)}
            x2={xOf(titr)}
            y1={padT}
            y2={H - padB}
            stroke={C.sand500}
            strokeWidth={1.5}
            strokeDasharray="3 3"
          />
          <Circle cx={xOf(titr)} cy={padT} r={3} fill={C.sand500} />
          {/* today marker */}
          <Line
            x1={xOf(today)}
            x2={xOf(today)}
            y1={padT}
            y2={H - padB}
            stroke={C.forest700}
            strokeWidth={1.5}
            strokeDasharray="2 3"
            opacity={0.6}
          />
          {/* line */}
          <Path
            d={line}
            fill="none"
            stroke={C.forest700}
            strokeWidth={2.5}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          {/* dots */}
          {pts.map((p, i) => (
            <Circle
              key={i}
              cx={xOf(p.day)}
              cy={yOf(p.mood)}
              r={p.source === 'dose' ? 4.5 : 3.2}
              fill={p.source === 'dose' ? C.sand500 : C.paper}
              stroke={C.forest700}
              strokeWidth={2}
            />
          ))}
          {/* x labels */}
          {[1, 4, 8, 12].map((wk) => (
            <SvgText
              key={wk}
              x={xOf((wk - 1) * 7)}
              y={H - 6}
              fontSize={9}
              fontFamily={F.mono}
              fill={pal.subtle}
              textAnchor={wk === 1 ? 'start' : wk === 12 ? 'end' : 'middle'}
            >
              {`нед ${wk}`}
            </SvgText>
          ))}
        </Svg>
      ) : null}
    </View>
  );
}

// ── Calendar heatmap ──────────────────────────────────────────
function Heatmap({ entries }: { entries: JournalEntry[] }) {
  const weeks = J.heatmap(entries);
  return (
    <View>
      {/* weekday header */}
      <View style={{ flexDirection: 'row', gap: 4, marginBottom: 6 }}>
        <View style={{ width: 30 }} />
        {['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'].map((wd) => (
          <Text
            key={wd}
            style={{
              flex: 1,
              textAlign: 'center',
              fontFamily: F.body,
              fontSize: 9,
              color: pal.placeholder,
              textTransform: 'uppercase',
              letterSpacing: 9 * 0.04,
            }}
          >
            {wd}
          </Text>
        ))}
      </View>
      <View style={{ gap: 4 }}>
        {weeks.map((row, wi) => (
          <View key={wi} style={{ flexDirection: 'row', gap: 4, alignItems: 'center' }}>
            <Text
              style={{
                width: 30,
                fontFamily: F.mono,
                fontSize: 9.5,
                color: pal.subtle,
                fontVariant: ['tabular-nums'],
              }}
            >
              {wi + 1}
            </Text>
            {row.map((cell, ci) => {
              if (!cell) return <View key={ci} style={{ flex: 1, aspectRatio: 1 }} />;
              const mood = cell.mood ? J.MOOD[cell.mood] : null;
              const isToday = cell.rel === 'today';
              const bg = mood
                ? mood.color
                : cell.rel === 'future'
                  ? 'transparent'
                  : pal.sunk;
              return (
                <View
                  key={ci}
                  style={{
                    flex: 1,
                    aspectRatio: 1,
                    borderRadius: 5,
                    backgroundColor: bg,
                    opacity: cell.rel === 'future' ? 1 : mood ? 1 : 0.7,
                    ...(isToday
                      ? { borderWidth: 1.5, borderColor: C.forest700 }
                      : cell.rel === 'future'
                        ? { borderWidth: 1, borderColor: pal.border, borderStyle: 'dashed' as const }
                        : { borderWidth: 1.5, borderColor: 'transparent' }),
                  }}
                >
                  {cell.titration ? (
                    <View
                      style={{
                        position: 'absolute',
                        top: -2,
                        right: -2,
                        width: 6,
                        height: 6,
                        borderRadius: 999,
                        backgroundColor: C.sand500,
                        borderWidth: 1,
                        borderColor: pal.bg,
                      }}
                    />
                  ) : null}
                </View>
              );
            })}
          </View>
        ))}
      </View>
      {/* legend */}
      <View
        style={{
          flexDirection: 'row',
          alignItems: 'center',
          gap: 8,
          marginTop: 12,
          justifyContent: 'space-between',
        }}
      >
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 5 }}>
          <Text style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle }}>Тяжело</Text>
          {[1, 2, 3, 4, 5].map((m) => (
            <View
              key={m}
              style={{ width: 12, height: 12, borderRadius: 4, backgroundColor: J.MOOD[m]!.color }}
            />
          ))}
          <Text style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle }}>Светло</Text>
        </View>
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 5 }}>
          <View style={{ width: 7, height: 7, borderRadius: 999, backgroundColor: C.sand500 }} />
          <Text style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle }}>титрация</Text>
        </View>
      </View>
    </View>
  );
}

// ── Timeline entry (expandable note) ──────────────────────────
function TimelineEntry({
  e,
  expanded,
  onToggle,
  last,
}: {
  e: JournalEntry;
  expanded: boolean;
  onToggle: () => void;
  last: boolean;
}) {
  const date = J.dateOf(e.day);
  const wd = J.WD_FULL[date.getDay()];
  const mood = J.MOOD[e.mood] as MoodScale;
  const hasNote = !!e.note;
  const content = (
    <View style={{ minWidth: 0 }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
        <Text
          style={{
            fontFamily: F.display,
            fontSize: 19,
            lineHeight: 21,
            letterSpacing: 19 * -0.012,
            color: pal.ink,
          }}
        >
          {mood.label}
        </Text>
        {e.source === 'dose' ? (
          <View
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              gap: 4,
              backgroundColor: C.forest50,
              borderRadius: 999,
              paddingVertical: 2,
              paddingHorizontal: 8,
            }}
          >
            <Icon name="beaker" size={11} color={C.forest700} />
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 10.5, color: C.forest700 }}>
              с дозой
            </Text>
          </View>
        ) : null}
      </View>
      {/* energy + sleep */}
      <View style={{ flexDirection: 'row', gap: 14, marginTop: 6 }}>
        <Text style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>
          энергия <Text style={{ color: pal.ink2 }}>{J.ENERGY_LABEL[e.energy]}</Text>
        </Text>
        <Text style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>
          сон <Text style={{ color: pal.ink2 }}>{J.SLEEP_LABEL[e.sleep]}</Text>
        </Text>
      </View>
      {/* tags */}
      {e.tags.length > 0 ? (
        <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 9 }}>
          {e.tags.map((t) => (
            <View
              key={t}
              style={{
                backgroundColor: C.sand100,
                borderRadius: 999,
                paddingVertical: 3,
                paddingHorizontal: 10,
              }}
            >
              <Text style={{ fontFamily: F.body, fontSize: 11.5, color: '#7a4a06' }}>
                {J.tagLabel(t)}
              </Text>
            </View>
          ))}
        </View>
      ) : null}
      {/* note */}
      {hasNote ? (
        <Text
          numberOfLines={expanded ? undefined : 2}
          style={{
            marginTop: 9,
            fontFamily: F.body,
            fontSize: 13.5,
            lineHeight: 13.5 * 1.5,
            color: pal.ink2,
          }}
        >
          {e.note}
        </Text>
      ) : null}
    </View>
  );
  return (
    <View
      style={{
        flexDirection: 'row',
        gap: 12,
        paddingVertical: 14,
        borderBottomWidth: last ? 0 : 1,
        borderBottomColor: pal.hairline,
      }}
    >
      {/* date rail */}
      <View style={{ width: 52, alignItems: 'center', paddingTop: 2 }}>
        <Text
          style={{
            fontFamily: F.mono,
            fontSize: 18,
            lineHeight: 18,
            color: pal.ink,
            fontVariant: ['tabular-nums'],
          }}
        >
          {date.getDate()}
        </Text>
        <Text style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle, marginTop: 2 }}>
          {wd.slice(0, 2).toLowerCase()}
        </Text>
        <View
          style={{
            width: 11,
            height: 11,
            borderRadius: 999,
            backgroundColor: mood.color,
            marginTop: 8,
          }}
        />
      </View>
      {/* content */}
      {hasNote ? (
        <Press onPress={onToggle} style={{ flex: 1, minWidth: 0 }}>
          {content}
        </Press>
      ) : (
        <View style={{ flex: 1, minWidth: 0 }}>{content}</View>
      )}
    </View>
  );
}

// ════════════════════════════════════════════════════════════════
// Journal screen
// ════════════════════════════════════════════════════════════════
export function JournalScreen({ onBack }: { onBack: () => void }) {
  const insets = useSafeAreaInsets();
  const { journalEntries, saveQuickFeel } = useAppState();
  const [mode, setMode] = useState<'diary' | 'trends'>('diary');
  const [openId, setOpenId] = useState<string | null>(null);
  const [quickFeelOpen, setQuickFeelOpen] = useState(false);

  const entries = journalEntries as JournalEntry[];
  const list = J.sortedDesc(entries);
  const st = J.stats(entries);
  const tally = J.tagTally(entries);
  const todayLogged = entries.some((e) => e.day === J.TODAY_DAY);
  const fmtAvg = st.avg ? st.avg.toFixed(1).replace('.', ',') : '—';

  const onQuickAdd = () => setQuickFeelOpen(true);

  const Seg = ({ id, label }: { id: 'diary' | 'trends'; label: string }) => {
    const on = mode === id;
    return (
      <Press
        onPress={() => setMode(id)}
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
          style={{ fontFamily: F.bodyMedium, fontSize: 13, color: on ? C.cream : pal.muted }}
        >
          {label}
        </Text>
      </Press>
    );
  };

  const cardShell = {
    backgroundColor: pal.paper,
    borderWidth: 1,
    borderColor: pal.hairline,
    shadowColor: '#2e2618',
    shadowOpacity: 0.05,
    shadowRadius: 6,
    shadowOffset: { width: 0, height: 2 },
    elevation: 2,
  } as const;

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
          <Text style={{ fontFamily: F.bodySemibold, fontSize: 15, color: pal.ink }}>
            Самочувствие
          </Text>
        </View>
        <View
          style={{
            flexDirection: 'row',
            gap: 4,
            backgroundColor: pal.sunk,
            padding: 3,
            borderRadius: 999,
            marginTop: 4,
            marginHorizontal: 16,
            marginBottom: 12,
          }}
        >
          <Seg id="diary" label="Дневник" />
          <Seg id="trends" label="Динамика" />
        </View>
      </View>

      <ScrollView
        style={{ flex: 1 }}
        contentContainerStyle={{ paddingBottom: 96 + insets.bottom }}
        showsVerticalScrollIndicator={false}
      >
        {mode === 'diary' ? (
          <>
            {/* Today prompt */}
            <View style={{ paddingTop: 6, paddingHorizontal: 16, paddingBottom: 8 }}>
              {todayLogged ? (
                <View
                  style={{
                    flexDirection: 'row',
                    alignItems: 'center',
                    gap: 10,
                    paddingVertical: 12,
                    paddingHorizontal: 14,
                    backgroundColor: C.forest50,
                    borderRadius: 16,
                  }}
                >
                  <Icon name="check-circle" size={20} color={C.forest700} />
                  <Text style={{ fontFamily: F.body, fontSize: 13, color: C.forest800 }}>
                    Сегодня отмечено — спасибо.
                  </Text>
                </View>
              ) : (
                <Press
                  onPress={onQuickAdd}
                  style={{
                    flexDirection: 'row',
                    gap: 12,
                    alignItems: 'center',
                    backgroundColor: C.forest800,
                    borderRadius: 18,
                    padding: 16,
                    shadowColor: C.forest900,
                    shadowOpacity: 0.18,
                    shadowRadius: 24,
                    shadowOffset: { width: 0, height: 8 },
                    elevation: 6,
                  }}
                >
                  <View
                    style={{
                      width: 44,
                      height: 44,
                      borderRadius: 12,
                      backgroundColor: 'rgba(212,165,116,.2)',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <Icon name="heart" size={22} color={C.sand300} />
                  </View>
                  <View style={{ flex: 1, minWidth: 0 }}>
                    <Text
                      style={{
                        fontFamily: F.display,
                        fontSize: 21,
                        lineHeight: 23,
                        letterSpacing: 21 * -0.012,
                        color: C.cream,
                      }}
                    >
                      Как вы себя{' '}
                      <Text style={{ fontFamily: F.displayItalic, color: C.sand300 }}>
                        чувствуете?
                      </Text>
                    </Text>
                    <Text
                      style={{
                        fontFamily: F.body,
                        fontSize: 12,
                        color: 'rgba(246,241,234,.7)',
                        marginTop: 2,
                      }}
                    >
                      Сегодня ещё не отмечено
                    </Text>
                  </View>
                  <Icon name="plus" size={20} color={C.sand300} />
                </Press>
              )}
            </View>

            {/* Timeline */}
            <View style={{ paddingTop: 4, paddingHorizontal: 20 }}>
              {list.map((e, i) => (
                <TimelineEntry
                  key={e.id}
                  e={e}
                  expanded={openId === e.id}
                  onToggle={() => setOpenId(openId === e.id ? null : e.id)}
                  last={i === list.length - 1}
                />
              ))}
            </View>
          </>
        ) : (
          <>
            {/* Stat row */}
            <View style={{ paddingTop: 6, paddingHorizontal: 16, paddingBottom: 14 }}>
              <View style={{ flexDirection: 'row', gap: 10 }}>
                {[
                  { v: fmtAvg, l: 'Настроение, ср.', text: false },
                  { v: String(st.count), l: 'Записей', text: false },
                  { v: tally[0] ? tally[0].label : '—', l: 'Частая отметка', text: true },
                ].map((s, i) => (
                  <View
                    key={i}
                    style={{
                      flex: 1,
                      borderRadius: 16,
                      paddingVertical: 13,
                      paddingHorizontal: 12,
                      alignItems: 'center',
                      ...cardShell,
                    }}
                  >
                    <Text
                      numberOfLines={1}
                      style={{
                        fontFamily: s.text ? F.display : F.monoMedium,
                        fontSize: s.text ? 18 : 24,
                        lineHeight: s.text ? 19 : 25,
                        letterSpacing: (s.text ? 18 : 24) * -0.02,
                        color: pal.ink,
                        ...(s.text ? null : { fontVariant: ['tabular-nums'] as const }),
                      }}
                    >
                      {s.v}
                    </Text>
                    <Text
                      numberOfLines={1}
                      style={{
                        fontFamily: F.body,
                        fontSize: 10.5,
                        lineHeight: 13,
                        color: pal.subtle,
                        marginTop: 5,
                      }}
                    >
                      {s.l}
                    </Text>
                  </View>
                ))}
              </View>
            </View>

            {/* Mood line */}
            <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
              <View style={{ borderRadius: 18, padding: 16, ...cardShell }}>
                <Eyebrow color={pal.subtle} style={{ marginBottom: 10 }}>
                  Настроение по неделям
                </Eyebrow>
                <MoodLine entries={entries} />
              </View>
            </View>

            {/* Heatmap */}
            <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
              <View style={{ borderRadius: 18, padding: 16, ...cardShell }}>
                <Eyebrow color={pal.subtle} style={{ marginBottom: 12 }}>
                  Карта дней · 12 недель
                </Eyebrow>
                <Heatmap entries={entries} />
              </View>
            </View>

            {/* Tag tally */}
            {tally.length > 0 ? (
              <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
                <Eyebrow
                  color={pal.subtle}
                  style={{ paddingHorizontal: 4, paddingBottom: 10 }}
                >
                  Отметки за курс
                </Eyebrow>
                <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
                  {tally.map((t) => (
                    <View
                      key={t.id}
                      style={{
                        flexDirection: 'row',
                        alignItems: 'center',
                        gap: 7,
                        backgroundColor: pal.paper,
                        borderWidth: 1,
                        borderColor: pal.hairline,
                        borderRadius: 999,
                        paddingVertical: 7,
                        paddingHorizontal: 13,
                      }}
                    >
                      <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.ink2 }}>
                        {t.label}
                      </Text>
                      <Text
                        style={{
                          fontFamily: F.mono,
                          fontSize: 12,
                          color: C.sand700,
                          fontVariant: ['tabular-nums'],
                        }}
                      >
                        ×{t.n}
                      </Text>
                    </View>
                  ))}
                </View>
              </View>
            ) : null}
          </>
        )}
      </ScrollView>

      {/* Floating quick-add (always available) */}
      <Press
        onPress={onQuickAdd}
        accessibilityLabel="Отметить самочувствие"
        style={{
          position: 'absolute',
          right: 18,
          bottom: Math.max(insets.bottom, 16) + 12,
          zIndex: 30,
          height: 52,
          paddingHorizontal: 20,
          borderRadius: 999,
          backgroundColor: C.forest700,
          flexDirection: 'row',
          alignItems: 'center',
          gap: 8,
          shadowColor: C.forest700,
          shadowOpacity: 0.4,
          shadowRadius: 20,
          shadowOffset: { width: 0, height: 8 },
          elevation: 8,
        }}
      >
        <Icon name="plus" size={19} strokeWidth={2} color={C.cream} />
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>Запись</Text>
      </Press>

      <QuickFeelSheet
        open={quickFeelOpen}
        onCancel={() => setQuickFeelOpen(false)}
        onSave={(entry) => {
          saveQuickFeel(entry);
          setQuickFeelOpen(false);
        }}
      />
    </View>
  );
}
