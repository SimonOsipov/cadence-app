// Cadence — Doctor dashboard data model
// Single endocrinologist (Доктор Ксения Первеева) tracking ~26 patients on peptide protocols.
// Status: 'attention' (needs action), 'watch' (keep an eye), 'track' (on track).
// Weight series are normalized 0..1 spark points (oldest → newest).

const DOCTOR = {
  name: 'Ксения Первеева',
  role: 'Эндокринолог',
  initial: 'К',
  clinic: 'Практика Cadence',
};

// Flag kinds → presentation
const FLAG_META = {
  missed:    { tone: 'danger',  icon: 'exclamation-circle', label: 'Пропуск дозы' },
  side:      { tone: 'warning', icon: 'heart',              label: 'Побочный эффект' },
  biomarker: { tone: 'warning', icon: 'arrow-trending-up',  label: 'Биомаркер вне нормы' },
  message:   { tone: 'info',    icon: 'chat-bubble',        label: 'Ждёт ответа' },
  titration: { tone: 'forest',  icon: 'beaker',             label: 'Пора титровать' },
  cycle:     { tone: 'sand',    icon: 'clock',              label: 'Цикл завершается' },
};

// avatar palette rotation
const AV = [
  { bg: '#2d5f3f', fg: '#f6f1ea' },
  { bg: '#d4a574', fg: '#1a1a1a' },
  { bg: '#1f4530', fg: '#f6f1ea' },
  { bg: '#e8d4b8', fg: '#6b4a25' },
  { bg: '#3d7a52', fg: '#f6f1ea' },
  { bg: '#b8895a', fg: '#1a1a1a' },
];

let _av = 0;
function avatar() { const a = AV[_av % AV.length]; _av++; return a; }

// Spark generators (normalized) -------------------------------------------------
const fall   = [0.95, 0.88, 0.82, 0.74, 0.66, 0.58, 0.50, 0.42];       // steady loss — good
const fallS  = [0.92, 0.80, 0.70, 0.60, 0.55, 0.52, 0.50, 0.49];       // loss then ease
const stall  = [0.80, 0.70, 0.62, 0.58, 0.57, 0.58, 0.57, 0.58];       // plateau — watch
const rise   = [0.42, 0.46, 0.44, 0.52, 0.55, 0.62, 0.66, 0.70];       // creeping up — attention
const early  = [0.98, 0.96, 0.93, 0.90, 0.88, 0.85, 0.83, 0.80];       // just started
const wobble = [0.70, 0.62, 0.68, 0.55, 0.60, 0.50, 0.54, 0.46];       // noisy but down

function p(o) {
  return { ...avatar(), ...o };
}

