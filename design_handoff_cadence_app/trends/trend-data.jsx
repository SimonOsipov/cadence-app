// Cadence — trend datasets for the 7 supported biomarkers.
// Each biomarker has series at four resolutions (7d, 4w, 3m, cycle).
// Days are referenced against TODAY_DAY (84 = today, 0 = cycle start).
// Protocol events + dose timeline are shared — overlaid on charts when annotations are on.

const TODAY_DAY = 84;

// Each event has a day index from cycle start (0..84).
const PROTOCOL_EVENTS = [
  { day: 0,  kind: 'start',     short: 'Старт',           label: 'Старт семаглутида · 0,25 мг' },
  { day: 28, kind: 'titration', short: '→ 0,5 мг',        label: 'Подняли до 0,5 мг' },
  { day: 49, kind: 'add',       short: '+ BPC-157',       label: 'Добавили BPC-157 · 250 мкг в день' },
  { day: 70, kind: 'titration', short: '→ 1,0 мг',        short_alt: '1,0 мг', label: 'Подняли до 1,0 мг' },
];

// Dose-level spans for the timeline pill row below the chart.
const DOSE_SPANS = [
  { fromDay:  0, toDay: 28, dose: '0,25 мг' },
  { fromDay: 28, toDay: 70, dose: '0,5 мг' },
  { fromDay: 70, toDay: 84, dose: '1,0 мг' },
];

// ── Series generators ────────────────────────────────────────────────────
// Deterministic, hand-tuned for nice-looking trends. No randomness at runtime.

function makeWeight() {
  // monotone-ish decline with weekly noise (kg) — overweight starting point
  const cycle = [118.0, 117.4, 116.6, 115.7, 114.8, 114.0, 113.2, 112.4, 111.6, 111.0, 110.5, 110.2, 110.0];
  const months = []; // last 28 days, ~daily granularity
  for (let i = 0; i < 28; i++) {
    const t = i / 27;
    // start near 111.5 → end 110.0 with small wave
    months.push(+(111.5 - t * 1.5 + Math.sin(i * 0.7) * 0.15).toFixed(1));
  }
  const week = [110.6, 110.4, 110.3, 110.4, 110.2, 110.1, 110.0];
  return { '7d': week, '4w': months, '3m': cycle.slice(1), cycle };
}

function makeHRV() {
  const cycle = [54, 55, 58, 56, 59, 61, 60, 63, 65, 67, 69, 70, 71];
  const months = [];
  for (let i = 0; i < 28; i++) {
    const t = i / 27;
    months.push(Math.round(60 + t * 11 + Math.sin(i * 0.9) * 2));
  }
  const week = [66, 68, 67, 70, 69, 71, 71];
  return { '7d': week, '4w': months, '3m': cycle.slice(1), cycle };
}

function makeRHR() {
  const cycle = [66, 65, 65, 64, 64, 63, 62, 62, 60, 59, 58, 58, 58];
  const months = [];
  for (let i = 0; i < 28; i++) {
    const t = i / 27;
    months.push(Math.round(62 - t * 4 + Math.sin(i * 0.8) * 0.6));
  }
  const week = [60, 59, 60, 58, 58, 58, 58];
  return { '7d': week, '4w': months, '3m': cycle.slice(1), cycle };
}

function makeSleep() {
  const cycle = [72, 74, 71, 76, 78, 80, 79, 83, 81, 85, 84, 86, 87];
  const months = [];
  for (let i = 0; i < 28; i++) {
    const t = i / 27;
    months.push(Math.round(76 + t * 11 + Math.sin(i * 1.1) * 3));
  }
  const week = [78, 82, 85, 81, 86, 84, 87];
  return { '7d': week, '4w': months, '3m': cycle.slice(1), cycle };
}

function makeBodyFat() {
  const cycle = [22.4, 22.2, 21.9, 21.6, 21.2, 21.0, 20.7, 20.3, 20.0, 19.8, 19.5, 19.2, 19.0];
  const months = [];
  for (let i = 0; i < 28; i++) {
    const t = i / 27;
    months.push(+(20.1 - t * 1.1 + Math.sin(i * 0.6) * 0.08).toFixed(1));
  }
  const week = [19.4, 19.3, 19.3, 19.2, 19.1, 19.1, 19.0];
  return { '7d': week, '4w': months, '3m': cycle.slice(1), cycle };
}

