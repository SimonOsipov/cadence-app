// Cadence · Log Dose — data, ported from log-dose/shared-log.jsx.

export interface Compound {
  id: string;
  name: string;
  queued: boolean;
  default: string;
  unit: string;
  mode: string;
  syringeMax: number;
  syringeFill: number;
}

export const COMPOUNDS: Compound[] = [
  { id: 'sema', name: 'Семаглутид', queued: true,  default: '0.25', unit: 'мг',  mode: 'п/к · еженедельно',  syringeMax: 100, syringeFill: 25 },
  { id: 'bpc',  name: 'BPC-157',     queued: false, default: '250',  unit: 'мкг', mode: 'п/к · 2× в день', syringeMax: 100, syringeFill: 50 },
  { id: 'tb',   name: 'TB-500',      queued: false, default: '2.5',  unit: 'мг',  mode: 'п/к · 2× в неделю', syringeMax: 100, syringeFill: 50 },
  { id: 'tes',  name: 'Тезаморелин', queued: false, default: '1.0',  unit: 'мг',  mode: 'п/к · ежедневно',     syringeMax: 100, syringeFill: 40 },
];

export interface Zone {
  id: string;
  label: string;
  cx: number;
  cy: number;
}

export const ZONES_FRONT: Zone[] = [
  { id: 'r-delt',    label: 'Правое плечо',   cx: 54,  cy: 88 },
  { id: 'l-delt',    label: 'Левое плечо',    cx: 146, cy: 88 },
  { id: 'r-abdomen', label: 'Правый живот',   cx: 82,  cy: 148 },
  { id: 'l-abdomen', label: 'Левый живот',    cx: 118, cy: 148 },
  { id: 'r-thigh',   label: 'Правое бедро',   cx: 82,  cy: 230 },
  { id: 'l-thigh',   label: 'Левое бедро',    cx: 118, cy: 230 },
];

export const ZONES_BACK: Zone[] = [
  { id: 'l-lback',   label: 'Левая поясница',   cx: 82,  cy: 175 },
  { id: 'r-lback',   label: 'Правая поясница',  cx: 118, cy: 175 },
  { id: 'l-glute',   label: 'Левая ягодица',    cx: 82,  cy: 210 },
  { id: 'r-glute',   label: 'Правая ягодица',   cx: 118, cy: 210 },
];

export const ALL_ZONES: Zone[] = [...ZONES_FRONT, ...ZONES_BACK];
export const zoneLabel = (id: string | null): string =>
  (id && ALL_ZONES.find((z) => z.id === id)?.label) || '—';

export const SIDE_EFFECTS = [
  { id: 'nausea',    label: 'Тошнота' },
  { id: 'fatigue',   label: 'Усталость' },
  { id: 'headache',  label: 'Голова' },
  { id: 'bloating',  label: 'Вздутие' },
  { id: 'insomnia',  label: 'Бессонница' },
  { id: 'site',      label: 'Шишка' },
  { id: 'appetite',  label: 'Нет аппетита' },
  { id: 'none',      label: 'Ничего' },
] as const;

export interface LogVial {
  id: string;
  compound: string;
  dose: string;
  remaining: number;
  total: number;
  opened: string;
  expires: string;
  active: boolean;
  warn?: boolean;
}

export const VIALS: LogVial[] = [
  { id: 'v1', compound: 'sema', dose: '0,25 мг',  remaining: 8,  total: 12, opened: '2 апр', expires: '14 июн', active: true },
  { id: 'v2', compound: 'sema', dose: '0,25 мг',  remaining: 12, total: 12, opened: '—',     expires: '22 авг', active: false },
  { id: 'v3', compound: 'bpc',  dose: '250 мкг',  remaining: 14, total: 30, opened: '18 апр', expires: '6 июн', active: true, warn: true },
];

export const compoundById = (id: string | null): Compound =>
  COMPOUNDS.find((c) => c.id === id) || COMPOUNDS[0];

// Russian decimal display: 0.25 -> "0,25".
export const fmtDose = (d: string): string => d.replace('.', ',');

export interface LogState {
  compound: string;
  dose: string;
  unit: string;
  vialId: string;
  site: string | null;
  suggested: string;
  lastUsed: string[];
  view: 'front' | 'back';
  mood: number;
  sides: string[];
  note: string;
  photo: 'pending' | 'attached' | null;
  time: string;
  date: string;
}

// Defaults pre-filled from today's queued protocol.
export const INITIAL_LOG_STATE: LogState = {
  compound: 'sema',
  dose: '0.25',
  unit: 'мг',
  vialId: 'v1',
  site: null,                  // selected zone id
  suggested: 'l-abdomen',      // rotation suggests left abdomen (last was right)
  lastUsed: ['r-abdomen'],
  view: 'front',
  mood: 3,                     // 1..5
  sides: [],                   // selected side-effect ids
  note: '',
  photo: null,
  time: '06:42',
  date: 'Сегодня',
};