const PATIENTS = [
  // ── Need attention ───────────────────────────────────────────────
  p({
    id: 'marina', name: 'Марина Левченко', age: 41, initial: 'М',
    compound: 'Семаглутид', dose: '1,0 мг', cadence: 'еженедельно',
    week: 12, cycleLen: 12, status: 'attention',
    weight: 110.0, weightStart: 118.0, unit: 'кг', spark: fallS,
    adherence: 96, lastSeen: '2 ч назад', goal: 100,
    flags: ['cycle', 'message'],
    note: 'Цикл завершается в воскресенье — решение о продлении или паузе.',
    metrics: { hrv: 71, rhr: 58, sleep: 87 },
  }),
  p({
    id: 'oleg', name: 'Олег Самойлов', age: 47, initial: 'О',
    compound: 'Тирзепатид', dose: '5,0 мг', cadence: 'еженедельно',
    week: 5, cycleLen: 12, status: 'attention',
    weight: 104.2, weightStart: 109.0, unit: 'кг', spark: rise,
    adherence: 71, lastSeen: 'вчера', goal: 92,
    flags: ['missed', 'biomarker'],
    note: 'Две пропущенные дозы подряд, вес пошёл вверх. Нужен звонок.',
    metrics: { hrv: 48, rhr: 67, sleep: 64 },
  }),
  p({
    id: 'sofia', name: 'София Ермакова', age: 35, initial: 'С',
    compound: 'Семаглутид', dose: '0,5 мг', cadence: 'еженедельно',
    week: 7, cycleLen: 12, status: 'attention',
    weight: 71.8, weightStart: 78.0, unit: 'кг', spark: wobble,
    adherence: 88, lastSeen: '5 ч назад', goal: 66,
    flags: ['side', 'message'],
    note: 'Тошнота на третий день после дозы — третий эпизод за неделю.',
    metrics: { hrv: 59, rhr: 62, sleep: 70 },
  }),
  p({
    id: 'dmitri', name: 'Дмитрий Орлов', age: 52, initial: 'Д',
    compound: 'Тесаморелин', dose: '2,0 мг', cadence: 'ежедневно',
    week: 9, cycleLen: 12, status: 'attention',
    weight: 88.4, weightStart: 94.0, unit: 'кг', spark: stall,
    adherence: 82, lastSeen: '3 дн назад', goal: 84,
    flags: ['biomarker', 'titration'],
    note: 'HRV просел две недели подряд, прогресс по весу встал.',
    metrics: { hrv: 41, rhr: 64, sleep: 61 },
  }),

  // ── Watch ────────────────────────────────────────────────────────
  p({
    id: 'anna', name: 'Анна Кравцова', age: 38, initial: 'А',
    compound: 'Семаглутид', dose: '0,25 мг', cadence: 'еженедельно',
    week: 2, cycleLen: 12, status: 'watch',
    weight: 82.1, weightStart: 84.0, unit: 'кг', spark: early,
    adherence: 100, lastSeen: '1 ч назад', goal: 72,
    flags: ['message'],
    note: 'Первая титрация через неделю — вопрос про утренний приём.',
    metrics: { hrv: 63, rhr: 60, sleep: 74 },
  }),
  p({
    id: 'pavel', name: 'Павел Гордеев', age: 44, initial: 'П',
    compound: 'BPC-157', dose: '250 мкг', cadence: 'ежедневно',
    week: 4, cycleLen: 8, status: 'watch',
    weight: 91.0, weightStart: 92.5, unit: 'кг', spark: stall,
    adherence: 79, lastSeen: 'вчера', goal: 88,
    flags: ['missed'],
    note: 'Восстановление связки — одна пропущенная инъекция в среду.',
    metrics: { hrv: 55, rhr: 59, sleep: 68 },
  }),
  p({
    id: 'irina', name: 'Ирина Соколова', age: 49, initial: 'И',
    compound: 'Тирзепатид', dose: '2,5 мг', cadence: 'еженедельно',
    week: 3, cycleLen: 12, status: 'watch',
    weight: 96.3, weightStart: 99.0, unit: 'кг', spark: fall,
    adherence: 92, lastSeen: '6 ч назад', goal: 84,
    flags: ['titration'],
    note: 'Хорошо переносит — на следующей неделе подъём до 5,0 мг.',
    metrics: { hrv: 58, rhr: 61, sleep: 72 },
  }),
  p({
    id: 'viktor', name: 'Виктор Зайцев', age: 56, initial: 'В',
    compound: 'CJC-1295', dose: '100 мкг', cadence: '2× в неделю',
    week: 6, cycleLen: 10, status: 'watch',
    weight: 99.5, weightStart: 102.0, unit: 'кг', spark: wobble,
    adherence: 85, lastSeen: '2 дн назад', goal: 90,
    flags: ['message'],
    note: 'Спрашивает про сочетание с тренировками натощак.',
    metrics: { hrv: 52, rhr: 63, sleep: 66 },
  }),

  // ── On track ─────────────────────────────────────────────────────
  p({
    id: 'elena', name: 'Елена Власова', age: 36, initial: 'Е',
    compound: 'Семаглутид', dose: '1,0 мг', cadence: 'еженедельно',
    week: 10, cycleLen: 12, status: 'track',
    weight: 68.2, weightStart: 79.0, unit: 'кг', spark: fall,
    adherence: 98, lastSeen: '4 ч назад', goal: 65,
    flags: [],
    note: '',
    metrics: { hrv: 69, rhr: 56, sleep: 88 },
  }),
  p({
    id: 'roman', name: 'Роман Беляев', age: 43, initial: 'Р',
    compound: 'Тирзепатид', dose: '7,5 мг', cadence: 'еженедельно',
    week: 8, cycleLen: 12, status: 'track',
    weight: 95.0, weightStart: 108.0, unit: 'кг', spark: fall,
    adherence: 100, lastSeen: 'сегодня', goal: 88,
    flags: [],
    note: '',
    metrics: { hrv: 66, rhr: 57, sleep: 84 },
  }),
  p({
    id: 'natalia', name: 'Наталья Демина', age: 39, initial: 'Н',
    compound: 'Семаглутид', dose: '0,5 мг', cadence: 'еженедельно',
    week: 6, cycleLen: 12, status: 'track',
    weight: 74.4, weightStart: 81.0, unit: 'кг', spark: fallS,
    adherence: 94, lastSeen: '8 ч назад', goal: 68,
    flags: [],
    note: '',
    metrics: { hrv: 64, rhr: 59, sleep: 80 },
  }),
  p({
    id: 'andrei', name: 'Андрей Тихонов', age: 50, initial: 'А',
    compound: 'Ипаморелин', dose: '200 мкг', cadence: 'ежедневно',
    week: 5, cycleLen: 10, status: 'track',
    weight: 86.7, weightStart: 90.0, unit: 'кг', spark: wobble,
    adherence: 91, lastSeen: 'вчера', goal: 82,
    flags: [],
    note: '',
    metrics: { hrv: 61, rhr: 58, sleep: 78 },
  }),
  p({
    id: 'yulia', name: 'Юлия Фомина', age: 33, initial: 'Ю',
    compound: 'Семаглутид', dose: '0,25 мг', cadence: 'еженедельно',
    week: 3, cycleLen: 12, status: 'track',
    weight: 77.0, weightStart: 80.0, unit: 'кг', spark: early,
    adherence: 100, lastSeen: '3 ч назад', goal: 70,
    flags: [],
    note: '',
    metrics: { hrv: 62, rhr: 60, sleep: 76 },
  }),
  p({
    id: 'maxim', name: 'Максим Корнев', age: 46, initial: 'М',
    compound: 'TB-500', dose: '2,0 мг', cadence: '2× в неделю',
    week: 7, cycleLen: 8, status: 'track',
    weight: 93.2, weightStart: 96.0, unit: 'кг', spark: fall,
    adherence: 96, lastSeen: 'сегодня', goal: 90,
    flags: [],
    note: '',
    metrics: { hrv: 60, rhr: 58, sleep: 79 },
  }),
  p({
    id: 'galina', name: 'Галина Орехова', age: 54, initial: 'Г',
    compound: 'Тирзепатид', dose: '5,0 мг', cadence: 'еженедельно',
    week: 11, cycleLen: 12, status: 'track',
    weight: 81.5, weightStart: 95.0, unit: 'кг', spark: fall,
    adherence: 99, lastSeen: '5 ч назад', goal: 78,
    flags: [],
    note: '',
    metrics: { hrv: 67, rhr: 56, sleep: 85 },
  }),
  p({
    id: 'sergei', name: 'Сергей Лапин', age: 41, initial: 'С',
    compound: 'Семаглутид', dose: '1,0 мг', cadence: 'еженедельно',
    week: 9, cycleLen: 12, status: 'track',
    weight: 89.0, weightStart: 101.0, unit: 'кг', spark: fallS,
    adherence: 93, lastSeen: 'вчера', goal: 84,
    flags: [],
    note: '',
    metrics: { hrv: 63, rhr: 57, sleep: 81 },
  }),
  p({
    id: 'kira', name: 'Кира Жукова', age: 37, initial: 'К',
    compound: 'Ретатрутид', dose: '4,0 мг', cadence: 'еженедельно',
    week: 4, cycleLen: 12, status: 'track',
    weight: 84.3, weightStart: 90.0, unit: 'кг', spark: fall,
    adherence: 97, lastSeen: '7 ч назад', goal: 76,
    flags: [],
    note: '',
    metrics: { hrv: 65, rhr: 58, sleep: 82 },
  }),
  p({
    id: 'boris', name: 'Борис Шевцов', age: 58, initial: 'Б',
    compound: 'Тесаморелин', dose: '1,0 мг', cadence: 'ежедневно',
    week: 8, cycleLen: 12, status: 'track',
    weight: 97.8, weightStart: 104.0, unit: 'кг', spark: wobble,
    adherence: 90, lastSeen: '2 дн назад', goal: 92,
    flags: [],
    note: '',
    metrics: { hrv: 54, rhr: 60, sleep: 73 },
  }),
  p({
    id: 'vera', name: 'Вера Зорина', age: 45, initial: 'В',
    compound: 'Семаглутид', dose: '0,5 мг', cadence: 'еженедельно',
    week: 5, cycleLen: 12, status: 'track',
    weight: 79.1, weightStart: 86.0, unit: 'кг', spark: fallS,
    adherence: 95, lastSeen: '4 ч назад', goal: 72,
    flags: [],
    note: '',
    metrics: { hrv: 61, rhr: 59, sleep: 78 },
  }),
  p({
    id: 'timur', name: 'Тимур Аскеров', age: 42, initial: 'Т',
    compound: 'Тирзепатид', dose: '10,0 мг', cadence: 'еженедельно',
    week: 10, cycleLen: 12, status: 'track',
    weight: 102.4, weightStart: 118.0, unit: 'кг', spark: fall,
    adherence: 98, lastSeen: 'сегодня', goal: 95,
    flags: [],
    note: '',
    metrics: { hrv: 64, rhr: 57, sleep: 83 },
  }),
  p({
    id: 'lidia', name: 'Лидия Панова', age: 51, initial: 'Л',
    compound: 'CJC-1295', dose: '100 мкг', cadence: '2× в неделю',
    week: 6, cycleLen: 10, status: 'track',
    weight: 75.6, weightStart: 80.0, unit: 'кг', spark: wobble,
    adherence: 92, lastSeen: 'вчера', goal: 70,
    flags: [],
    note: '',
    metrics: { hrv: 59, rhr: 58, sleep: 77 },
  }),
  p({
    id: 'egor', name: 'Егор Власенко', age: 39, initial: 'Е',
    compound: 'Семаглутид', dose: '0,25 мг', cadence: 'еженедельно',
    week: 1, cycleLen: 12, status: 'track',
    weight: 88.0, weightStart: 89.0, unit: 'кг', spark: early,
    adherence: 100, lastSeen: '6 ч назад', goal: 80,
    flags: [],
    note: '',
    metrics: { hrv: 60, rhr: 61, sleep: 75 },
  }),
  p({
    id: 'alina', name: 'Алина Серова', age: 34, initial: 'А',
    compound: 'Ипаморелин', dose: '200 мкг', cadence: 'ежедневно',
    week: 7, cycleLen: 10, status: 'track',
    weight: 64.8, weightStart: 69.0, unit: 'кг', spark: fallS,
    adherence: 96, lastSeen: '3 ч назад', goal: 60,
    flags: [],
    note: '',
    metrics: { hrv: 66, rhr: 57, sleep: 84 },
  }),
  p({
    id: 'nikita', name: 'Никита Громов', age: 48, initial: 'Н',
    compound: 'Тирзепатид', dose: '5,0 мг', cadence: 'еженедельно',
    week: 9, cycleLen: 12, status: 'track',
    weight: 100.1, weightStart: 112.0, unit: 'кг', spark: fall,
    adherence: 94, lastSeen: 'вчера', goal: 92,
    flags: [],
    note: '',
    metrics: { hrv: 62, rhr: 58, sleep: 80 },
  }),
  p({
    id: 'darya', name: 'Дарья Котова', age: 40, initial: 'Д',
    compound: 'Семаглутид', dose: '0,5 мг', cadence: 'еженедельно',
    week: 4, cycleLen: 12, status: 'track',
    weight: 73.7, weightStart: 79.0, unit: 'кг', spark: fallS,
    adherence: 97, lastSeen: '5 ч назад', goal: 68,
    flags: [],
    note: '',
    metrics: { hrv: 63, rhr: 59, sleep: 79 },
  }),
];

