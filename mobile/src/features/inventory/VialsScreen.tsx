// Cadence — Аптечка (Vials) landing screen.
// Ported from design_handoff_cadence_app/inventory/inventory.jsx (VialsScreen).
import React, { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { C, F, pal, R } from '../../theme';
import { Eyebrow, Press, Title, TitleEm } from '../../components/primitives';
import { Icon, IconName } from '../../components/Icon';
import { CadenceTabBar, TabId } from '../../components/shared';
import { useAppState } from '../../state/AppState';
import { COMPOUND_META, CompoundMeta, InventorySummary, Vial } from './data';
import { VialDetailSheet } from './VialDetailSheet';

// Russian plural helpers (exact prototype logic).
const daysWord = (n: number) => (n === 1 ? 'день' : n < 5 ? 'дня' : 'дней');
const weeksWord = (n: number) => (n === 1 ? 'неделю' : n < 5 ? 'недели' : 'недель');
const vialsWord = (n: number) => (n === 1 ? 'флакон' : n < 5 ? 'флакона' : 'флаконов');

export function VialsScreen({
  onBack,
  onAddVial,
  onLogDose,
  onChangeTab,
}: {
  onBack: () => void;
  onAddVial: () => void;
  onLogDose: () => void;
  onChangeTab: (id: TabId) => void;
}) {
  void onBack; // tab-level screen — the prototype top bar has no back control
  const insets = useSafeAreaInsets();
  const { vials, invSummary, sealedOpen, setSealedOpen } = useAppState();
  const inv = vials as Vial[];
  const sum = invSummary as InventorySummary;

  const [filter, setFilter] = useState('all');
  const [vialDetailId, setVialDetailId] = useState<string | null>(null);
  const detailVial = inv.find((v) => v.id === vialDetailId) || null;

  // Filter inventory based on chip
  const filterVial = (v: Vial) => {
    if (filter === 'all') return true;
    if (filter === 'active') return v.opened !== null && v.status !== 'expiring';
    if (filter === 'expiring') return v.status === 'expiring';
    if (filter === 'sealed') return v.opened === null;
    return v.compound === filter;
  };

  const active = inv.filter((v) => v.opened !== null).filter(filterVial);
  const sealed = inv.filter((v) => v.opened === null).filter(filterVial);

  // Distinct compounds for chip filter
  const compounds = Array.from(new Set(inv.map((v) => v.compound)));

  const chips: { id: string; label: string; count: number; danger?: boolean }[] = [
    { id: 'all', label: 'Все', count: inv.length },
    { id: 'active', label: 'Активные', count: sum.active.length },
    { id: 'expiring', label: 'Истекают', count: sum.expiring.length, danger: sum.expiring.length > 0 },
    { id: 'sealed', label: 'Запас', count: sum.sealed.length },
    ...compounds.map((c) => ({
      id: c,
      label: COMPOUND_META[c]?.name || c,
      count: inv.filter((v) => v.compound === c).length,
    })),
  ];

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      {/* Top bar — fixed outside the scroll region */}
      <View
        style={{
          paddingTop: insets.top + 18,
          paddingHorizontal: 20,
          paddingBottom: 14,
          backgroundColor: pal.bg,
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'flex-end',
          gap: 12,
        }}
      >
        <View style={{ minWidth: 0, flex: 1 }}>
          <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginBottom: 4 }}>
            {inv.length} {vialsWord(inv.length)} в холодильнике
          </Text>
          <Title size={36}>
            Ваша <TitleEm size={36}>аптечка</TitleEm>
          </Title>
        </View>
        <Press
          onPress={onAddVial}
          style={{
            width: 44,
            height: 44,
            borderRadius: R.pill,
            backgroundColor: C.forest700,
            alignItems: 'center',
            justifyContent: 'center',
            shadowColor: C.forest700,
            shadowOpacity: 0.28,
            shadowRadius: 12,
            shadowOffset: { width: 0, height: 4 },
            elevation: 6,
          }}
        >
          <Icon name="plus" size={20} strokeWidth={2} color={C.cream} />
        </Press>
      </View>

      <ScrollView
        style={{ flex: 1 }}
        showsVerticalScrollIndicator={false}
        contentContainerStyle={{ paddingBottom: 130 }}
      >
        {/* Summary hero card */}
        <View style={{ paddingHorizontal: 16, paddingBottom: 14 }}>
          <InventorySummaryCard sum={sum} inv={inv} />
        </View>

        {/* Filter chips */}
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          style={{ marginBottom: 14 }}
          contentContainerStyle={{ paddingHorizontal: 16, gap: 6 }}
        >
          {chips.map((chip) => {
            const on = filter === chip.id;
            const danger = !!chip.danger && !on;
            return (
              <Press
                key={chip.id}
                onPress={() => setFilter(chip.id)}
                style={{
                  paddingVertical: 8,
                  paddingHorizontal: 12,
                  borderRadius: R.pill,
                  backgroundColor: on ? C.forest700 : danger ? C.warningBg : pal.paper,
                  borderWidth: 1,
                  borderColor: on || danger ? 'transparent' : pal.border,
                  flexDirection: 'row',
                  alignItems: 'center',
                  gap: 6,
                }}
              >
                <Text
                  numberOfLines={1}
                  style={{
                    fontFamily: F.bodyMedium,
                    fontSize: 13,
                    color: on ? C.cream : danger ? C.warning : pal.muted,
                  }}
                >
                  {chip.label}
                </Text>
                <Text
                  style={{
                    fontFamily: F.mono,
                    fontSize: 10,
                    fontVariant: ['tabular-nums'],
                    color: on ? 'rgba(246,241,234,.75)' : danger ? C.warning : pal.subtle,
                  }}
                >
                  {chip.count}
                </Text>
              </Press>
            );
          })}
        </ScrollView>

        {/* Warning + reorder cards (only when on relevant filters) */}
        {(filter === 'all' || filter === 'expiring') &&
          sum.expiring.map((v) => (
            <View key={v.id} style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
              <WarningCard
                tone="warn"
                icon="exclamation-circle"
                title={`${(v.compoundMeta as CompoundMeta).name} истекает через ${v.daysToExpiry} ${daysWord(v.daysToExpiry)}`}
                sub={
                  v.opened
                    ? `Открыт ${v.openedLabel} · ${v.remaining}/${v.total} доз`
                    : `Запечатан · до ${v.expiresLabel}`
                }
                ctaLabel="Открыть"
                onPress={() => setVialDetailId(v.id)}
              />
            </View>
          ))}

        {filter === 'all' &&
          sum.reorder.map((r) => (
            <View key={r.compound} style={{ paddingHorizontal: 16, paddingBottom: 10 }}>
              <WarningCard
                tone="info"
                icon="information-circle"
                title={`${r.meta.name} закончится через ~${r.weeksLeft} ${weeksWord(r.weeksLeft)}`}
                sub="Запасного флакона нет"
                ctaLabel="Добавить"
                onPress={onAddVial}
              />
            </View>
          ))}

        {/* Empty state */}
        {inv.length === 0 && (
          <View style={{ paddingTop: 24, paddingHorizontal: 16 }}>
            <EmptyState onAddVial={onAddVial} />
          </View>
        )}

        {/* Active section */}
        {active.length > 0 && (
          <View style={{ paddingTop: 4, paddingHorizontal: 16, paddingBottom: 14 }}>
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'baseline',
                paddingHorizontal: 4,
                paddingBottom: 10,
              }}
            >
              <Eyebrow color={pal.subtle}>Активные</Eyebrow>
              <Text
                style={{
                  fontFamily: F.mono,
                  fontSize: 11,
                  color: pal.subtle,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {active.length} в работе
              </Text>
            </View>
            <View style={{ gap: 10 }}>
              {active.map((v) => (
                <VialCard key={v.id} vial={v} onPress={() => setVialDetailId(v.id)} />
              ))}
            </View>
          </View>
        )}

        {/* Sealed / spare section — collapsible */}
        {sealed.length > 0 && (
          <View style={{ paddingTop: 4, paddingHorizontal: 16 }}>
            <Press
              onPress={() => setSealedOpen(!sealedOpen)}
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'baseline',
                paddingTop: 4,
                paddingHorizontal: 4,
                paddingBottom: 10,
              }}
            >
              <Eyebrow color={pal.subtle}>В запасе</Eyebrow>
              <View style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}>
                <Text
                  style={{
                    fontFamily: F.mono,
                    fontSize: 11,
                    fontVariant: ['tabular-nums'],
                    color: pal.subtle,
                  }}
                >
                  {sealed.length}
                </Text>
                <Icon name={sealedOpen ? 'chevron-up' : 'chevron-down'} size={14} color={pal.muted} />
              </View>
            </Press>
            {sealedOpen && (
              <View
                style={{
                  backgroundColor: pal.paper,
                  borderRadius: R.lg,
                  borderWidth: 1,
                  borderColor: pal.hairline,
                }}
              >
                {sealed.map((v, i) => (
                  <React.Fragment key={v.id}>
                    <VialRow vial={v} onPress={() => setVialDetailId(v.id)} />
                    {i < sealed.length - 1 && (
                      <View style={{ height: 1, backgroundColor: pal.hairline, marginLeft: 60 }} />
                    )}
                  </React.Fragment>
                ))}
              </View>
            )}
          </View>
        )}
      </ScrollView>

      <CadenceTabBar active="inventory" onChange={onChangeTab} />

      <VialDetailSheet
        open={detailVial !== null}
        vial={detailVial}
        onClose={() => setVialDetailId(null)}
        onLogDose={() => {
          setVialDetailId(null);
          onLogDose();
        }}
      />
    </View>
  );
}