function makeThigh() {
  const cycle = [64.5, 64.2, 63.8, 63.5, 63.0, 62.6, 62.2, 61.8, 61.4, 61.0, 60.6, 60.3, 60.0];
  const months = [];
  for (let i = 0; i < 28; i++) {
    const t = i / 27;
    months.push(+(60.8 - t * 0.8 + Math.sin(i * 0.6) * 0.06).toFixed(1));
  }
  const week = [60.2, 60.1, 60.1, 60.0, 60.0, 60.0, 60.0];
  return { '7d': week, '4w': months, '3m': cycle.slice(1), cycle };
}

function makeWaist() {
  const cycle = [37.5, 37.4, 37.2, 37.0, 36.8, 36.5, 36.3, 36.1, 35.9, 35.6, 35.4, 35.2, 35.0];
  const months = [];
  for (let i = 0; i < 28; i++) {
    const t = i / 27;
    months.push(+(36.0 - t * 1.0 + Math.sin(i * 0.5) * 0.05).toFixed(1));
  }
  const week = [35.2, 35.2, 35.1, 35.1, 35.0, 35.0, 35.0];
  return { '7d': week, '4w': months, '3m': cycle.slice(1), cycle };
}

// Day indexes parallel to each series (so the chart can map points → cycle day → annotation).
function daysFor(seriesLength, span) {
  // span = total days the series spans (ending at TODAY_DAY)
  const start = TODAY_DAY - span;
  return Array.from({ length: seriesLength }, (_, i) =>
    +(start + (i * span) / (seriesLength - 1)).toFixed(2)
  );
}

function withDays(seriesMap) {
  return {
    '7d':    { values: seriesMap['7d'],    days: daysFor(seriesMap['7d'].length,    6),  spanLabel: 'за 7 дней' },
    '4w':    { values: seriesMap['4w'],    days: daysFor(seriesMap['4w'].length,    27), spanLabel: 'за 4 недели' },
    '3m':    { values: seriesMap['3m'],    days: daysFor(seriesMap['3m'].length,    84), spanLabel: 'за 3 месяца' },
    'cycle': { values: seriesMap['cycle'], days: daysFor(seriesMap['cycle'].length, 84), spanLabel: 'весь 12-недельный цикл' },
  };
}

