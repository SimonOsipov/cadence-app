// Cadence — meal data: sample parsed inputs + day-state presets.
// Hardcoded so the prototype demos feel fast without a real AI round-trip.

const MEAL_TARGETS = {
  kcal:    1800,
  protein:  140,  // g
  carbs:    200,  // g
  fat:       60,  // g
  water:     80,  // oz
};

// Macro → kcal rule of thumb (used to back-compute totals when grams are edited)
const KCAL_PER_G = { p: 4, c: 4, f: 9 };

// ── Sample parse outputs ────────────────────────────────────────────────
// The "AI" in the demo doesn't actually parse — it returns one of these
// canned meals based on what came in. For chat input, we cycle through;
// for photo we always return the "lunch bowl" set, for voice the "snack".

const SAMPLE_PARSES = [
  {
    id: 'breakfast',
    triggerWords: ['yogurt', 'eggs', 'sourdough', 'avocado', 'berries', 'oats', 'oatmeal', 'йогурт', 'яйц', 'хлеб', 'авокадо', 'ягод'],
    placeholder: '«Греческий йогурт с ягодами, два варёных яйца и тост с авокадо»',
    transcript: 'Греческий йогурт с ягодами, два варёных яйца и кусок ржаного хлеба с авокадо.',
    mealName: 'Завтрак',
    items: [
      { id: 'gy', name: 'Греческий йогурт', grams: 150, kcal: 130, p: 17, c:  6, f:  4 },
      { id: 'br', name: 'Ягодная смесь',    grams:  80, kcal:  45, p:  1, c: 11, f:  0 },
      { id: 'eg', name: 'Варёные яйца',     grams: 100, kcal: 155, p: 13, c:  1, f: 11 },
      { id: 'sd', name: 'Ржаной хлеб',      grams:  40, kcal: 110, p:  4, c: 22, f:  1 },
      { id: 'av', name: 'Авокадо',          grams:  60, kcal:  96, p:  1, c:  5, f:  9 },
    ],
  },
  {
    id: 'lunch-bowl',
    triggerWords: ['bowl', 'chicken', 'rice', 'salmon', 'quinoa', 'salad', 'lunch', 'обед', 'курица', 'рис', 'батат', 'тахини', 'капуста'],
    placeholder: '«Куриная боул-чашка с рисом, капустой и тахини»',
    transcript: 'Боул с куриной грудкой: бурый рис, кейл, печёный батат и капля тахини.',
    mealName: 'Обед',
    items: [
      { id: 'ck', name: 'Куриная грудка',   grams: 150, kcal: 240, p: 45, c:  0, f:  6 },
      { id: 'br', name: 'Бурый рис',        grams: 120, kcal: 145, p:  3, c: 30, f:  1 },
      { id: 'kl', name: 'Кейл с заправкой', grams:  80, kcal:  60, p:  3, c:  6, f:  3 },
      { id: 'sw', name: 'Батат',            grams: 100, kcal:  90, p:  2, c: 21, f:  0 },
      { id: 'th', name: 'Тахини',           grams:  15, kcal:  90, p:  3, c:  3, f:  8 },
    ],
  },
  {
    id: 'snack',
    triggerWords: ['snack', 'cottage', 'almond', 'apple', 'peanut', 'перекус', 'творог', 'яблок', 'миндаль'],
    placeholder: '«Творог с яблоком»',
    transcript: 'Творог с нарезанным яблоком и каплей миндальной пасты.',
    mealName: 'Перекус',
    items: [
      { id: 'cc', name: 'Творог',           grams: 150, kcal: 145, p: 21, c:  6, f:  4 },
      { id: 'ap', name: 'Яблоко',           grams: 180, kcal:  95, p:  0, c: 25, f:  0 },
      { id: 'ab', name: 'Миндальная паста', grams:  15, kcal:  95, p:  3, c:  3, f:  8 },
    ],
  },
];

// Pick a sample based on input text (very rough — keyword match).
function pickSampleForInput(text) {
  const t = (text || '').toLowerCase();
  let best = SAMPLE_PARSES[0], bestScore = -1;
  for (const s of SAMPLE_PARSES) {
    const score = s.triggerWords.reduce((acc, w) => acc + (t.includes(w) ? 1 : 0), 0);
    if (score > bestScore) { best = s; bestScore = score; }
  }
  return best;
}

// Sum a parsed item list to totals
function totalMeal(items) {
  return items.reduce((acc, it) => ({
    kcal: acc.kcal + it.kcal,
    p:    acc.p    + it.p,
    c:    acc.c    + it.c,
    f:    acc.f    + it.f,
  }), { kcal: 0, p: 0, c: 0, f: 0 });
}