// Today's schedule — doses & check-ins due today, in chronological order.
const SCHEDULE = [
  { id: 's1', time: '07:00', patientId: 'marina',  kind: 'dose',  label: 'Семаглутид · 1,0 мг', state: 'done' },
  { id: 's2', time: '07:30', patientId: 'dmitri',  kind: 'dose',  label: 'Тесаморелин · 2,0 мг', state: 'done' },
  { id: 's3', time: '08:00', patientId: 'roman',   kind: 'dose',  label: 'Тирзепатид · 7,5 мг', state: 'done' },
  { id: 's4', time: '09:15', patientId: 'sofia',   kind: 'checkin', label: 'Звонок · побочные эффекты', state: 'now' },
  { id: 's5', time: '11:00', patientId: 'anna',    kind: 'dose',  label: 'Семаглутид · 0,25 мг', state: 'due' },
  { id: 's6', time: '13:30', patientId: 'oleg',    kind: 'checkin', label: 'Звонок · пропуски дозы', state: 'due' },
  { id: 's7', time: '17:00', patientId: 'pavel',   kind: 'dose',  label: 'BPC-157 · 250 мкг', state: 'due' },
  { id: 's8', time: '19:00', patientId: 'andrei',  kind: 'dose',  label: 'Ипаморелин · 200 мкг', state: 'due' },
];