// ── Summary hero card ───────────────────────────────────────────────

function InventorySummaryCard({ sum, inv }: { sum: InventorySummary; inv: Vial[] }) {
  const total = sum.active.length + sum.sealed.length;
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 20,
        padding: 18,
        borderWidth: 1,
        borderColor: pal.hairline,
        shadowColor: '#2e2618',
        shadowOpacity: 0.05,
        shadowRadius: 8,
        shadowOffset: { width: 0, height: 2 },
        elevation: 2,
      }}
    >
      <View style={{ marginBottom: 14 }}>
        <Eyebrow color={pal.subtle} style={{ marginBottom: 4 }}>
          Запас в норме
        </Eyebrow>
        <Text
          style={{
            fontFamily: F.display,
            fontSize: 22,
            lineHeight: 22 * 1.15,
            letterSpacing: 22 * -0.012,
            color: pal.ink,
          }}
        >
          <Text style={{ color: C.forest700 }}>{sum.active.length}</Text> открыты,{' '}
          <Text style={{ color: C.sand700 }}>{sum.sealed.length}</Text> в запасе
        </Text>
      </View>

      {/* Status dots row — visual representation of every vial */}
      <View style={{ flexDirection: 'row', gap: 5, marginBottom: 14, flexWrap: 'wrap' }}>
        {inv.map((v) => (
          <View
            key={v.id}
            style={{
              width: 10,
              height: 10,
              borderRadius: R.pill,
              backgroundColor:
                v.status === 'expiring'
                  ? C.warning
                  : v.status === 'low'
                    ? C.danger
                    : v.status === 'sealed'
                      ? C.sand500
                      : C.forest700,
              opacity: v.status === 'sealed' ? 0.6 : 1,
            }}
          />
        ))}
      </View>

      {/* Mini stats grid */}
      <View
        style={{
          flexDirection: 'row',
          gap: 12,
          paddingTop: 12,
          borderTopWidth: 1,
          borderTopColor: pal.hairline,
        }}
      >
        <SummaryStat label="истекает срок годности" value={sum.expiring.length} color={C.warning} />
        <SummaryStat label="заканчиваются" value={sum.low.length} color={C.danger} />
        <SummaryStat label="всего" value={total} color={C.forest700} />
      </View>
    </View>
  );
}