const TREND_DATA = {
  weight: {
    id: 'weight',
    label: 'Вес',
    eyebrow: 'Масса тела',
    unit: 'кг',
    decimals: 1,
    series: withDays(makeWeight()),
    baseline: 118.0,
    goal: 100.0,
    direction: 'down',                  // direction that counts as "progress"
    accent: 'forest',                   // chart color family
    narrative: "Вы плавно идёте вниз — примерно по килограмму в неделю. Та самая золотая середина: жир уходит, мышцы держатся.",
    coach: { lead: "Вы стали", emph: 'на восемь кило легче', tail: '— и спокойнее, чем мы ждали.' },
    correlations: [
      { withId: 'sleep', label: 'Сон',       strength: 0.62, note: "Недели с лучшим сном — заметнее минус на весах." },
      { withId: 'waist', label: 'Талия',     strength: 0.84, note: "Идут вместе — это жир, не вода." },
    ],
    recent: [
      { day: 84, value: 110.0, note: 'Утром · натощак' },
      { day: 83, value: 110.1, note: '' },
      { day: 82, value: 110.3, note: 'День в дороге' },
      { day: 81, value: 110.4, note: '' },
      { day: 80, value: 110.2, note: '' },
    ],
  },

  hrv: {
    id: 'hrv',
    label: 'HRV',
    eyebrow: 'Вариабельность сердца',
    unit: 'мс',
    decimals: 0,
    series: withDays(makeHRV()),
    baseline: 54,
    direction: 'up',
    accent: 'forest',
    narrative: "Восстановление догоняет нагрузку. Нервная система выравнивается — сон и связка с BPC-157 работают.",
    coach: { lead: 'Восстановление', emph: 'уверенно растёт', tail: '— на семнадцать пунктов выше старта.' },
    correlations: [
      { withId: 'sleep', label: 'Сон',          strength: 0.78, note: "Сильная связь. Лучший HRV — после самых глубоких ночей." },
      { withId: 'rhr',   label: 'ЧСС покоя',    strength: -0.71, note: "Обратная связь, как и ждём — спокойнее пульс, выше вариативность." },
    ],
    recent: [
      { day: 84, value: 71, note: 'Утреннее измерение' },
      { day: 83, value: 71, note: '' },
      { day: 82, value: 69, note: 'Поздний ужин' },
      { day: 81, value: 70, note: '' },
      { day: 80, value: 67, note: 'Тяжёлая тренировка' },
    ],
  },

  rhr: {
    id: 'rhr',
    label: 'ЧСС покоя',
    eyebrow: 'Пульс в покое',
    unit: 'уд/мин',
    decimals: 0,
    series: withDays(makeRHR()),
    baseline: 66,
    direction: 'down',
    accent: 'forest',
    narrative: "Пульс в покое продолжает снижаться. Аэробная база расширяется — лёгкие дни ощущаются легче.",
    coach: { lead: 'Ваш мотор', emph: 'работает тише', tail: '— на восемь ударов ниже стартового уровня.' },
    correlations: [
      { withId: 'hrv',   label: 'HRV',  strength: -0.71, note: "Зеркало — ниже в покое, выше в вариативности." },
      { withId: 'sleep', label: 'Сон',  strength: 0.44,  note: "Слабая связь. Лучше спите — спокойнее утром." },
    ],
    recent: [
      { day: 84, value: 58, note: '' },
      { day: 83, value: 58, note: '' },
      { day: 82, value: 58, note: '' },
      { day: 81, value: 58, note: '' },
      { day: 80, value: 60, note: 'Поздний кофе' },
    ],
  },

  sleep: {
    id: 'sleep',
    label: 'Сон',
    eyebrow: 'Качество сна',
    unit: '/100',
    decimals: 0,
    series: withDays(makeSleep()),
    baseline: 72,
    direction: 'up',
    accent: 'sand',
    narrative: "Глубже, дольше и стабильнее — глицин и вечерний ритм делают своё дело.",
    coach: { lead: 'Вы спите', emph: 'глубже и дольше', tail: '— заметный сдвиг с первой недели.' },
    correlations: [
      { withId: 'hrv',    label: 'HRV',  strength: 0.78,  note: 'Тесная связь — лучше сон, лучше восстановление.' },
      { withId: 'weight', label: 'Вес',  strength: 0.62,  note: 'Заметный минус на весах — после лучших ночей.' },
    ],
    recent: [
      { day: 84, value: 87, note: '7ч 42м · 1ч 38м глубокого' },
      { day: 83, value: 84, note: '7ч 18м' },
      { day: 82, value: 86, note: 'Глицин + магний' },
      { day: 81, value: 81, note: '6ч 50м' },
      { day: 80, value: 85, note: '' },
    ],
  },

  bodyfat: {
    id: 'bodyfat',
    label: '% жира',
    eyebrow: 'Состав тела',
    unit: '%',
    decimals: 1,
    series: withDays(makeBodyFat()),
    baseline: 22.4,
    direction: 'down',
    accent: 'forest',
    narrative: "Состав тела меняется в вашу пользу — жир уходит, мышцы держатся. Норма белка делает своё дело.",
    coach: { lead: 'Жира', emph: 'на три пункта меньше', tail: '— а мышцы на месте.' },
    correlations: [
      { withId: 'weight', label: 'Вес',   strength: 0.91, note: 'Почти в унисон — минус на весах — это жир.' },
      { withId: 'waist',  label: 'Талия', strength: 0.86, note: 'Состав и сантиметр сходятся.' },
    ],
    recent: [
      { day: 84, value: 19.0, note: 'DEXA' },
      { day: 70, value: 19.5, note: 'Биоимпеданс' },
      { day: 56, value: 20.2, note: '' },
      { day: 42, value: 20.8, note: 'DEXA' },
      { day: 28, value: 21.4, note: '' },
    ],
  },

  thigh: {
    id: 'thigh',
    label: 'Обхват бедра',
    eyebrow: 'Обхват бедра',
    unit: 'см',
    decimals: 1,
    series: withDays(makeThigh()),
    baseline: 64.5,
    direction: 'down',
    accent: 'forest',
    narrative: "Бёдра стали уже на четыре с половиной сантиметра. Медленно и ровно — как надо.",
    coach: { lead: 'Бёдра', emph: 'на 4,5 см уже', tail: '— и объём уходит ровно с весом.' },
    correlations: [
      { withId: 'weight', label: 'Вес',   strength: 0.88, note: 'Идут вместе — это жир, не вода.' },
      { withId: 'waist',  label: 'Талия', strength: 0.83, note: 'Сантиметр в низу и верху живота согласны.' },
    ],
    recent: [
      { day: 84, value: 60.0, note: 'Утром' },
      { day: 77, value: 60.3, note: '' },
      { day: 70, value: 60.6, note: '' },
      { day: 56, value: 61.4, note: '' },
      { day: 28, value: 62.6, note: '' },
    ],
  },

  waist: {
    id: 'waist',
    label: 'Талия',
    eyebrow: 'Обхват талии',
    unit: 'см',
    decimals: 1,
    series: withDays(makeWaist()),
    baseline: 37.5,
    direction: 'down',
    accent: 'forest',
    narrative: "Сантиметр ушёл вниз на шесть с половиной — пора пересаживаться на дырочку поглубже.",
    coach: { lead: 'Талия', emph: 'на 6 см уже', tail: 'чем в первую неделю.' },
    correlations: [
      { withId: 'weight',  label: 'Вес',     strength: 0.84, note: 'Идут вместе — это жир, не вода.' },
      { withId: 'bodyfat', label: '% жира', strength: 0.86, note: 'Состав и сантиметр сходятся.' },
    ],
    recent: [
      { day: 84, value: 35.0, note: 'Утром' },
      { day: 77, value: 35.2, note: '' },
      { day: 70, value: 35.4, note: '' },
      { day: 56, value: 35.9, note: '' },
      { day: 28, value: 36.5, note: '' },
    ],
  },
};

