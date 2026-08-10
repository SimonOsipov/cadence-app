// Cadence — Doctor messages: 1:1 threads between Доктор Ксения ('me') and patients ('them').
// Data cards: weight | dose | meal | photo. Threads awaiting Ксения's reply are unread.

const THREADS = [
  {
    patientId: 'sofia', unread: 2, pinned: true, lastTime: '5 ч',
    messages: [
      { id: 'sf1', from: 'them', kind: 'text', day: -1, time: '21:10', text: 'Здравствуйте, Ксения. Опять тошнота на третий день после укола — третий раз за неделю.' },
      { id: 'sf2', from: 'me',   kind: 'text', day: -1, time: '21:34', text: 'Спасибо, что сразу написали. Давайте посмотрим — сильная или терпимая? И как со стулом и аппетитом?' },
      { id: 'sf3', from: 'them', kind: 'text', day: 0, time: '09:02', text: 'Терпимая, но мешает работать. Аппетит почти пропал к вечеру.' },
      { id: 'sf4', from: 'them', kind: 'photo', day: 0, time: '09:03', data: { caption: 'Место укола, +24 ч — немного покраснело' } },
    ],
  },
  {
    patientId: 'marina', unread: 1, lastTime: '2 ч',
    messages: [
      { id: 'mr1', from: 'me',   kind: 'text', day: -1, time: '18:20', text: 'Марина, в воскресенье завершается 12-я неделя. Хочу свериться по самочувствию перед решением.' },
      { id: 'mr2', from: 'them', kind: 'text', day: 0, time: '08:24', text: 'Поняла. Прикрепляю вес — вот как идём. Чувствую себя ровно, сон отличный.' },
      { id: 'mr3', from: 'them', kind: 'weight', day: 0, time: '08:24', data: { eyebrow: 'Вес', title: '110,0 кг', delta: '↓ 8,0 кг с начала курса', meta: 'За последние 12 недель' } },
      { id: 'mr4', from: 'them', kind: 'text', day: 0, time: '08:25', text: 'Продлеваем ещё на цикл или делаем паузу?' },
    ],
  },
  {
    patientId: 'anna', unread: 1, lastTime: '1 ч',
    messages: [
      { id: 'an1', from: 'them', kind: 'text', day: 0, time: '07:50', text: 'Доброе утро! Можно делать укол утром вместо вечера? Так удобнее по графику.' },
    ],
  },
  {
    patientId: 'viktor', unread: 1, lastTime: '2 дн',
    messages: [
      { id: 'vk1', from: 'them', kind: 'text', day: -2, time: '14:02', text: 'Ксения, тренируюсь натощак по утрам. Это нормально на CJC-1295 или лучше поесть до зала?' },
    ],
  },
  {
    patientId: 'oleg', unread: 0, lastTime: 'вчера',
    messages: [
      { id: 'og1', from: 'me',   kind: 'text', day: -1, time: '13:40', text: 'Олег, вижу две пропущенные дозы подряд и вес немного пошёл вверх. Что случилось — забываете или что-то беспокоит?' },
      { id: 'og2', from: 'me',   kind: 'text', day: -1, time: '13:41', text: 'Давайте созвонимся сегодня в 13:30, я уже поставила в расписание. Не переживайте — это поправимо.' },
    ],
  },
  {
    patientId: 'roman', unread: 0, lastTime: '6 ч',
    messages: [
      { id: 'rm1', from: 'them', kind: 'text', day: 0, time: '08:05', text: 'Доза за сегодня сделана, всё по плану.' },
      { id: 'rm2', from: 'them', kind: 'dose', day: 0, time: '08:05', data: { eyebrow: 'Последняя доза', title: 'Тирзепатид · 7,5 мг', delta: 'Левый живот · ротация', meta: 'Чт 8:00' } },
      { id: 'rm3', from: 'me',   kind: 'text', day: 0, time: '11:12', text: 'Отлично, Роман. Минус тринадцать кило за восемь недель — держите ритм, всё ровно.' },
    ],
  },
  {
    patientId: 'elena', unread: 0, lastTime: 'вчера',
    messages: [
      { id: 'el1', from: 'them', kind: 'text', day: -1, time: '12:30', text: 'Сегодня собрала идеальный обед по белку, как договаривались.' },
      { id: 'el2', from: 'them', kind: 'meal', day: -1, time: '12:31', data: { eyebrow: 'Сегодня · обед', title: 'Боул с курицей', delta: '625 ккал · 56 г белка', meta: '5 позиций' } },
      { id: 'el3', from: 'me',   kind: 'text', day: -1, time: '13:00', text: 'Прекрасно сбалансировано. Вы стали заметно спокойнее к вечеру — это видно и по сну.' },
    ],
  },
];

// Quick coach-tone replies (canned)
const QUICK_REPLIES = [
  'Спасибо, что написали — давайте разберёмся.',
  'Это в пределах ожидаемого, не переживайте.',
  'Остаёмся на текущей дозе ещё неделю.',
  'Давайте созвонимся сегодня — поставлю в расписание.',
  'Пейте больше воды до и после укола.',
];

// Data the doctor can request / attach
const SHARE_ACTIONS = [
  { id: 'req-weight', icon: 'scale',         label: 'Запросить вес',      kind: 'request', text: 'Можете прислать свежий вес? Хочу свериться перед решением.' },
  { id: 'req-photo',  icon: 'camera',        label: 'Запросить фото',     kind: 'request', text: 'Пришлите, пожалуйста, фото места укола — посмотрю покраснение.' },
  { id: 'send-note',  icon: 'document-text', label: 'Прикрепить протокол', kind: 'note',   text: 'Прикрепила обновлённый протокол на следующую неделю.' },
];

function threadByPatient(id) { return THREADS.find(t => t.patientId === id); }

// Group messages into runs by day for date separators
function groupThreadByDay(messages) {
  const groups = [];
  let cur = null;
  for (const m of messages) {
    if (!cur || cur.day !== m.day) { cur = { day: m.day, items: [] }; groups.push(cur); }
    cur.items.push(m);
  }
  return groups;
}
function chatDay(day) {
  if (day === 0) return 'Сегодня';
  if (day === -1) return 'Вчера';
  if (day > -7) return `${-day} дн назад`;
  return `${Math.round(-day / 7)} нед назад`;
}

const TOTAL_UNREAD = THREADS.reduce((n, t) => n + (t.unread ? 1 : 0), 0);

Object.assign(window, {
  THREADS, QUICK_REPLIES, SHARE_ACTIONS,
  threadByPatient, groupThreadByDay, chatDay, TOTAL_UNREAD,
});
