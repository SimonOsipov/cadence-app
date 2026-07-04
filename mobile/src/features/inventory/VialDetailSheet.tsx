// Cadence — Vial detail bottom sheet.
// Ported from design_handoff_cadence_app/inventory/vial-detail.jsx (VialDetailSheet).
// Full info + actions. In this clickable mockup only "log dose from vial"
// routes onward; the other actions (open, edit, photo, dispose, move to
// spare) close the sheet — same stub behavior as the prototype.
import React from 'react';
import { ScrollView, Text, View } from 'react-native';
import { C, F, pal, R } from '../../theme';
import { Eyebrow, Pill, Press, Title, TitleEm } from '../../components/primitives';
import { Icon, IconName } from '../../components/Icon';
import { Sheet } from '../../components/shared';
import { CompoundMeta, Vial, formatDayLabel } from './data';

const daysWord = (n: number) => (n === 1 ? 'день' : n < 5 ? 'дня' : 'дней');

export function VialDetailSheet({
  open,
  vial,
  onClose,
  onLogDose,
}: {
  open: boolean;
  vial: Vial | null;
  onClose: () => void;
  onLogDose: (vialId: string) => void;
}) {
  if (!vial) return null;
  const meta = vial.compoundMeta as CompoundMeta;
  const status = vial.status;

  const statusPill =
    status === 'expiring'
      ? { label: 'Истекает скоро', bg: C.warningBg, fg: C.warning }
      : status === 'low'
        ? { label: 'На исходе', bg: C.dangerBg, fg: C.danger }
        : status === 'sealed'
          ? { label: 'Запечатан', bg: C.sand100, fg: C.sand900 }
          : { label: 'Активный', bg: C.forest50, fg: C.forest800 };

  const barColor = status === 'expiring' ? C.warning : status === 'low' ? C.danger : C.forest700;

  return (
    <Sheet open={open} onClose={onClose} contentStyle={{ maxHeight: '92%' }}>
      <ScrollView showsVerticalScrollIndicator={false}>
        {/* Hero */}
        <View style={{ paddingBottom: 18 }}>
          <View
            style={{
              flexDirection: 'row',
              justifyContent: 'space-between',
              alignItems: 'flex-start',
              marginBottom: 14,
              gap: 12,
            }}
          >
            <View style={{ minWidth: 0, flex: 1 }}>
              <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
                Флакон · лот {vial.lot}
              </Eyebrow>
              <Title size={28}>
                {meta.name}
                {'\n'}
                <TitleEm size={28}>{vial.dose}</TitleEm>
              </Title>
            </View>
            <Press
              onPress={onClose}
              style={{
                width: 36,
                height: 36,
                borderRadius: R.pill,
                backgroundColor: pal.sunk,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name="x-mark" size={18} color={pal.ink2} />
            </Press>
          </View>
          <View
            style={{
              paddingVertical: 4,
              paddingHorizontal: 10,
              borderRadius: R.pill,
              backgroundColor: statusPill.bg,
              alignSelf: 'flex-start',
            }}
          >
            <Text style={{ fontFamily: F.bodyMedium, fontSize: 11, color: statusPill.fg }}>
              {statusPill.label}
            </Text>
          </View>
        </View>

        {/* Doses bar — only if opened */}
        {vial.opened && (
          <View style={{ paddingBottom: 16 }}>
            <View
              style={{
                backgroundColor: pal.paper,
                borderRadius: 16,
                padding: 14,
                borderWidth: 1,
                borderColor: pal.hairline,
              }}
            >
              <View
                style={{
                  flexDirection: 'row',
                  alignItems: 'flex-end',
                  justifyContent: 'space-between',
                  marginBottom: 8,
                  gap: 12,
                }}
              >
                <View>
                  <Eyebrow color={pal.subtle} style={{ fontSize: 10, marginBottom: 4 }}>
                    Доз осталось
                  </Eyebrow>
                  <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 4 }}>
                    <Text
                      style={{
                        fontFamily: F.monoMedium,
                        fontSize: 28,
                        color: pal.ink,
                        letterSpacing: 28 * -0.025,
                        fontVariant: ['tabular-nums'],
                        lineHeight: 28,
                      }}
                    >
                      {vial.remaining}
                    </Text>
                    <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.subtle }}>
                      / {vial.total}
                    </Text>
                  </View>
                </View>
                <Pill tone="forest">{Math.round(vial.pct * 100)}%</Pill>
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
          </View>
        )}

        {/* Key facts grid */}
        <View style={{ paddingBottom: 16 }}>
          <View style={{ flexDirection: 'row', gap: 10, marginBottom: 10 }}>
            <FactCard
              label="Открыт"
              value={
                vial.opened
                  ? `${vial.daysSinceOpened} ${daysWord(vial.daysSinceOpened ?? 0)} назад`
                  : 'Запечатан'
              }
              sub={vial.opened ? vial.openedLabel : null}
            />
            <FactCard
              label="Истекает"
              value={`${vial.daysToExpiry} ${daysWord(vial.daysToExpiry)}`}
              sub={vial.expiresLabel}
              accent={vial.daysToExpiry <= 14 ? C.warning : null}
            />
          </View>
          <View style={{ flexDirection: 'row', gap: 10 }}>
            <FactCard label="Лот" value={vial.lot} mono />
            <FactCard
              label="Хранится"
              value={vial.location.split(',')[1]?.trim() || vial.location}
              sub={vial.location.split(',')[0]}
            />
          </View>
        </View>

        {/* Recent doses */}
        {vial.recent && vial.recent.length > 0 && (
          <View style={{ paddingBottom: 16 }}>
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'baseline',
                paddingBottom: 10,
              }}
            >
              <Eyebrow color={pal.subtle}>Последние записи</Eyebrow>
              <Text
                style={{
                  fontFamily: F.mono,
                  fontSize: 11,
                  color: pal.subtle,
                  fontVariant: ['tabular-nums'],
                }}
              >
                {vial.recent.length}
              </Text>
            </View>
            <View
              style={{
                backgroundColor: pal.paper,
                borderRadius: 16,
                borderWidth: 1,
                borderColor: pal.hairline,
              }}
            >
              {vial.recent.map((r, i) => (
                <React.Fragment key={i}>
                  <View
                    style={{
                      flexDirection: 'row',
                      gap: 12,
                      alignItems: 'center',
                      paddingVertical: 11,
                      paddingHorizontal: 14,
                    }}
                  >
                    <View style={{ flex: 1, minWidth: 0 }}>
                      <Text style={{ fontFamily: F.bodyMedium, fontSize: 13, color: pal.ink }}>
                        {formatDayLabel(r.day)}
                      </Text>
                      <Text
                        style={{
                          fontFamily: F.body,
                          fontSize: 11.5,
                          color: pal.subtle,
                          marginTop: 1,
                        }}
                      >
                        {r.site}
                      </Text>
                    </View>
                    <Text
                      style={{
                        fontFamily: F.mono,
                        fontSize: 12,
                        color: pal.ink2,
                        fontVariant: ['tabular-nums'],
                      }}
                    >
                      {r.dose}
                    </Text>
                  </View>
                  {i < vial.recent.length - 1 && (
                    <View style={{ height: 1, backgroundColor: pal.hairline, marginLeft: 14 }} />
                  )}
                </React.Fragment>
              ))}
            </View>
          </View>
        )}

        {/* Action buttons */}
        <View style={{ gap: 8 }}>
          {!vial.opened && (
            <ActionRow
              icon="check-circle"
              tone="primary"
              title="Открыть флакон"
              sub="Начать отсчёт сроков и доз"
              onPress={onClose}
            />
          )}
          {vial.opened && (
            <ActionRow
              icon="paper-airplane"
              tone="primary"
              title="Записать дозу из этого флакона"
              sub="Откроем мастер с пред-заполненным флаконом"
              onPress={() => onLogDose(vial.id)}
            />
          )}
          <ActionRow icon="pencil" title="Изменить лот, дату или дозу" onPress={onClose} />
          <ActionRow icon="camera" title="Прикрепить фото" sub="Этикетка или коробка" onPress={onClose} />
          {vial.opened ? (
            <ActionRow
              icon="trash"
              tone="danger"
              title="Утилизировать"
              sub="Флакон пуст или испорчен"
              onPress={onClose}
            />
          ) : (
            <ActionRow icon="arrow-path" title="Перенести в запас" onPress={onClose} />
          )}
        </View>
      </ScrollView>
    </Sheet>
  );
}