// Ordered list for the landing grid.
const TREND_ORDER = ['weight', 'hrv', 'rhr', 'sleep', 'bodyfat', 'thigh', 'waist'];

// Helpers ───────────────────────────────────────────────────────────────

function fmtValue(v, decimals) {
  if (decimals === 0) return Math.round(v).toString();
  return v.toFixed(decimals);
}

// Δ between first and last point of a series.
function seriesDelta(seriesAtTf, decimals) {
  const v = seriesAtTf.values;
  const first = v[0];
  const last = v[v.length - 1];
  const diff = last - first;
  return { first, last, diff, abs: Math.abs(diff), decimals };
}

// Format a Δ as a signed pill string ("↓ 2.4 lb" / "↑ 13 ms").
function formatDeltaPill(seriesAtTf, biom) {
  const { diff } = seriesDelta(seriesAtTf, biom.decimals);
  if (diff === 0) return '— без изменений';
  const arrow = diff < 0 ? '↓' : '↑';
  const mag = fmtValue(Math.abs(diff), biom.decimals);
  return `${arrow} ${mag} ${biom.unit.startsWith('/') ? '' : biom.unit}`.trim();
}

// True when the trend is moving in the direction we consider "progress".
function isProgress(seriesAtTf, biom) {
  const { diff } = seriesDelta(seriesAtTf, biom.decimals);
  if (diff === 0) return null;
  if (biom.direction === 'down') return diff < 0;
  return diff > 0;
}

// Stat strip values
function seriesStats(seriesAtTf, biom) {
  const v = seriesAtTf.values;
  const avg = v.reduce((a, b) => a + b, 0) / v.length;
  const min = Math.min(...v);
  const max = Math.max(...v);
  return {
    avg: fmtValue(avg, biom.decimals),
    min: fmtValue(min, biom.decimals),
    max: fmtValue(max, biom.decimals),
  };
}

// Map a day index (0..84) to an x-fraction within the series.
function dayToFraction(day, seriesAtTf) {
  const days = seriesAtTf.days;
  const first = days[0];
  const last = days[days.length - 1];
  if (day < first || day > last) return null;
  return (day - first) / (last - first);
}

// Sparkline-friendly normalized series for landing cards (0..1 over the range).
function normalizeForSpark(seriesAtTf) {
  const v = seriesAtTf.values;
  const min = Math.min(...v), max = Math.max(...v);
  if (max === min) return v.map(() => 0.5);
  return v.map(x => (x - min) / (max - min));
}

Object.assign(window, {
  TODAY_DAY, PROTOCOL_EVENTS, DOSE_SPANS,
  TREND_DATA, TREND_ORDER,
  fmtValue, seriesDelta, formatDeltaPill, isProgress, seriesStats,
  dayToFraction, normalizeForSpark,
});
