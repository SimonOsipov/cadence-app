// Cadence · Самочувствие (Side-effects journal) — data model
// Subjective wellbeing entries across the cycle. Aligns to the schedule anchor
// (SCHED): cycle started 10 мая, today = 31 мая (day 21, week 4).
//
// Entry: { id, day, mood, energy, sleep, tags[], note, source }
//   mood/energy/sleep — 1..5
//   tags — side-effect ids
//   source — 'dose' (logged with an injection) | 'manual'

import { CYCLE_START, TODAY, addDays, daysBetween, longDate, WD_FULL } from '../schedule/data';

export type EntrySource = 'dose' | 'manual';
export type Rel = 'past' | 'today' | 'future';

export interface MoodScale { v: number; label: string; color: string; soft: string }
export interface Tag { id: string; label: string }
export interface JournalEntry {
  id: string;
  day: number;
  mood: number;
  energy: number;
  sleep: number;
  tags: string[];
  note: string;
  source: EntrySource;
}
export interface MoodPoint { day: number; mood: number; source: EntrySource }
export interface JournalStats {
  avg: number;
  count: number;
  topTag: string | null;
  topTagCount?: number;
  counts?: Record<string, number>;
}
export interface TagTallyItem { id: string; label: string; n: number }
export interface HeatmapCell {
  day: number;
  date: Date;
  entry: JournalEntry | null;
  mood: number | null;
  rel: Rel;
  titration: boolean;
}

// ── Scales ───────────────────────────────────────────────
export const MOOD: (MoodScale | null)[] = [
  null,
  { v: 1, label: 'Тяжело',    color: '#b8503c', soft: '#f4dfd6' },
  { v: 2, label: 'Так себе',  color: '#c2780a', soft: '#fbeed1' },
  { v: 3, label: 'Ровно',     color: '#8a857d', soft: '#ede5d6' },
  { v: 4, label: 'Хорошо',    color: '#3d7a52', soft: '#eaf0eb' },
  { v: 5, label: 'Светло',    color: '#2d5f3f', soft: '#dcebe0' },
];
export const ENERGY_LABEL: Record<number, string> = { 1: 'низкая', 2: 'низкая', 3: 'средняя', 4: 'хорошая', 5: 'высокая' };
export const SLEEP_LABEL: Record<number, string>  = { 1: 'плохой', 2: 'так себе', 3: 'нормальный', 4: 'крепкий', 5: 'отличный' };

export const TAGS: Tag[] = [
  { id: 'nausea',   label: 'Тошнота' },
  { id: 'fatigue',  label: 'Усталость' },
  { id: 'headache', label: 'Голова' },
  { id: 'bloating', label: 'Вздутие' },
  { id: 'insomnia', label: 'Бессонница' },
  { id: 'site',     label: 'Шишка' },
  { id: 'appetite', label: 'Нет аппетита' },
];
export function tagLabel(id: string): string { const t = TAGS.find(x => x.id === id); return t ? t.label : id; }

// ── Seed entries (days 0..20; today=21 left open for quick-add) ──
export const SEED: Omit<JournalEntry, 'id'>[] = [
  { day: 0,  mood: 3, energy: 3, sleep: 3, tags: [],                     note: 'Первая доза. Волновалась, но всё прошло спокойно.', source: 'dose' },
  { day: 1,  mood: 2, energy: 2, sleep: 3, tags: ['nausea', 'fatigue'],  note: 'К обеду подташнивало, к вечеру отпустило.',         source: 'manual' },
  { day: 2,  mood: 2, energy: 2, sleep: 2, tags: ['nausea', 'appetite'], note: 'Аппетита почти нет. Ела совсем понемногу.',         source: 'manual' },
  { day: 4,  mood: 3, energy: 3, sleep: 3, tags: [],                     note: '',                                                  source: 'manual' },
  { day: 6,  mood: 3, energy: 4, sleep: 4, tags: [],                     note: 'Тошнота ушла. Стало заметно легче.',                source: 'manual' },
  { day: 7,  mood: 3, energy: 3, sleep: 3, tags: ['fatigue'],            note: 'Вторая доза. Под вечер немного устала.',            source: 'dose' },
  { day: 9,  mood: 3, energy: 3, sleep: 4, tags: [],                     note: '',                                                  source: 'manual' },
  { day: 11, mood: 4, energy: 4, sleep: 4, tags: ['headache'],           note: 'Голова к вечеру, выпила воды — прошло.',             source: 'manual' },
  { day: 13, mood: 4, energy: 4, sleep: 4, tags: [],                     note: '',                                                  source: 'manual' },
  { day: 14, mood: 4, energy: 4, sleep: 4, tags: [],                     note: 'Третья доза. Переношу заметно легче.',              source: 'dose' },
  { day: 16, mood: 4, energy: 4, sleep: 5, tags: [],                     note: 'Сон стал крепче — высыпаюсь.',                      source: 'manual' },
  { day: 18, mood: 3, energy: 3, sleep: 3, tags: ['fatigue', 'bloating'],note: 'День потяжелее, без явной причины. Бывает.',         source: 'manual' },
  { day: 20, mood: 4, energy: 5, sleep: 4, tags: [],                     note: '',                                                  source: 'manual' },
];
export const ENTRIES: JournalEntry[] = SEED.map((e, i) => ({ id: `seed-${i}`, ...e }));

