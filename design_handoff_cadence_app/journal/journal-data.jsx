// Cadence · Самочувствие (Side-effects journal) — data model
// Subjective wellbeing entries across the cycle. Aligns to the schedule anchor
// (window.SCHED): cycle started 10 мая, today = 31 мая (day 21, week 4).
// Exposes: JOURNAL on window.
//
// Entry: { id, day, mood, energy, sleep, tags[], note, source }
//   mood/energy/sleep — 1..5
//   tags — side-effect ids
//   source — 'dose' (logged with an injection) | 'manual'

(function () {
  const S = window.SCHED;

  // ── Scales ───────────────────────────────────────────────
  const MOOD = [
    null,
    { v: 1, label: 'Тяжело',    color: '#b8503c', soft: '#f4dfd6' },
    { v: 2, label: 'Так себе',  color: '#c2780a', soft: '#fbeed1' },
    { v: 3, label: 'Ровно',     color: '#8a857d', soft: '#ede5d6' },
    { v: 4, label: 'Хорошо',    color: '#3d7a52', soft: '#eaf0eb' },
    { v: 5, label: 'Светло',    color: '#2d5f3f', soft: '#dcebe0' },
  ];
  const ENERGY_LABEL = { 1: 'низкая', 2: 'низкая', 3: 'средняя', 4: 'хорошая', 5: 'высокая' };
  const SLEEP_LABEL  = { 1: 'плохой', 2: 'так себе', 3: 'нормальный', 4: 'крепкий', 5: 'отличный' };

  const TAGS = [
    { id: 'nausea',   label: 'Тошнота' },
    { id: 'fatigue',  label: 'Усталость' },
    { id: 'headache', label: 'Голова' },
    { id: 'bloating', label: 'Вздутие' },
    { id: 'insomnia', label: 'Бессонница' },
    { id: 'site',     label: 'Шишка' },
    { id: 'appetite', label: 'Нет аппетита' },
  ];
  function tagLabel(id) { const t = TAGS.find(x => x.id === id); return t ? t.label : id; }

  // ── Seed entries (days 0..20; today=21 left open for quick-add) ──
  const SEED = [
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
  const ENTRIES = SEED.map((e, i) => ({ id: `seed-${i}`, ...e }));

  // ── Date helpers (via SCHED) ─────────────────────────────
  const TODAY_DAY = S.daysBetween(S.CYCLE_START, S.TODAY); // 21
  function dateOf(day) { return S.addDays(S.CYCLE_START, day); }
  function weekOf(day) { return Math.floor(day / 7) + 1; }
  function relOf(day) { return day < TODAY_DAY ? 'past' : day === TODAY_DAY ? 'today' : 'future'; }

  // Titration boundary (week 5 = day 28) for chart overlay
  const TITRATION_DAY = 28;

  // ── Derived ──────────────────────────────────────────────
  function sortedDesc(list) { return [...list].sort((a, b) => b.day - a.day); }

  function moodPoints(list) {
    // one point per entry, ascending by day
    return [...list].sort((a, b) => a.day - b.day).map(e => ({ day: e.day, mood: e.mood, source: e.source }));
  }

  function stats(list) {
    if (!list.length) return { avg: 0, count: 0, topTag: null };
    const avg = list.reduce((s, e) => s + e.mood, 0) / list.length;
    const counts = {};
    list.forEach(e => e.tags.forEach(t => { counts[t] = (counts[t] || 0) + 1; }));
    const topTag = Object.keys(counts).sort((a, b) => counts[b] - counts[a])[0] || null;
    return { avg, count: list.length, topTag, topTagCount: topTag ? counts[topTag] : 0, counts };
  }

  function tagTally(list) {
    const counts = {};
    list.forEach(e => e.tags.forEach(t => { counts[t] = (counts[t] || 0) + 1; }));
    return Object.keys(counts).map(id => ({ id, label: tagLabel(id), n: counts[id] })).sort((a, b) => b.n - a.n);
  }

  // Heatmap: 12 weeks × 7 days, Monday-first, from CYCLE_START's week.
  function heatmap(list) {
    const byDay = {};
    list.forEach(e => { byDay[e.day] = e; });
    const weeks = [];
    // CYCLE_START is a Sunday → its Monday-first column = 6. Build a grid that
    // starts on the Monday of that week so columns line up Пн..Вс.
    const startLead = (S.CYCLE_START.getDay() + 6) % 7; // 6 for Sunday
    for (let w = 0; w < 12; w++) {
      const row = [];
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

  window.JOURNAL = {
    MOOD, ENERGY_LABEL, SLEEP_LABEL, TAGS, tagLabel,
    ENTRIES, TODAY_DAY, TITRATION_DAY,
    dateOf, weekOf, relOf, sortedDesc, moodPoints, stats, tagTally, heatmap,
    longDate: S.longDate, WD_FULL: S.WD_FULL,
  };
})();