// Recompute an item's macros + kcal when grams change (linear scale)
function rescaleItem(item, newGrams) {
  if (newGrams === item.grams || newGrams <= 0) return { ...item, grams: Math.max(0, newGrams) };
  const r = newGrams / item.grams;
  return {
    ...item,
    grams: newGrams,
    kcal:  Math.round(item.kcal * r),
    p:     Math.round(item.p    * r * 10) / 10,
    c:     Math.round(item.c    * r * 10) / 10,
    f:     Math.round(item.f    * r * 10) / 10,
  };
}

// ── Day-state presets ───────────────────────────────────────────────────
// Each preset is an array of "logged meals" that already exist on Today.

const DAY_STATES = {
  empty: {
    label: 'Пусто · до завтрака',
    now: '08:42',
    suggestion: { eyebrow: 'Начнём день', title: 'Завтрак?', meta: 'Целимся в 35 г белка.' },
    meals: [],
  },
  midday: {
    label: 'День · после обеда',
    now: '13:14',
    suggestion: { eyebrow: 'Следующий приём', title: 'Перекус?', meta: 'Осталось 78 г белка.' },
    meals: [
      {
        id: 'm1', time: '08:18', mealName: 'Завтрак',
        items: SAMPLE_PARSES[0].items,
      },
      {
        id: 'm2', time: '12:42', mealName: 'Обед',
        items: SAMPLE_PARSES[1].items,
      },
    ],
  },
  evening: {
    label: 'Близко к цели · вечер',
    now: '19:30',
    suggestion: { eyebrow: 'Последний шанс', title: 'Лёгкий ужин?', meta: 'Осталось 380 ккал.' },
    meals: [
      {
        id: 'm1', time: '08:18', mealName: 'Завтрак',
        items: SAMPLE_PARSES[0].items,
      },
      {
        id: 'm2', time: '12:42', mealName: 'Обед',
        items: SAMPLE_PARSES[1].items,
      },
      {
        id: 'm3', time: '16:18', mealName: 'Перекус',
        items: SAMPLE_PARSES[2].items,
      },
    ],
  },
};

// Pre-compute totals on each meal for convenience
for (const k of Object.keys(DAY_STATES)) {
  for (const m of DAY_STATES[k].meals) {
    m.totals = totalMeal(m.items);
  }
}

// Day-level totals from a meal list
function dayTotals(meals) {
  return meals.reduce((acc, m) => ({
    kcal: acc.kcal + m.totals.kcal,
    p:    acc.p    + m.totals.p,
    c:    acc.c    + m.totals.c,
    f:    acc.f    + m.totals.f,
  }), { kcal: 0, p: 0, c: 0, f: 0 });
}

// Coach lines, picked after a save based on protein progress
const MEAL_COACH_LINES = [
  { trigger: 'low_protein',
    lead: 'Белка',  emph: 'пока маловато', tail: '— оставьте граммов на ужин.',
    note: "У вас 38 г из 140. Порция творога закроет половину пути." },
  { trigger: 'on_track',
    lead: 'Белок',  emph: 'идёт ровно', tail: '— как и хотели.',
    note: "Держите темп — норма будет с запасом." },
  { trigger: 'near_goal',
    lead: "Вы",     emph: 'уже у цели', tail: '— по белку всё.',
    note: "Дальше можно поменьше и полегче на желудок." },
  { trigger: 'first_meal',
    lead: 'Первый приём.', emph: '538 ккал', tail: 'и 35 г белка в копилке.',
    note: "Спокойный старт — у дня есть простор." },
];

function pickMealCoach({ meals, lastMeal, totals }) {
  if (meals.length === 1) return MEAL_COACH_LINES.find(c => c.trigger === 'first_meal');
  const p = totals.p, pct = p / MEAL_TARGETS.protein;
  if (pct < 0.35) return MEAL_COACH_LINES.find(c => c.trigger === 'low_protein');
  if (pct < 0.85) return MEAL_COACH_LINES.find(c => c.trigger === 'on_track');
  return MEAL_COACH_LINES.find(c => c.trigger === 'near_goal');
}

Object.assign(window, {
  MEAL_TARGETS, KCAL_PER_G,
  SAMPLE_PARSES, pickSampleForInput,
  totalMeal, rescaleItem, dayTotals,
  DAY_STATES, MEAL_COACH_LINES, pickMealCoach,
});