function SummaryStat({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <View style={{ flex: 1 }}>
      <Text
        style={{
          fontFamily: F.monoMedium,
          fontSize: 22,
          color: value === 0 ? pal.subtle : color,
          letterSpacing: 22 * -0.02,
          fontVariant: ['tabular-nums'],
          lineHeight: 22,
        }}
      >
        {value}
      </Text>
      {/* Single-line eyebrow — shrinks instead of wrapping mid-word ('ЗАКАНЧИВАЮТСЯ'). */}
      <Text
        numberOfLines={1}
        adjustsFontSizeToFit
        minimumFontScale={0.8}
        style={{
          fontFamily: F.bodyMedium,
          fontSize: 10,
          letterSpacing: 10 * 0.14,
          textTransform: 'uppercase',
          color: pal.subtle,
          marginTop: 4,
          lineHeight: 10 * 1.25,
        }}
      >
        {label}
      </Text>
    </View>
  );
}

// ── WarningCard — amber for warn, slate for info ────────────────────

function WarningCard({
  tone,
  icon,
  title,
  sub,
  ctaLabel,
  onPress,
}: {
  tone: 'warn' | 'info';
  icon: IconName;
  title: string;
  sub: string;
  ctaLabel: string;
  onPress: () => void;
}) {
  // Warn fg '#7a4a06' and 18%-warning border come straight from the prototype.
  const palette =
    tone === 'warn'
      ? { bg: C.warningBg, border: 'rgba(194,120,10,.18)', fg: '#7a4a06', accent: C.warning }
      : { bg: pal.sunk, border: pal.hairline, fg: pal.ink2, accent: C.forest700 };
  return (
    <Press
      onPress={onPress}
      style={{
        flexDirection: 'row',
        gap: 12,
        alignItems: 'center',
        backgroundColor: palette.bg,
        borderWidth: 1,
        borderColor: palette.border,
        borderRadius: 16,
        padding: 14,
      }}
    >
      <View
        style={{
          width: 40,
          height: 40,
          borderRadius: 12,
          backgroundColor: tone === 'warn' ? 'rgba(194,120,10,.18)' : pal.paper,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={icon} size={20} color={palette.accent} />
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: palette.fg }}>{title}</Text>
        <Text
          style={{
            fontFamily: F.body,
            fontSize: 11.5,
            color: palette.fg,
            opacity: 0.7,
            marginTop: 1,
            lineHeight: 11.5 * 1.35,
          }}
        >
          {sub}
        </Text>
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}>
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: palette.accent }}>
          {ctaLabel}
        </Text>
        <Icon name="arrow-right" size={14} color={palette.accent} />
      </View>
    </Press>
  );
}

