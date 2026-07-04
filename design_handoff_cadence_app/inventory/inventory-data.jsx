// Cadence — Inventory ("Аптечка") data model.
// Vials with full metadata: status, opened/expiry dates, lot, location,
// recent doses, weekly-usage sparkline.
//
// All copy is Russian (matches the rest of the prototype). Anchors all
// dates against `TODAY` so the demo feels live.

// Today is May 28, 2026 (matches the system prompt).
const VIAL_TODAY = new Date(2026, 4, 28);

// Compound display data (mirrors COMPOUNDS in log-dose, plus visuals).
const COMPOUND_META = {
  sema: { name: 'Семаглутид', defaultDose: '0,25 мг',  unit: 'мг',  weeklyDoses: 1, icon: 'beaker' },
  bpc:  { name: 'BPC-157',    defaultDose: '250 мкг',  unit: 'мкг', weeklyDoses: 7, icon: 'beaker' },
  tb:   { name: 'TB-500',     defaultDose: '2,5 мг',   unit: 'мг',  weeklyDoses: 2, icon: 'beaker' },
  tes:  { name: 'Тезаморелин', defaultDose: '1,0 мг',  unit: 'мг',  weeklyDoses: 7, icon: 'beaker' },
};

// Helper to make a Date relative to TODAY (positive = future, negative = past).
const dayOffset = (days) => {
  const d = new Date(VIAL_TODAY);
  d.setDate(d.getDate() + days);
  return d;
};

const MONTHS_SHORT = ['янв','фев','мар','апр','мая','июн','июл','авг','сен','окт','ноя','дек'];
function formatDateShort(date) {
  return `${date.getDate()} ${MONTHS_SHORT[date.getMonth()]}`;
}
function daysBetween(a, b) {
  return Math.round((b - a) / 86400000);
}

// Status derivation rules:
// - 'sealed'   — never opened
// - 'expiring' — opens within 14 days OR active vial expires within 14 days
// - 'low'      — opened, < 25% doses remaining
// - 'active'   — opened, healthy stock
function deriveStatus(v) {
  if (!v.opened) {
    const daysToExpiry = daysBetween(VIAL_TODAY, v.expires);
    if (daysToExpiry <= 14) return 'expiring';
    return 'sealed';
  }
  const daysToExpiry = daysBetween(VIAL_TODAY, v.expires);
  if (daysToExpiry <= 14) return 'expiring';
  const remainingPct = v.remaining / v.total;
  if (remainingPct < 0.25) return 'low';
  return 'active';
}

// ── Mock inventory ──────────────────────────────────────────────────

const VIAL_INVENTORY_RAW = [
  // Sema — active vial, healthy stock, opened 9 days ago, expires in ~17 days
  {
    id: 'v-sema-1',
    compound: 'sema',
    dose: '0,25 мг',
    remaining: 8, total: 12,
    opened: dayOffset(-9),
    expires: dayOffset(17),
    lot: 'A24-0312',
    location: 'Холодильник, полка 2',
    recent: [
      { day: -2, dose: '0,25 мг', site: 'Правый живот' },
      { day: -9, dose: '0,25 мг', site: 'Левое бедро' },
      { day: -16, dose: '0,25 мг', site: 'Правое бедро' },
      { day: -23, dose: '0,25 мг', site: 'Левый живот' },
    ],
    usage: [0, 1, 1, 1, 1], // weeks since opened, doses per week
  },
  // Sema — sealed spare
  {
    id: 'v-sema-2',
    compound: 'sema',
    dose: '0,25 мг',
    remaining: 12, total: 12,
    opened: null,
    expires: dayOffset(86),
    lot: 'A24-0518',
    location: 'Холодильник, полка 2',
    recent: [],
    usage: [],
  },

  // BPC-157 — active, expiring in 9 days (warn)
  {
    id: 'v-bpc-1',
    compound: 'bpc',
    dose: '250 мкг',
    remaining: 14, total: 30,
    opened: dayOffset(-16),
    expires: dayOffset(9),
    lot: 'B24-0204',
    location: 'Холодильник, полка 1',
    recent: [
      { day: -1, dose: '250 мкг', site: 'Левый живот' },
      { day: -2, dose: '250 мкг', site: 'Правое плечо' },
      { day: -3, dose: '250 мкг', site: 'Левое плечо' },
      { day: -4, dose: '250 мкг', site: 'Правый живот' },
      { day: -5, dose: '250 мкг', site: 'Левый живот' },
    ],
    usage: [4, 7, 5], // doses per week
  },
  // BPC-157 — sealed
  {
    id: 'v-bpc-2',
    compound: 'bpc',
    dose: '250 мкг',
    remaining: 30, total: 30,
    opened: null,
    expires: dayOffset(137),
    lot: 'B24-0510',
    location: 'Холодильник, полка 1',
    recent: [],
    usage: [],
  },

  // TB-500 — active, low stock (5/20)
  {
    id: 'v-tb-1',
    compound: 'tb',
    dose: '2,5 мг',
    remaining: 5, total: 20,
    opened: dayOffset(-30),
    expires: dayOffset(31),
    lot: 'T24-0201',
    location: 'Холодильник, полка 3',
    recent: [
      { day: -3, dose: '2,5 мг', site: 'Правое плечо' },
      { day: -7, dose: '2,5 мг', site: 'Левое плечо' },
      { day: -10, dose: '2,5 мг', site: 'Правое бедро' },
      { day: -14, dose: '2,5 мг', site: 'Левый живот' },
    ],
    usage: [2, 2, 2, 1, 2, 2], // doses per week
  },

  // Tesamorelin — active, just opened, no spare → reorder warning
  {
    id: 'v-tes-1',
    compound: 'tes',
    dose: '1,0 мг',
    remaining: 20, total: 20,
    opened: dayOffset(-1),
    expires: dayOffset(25),
    lot: 'TS24-0405',
    location: 'Холодильник, полка 3',
    recent: [
      { day: 0, dose: '1,0 мг', site: 'Левое плечо' },
    ],
    usage: [1],
  },
];

