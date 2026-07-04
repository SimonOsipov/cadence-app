// Cadence · Самочувствие — quick-add "log how you feel" bottom sheet.
// Ported from journal/journal.jsx (QuickFeelSheet). Built on the Sheet primitive.
import React, { useEffect, useState } from 'react';
import { ScrollView, Text, TextInput, View } from 'react-native';
import { C, F, pal } from '../../theme';
import { Icon } from '../../components/Icon';
import { Eyebrow, Press } from '../../components/primitives';
import { Sheet } from '../../components/shared';
import { JOURNAL } from './data';

const J = JOURNAL;

function SheetSection({
  title,
  sub,
  children,
}: {
  title: string;
  sub?: string;
  children: React.ReactNode;
}) {
  return (
    <View style={{ marginBottom: 20 }}>
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'baseline',
          marginBottom: 10,
        }}
      >
        <Eyebrow color={pal.subtle}>{title}</Eyebrow>
        {sub ? (
          <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.ink2 }}>{sub}</Text>
        ) : null}
      </View>
      {children}
    </View>
  );
}

function StepDots({
  value,
  onChange,
  accent,
}: {
  value: number;
  onChange: (n: number) => void;
  accent: string;
}) {
  return (
    <View style={{ flexDirection: 'row', gap: 8 }}>
      {[1, 2, 3, 4, 5].map((n) => {
        const on = n <= value;
        return (
          <Press
            key={n}
            onPress={() => onChange(n)}
            accessibilityLabel={`${n}`}
            style={{
              flex: 1,
              height: 38,
              borderRadius: 12,
              backgroundColor: on ? accent : pal.sunk,
            }}
          />
        );
      })}
    </View>
  );
}

export function QuickFeelSheet({
  open,
  onCancel,
  onSave,
}: {
  open: boolean;
  onCancel: () => void;
  onSave: (entry: any) => void;
}) {
  const [mood, setMood] = useState(0);
  const [energy, setEnergy] = useState(3);
  const [sleep, setSleep] = useState(3);
  const [tags, setTags] = useState<string[]>([]);
  const [note, setNote] = useState('');

  // Fresh state on every open (the prototype remounts the sheet).
  useEffect(() => {
    if (open) {
      setMood(0);
      setEnergy(3);
      setSleep(3);
      setTags([]);
      setNote('');
    }
  }, [open]);

  const toggleTag = (id: string) =>
    setTags((t) => (t.includes(id) ? t.filter((x) => x !== id) : [...t, id]));
  const moodMeta = mood ? J.MOOD[mood] : null;

  return (
    <Sheet open={open} onClose={onCancel} contentStyle={{ maxHeight: '92%', paddingHorizontal: 0 }}>
      {/* Header */}
      <View
        style={{
          paddingHorizontal: 20,
          paddingTop: 2,
          paddingBottom: 8,
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <Text
          style={{
            fontFamily: F.display,
            fontSize: 26,
            letterSpacing: 26 * -0.018,
            color: pal.ink,
          }}
        >
          Как вы себя{' '}
          <Text style={{ fontFamily: F.displayItalic, color: C.forest700 }}>чувствуете?</Text>
        </Text>
        <Press
          onPress={onCancel}
          style={{
            width: 34,
            height: 34,
            borderRadius: 999,
            backgroundColor: pal.sunk,
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icon name="x-mark" size={17} color={pal.ink2} />
        </Press>
      </View>

      <ScrollView
        style={{ flexGrow: 0 }}
        contentContainerStyle={{ paddingTop: 12, paddingHorizontal: 20, paddingBottom: 8 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Mood */}
        <SheetSection title="Настроение" sub={moodMeta ? moodMeta.label : ''}>
          <View style={{ flexDirection: 'row', gap: 7 }}>
            {[1, 2, 3, 4, 5].map((m) => {
              const meta = J.MOOD[m]!;
              const on = mood === m;
              return (
                <Press
                  key={m}
                  onPress={() => setMood(m)}
                  style={{
                    flex: 1,
                    alignItems: 'center',
                    gap: 7,
                    paddingVertical: 12,
                    paddingHorizontal: 4,
                    borderRadius: 14,
                    backgroundColor: on ? meta.soft : pal.sunk,
                    borderWidth: 1.5,
                    borderColor: on ? meta.color : 'transparent',
                  }}
                >
                  <View
                    style={{ width: 22, height: 22, borderRadius: 999, backgroundColor: meta.color }}
                  />
                  <Text
                    numberOfLines={1}
                    style={{
                      fontFamily: F.bodyMedium,
                      fontSize: 10,
                      lineHeight: 11,
                      color: on ? meta.color : pal.subtle,
                    }}
                  >
                    {meta.label}
                  </Text>
                </Press>
              );
            })}
          </View>
        </SheetSection>

        {/* Energy */}
        <SheetSection title="Энергия" sub={J.ENERGY_LABEL[energy]}>
          <StepDots value={energy} onChange={setEnergy} accent={C.forest700} />
        </SheetSection>

        {/* Sleep */}
        <SheetSection title="Сон" sub={J.SLEEP_LABEL[sleep]}>
          <StepDots value={sleep} onChange={setSleep} accent={C.sand700} />
        </SheetSection>

        {/* Tags */}
        <SheetSection title="Что отметить">
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {J.TAGS.map((t) => {
              const on = tags.includes(t.id);
              return (
                <Press
                  key={t.id}
                  onPress={() => toggleTag(t.id)}
                  style={{
                    paddingVertical: 8,
                    paddingHorizontal: 14,
                    borderRadius: 999,
                    backgroundColor: on ? C.forest700 : 'transparent',
                    borderWidth: 1,
                    borderColor: on ? C.forest700 : pal.border,
                  }}
                >
                  <Text
                    style={{
                      fontFamily: F.bodyMedium,
                      fontSize: 13,
                      color: on ? C.cream : pal.ink2,
                    }}
                  >
                    {t.label}
                  </Text>
                </Press>
              );
            })}
          </View>
        </SheetSection>

        {/* Note */}
        <SheetSection title="Заметка">
          <TextInput
            value={note}
            onChangeText={setNote}
            multiline
            placeholder="Пара слов о дне — по желанию"
            placeholderTextColor={pal.placeholder}
            textAlignVertical="top"
            style={{
              minHeight: 88,
              borderWidth: 1,
              borderColor: pal.border,
              borderRadius: 14,
              paddingVertical: 12,
              paddingHorizontal: 14,
              backgroundColor: pal.paper,
              fontFamily: F.body,
              fontSize: 14,
              lineHeight: 21,
              color: pal.ink,
            }}
          />
        </SheetSection>
      </ScrollView>

      {/* Save */}
      <View
        style={{
          paddingHorizontal: 20,
          paddingTop: 10,
          borderTopWidth: 1,
          borderTopColor: pal.hairline,
        }}
      >
        <Press
          disabled={!mood}
          onPress={() =>
            mood && onSave({ mood, energy, sleep, tags, note: note.trim(), source: 'manual' })
          }
          style={{
            width: '100%',
            paddingVertical: 15,
            paddingHorizontal: 20,
            borderRadius: 999,
            opacity: mood ? 1 : 0.4,
            backgroundColor: C.forest700,
            flexDirection: 'row',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 8,
          }}
        >
          <Text style={{ fontFamily: F.bodyMedium, fontSize: 15, color: C.cream }}>
            Сохранить запись
          </Text>
          {mood ? <Icon name="check" size={16} strokeWidth={2} color={C.cream} /> : null}
        </Press>
      </View>
    </Sheet>
  );
}