// ── FactCard — labelled stat tile ───────────────────────────────────

function FactCard({
  label,
  value,
  sub,
  mono,
  accent,
}: {
  label: string;
  value: string;
  sub?: string | null;
  mono?: boolean;
  accent?: string | null;
}) {
  return (
    <View
      style={{
        flex: 1,
        backgroundColor: pal.paper,
        borderRadius: 14,
        padding: 12,
        borderWidth: 1,
        borderColor: pal.hairline,
      }}
    >
      <Eyebrow color={pal.subtle} style={{ fontSize: 10, marginBottom: 4 }}>
        {label}
      </Eyebrow>
      <Text
        style={{
          fontFamily: mono ? F.monoMedium : F.bodyMedium,
          fontSize: 14,
          color: accent || pal.ink,
          ...(mono ? { fontVariant: ['tabular-nums'] as const, letterSpacing: 14 * -0.01 } : null),
        }}
      >
        {value}
      </Text>
      {sub ? (
        <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}>
          {sub}
        </Text>
      ) : null}
    </View>
  );
}

// ── ActionRow — one action button in the sheet ──────────────────────

function ActionRow({
  icon,
  title,
  sub,
  tone,
  onPress,
}: {
  icon: IconName;
  title: string;
  sub?: string;
  tone?: 'primary' | 'danger';
  onPress: () => void;
}) {
  const palette =
    tone === 'primary'
      ? { fg: pal.ink, accent: C.forest700, accentBg: C.forest50 }
      : tone === 'danger'
        ? { fg: C.danger, accent: C.danger, accentBg: C.dangerBg }
        : { fg: pal.ink, accent: pal.ink2, accentBg: pal.sunk };
  return (
    <Press
      onPress={onPress}
      style={{
        flexDirection: 'row',
        gap: 12,
        alignItems: 'center',
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
          backgroundColor: palette.accentBg,
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Icon name={icon} size={18} color={palette.accent} />
      </View>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text style={{ fontFamily: F.bodyMedium, fontSize: 13.5, color: palette.fg }}>{title}</Text>
        {sub ? (
          <Text
            style={{
              fontFamily: F.body,
              fontSize: 11.5,
              color: pal.subtle,
              marginTop: 1,
              lineHeight: 11.5 * 1.35,
            }}
          >
            {sub}
          </Text>
        ) : null}
      </View>
      <Icon name="chevron-right" size={16} color={pal.subtle} />
    </Press>
  );
}