// ── Status pill (plain, no leading dot — matches the prototype span) ─

function StatusPill({ label, bg, fg }: { label: string; bg: string; fg: string }) {
  return (
    <View
      style={{
        paddingVertical: 4,
        paddingHorizontal: 10,
        borderRadius: R.pill,
        backgroundColor: bg,
        alignSelf: 'flex-start',
      }}
    >
      <Text numberOfLines={1} style={{ fontFamily: F.bodyMedium, fontSize: 11, color: fg }}>
        {label}
      </Text>
    </View>
  );
}

// ── VialCard — paper card for active vials ──────────────────────────

export function VialCard({ vial, onPress }: { vial: Vial; onPress: () => void }) {
  const status = vial.status;
  const meta = vial.compoundMeta as CompoundMeta;

  const statusPill =
    status === 'expiring'
      ? { label: 'Истекает', bg: C.warningBg, fg: C.warning }
      : status === 'low'
        ? { label: 'На исходе', bg: C.dangerBg, fg: C.danger }
        : { label: 'Активный', bg: C.forest50, fg: C.forest800 };

  const barColor = status === 'expiring' ? C.warning : status === 'low' ? C.danger : C.forest700;

  return (
    <Press
      onPress={onPress}
      style={{
        backgroundColor: pal.paper,
        borderRadius: R.lg,
        padding: 16,
        borderWidth: 1,
        borderColor: pal.hairline,
        shadowColor: '#2e2618',
        shadowOpacity: 0.04,
        shadowRadius: 6,
        shadowOffset: { width: 0, height: 2 },
        elevation: 2,
      }}
    >
      {/* Header row */}
      <View style={{ flexDirection: 'row', gap: 12, alignItems: 'center', marginBottom: 14 }}>
        <View
          style={{
            width: 40,
            height: 40,
            borderRadius: 12,
            backgroundColor: pal.sunk,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name={(meta.icon as IconName) || 'beaker'} size={20} color={pal.ink2} />
        </View>
        <View style={{ flex: 1, minWidth: 0 }}>
          <Text
            numberOfLines={1}
            style={{
              fontFamily: F.display,
              fontSize: 19,
              lineHeight: 19 * 1.15,
              letterSpacing: 19 * -0.012,
              color: pal.ink,
            }}
          >
            {meta.name}
          </Text>
          <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.muted, marginTop: 1 }}>
            {vial.dose}
          </Text>
        </View>
        <StatusPill label={statusPill.label} bg={statusPill.bg} fg={statusPill.fg} />
      </View>

      {/* Doses bar */}
      <View style={{ marginBottom: 12 }}>
        <View
          style={{
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'baseline',
            marginBottom: 4,
          }}
        >
          <Text style={{ fontFamily: F.body, fontSize: 11.5, color: pal.muted }}>Доз осталось</Text>
          <Text style={{ fontFamily: F.mono, fontSize: 12.5, fontVariant: ['tabular-nums'] }}>
            <Text style={{ fontFamily: F.monoMedium, color: pal.ink }}>{vial.remaining}</Text>
            <Text style={{ color: pal.subtle }}> / {vial.total}</Text>
            <Text style={{ color: pal.subtle }}>{'  '}{Math.round(vial.pct * 100)}%</Text>
          </Text>
        </View>
        <View
          style={{
            height: 6,
            backgroundColor: pal.sunk,
            borderRadius: R.pill,
            overflow: 'hidden',
          }}
        >
          <View
            style={{
              width: `${vial.pct * 100}%`,
              height: '100%',
              backgroundColor: barColor,
              borderRadius: R.pill,
            }}
          />
        </View>
      </View>

      {/* Meta strip — opened · last dose · expiry */}
      <View
        style={{
          flexDirection: 'row',
          gap: 12,
          paddingTop: 13,
          marginBottom: 4,
          borderTopWidth: 1,
          borderTopColor: pal.hairline,
        }}
      >
        <View style={{ flex: 1, minWidth: 0 }}>
          <Eyebrow color={pal.subtle} style={{ fontSize: 10, marginBottom: 3 }}>
            Открыт
          </Eyebrow>
          <Text style={{ fontFamily: F.body, fontSize: 12.5, color: pal.ink2 }}>
            {vial.daysSinceOpened} {daysWord(vial.daysSinceOpened ?? 0)} назад
          </Text>
        </View>
        <View style={{ flex: 1, minWidth: 0 }}>
          <Eyebrow color={pal.subtle} style={{ fontSize: 10, marginBottom: 3 }}>
            Последняя
          </Eyebrow>
          <Text numberOfLines={1} style={{ fontFamily: F.body, fontSize: 12.5, color: pal.ink2 }}>
            {vial.lastDose ? vial.lastDose.dateLabel : '—'}
          </Text>
        </View>
        <View style={{ flex: 1, minWidth: 0, alignItems: 'flex-end' }}>
          <Eyebrow color={pal.subtle} style={{ fontSize: 10, marginBottom: 3 }}>
            До истечения
          </Eyebrow>
          <Text
            numberOfLines={1}
            style={{
              fontFamily: status === 'expiring' ? F.bodyMedium : F.body,
              fontSize: 12.5,
              color: status === 'expiring' ? C.warning : pal.ink2,
            }}
          >
            {vial.expiresLabel}
          </Text>
        </View>
      </View>

      {/* Lot + location */}
      <View
        style={{
          paddingTop: 12,
          marginTop: 6,
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: 12,
        }}
      >
        <Text
          style={{
            fontFamily: F.mono,
            fontSize: 10,
            color: pal.subtle,
            fontVariant: ['tabular-nums'],
            letterSpacing: 10 * -0.01,
          }}
        >
          Лот {vial.lot}
        </Text>
        <Text
          numberOfLines={1}
          style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, flexShrink: 1 }}
        >
          {vial.location}
        </Text>
      </View>
    </Press>
  );
}

