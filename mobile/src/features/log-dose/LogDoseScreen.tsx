// Cadence · Log Dose — five-step wizard ported from log-dose/wizards.jsx (LogDoseV1).
// Compound → Dose → Site → Check-in → Review → warm confirmation sheet.
import React, { useEffect, useRef, useState } from 'react';
import { Text, TextInput, View } from 'react-native';
import { C, F, pal } from '../../theme';
import { Icon, IconName } from '../../components/Icon';
import { Body, Eyebrow, Press, Title, TitleEm } from '../../components/primitives';
import { Sheet } from '../../components/shared';
import {
  COMPOUNDS,
  INITIAL_LOG_STATE,
  LogState,
  SIDE_EFFECTS,
  VIALS,
  compoundById,
  fmtDose,
  zoneLabel,
} from './data';
import {
  BodyDiagram,
  ChipsRow,
  DoseStepper,
  MoodSlider,
  PhotoSlot,
  RadioDot,
  SyringeBar,
  VialPicker,
  WizardChrome,
} from './components';

const TOTAL = 5;

export function LogDoseScreen({
  onCancel,
  onComplete,
}: {
  onCancel: () => void;
  onComplete: (site?: string) => void;
}) {
  const [state, setState] = useState<LogState>(INITIAL_LOG_STATE);
  const [step, setStep] = useState(1);
  const [confirmed, setConfirmed] = useState(false);
  const update = (patch: Partial<LogState>) => setState((s) => ({ ...s, ...patch }));

  const c = compoundById(state.compound);

  const next = () => {
    if (step === TOTAL) {
      setConfirmed(true);
      return;
    }
    setStep((s) => Math.min(TOTAL, s + 1));
  };
  const prev = () => setStep((s) => Math.max(1, s - 1));
  const closeConfirm = () => {
    setConfirmed(false);
    onComplete(zoneLabel(state.site));
  };

  // Auto-dismiss the warm confirmation after 2.6s (prototype behavior).
  useEffect(() => {
    if (!confirmed) return;
    const t = setTimeout(closeConfirm, 2600);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [confirmed]);

  const stepDefs: {
    eyebrow: string;
    title: React.ReactNode;
    sub: string | null;
    nextDisabled?: boolean;
    body: React.ReactNode;
  }[] = [
    {
      eyebrow: 'Шаг 1 · Препарат',
      title: (
        <Title size={34}>
          Что вы <TitleEm size={34}>приняли</TitleEm>?
        </Title>
      ),
      sub: 'Сегодняшняя доза уже выбрана.',
      nextDisabled: !state.compound,
      body: (
        <View style={{ gap: 8 }}>
          {COMPOUNDS.map((comp) => {
            const sel = state.compound === comp.id;
            return (
              <Press
                key={comp.id}
                onPress={() =>
                  update({
                    compound: comp.id,
                    dose: comp.default,
                    unit: comp.unit,
                    vialId:
                      VIALS.find((v) => v.compound === comp.id && v.active)?.id || state.vialId,
                  })
                }
                style={{
                  paddingVertical: 14,
                  paddingHorizontal: 16,
                  borderRadius: 14,
                  backgroundColor: sel ? pal.paper : 'transparent',
                  borderWidth: 1,
                  borderColor: sel ? C.forest700 : pal.border,
                  flexDirection: 'row',
                  gap: 12,
                  alignItems: 'center',
                }}
              >
                <RadioDot selected={sel} />
                <View style={{ flex: 1 }}>
                  <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8 }}>
                    <Text style={{ fontFamily: F.bodyMedium, fontSize: 15, color: pal.ink }}>
                      {comp.name}
                    </Text>
                    {comp.queued && (
                      <View
                        style={{
                          paddingVertical: 2,
                          paddingHorizontal: 8,
                          borderRadius: 999,
                          backgroundColor: C.sand100,
                        }}
                      >
                        <Text
                          style={{ fontFamily: F.bodyMedium, fontSize: 10, color: C.sand700 }}
                        >
                          сегодня
                        </Text>
                      </View>
                    )}
                  </View>
                  <Text
                    style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}
                  >
                    {fmtDose(comp.default)} {comp.unit} · {comp.mode}
                  </Text>
                </View>
              </Press>
            );
          })}
        </View>
      ),
    },
    {
      eyebrow: 'Шаг 2 · Доза',
      title: (
        <Title size={34}>
          Сколько <TitleEm size={34}>взять</TitleEm>?
        </Title>
      ),
      sub: `По умолчанию для ${c.name} — ${fmtDose(c.default)} ${c.unit}. Нажмите +/− для подстройки.`,
      body: (
        <View style={{ gap: 22 }}>
          {/* Dose stepper */}
          <View
            style={{
              paddingTop: 28,
              paddingHorizontal: 16,
              paddingBottom: 22,
              backgroundColor: pal.paper,
              borderRadius: 20,
              borderWidth: 1,
              borderColor: pal.hairline,
            }}
          >
            <DoseStepper
              value={state.dose}
              onChange={(v) => update({ dose: v })}
              step={state.unit === 'мкг' ? 25 : 0.05}
              unit={state.unit}
            />
            {/* Syringe */}
            <View style={{ marginTop: 24 }}>
              <Eyebrow style={{ marginBottom: 10 }}>На шприце 100 ед.</Eyebrow>
              <SyringeBar
                fill={Math.min(
                  100,
                  Math.max(
                    1,
                    (parseFloat(state.dose || '0') / parseFloat(c.default)) * c.syringeFill
                  )
                )}
                max={c.syringeMax}
              />
            </View>
          </View>

          {/* Vial picker */}
          <View>
            <View
              style={{
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'baseline',
                paddingHorizontal: 4,
                paddingBottom: 8,
              }}
            >
              <Eyebrow>Из вашей аптечки</Eyebrow>
              <Press onPress={() => {}}>
                <Text style={{ fontFamily: F.bodyMedium, fontSize: 12, color: C.forest700 }}>
                  Добавить
                </Text>
              </Press>
            </View>
            <VialPicker state={state} update={update} compoundFilter={state.compound} />
          </View>
        </View>
      ),
    },
    {
      eyebrow: 'Шаг 3 · Место',
      title: (
        <Title size={34}>
          Куда на <TitleEm size={34}>теле</TitleEm>?
        </Title>
      ),
      sub: 'Предложим следующую зону по ротации — выберите любую.',
      nextDisabled: !state.site,
      body: (
        <View style={{ alignItems: 'center' }}>
          <BodyDiagram state={state} update={update} size={210} />
        </View>
      ),
    },
    {
      eyebrow: 'Шаг 4 · Контекст',
      title: (
        <Title size={34}>
          Как вы <TitleEm size={34}>себя чувствуете</TitleEm>?
        </Title>
      ),
      sub: 'Короткая сверка — всё по желанию.',
      body: (
        <View style={{ gap: 22 }}>
          {/* Mood */}
          <View>
            <Eyebrow style={{ marginBottom: 10 }}>Энергия · сегодня</Eyebrow>
            <MoodSlider value={state.mood} onChange={(v) => update({ mood: v })} />
          </View>

          {/* Side effects */}
          <View>
            <Eyebrow style={{ marginBottom: 10 }}>Что-то беспокоит?</Eyebrow>
            <ChipsRow value={state.sides} onChange={(v) => update({ sides: v })} />
          </View>

          {/* Notes */}
          <View>
            <Eyebrow style={{ marginBottom: 10 }}>Заметки</Eyebrow>
            <TextInput
              value={state.note}
              onChangeText={(t) => update({ note: t })}
              placeholder="Что-то важное про эту дозу?"
              placeholderTextColor={pal.placeholder}
              multiline
              style={{
                minHeight: 70,
                paddingVertical: 12,
                paddingHorizontal: 14,
                backgroundColor: pal.paper,
                borderWidth: 1,
                borderColor: pal.border,
                borderRadius: 14,
                fontFamily: F.body,
                fontSize: 13,
                color: pal.ink,
                textAlignVertical: 'top',
              }}
            />
          </View>

          {/* Photo */}
          <PhotoSlot state={state} update={update} />
        </View>
      ),
    },
    {
      eyebrow: 'Шаг 5 · Проверка',
      title: (
        <Title size={34}>
          Последний <TitleEm size={34}>взгляд</TitleEm>.
        </Title>
      ),
      sub: null,
      body: <ReviewSummary state={state} />,
    },
  ];

  const cur = stepDefs[step - 1];

  return (
    <View style={{ flex: 1, backgroundColor: pal.bg }}>
      <WizardChrome
        step={step}
        total={TOTAL}
        onCancel={onCancel}
        onPrev={prev}
        onNext={next}
        nextLabel={step === TOTAL ? 'Сохранить дозу' : 'Дальше'}
        nextDisabled={cur.nextDisabled}
        eyebrow={cur.eyebrow}
        title={cur.title}
        sub={cur.sub}
      >
        {cur.body}
      </WizardChrome>

      {/* Warm confirmation — serif moment, auto-dismiss */}
      <Sheet
        open={confirmed}
        onClose={closeConfirm}
        scrim="rgba(20,44,31,0.45)"
        contentStyle={{ paddingHorizontal: 24 }}
      >
        <View style={{ alignItems: 'center', marginBottom: 14, marginTop: 4 }}>
          <View
            style={{
              width: 56,
              height: 56,
              borderRadius: 999,
              backgroundColor: C.forest700,
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Icon name="check" size={28} strokeWidth={2} color={C.cream} />
          </View>
        </View>
        <Title size={32} style={{ textAlign: 'center', marginBottom: 8 }}>
          Записано. <TitleEm size={32}>Отлично.</TitleEm>
        </Title>
        <Body size={13} color={pal.muted} style={{ textAlign: 'center', marginBottom: 14 }}>
          {c.name} · {fmtDose(state.dose)} {state.unit} · {zoneLabel(state.site)} в {state.time}
        </Body>
      </Sheet>
    </View>
  );
}

// ─────────────────────────────────────────────────────────────
// Review summary — used in step 5
// ─────────────────────────────────────────────────────────────
function ReviewSummary({ state }: { state: LogState }) {
  const c = compoundById(state.compound);
  const moodLabels = ['Никак', 'Слабо', 'Ровно', 'Хорошо', 'Светло'];
  const sideLabels =
    state.sides.length === 0
      ? '—'
      : state.sides.includes('none')
        ? 'Без замечаний'
        : state.sides
            .map((id) => SIDE_EFFECTS.find((s) => s.id === id)?.label)
            .filter(Boolean)
            .join(' · ');
  const vial = VIALS.find((v) => v.id === state.vialId);

  const rows: { icon: IconName; label: string; value: string }[] = [
    { icon: 'beaker', label: 'Флакон', value: `${vial?.dose || '—'} · ${vial?.remaining ?? '—'} доз` },
    { icon: 'bolt', label: 'Энергия', value: moodLabels[state.mood - 1] || '—' },
    { icon: 'exclamation-circle', label: 'Заметки', value: sideLabels },
    ...(state.note ? [{ icon: 'pencil' as IconName, label: 'Заметка', value: state.note }] : []),
    ...(state.photo === 'attached'
      ? [{ icon: 'camera' as IconName, label: 'Фото', value: 'Прикреплено' }]
      : []),
  ];

  return (
    <View style={{ gap: 10 }}>
      {/* Hero summary card */}
      <View
        style={{
          backgroundColor: pal.forestBg,
          borderRadius: 22,
          padding: 22,
          position: 'relative',
          overflow: 'hidden',
        }}
      >
        <View
          pointerEvents="none"
          style={{
            position: 'absolute',
            top: 8,
            left: 8,
            right: 8,
            bottom: 8,
            borderRadius: 16,
            borderWidth: 1,
            borderColor: 'rgba(212,165,116,.22)',
          }}
        />
        <Eyebrow color={C.sand300} style={{ fontSize: 10, letterSpacing: 10 * 0.14, marginBottom: 10 }}>
          {state.date} · {state.time}
        </Eyebrow>
        <Title size={30} color={C.cream}>
          {c.name}
        </Title>
        <Title size={30} italic color={C.sand300}>
          {fmtDose(state.dose)} {state.unit}
        </Title>
        <Text
          style={{
            fontFamily: F.body,
            fontSize: 13,
            color: 'rgba(246,241,234,.7)',
            marginTop: 10,
          }}
        >
          {zoneLabel(state.site)}
        </Text>
      </View>

      {/* Detail rows */}
      <View
        style={{
          backgroundColor: pal.paper,
          borderRadius: 16,
          borderWidth: 1,
          borderColor: pal.hairline,
          overflow: 'hidden',
        }}
      >
        {rows.map((r, i) => (
          <View
            key={i}
            style={{
              flexDirection: 'row',
              gap: 12,
              alignItems: 'flex-start',
              paddingVertical: 12,
              paddingHorizontal: 14,
              borderBottomWidth: i < rows.length - 1 ? 1 : 0,
              borderBottomColor: pal.hairline,
            }}
          >
            <View
              style={{
                width: 28,
                height: 28,
                borderRadius: 8,
                backgroundColor: pal.sunk,
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Icon name={r.icon} size={16} color={pal.muted} />
            </View>
            <View style={{ flex: 1 }}>
              <Text style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginBottom: 2 }}>
                {r.label}
              </Text>
              <Text style={{ fontFamily: F.body, fontSize: 13, color: pal.ink, lineHeight: 13 * 1.4 }}>
                {r.value}
              </Text>
            </View>
          </View>
        ))}
      </View>

      <Text
        style={{
          fontFamily: F.body,
          fontSize: 12,
          color: pal.subtle,
          textAlign: 'center',
          paddingVertical: 6,
          paddingHorizontal: 16,
          lineHeight: 12 * 1.5,
        }}
      >
        Нажмите <Text style={{ fontStyle: 'italic' }}>Сохранить дозу</Text>, чтобы подтвердить.
        Позже можно поправить на «Сегодня».
      </Text>
    </View>
  );
}
