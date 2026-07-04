// Biomarker bottom sheet — quick stat + mini chart + "open trend detail" CTA.
// Ported from shared.jsx BiomarkerSheet onto the Sheet primitive.
import React from 'react';
import { Text, View, useWindowDimensions } from 'react-native';
import { C, F, pal } from '../../theme';
import { Btn, Eyebrow, IconBtn, Pill, Title } from '../../components/primitives';
import { Sheet, Spark } from '../../components/shared';

export type BiomarkerSheetData = {
  trendId?: string;
  eyebrow: string;
  title: string;
  value: string;
  unit: string;
  delta: string;
  trend?: 'up' | 'down' | 'flat';
  series: number[];
  note: string;
};

export function BiomarkerSheet({
  open,
  biomarker,
  onClose,
  onOpenTrend,
}: {
  open: boolean;
  biomarker: BiomarkerSheetData | null;
  onClose: () => void;
  onOpenTrend: (id: string) => void;
}) {
  const { width } = useWindowDimensions();
  if (!open || !biomarker) return null;
  // Sheet pads 20 each side; the chart box pads 16 each side.
  const chartW = width - 40 - 32;
  return (
    <Sheet open={open} onClose={onClose}>
      <View
        style={{
          flexDirection: 'row',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          marginBottom: 14,
        }}
      >
        <View style={{ flex: 1, minWidth: 0, paddingRight: 12 }}>
          <Eyebrow color={pal.subtle} style={{ marginBottom: 6 }}>
            {biomarker.eyebrow}
          </Eyebrow>
          <Title size={32}>{biomarker.title}</Title>
        </View>
        <IconBtn
          name="x-mark"
          size={18}
          bg={pal.sunk}
          color={pal.ink2}
          onPress={onClose}
          style={{ width: 36, height: 36 }}
        />
      </View>

      <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8, marginBottom: 6 }}>
        <Text
          style={{
            fontFamily: F.monoMedium,
            fontSize: 56,
            color: pal.ink,
            letterSpacing: 56 * -0.03,
            lineHeight: 56 * 1.05,
            fontVariant: ['tabular-nums'],
          }}
        >
          {biomarker.value}
        </Text>
        <Text style={{ fontFamily: F.displayItalic, fontSize: 22, color: pal.muted }}>
          {biomarker.unit}
        </Text>
      </View>
      <View style={{ flexDirection: 'row', gap: 10, alignItems: 'center', marginBottom: 20 }}>
        <Pill tone={biomarker.trend === 'flat' ? 'neutral' : 'forest'}>{biomarker.delta}</Pill>
        <Text style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle }}>
          к прошлой неделе
        </Text>
      </View>

      {/* Mini chart */}
      <View style={{ padding: 16, backgroundColor: pal.sunk, borderRadius: 18, marginBottom: 16 }}>
        <Spark data={biomarker.series} color={C.forest700} width={chartW} height={70} />
        <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginTop: 8 }}>
          {['Пн', 'Ср', 'Пт', 'Сегодня'].map((d) => (
            <Text
              key={d}
              style={{
                fontFamily: F.mono,
                fontSize: 10,
                color: pal.subtle,
                fontVariant: ['tabular-nums'],
              }}
            >
              {d}
            </Text>
          ))}
        </View>
      </View>

      <Text
        style={{
          fontFamily: F.body,
          fontSize: 13,
          lineHeight: 13 * 1.55,
          color: pal.ink2,
          marginBottom: 18,
        }}
      >
        {biomarker.note}
      </Text>

      <Btn
        kind="primary"
        full
        onPress={() => {
          onClose();
          onOpenTrend(biomarker.trendId || 'weight');
        }}
      >
        Открыть детали тренда
      </Btn>
    </Sheet>
  );
}