// ── VialRow — compact list row for sealed/spare vials ───────────────

export function VialRow({ vial, onPress }: { vial: Vial; onPress: () => void }) {
  const meta = vial.compoundMeta as CompoundMeta;
  return (
    <Press
      onPress={onPress}
      style={{
        flexDirection: 'row',
        gap: 12,
        alignItems: 'center',
        paddingVertical: 12,
        paddingHorizontal: 14,
      }}
    >
      <View
        style={{
          width: 32,
          height: 32,
          borderRadius: R.pill,
          borderWidth: 1.5,
          borderColor: pal.border,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={(meta.icon as IconName) || 'beaker'} size={16} color={pal.muted} />
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text numberOfLines={1} style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: pal.ink }}>
          {meta.name} · {vial.dose}
        </Text>
        <Text
          numberOfLines={1}
          style={{
            fontFamily: F.body,
            fontSize: 11.5,
            color: pal.subtle,
            marginTop: 1,
            fontVariant: ['tabular-nums'],
          }}
        >
          {vial.remaining}/{vial.total} доз · лот {vial.lot}
        </Text>
      </View>
      <View style={{ alignItems: 'flex-end' }}>
        <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>до</Text>
        <Text
          style={{
            fontFamily: F.mono,
            fontSize: 11.5,
            color: pal.ink2,
            fontVariant: ['tabular-nums'],
          }}
        >
          {vial.expiresLabel}
        </Text>
      </View>
      <Icon name="chevron-right" size={14} color={pal.subtle} />
    </Press>
  );
}