// Recent patient activity feed
const ACTIVITY = [
  { id: 'a1', patientId: 'marina', icon: 'scale',       text: 'записала вес 110,0 кг', sub: '↓ 0,6 кг за неделю', time: '2 ч', tone: 'forest' },
  { id: 'a2', patientId: 'sofia',  icon: 'heart',       text: 'отметила тошноту', sub: 'через 3 дня после дозы', time: '5 ч', tone: 'warning' },
  { id: 'a3', patientId: 'roman',  icon: 'beaker',      text: 'записал дозу', sub: 'Тирзепатид · 7,5 мг', time: '6 ч', tone: 'forest' },
  { id: 'a4', patientId: 'oleg',   icon: 'exclamation-circle', text: 'пропустил дозу', sub: 'вторая подряд', time: 'вчера', tone: 'danger' },
  { id: 'a5', patientId: 'elena',  icon: 'cake',        text: 'добавила приём пищи', sub: '625 ккал · 56 г белка', time: 'вчера', tone: 'sand' },
  { id: 'a6', patientId: 'anna',   icon: 'chat-bubble', text: 'задала вопрос', sub: 'про утренний приём', time: 'вчера', tone: 'info' },
];

// ── Derived helpers ──────────────────────────────────────────────────────
function lost(pt) { return +(pt.weightStart - pt.weight).toFixed(1); }
function lostPct(pt) {
  const total = pt.weightStart - pt.goal;
  if (total <= 0) return 100;
  return Math.round(((pt.weightStart - pt.weight) / total) * 100);
}
function statusMeta(s) {
  return {
    attention: { label: 'Внимание',  tone: 'danger',  dot: '#b8503c' },
    watch:     { label: 'Наблюдение', tone: 'warning', dot: '#c2780a' },
    track:     { label: 'В норме',    tone: 'forest',  dot: '#2d5f3f' },
  }[s];
}

const STATS = (() => {
  const total = PATIENTS.length;
  const attention = PATIENTS.filter(x => x.status === 'attention').length;
  const watch = PATIENTS.filter(x => x.status === 'watch').length;
  const dosesToday = SCHEDULE.filter(s => s.kind === 'dose').length;
  const dosesDone = SCHEDULE.filter(s => s.kind === 'dose' && s.state === 'done').length;
  const unread = PATIENTS.reduce((n, x) => n + (x.flags.includes('message') ? 1 : 0), 0);
  const avgAdh = Math.round(PATIENTS.reduce((n, x) => n + x.adherence, 0) / total);
  return { total, attention, watch, dosesToday, dosesDone, unread, avgAdh };
})();

function patientById(id) { return PATIENTS.find(x => x.id === id); }

Object.assign(window, {
  DOCTOR, PATIENTS, SCHEDULE, ACTIVITY, FLAG_META, STATS,
  lost, lostPct, statusMeta, patientById,
});
