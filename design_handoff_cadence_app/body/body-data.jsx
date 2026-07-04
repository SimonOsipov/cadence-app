// Cadence · Тело (Body metrics) — data model
// Self-consistent with the Profile journey (118 → 110 кг by week 4).
// Weekly samples Нед 1..4. Exposes: BODY on window.
//
// State that the add-flows mutate (histories, photos) lives in the prototype;
// this module provides the seed + pure helpers.

(function () {
  const HEIGHT_M = 1.88;                         // → BMI 31.2 at 110 кг
  const WEEK_LABELS = ['Нед 1', 'Нед 2', 'Нед 3', 'Нед 4'];

  // Metric metadata. `editable` ones appear in the add sheet; bmi is derived.
  const METRICS = [
    { id: 'weight',  label: 'Вес',     unit: 'кг', dec: 1, editable: true,  min: 80,  max: 140, step: 0.1 },
    { id: 'bodyfat', label: '% жира',  unit: '%',  dec: 1, editable: true,  min: 20,  max: 55,  step: 0.1 },
    { id: 'waist',   label: 'Талия',   unit: 'см', dec: 0, editable: true,  min: 70,  max: 120, step: 1, trendId: 'waist' },
    { id: 'hip',     label: 'Бёдра',   unit: 'см', dec: 0, editable: true,  min: 90,  max: 140, step: 1 },
    { id: 'chest',   label: 'Грудь',   unit: 'см', dec: 0, editable: true,  min: 85,  max: 130, step: 1 },
    { id: 'bmi',     label: 'ИМТ',     unit: '',   dec: 1, editable: false, derived: true },
  ];
  function metricMeta(id) { return METRICS.find(m => m.id === id); }

  // Seed weekly histories (Нед 1 → Нед 4 = today)
  const SEED_HIST = {
    weight:  [118.0, 115.4, 112.6, 110.0],
    bodyfat: [44.0, 42.6, 41.4, 40.5],
    waist:   [101, 97, 94, 92],
    hip:     [116, 112, 110, 108],
    chest:   [112, 108, 106, 105],
  };

  const SEED_PHOTOS = [
    { id: 'p1', week: 'Нед 1', weight: 118.0 },
    { id: 'p2', week: 'Нед 2', weight: 115.4 },
    { id: 'p3', week: 'Нед 4', weight: 110.0 },
  ];

  const TARGET = { weight: 102.0 };

  // ── Pure helpers ─────────────────────────────────────────
  function seed() {
    return {
      hist: JSON.parse(JSON.stringify(SEED_HIST)),
      photos: JSON.parse(JSON.stringify(SEED_PHOTOS)),
    };
  }
  function latest(arr) { return arr[arr.length - 1]; }
  function first(arr) { return arr[0]; }
  function bmiOf(weight) { return weight / (HEIGHT_M * HEIGHT_M); }
  function leanOf(weight, bf) { return weight * (1 - bf / 100); }

  // history for a metric, computing derived bmi from the weight series
  function histOf(state, id) {
    if (id === 'bmi') return state.hist.weight.map(w => +bmiOf(w).toFixed(1));
    return state.hist[id] || [];
  }

  function fmt(v, dec = 0) {
    if (v == null || isNaN(v)) return '—';
    return Number(v).toFixed(dec).replace('.', ',');
  }

  // delta from first→latest; for these metrics down = progress
  function delta(arr) {
    if (!arr || arr.length < 2) return { raw: 0, down: true };
    const d = latest(arr) - first(arr);
    return { raw: d, down: d <= 0 };
  }
  function fmtDelta(arr, dec = 0, unit = '') {
    const d = delta(arr);
    const arrow = d.raw <= 0 ? '↓' : '↑';
    return `${arrow} ${fmt(Math.abs(d.raw), dec)}${unit ? ' ' + unit : ''}`;
  }

  window.BODY = {
    HEIGHT_M, WEEK_LABELS, METRICS, metricMeta, TARGET,
    seed, latest, first, bmiOf, leanOf, histOf, fmt, delta, fmtDelta,
  };
})();