// ── Empty state ─────────────────────────────────────────────────────

function EmptyState({ onAddVial }: { onAddVial: () => void }) {
  return (
    <View
      style={{
        backgroundColor: pal.paper,
        borderRadius: 22,
        paddingVertical: 32,
        paddingHorizontal: 24,
        borderWidth: 1,
        borderStyle: 'dashed',
        borderColor: pal.border,
        alignItems: 'center',
      }}
    >
      <View
        style={{
          width: 64,
          height: 64,
          borderRadius: R.pill,
          backgroundColor: C.sand100,
          alignItems: 'center',
          justifyContent: 'center',
          marginBottom: 16,
        }}
      >
        <Icon name="beaker" size={28} color={C.sand700} />
      </View>
      <Title size={26} style={{ marginBottom: 8, textAlign: 'center' }}>
        Аптечка <TitleEm size={26} color={C.sand700}>пуста</TitleEm>
      </Title>
      <Text
        style={{
          fontFamily: F.body,
          fontSize: 13,
          color: pal.muted,
          lineHeight: 13 * 1.5,
          marginBottom: 18,
          maxWidth: 260,
          textAlign: 'center',
        }}
      >
        Добавьте первый флакон, чтобы отсчёт сроков и расход доз пошёл сам.
      </Text>
      <Press
        onPress={onAddVial}
        style={{
          paddingVertical: 12,
          paddingHorizontal: 24,
          borderRadius: R.pill,
          backgroundColor: C.forest700,
          flexDirection: 'row',
          alignItems: 'center',
          gap: 8,
        }}
      >
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 14, color: C.cream }}>
          Добавить флакон
        </Text>
        <Text style={{ fontFamily: F.displayItalic, fontSize: 14, color: C.cream }}>→</Text>
      </Press>
    </View>
  );
}