// ── Date helpers (via SCHED) ─────────────────────────────
export const TODAY_DAY = daysBetween(CYCLE_START, TODAY); // 21
export function dateOf(day: number): Date { return addDays(CYCLE_START, day); }
export function weekOf(day: number): number { return Math.floor(day / 7) + 1; }
export function relOf(day: number): Rel { return day < TODAY_DAY ? 'past' : day === TODAY_DAY ? 'today' : 'future'; }

// Titration boundary (week 5 = day 28) for chart overlay
export const TITRATION_DAY = 28;

// ── Derived ──────────────────────────────────────────────
export function sortedDesc(list: JournalEntry[]): JournalEntry[] { return [...list].sort((a, b) => b.day - a.day); }

export function moodPoints(list: JournalEntry[]): MoodPoint[] {
  // one point per entry, ascending by day
  return [...list].sort((a, b) => a.day - b.day).map(e => ({ day: e.day, mood: e.mood, source: e.source }));
}

export function stats(list: JournalEntry[]): JournalStats {
  if (!list.length) return { avg: 0, count: 0, topTag: null };
  const avg = list.reduce((s, e) => s + e.mood, 0) / list.length;
  const counts: Record<string, number> = {};
  list.forEach(e => e.tags.forEach(t => { counts[t] = (counts[t] || 0) + 1; }));
  const topTag = Object.keys(counts).sort((a, b) => counts[b] - counts[a])[0] || null;
  return { avg, count: list.length, topTag, topTagCount: topTag ? counts[topTag] : 0, counts };
}

export function tagTally(list: JournalEntry[]): TagTallyItem[] {
  const counts: Record<string, number> = {};
  list.forEach(e => e.tags.forEach(t => { counts[t] = (counts[t] || 0) + 1; }));
  return Object.keys(counts).map(id => ({ id, label: tagLabel(id), n: counts[id] })).sort((a, b) => b.n - a.n);
}

// Heatmap: 12 weeks × 7 days, Monday-first, from CYCLE_START's week.
export function heatmap(list: JournalEntry[]): (HeatmapCell | null)[][] {
  const byDay: Record<number, JournalEntry> = {};
  list.forEach(e => { byDay[e.day] = e; });
  const weeks: (HeatmapCell | null)[][] = [];
  // CYCLE_START is a Sunday → its Monday-first column = 6. Build a grid that
  // starts on the Monday of that week so columns line up Пн..Вс.
  const startLead = (CYCLE_START.getDay() + 6) % 7; // 6 for Sunday
  for (let w = 0; w < 12; w++) {
    const row: (HeatmapCell | null)[] = [];
    for (let c = 0; c < 7; c++) {
      const day = w * 7 + c - startLead; // day index from cycle start
      if (day < 0 || day >= 84) { row.push(null); continue; }
      const e = byDay[day] || null;
      row.push({ day, date: dateOf(day), entry: e, mood: e ? e.mood : null, rel: relOf(day), titration: day === TITRATION_DAY });
    }
    weeks.push(row);
  }
  return weeks;
}

export const JOURNAL = {
  MOOD, ENERGY_LABEL, SLEEP_LABEL, TAGS, tagLabel,
  ENTRIES, TODAY_DAY, TITRATION_DAY,
  dateOf, weekOf, relOf, sortedDesc, moodPoints, stats, tagTally, heatmap,
  longDate, WD_FULL,
};
