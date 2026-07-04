// Cadence — Chat with care team.
// Data: care-team threads + sample messages for the active thread.
// Mid-thread state: the user just shared weight data and is awaiting a reply.

export interface CareTeamPreview {
  text: string;
  time: string;
  unread: number;
}

export interface CareTeamMember {
  id: string;
  name: string;
  role: string;
  initial: string;
  avatarBg: string;
  avatarFg: string;
  preview: CareTeamPreview;
}

export interface ChatDataCard {
  kind: string;
  eyebrow?: string;
  title?: string;
  delta?: string;
  meta?: string;
  caption?: string;
}

export interface ChatMessage {
  id: string;
  from: 'doctor' | 'me';
  kind: 'text' | 'data';
  day: number;
  time: string;
  text?: string;
  data?: ChatDataCard;
}

export interface ShareOption {
  id: string;
  label: string;
  icon: string;
  data: ChatDataCard;
}

export const CARE_TEAM: CareTeamMember[] = [
  {
    id: 'ksenia',
    name: 'Доктор Ксения Первеева',
    role: 'эндокринолог',
    initial: 'К',
    avatarBg: '#2D5F3F',
    avatarFg: '#F6F1EA',
    preview: { text: 'Утренние подойдут лучше — нагрузка на ЖКТ к вечеру…', time: 'вчера', unread: 0 },
  },
  {
    id: 'maria',
    name: 'Мария Светова',
    role: 'диетолог',
    initial: 'М',
    avatarBg: '#D4A574',
    avatarFg: '#1A1A1A',
    preview: { text: 'Посмотрела рацион — белок отлично распределён.', time: 'пн', unread: 1 },
  },
  {
    id: 'tatiana',
    name: 'Татьяна Лесова',
    role: 'медсестра',
    initial: 'Т',
    avatarBg: '#E8D4B8',
    avatarFg: '#6b4a25',
    preview: { text: 'Напоминаю: воскресный укол в 7:00. Уже подготовила…', time: 'пт', unread: 0 },
  },
];

// Anchored messages for the main thread (Ксения). Day field: -3 = three days ago.
export const CHAT_MESSAGES_KSENIA: ChatMessage[] = [
  // Day -3
  { id: 'm1', from: 'doctor', kind: 'text',
    day: -3, time: '12:14',
    text: 'Здравствуйте. Как переносите четвёртую неделю?' },
  { id: 'm2', from: 'me', kind: 'text',
    day: -3, time: '12:42',
    text: 'После утренней дозы было лёгкое подташнивание. Прошло к обеду.' },
  { id: 'm3', from: 'doctor', kind: 'text',
    day: -3, time: '13:01',
    text: 'Это в пределах ожидаемого. На этой неделе остаёмся на 0,25 мг. Если эпизоды повторятся — сообщите.' },

  // Day -1 (yesterday)
  { id: 'm4', from: 'me', kind: 'text',
    day: -1, time: '19:38',
    text: 'Понятно. Хотела спросить: можно ли заменить вечернюю тренировку на утреннюю?' },
  { id: 'm5', from: 'doctor', kind: 'text',
    day: -1, time: '19:51',
    text: 'Утром даже предпочтительнее — нагрузка на ЖКТ к вечеру может усиливать тошноту. Главное: пейте воду до и во время.' },

  // Today
  { id: 'm6', from: 'me', kind: 'text',
    day: 0, time: '08:24',
    text: 'Поняла, спасибо. Прикрепляю вес — вот как идём.' },
  { id: 'm7', from: 'me', kind: 'data',
    day: 0, time: '08:24',
    data: {
      kind: 'weight',
      eyebrow: 'Вес',
      title: '110,0 кг',
      delta: '↓ 8,0 кг с начала курса',
      meta: 'За последние 12 недель',
    },
  },
  // No doctor reply yet → conversation ends pending.
];

// Empty-ish previews for the other threads (when tapped — minimal content).
export const CHAT_MESSAGES_MARIA: ChatMessage[] = [
  { id: 'mm1', from: 'doctor', kind: 'text',
    day: -2, time: '14:18',
    text: 'Посмотрела ваш рацион за неделю — белок отлично распределён по приёмам. Молодец.' },
  { id: 'mm2', from: 'doctor', kind: 'text',
    day: -2, time: '14:19',
    text: 'Одно: попробуйте сместить часть углеводов на обед — будет ровнее с энергией ко второй половине дня.' },
];

export const CHAT_MESSAGES_TATIANA: ChatMessage[] = [
  { id: 'mt1', from: 'doctor', kind: 'text',
    day: -3, time: '09:02',
    text: 'Напоминаю: воскресный укол в 7:00. Флакон уже подготовила, забирайте в любое время.' },
];

// Lookup by thread id
export const CHAT_MESSAGES: Record<string, ChatMessage[]> = {
  ksenia: CHAT_MESSAGES_KSENIA,
  maria: CHAT_MESSAGES_MARIA,
  tatiana: CHAT_MESSAGES_TATIANA,
};

// Helpers
export function chatDayLabel(day: number): string {
  if (day === 0)  return 'Сегодня';
  if (day === -1) return 'Вчера';
  if (day > -7)   return `${-day} дн назад`;
  return `${Math.round(-day / 7)} нед назад`;
}

export interface ChatDayGroup {
  day: number;
  messages: ChatMessage[];
}

// Group messages into runs by date for date separators
export function groupByDay(messages: ChatMessage[]): ChatDayGroup[] {
  const groups: ChatDayGroup[] = [];
  let cur: ChatDayGroup | null = null;
  for (const m of messages) {
    if (!cur || cur.day !== m.day) {
      cur = { day: m.day, messages: [] };
      groups.push(cur);
    }
    cur.messages.push(m);
  }
  return groups;
}

// Data-share quick chips (inserted above the input). Each generates a data card.
export const SHARE_OPTIONS: ShareOption[] = [
  {
    id: 'weight',
    label: 'Поделиться весом',
    icon: 'scale',
    data: {
      kind: 'weight',
      eyebrow: 'Вес',
      title: '110,0 кг',
      delta: '↓ 8,0 кг с начала курса',
      meta: 'За последние 12 недель',
    },
  },
  {
    id: 'dose',
    label: 'Последняя доза',
    icon: 'beaker',
    data: {
      kind: 'dose',
      eyebrow: 'Последняя доза',
      title: 'Семаглутид · 0,25 мг',
      delta: 'Правый живот · ротация',
      meta: 'Вс 7:00 · 9 дн назад',
    },
  },
  {
    id: 'meal',
    label: 'Сегодняшний приём',
    icon: 'cake',
    data: {
      kind: 'meal',
      eyebrow: 'Сегодня · обед',
      title: 'Боул с курицей',
      delta: '625 ккал · 56 г белка',
      meta: '5 позиций · 12:42',
    },
  },
  {
    id: 'photo',
    label: 'Фото места укола',
    icon: 'camera',
    data: {
      kind: 'photo',
      caption: 'Правый живот, через 24 ч',
    },
  },
];
