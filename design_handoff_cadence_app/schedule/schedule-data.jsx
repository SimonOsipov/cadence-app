// Cadence · График (Injection schedule) — data model
// A 12-week cycle anchored to a fixed "today" (Sunday, 31 мая 2026 — week 4).
// Exposes: SCHED (everything the screen needs) on window.
//
// The cycle carries four event categories:
//   inj   — injections (Семаглутид weekly, BPC-157 daily)
//   supp  — supplements (Глицин + магний nightly)
//   weigh — weekly weigh-in (Воскресенье morning)
//   meal  — meals (summarised per day; live total for today)
//
// Семаглутид titrates: 0,25 мг (нед 1–4) → 0,5 мг (нед 5–8) → 1,0 мг (нед 9–12).

(function () {
  // ── Anchors ───────────────────────────────────────────────
  const TODAY = new Date(2026, 4, 31);          // Sunday, 31 May 2026
  const CYCLE_START = new Date(2026, 4, 10);    // Sunday, 10 May 2026 — week 1
  const CYCLE_WEEKS = 12;
  const CYCLE_END = addDays(CYCLE_START, CYCLE_WEEKS * 7 - 1); // last day (Sat)
  const INJ_DOW = CYCLE_START.getDay();         // 0 = Sunday (Семаглутид day)

  // ── Date helpers ──────────────────────────────────────────
  function addDays(d, n) { const x = new Date(d); x.setDate(x.getDate() + n); return x; }
  function startOfDay(d) { return new Date(d.getFullYear(), d.getMonth(), d.getDate()); }
  function iso(d) { return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`; }
  function sameDay(a, b) { return iso(a) === iso(b); }
  function daysBetween(a, b) { return Math.round((startOfDay(b) - startOfDay(a)) / 86400000); }
  function monStart(d) { return (d.getDay() + 6) % 7; } // Monday-first column index

  const T_ISO = iso(TODAY);
  function rel(d) {
    const k = daysBetween(TODAY, d);
    return k < 0 ? 'past' : k === 0 ? 'today' : 'future';
  }

  // ── RU labels ─────────────────────────────────────────────
  const MONTHS_NOM = ['Январь','Февраль','Март','Апрель','Май','Июнь','Июль','Август','Сентябрь','Октябрь','Ноябрь','Декабрь'];
  const MONTHS_GEN = ['января','февраля','марта','апреля','мая','июня','июля','августа','сентября','октября','ноября','декабря'];
  const WD_FULL = ['Воскресенье','Понедельник','Вторник','Среда','Четверг','Пятница','Суббота'];
  const WD_SHORT = ['Пн','Вт','Ср','Чт','Пт','Сб','Вс'];

  function ruWeeks(n) { const a = Math.abs(n) % 100, b = a % 10;
    if (a > 10 && a < 20) return 'недель'; if (b > 1 && b < 5) return 'недели'; if (b === 1) return 'неделю'; return 'недель'; }

  function weekOfCycle(d) { return Math.floor(daysBetween(CYCLE_START, d) / 7) + 1; }
  function inCycle(d) { return daysBetween(CYCLE_START, d) >= 0 && daysBetween(d, CYCLE_END) >= 0; }

  // ── Titration ─────────────────────────────────────────────
  // Week → Семаглутид dose. Step markers fall on the first Sunday of each band.
  function semaDose(week) {
    if (week <= 4) return { value: '0,25', unit: 'мг' };
    if (week <= 8) return { value: '0,5', unit: 'мг' };
    return { value: '1,0', unit: 'мг' };
  }
  const TITRATION_STEPS = [
    { week: 5, from: '0,25', to: '0,5', date: addDays(CYCLE_START, 4 * 7) },
    { week: 9, from: '0,5', to: '1,0', date: addDays(CYCLE_START, 8 * 7) },
  ];
  function nextTitration() {
    return TITRATION_STEPS.find(s => daysBetween(TODAY, s.date) > 0) || null;
  }
  function isTitrationDay(d) {
    return TITRATION_STEPS.find(s => sameDay(s.date, d)) || null;
  }

  // A couple of realistic past blemishes for texture (mostly the cycle is clean).
  const SKIPPED = new Set(['2026-05-21']);      // a skipped BPC day
  // Static meal summaries for past days (today uses the live total).
  function pastMealSummary(d) {
    const seed = (d.getDate() * 7 + d.getMonth()) % 5;
    const kcal = [1460, 1520, 1380, 1610, 1490][seed];
    const count = [3, 3, 2, 4, 3][seed];
    return { count, kcal };
  }

  // ── Category styling (consumed by the screen) ─────────────
  const CATS = {
    inj:   { id: 'inj',   label: 'Инъекция',     icon: 'beaker', dot: '#2d5f3f', soft: '#eaf0eb', fg: '#2d5f3f' },
    supp:  { id: 'supp',  label: 'Добавка',      icon: 'moon',   dot: '#8a857d', soft: '#ede5d6', fg: '#5c5852' },
    weigh: { id: 'weigh', label: 'Взвешивание',  icon: 'scale',  dot: '#5a7184', soft: '#e6ecf2', fg: '#41566b' },
    meal:  { id: 'meal',  label: 'Питание',      icon: 'cake',   dot: '#b8895a', soft: '#f3e8d6', fg: '#6b4a25' },
  };

  // ── Events for a given date ───────────────────────────────
  // ctx: { doseLogged, todayMeals, todayKcal }
  function eventsForDate(d, ctx = {}) {
    if (!inCycle(d)) return [];
    const week = weekOfCycle(d);
    const r = rel(d);
    const out = [];

    // Семаглутид — weekly on the injection weekday
    if (d.getDay() === INJ_DOW) {
      const dose = semaDose(week);
      const step = isTitrationDay(d);
      let status = r === 'past' ? 'done' : r === 'today' ? (ctx.doseLogged ? 'done' : 'pending') : 'scheduled';
      out.push({
        id: 'sema', cat: 'inj', title: 'Семаглутид',
        dose: `${dose.value} ${dose.unit}`, time: '07:00',
        sub: step ? `Новая доза · ${step.to} мг` : 'Подкожно · еженедельно',
        loggable: true, step: !!step, status,
      });
    }

    // BPC-157 — daily
    {
      let status = r === 'past' ? (SKIPPED.has(iso(d)) ? 'skipped' : 'done')
                 : r === 'today' ? 'pending' : 'scheduled';
      out.push({
        id: 'bpc', cat: 'inj', title: 'BPC-157',
        dose: '250 мкг', time: '08:00 · 20:00',
        sub: 'Подкожно · 2× в день', loggable: true, status,
      });
    }

    // Глицин + магний — nightly
    {
      let status = r === 'past' ? 'done' : r === 'today' ? 'scheduled' : 'scheduled';
      out.push({
        id: 'glycine', cat: 'supp', title: 'Глицин + магний',
        dose: '', time: '21:30',
        sub: 'За 30 мин до сна', loggable: false, status,
      });
    }

    // Взвешивание — weekly, same morning as the injection
    if (d.getDay() === INJ_DOW) {
      let status = r === 'past' ? 'done' : r === 'today' ? 'pending' : 'scheduled';
      out.push({
        id: 'weigh', cat: 'weigh', title: 'Взвешивание',
        dose: '', time: '07:30',
        sub: 'Утром, до завтрака', loggable: false, status,
      });
    }

    // Питание — past/today only, as a single summary row
    if (r !== 'future') {
      const s = r === 'today'
        ? { count: ctx.todayMeals != null ? ctx.todayMeals : 0, kcal: ctx.todayKcal != null ? ctx.todayKcal : 0 }
        : pastMealSummary(d);
      out.push({
        id: 'meal', cat: 'meal', title: 'Питание',
        dose: '', time: '',
        sub: s.count ? `${s.count} ${mealWord(s.count)} · ${s.kcal.toLocaleString('ru-RU')} ккал`
                     : 'Пока ничего не записано',
        loggable: false, status: r === 'today' ? 'open' : 'logged',
        meal: true,
      });
    }

    return out;
  }
  function mealWord(n) { const a = n % 100, b = n % 10;
    if (a > 10 && a < 20) return 'приёмов'; if (b > 1 && b < 5) return 'приёма'; if (b === 1) return 'приём'; return 'приёмов'; }

  // ── Day dot summary (for the calendar grid) ───────────────
  function dotsForDate(d, ctx = {}) {
    const evs = eventsForDate(d, ctx);
    const cats = [];
    ['inj', 'supp', 'weigh', 'meal'].forEach(c => { if (evs.some(e => e.cat === c)) cats.push(c); });
    return {
      cats,
      injection: evs.some(e => e.id === 'sema'),     // the weekly anchor dose
      step: !!isTitrationDay(d),
      anyPending: evs.some(e => e.status === 'pending'),
      allDone: rel(d) === 'past' && evs.filter(e => e.loggable).every(e => e.status === 'done'),
      week: weekOfCycle(d),
    };
  }

  // ── Months spanning the cycle (for vertical scroll) ───────
  function monthsInCycle() {
    const months = [];
    let y = CYCLE_START.getFullYear(), m = CYCLE_START.getMonth();
    const endKey = CYCLE_END.getFullYear() * 12 + CYCLE_END.getMonth();
    while (y * 12 + m <= endKey) {
      const first = new Date(y, m, 1);
      const days = new Date(y, m + 1, 0).getDate();
      const lead = monStart(first);
      const cells = [];
      for (let i = 0; i < lead; i++) cells.push(null);
      for (let day = 1; day <= days; day++) cells.push(new Date(y, m, day));
      while (cells.length % 7 !== 0) cells.push(null);
      months.push({ key: `${y}-${m}`, year: y, month: m, label: MONTHS_NOM[m], cells, isTodayMonth: (y === TODAY.getFullYear() && m === TODAY.getMonth()) });
      m++; if (m > 11) { m = 0; y++; }
    }
    return months;
  }

  function longDate(d) { return `${d.getDate()} ${MONTHS_GEN[d.getMonth()]}`; }

  window.SCHED = {
    TODAY, CYCLE_START, CYCLE_END, CYCLE_WEEKS, T_ISO,
    iso, sameDay, rel, addDays, daysBetween, longDate,
    WD_FULL, WD_SHORT, MONTHS_NOM, MONTHS_GEN, ruWeeks,
    weekOfCycle, inCycle, semaDose, nextTitration, isTitrationDay,
    CATS, eventsForDate, dotsForDate, monthsInCycle,
  };
})();