// ── Derived ─────────────────────────────────────────────────────────

function vialWithDerived(v) {
  const status = deriveStatus(v);
  const daysToExpiry = daysBetween(VIAL_TODAY, v.expires);
  const daysSinceOpened = v.opened ? daysBetween(v.opened, VIAL_TODAY) : null;
  const lastDose = v.recent && v.recent.length > 0
    ? { ...v.recent[0], dateLabel: formatDayLabel(v.recent[0].day) }
    : null;
  return {
    ...v,
    status,
    daysToExpiry,
    daysSinceOpened,
    expiresLabel: formatDateShort(v.expires),
    openedLabel: v.opened ? formatDateShort(v.opened) : null,
    compoundMeta: COMPOUND_META[v.compound] || {},
    lastDose,
    pct: v.remaining / v.total,
  };
}

const VIAL_INVENTORY = VIAL_INVENTORY_RAW.map(vialWithDerived);

// Day label for relative-day strings (today / yesterday / N days ago).
function formatDayLabel(day) {
  if (day >= 0) return 'сегодня';
  if (day === -1) return 'вчера';
  if (day > -7) return `${-day} дн назад`;
  const w = Math.round(-day / 7);
  return `${w} нед назад`;
}

// ── Summary stats ───────────────────────────────────────────────────

function inventorySummary(inv) {
  const active   = inv.filter(v => v.status === 'active' || v.status === 'low' || (v.status === 'expiring' && v.opened));
  const sealed   = inv.filter(v => v.status === 'sealed' || (v.status === 'expiring' && !v.opened));
  const expiring = inv.filter(v => v.status === 'expiring');
  const low      = inv.filter(v => v.status === 'low');

  // Reorder hints: for each compound, estimate weeks until total stock runs out.
  // Only compounds with NO sealed spare and < 4 weeks of active supply trigger.
  const byCompound = {};
  for (const v of inv) {
    if (!byCompound[v.compound]) byCompound[v.compound] = { compound: v.compound, active: 0, sealed: 0, weeksLeft: 0 };
    if (v.opened) byCompound[v.compound].active += v.remaining;
    else          byCompound[v.compound].sealed += v.remaining;
  }
  const reorder = [];
  for (const id of Object.keys(byCompound)) {
    const c = byCompound[id];
    const meta = COMPOUND_META[id];
    if (!meta) continue;
    const totalDoses = c.active + c.sealed;
    const weeksLeft = Math.floor(totalDoses / Math.max(1, meta.weeklyDoses));
    if (c.sealed === 0 && weeksLeft <= 4) {
      reorder.push({ compound: id, meta, weeksLeft });
    }
  }

  return { active, sealed, expiring, low, reorder };
}

Object.assign(window, {
  VIAL_TODAY, COMPOUND_META,
  VIAL_INVENTORY, VIAL_INVENTORY_RAW,
  vialWithDerived, deriveStatus,
  inventorySummary, formatDayLabel, formatDateShort, daysBetween, dayOffset,
});
