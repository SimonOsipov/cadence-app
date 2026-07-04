// Cadence — Профиль (Profile) screen + navigable Settings sub-screen.
// Opens from the "М" avatar in the Today header. Slides in from the right,
// matching Trends / Vials / Chat. Centerpiece is the week 4-of-12 cycle ring.
//
// Exposes: ProfileScreen
//
// Props (ProfileScreen):
//   pal        — palette from getPalette(dark)
//   platform   — 'ios' | 'android' (top padding for the status bar)
//   showMembership — bool (Tweak)
//   density        — 'airy' | 'compact' (Tweak)
//   onBack         — close the profile layer
//   onOpenChat     — open a care-team thread (id)
//   onOpenTrend    — open a biomarker trend detail (id)

// ── Persona facts (single source so copy stays consistent) ──────────
const PROFILE = {
  name: 'Марина',
  full: 'Марина Волкова',
  email: 'marina.volkova@gmail.com',
  since: 'В Cadence с марта 2026',
  goalPill: 'Лёгкость к лету',
  cycle: { week: 4, total: 12, compound: 'Семаглутид', dose: '0,25 мг', endLabel: 'до 20 июля' },
  journey: { start: 118.0, now: 110.0, target: 102.0, unit: 'кг' },
  stats: [
    { id: 'streak', value: '8', unit: 'нед', label: 'Серия доз' },
    { id: 'doses',  value: '11', unit: '',    label: 'Доз записано' },
    { id: 'adhere', value: '96', unit: '%',   label: 'Дисциплина' },
  ],
  photos: [
    { week: 'Нед 1', w: '118,0' },
    { week: 'Нед 2', w: '115,4' },
    { week: 'Нед 4', w: '110,0' },
  ],
  measures: [
    { id: 'waist', label: 'Талия', value: '92', unit: 'см', delta: '↓ 4' },
    { id: 'hip',   label: 'Бёдра', value: '108', unit: 'см', delta: '↓ 3' },
    { id: 'bmi',   label: 'ИМТ', value: '31,2', unit: '', delta: '↓ 2,1' },
  ],
};

// ── Brand toggle switch (forest when on) ────────────────────────────
function PrefToggle({ on, onChange, pal }) {
  return (
    <button
      onClick={() => onChange(!on)}
      role="switch"
      aria-checked={on}
      style={{
        width: 46, height: 28, borderRadius: 999, border: 'none', cursor: 'pointer',
        background: on ? C.forest700 : pal.border, position: 'relative', flexShrink: 0,
        transition: 'background 220ms var(--ease-out)', padding: 0,
      }}
    >
      <span style={{
        position: 'absolute', top: 3, left: on ? 21 : 3,
        width: 22, height: 22, borderRadius: 999, background: C.paper,
        boxShadow: '0 1px 3px rgba(46,38,24,.28)',
        transition: 'left 220ms var(--ease-spring)',
      }} />
    </button>
  );
}

// ── Brand segmented control ─────────────────────────────────────────
function SegControl({ value, options, onChange, pal }) {
  return (
    <div style={{ display: 'flex', gap: 4, background: pal.sunk, padding: 3, borderRadius: 999 }}>
      {options.map(o => {
        const on = value === o.id;
        return (
          <button key={o.id} onClick={() => onChange(o.id)} className="press" style={{
            flex: 1, padding: '8px 10px', borderRadius: 999, border: 'none', cursor: 'pointer',
            background: on ? C.forest700 : 'transparent', color: on ? C.cream : pal.muted,
            fontFamily: F.body, fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap',
            transition: 'background 180ms var(--ease-out), color 180ms var(--ease-out)',
          }}>{o.label}</button>
        );
      })}
    </div>
  );
}

// ── Section eyebrow + optional trailing action ──────────────────────
function SectionHead({ children, action, onAction, pal }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px 10px' }}>
      <Eyebrow style={{ color: pal.subtle }}>{children}</Eyebrow>
      {action && (
        <button onClick={onAction} style={{
          background: 'none', border: 'none', cursor: 'pointer',
          fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500,
        }}>{action}</button>
      )}
    </div>
  );
}

// ── A tappable list row inside an outline card group ────────────────
function NavRow({ iconName, iconTone = 'forest', title, sub, trail, onClick, pal, last }) {
  const tones = {
    forest: { bg: C.forest50, fg: C.forest700 },
    sand:   { bg: C.sand100, fg: '#6b4a25' },
    linen:  { bg: pal.sunk, fg: pal.ink2 },
    danger: { bg: C.dangerBg, fg: C.danger },
  }[iconTone];
  return (
    <div onClick={onClick} className={onClick ? 'press' : ''} style={{
      display: 'grid', gridTemplateColumns: '40px 1fr auto', gap: 14, alignItems: 'center',
      padding: '13px 14px', cursor: onClick ? 'pointer' : 'default',
      borderBottom: last ? 'none' : `1px solid ${pal.hairline}`,
    }}>
      <div style={{
        width: 40, height: 40, borderRadius: 12, background: tones.bg, color: tones.fg,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <Icon name={iconName} size={20} />
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: iconTone === 'danger' ? C.danger : pal.ink }}>{title}</div>
        {sub && <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>{sub}</div>}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        {trail && <span style={{ fontFamily: F.body, fontSize: 13, color: pal.subtle }}>{trail}</span>}
        {onClick && <Icon name="chevron-right" size={16} color={pal.placeholder} />}
      </div>
    </div>
  );
}

// ── Cycle ring hero — the emotional centerpiece ─────────────────────
function CycleHero({ pal, onOpenTrend, onOpenSchedule }) {
  const { week, total, compound, dose, endLabel } = PROFILE.cycle;
  return (
    <div style={{
      background: C.forest800, color: C.cream, borderRadius: 24, padding: 22,
      boxShadow: '0 8px 24px rgba(20,44,31,.18)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div style={{
          fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em',
          textTransform: 'uppercase', color: C.sand300,
        }}>Текущий курс</div>
        <span style={{ fontFamily: F.mono, fontSize: 11, color: 'rgba(246,241,234,.6)' }}>{endLabel}</span>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: 18, alignItems: 'center' }}>
        {/* Ring with center week label */}
        <div style={{ position: 'relative', width: 132, height: 132, flexShrink: 0 }}>
          <CycleRing
            week={week} total={total} size={132} stroke={9}
            pal={{ ...pal, bone: 'rgba(246,241,234,.14)', subtle: 'rgba(246,241,234,.4)', forestBg: '#1a3527' }}
            color={C.sand500}
          />
          <div style={{
            position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column',
            alignItems: 'center', justifyContent: 'center', gap: 0,
          }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 2 }}>
              <span style={{ fontFamily: F.mono, fontSize: 36, fontWeight: 500, color: C.cream, lineHeight: 1, letterSpacing: '-0.03em' }}>{week}</span>
              <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 18, color: C.sand300 }}>/{total}</span>
            </div>
            <span style={{
              fontFamily: F.body, fontSize: 10, fontWeight: 500, letterSpacing: '.08em',
              textTransform: 'uppercase', color: 'rgba(246,241,234,.55)', marginTop: 3,
            }}>неделя</span>
          </div>
        </div>

        {/* Copy */}
        <div style={{ minWidth: 0 }}>
          <div style={{ fontFamily: F.display, fontSize: 28, color: C.cream, lineHeight: 1.04, letterSpacing: '-0.018em' }}>
            Четвёртая неделя — вы <span style={{ fontStyle: 'italic', color: C.sand300 }}>в ритме</span>.
          </div>
          <div style={{ fontFamily: F.body, fontSize: 13, color: 'rgba(246,241,234,.72)', lineHeight: 1.5, marginTop: 8 }}>
            {compound} · {dose} · еженедельно
          </div>
        </div>
      </div>

      <button onClick={() => onOpenSchedule ? onOpenSchedule() : (onOpenTrend && onOpenTrend('weight'))} className="press" style={{
        display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
        width: '100%', marginTop: 18, padding: '12px 18px', borderRadius: 999,
        background: 'rgba(246,241,234,.12)', color: C.cream, border: 'none',
        fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer',
      }}>
        Открыть курс целиком
        <Icon name="arrow-right" size={15} />
      </button>
    </div>
  );
}

// ── Weight journey: start → now → target with progress track ────────
function JourneyCard({ pal, onOpenTrend }) {
  const { start, now, target, unit } = PROFILE.journey;
  const pct = Math.max(0, Math.min(1, (start - now) / (start - target)));
  const fmt = (n) => n.toFixed(1).replace('.', ',');
  return (
    <div onClick={() => onOpenTrend && onOpenTrend('weight')} className="press" style={{
      background: pal.paper, borderRadius: 18, padding: 16, cursor: 'pointer',
      border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)',
    }}>
      <SectionHead pal={pal} action="Тренд →" onAction={(e) => { e.stopPropagation(); onOpenTrend && onOpenTrend('weight'); }}>Путь к цели</SectionHead>

      <div style={{ fontFamily: F.display, fontSize: 22, color: pal.ink, lineHeight: 1.1, letterSpacing: '-0.012em', padding: '2px 4px 16px' }}>
        Восемь килограммов позади — <span style={{ fontStyle: 'italic', color: C.forest700 }}>половина пути</span>.
      </div>

      {/* Track */}
      <div style={{ padding: '0 4px' }}>
        <div style={{ position: 'relative', height: 8, borderRadius: 999, background: pal.sunk, marginBottom: 12 }}>
          <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${pct * 100}%`, borderRadius: 999, background: C.forest700 }} />
          <div style={{
            position: 'absolute', top: '50%', left: `${pct * 100}%`, transform: 'translate(-50%,-50%)',
            width: 16, height: 16, borderRadius: 999, background: C.sand500, border: `3px solid ${pal.paper}`,
            boxShadow: '0 1px 4px rgba(46,38,24,.25)',
          }} />
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          {[
            { k: 'Старт', v: start, muted: true },
            { k: 'Сейчас', v: now, accent: true },
            { k: 'Цель', v: target, muted: true },
          ].map((s, i) => (
            <div key={i} style={{ textAlign: i === 0 ? 'left' : i === 2 ? 'right' : 'center' }}>
              <div style={{ fontFamily: F.body, fontSize: 10, fontWeight: 500, letterSpacing: '.1em', textTransform: 'uppercase', color: pal.subtle, marginBottom: 3 }}>{s.k}</div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 3, justifyContent: i === 0 ? 'flex-start' : i === 2 ? 'flex-end' : 'center' }}>
                <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: s.accent ? 20 : 16, color: s.accent ? pal.ink : pal.muted, fontVariantNumeric: 'tabular-nums', letterSpacing: '-0.02em' }}>{fmt(s.v)}</span>
                <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 11, color: pal.subtle }}>{unit}</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ── Three adherence stat tiles ──────────────────────────────────────
function StatTiles({ pal }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10 }}>
      {PROFILE.stats.map(s => (
        <div key={s.id} style={{
          background: pal.paper, borderRadius: 16, padding: '14px 12px',
          border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)',
          textAlign: 'center',
        }}>
          <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'center', gap: 2 }}>
            <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: 26, color: pal.ink, letterSpacing: '-0.03em', fontVariantNumeric: 'tabular-nums' }}>{s.value}</span>
            {s.unit && <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 13, color: pal.muted }}>{s.unit}</span>}
          </div>
          <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 5, lineHeight: 1.2 }}>{s.label}</div>
        </div>
      ))}
    </div>
  );
}

// ── Body metrics & progress photos ──────────────────────────────────
function BodyCard({ pal, onOpenBody }) {
  return (
    <div onClick={onOpenBody} className={onOpenBody ? 'press' : ''} style={{ background: pal.paper, borderRadius: 18, padding: 16, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)', cursor: onOpenBody ? 'pointer' : 'default' }}>
      <SectionHead pal={pal} action="Открыть →" onAction={(e) => { e.stopPropagation(); onOpenBody && onOpenBody(); }}>Тело · снимки</SectionHead>

      {/* Photo strip */}
      <div className="ds-scroll" style={{ display: 'flex', gap: 10, overflowX: 'auto', padding: '0 4px 4px', margin: '0 -4px' }}>
        {PROFILE.photos.map((p, i) => (
          <div key={i} style={{
            flexShrink: 0, width: 96, height: 132, borderRadius: 14, position: 'relative', overflow: 'hidden',
            background: `linear-gradient(165deg, ${C.sand300} 0%, ${C.linen} 70%, ${C.bone} 100%)`,
            border: `1px solid ${pal.hairline}`,
          }}>
            <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'rgba(107,74,37,.28)' }}>
              <Icon name="camera" size={26} />
            </div>
            <div style={{
              position: 'absolute', left: 0, right: 0, bottom: 0, padding: '14px 8px 7px',
              background: 'linear-gradient(180deg, rgba(26,26,26,0) 0%, rgba(26,26,26,.42) 100%)',
            }}>
              <div style={{ fontFamily: F.body, fontSize: 10, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: C.cream }}>{p.week}</div>
              <div style={{ fontFamily: F.mono, fontSize: 12, color: C.cream, fontVariantNumeric: 'tabular-nums' }}>{p.w} кг</div>
            </div>
          </div>
        ))}
        {/* Add tile */}
        <button className="press" style={{
          flexShrink: 0, width: 96, height: 132, borderRadius: 14, cursor: 'pointer',
          background: 'transparent', border: `1.5px dashed ${pal.border}`, color: pal.muted,
          display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 6,
        }}>
          <Icon name="plus" size={22} />
          <span style={{ fontFamily: F.body, fontSize: 11 }}>Снимок</span>
        </button>
      </div>

      {/* Measurements */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10, marginTop: 14 }}>
        {PROFILE.measures.map(m => (
          <div key={m.id} style={{ background: pal.sunk, borderRadius: 12, padding: '10px 12px' }}>
            <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginBottom: 4 }}>{m.label}</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 3 }}>
              <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: 17, color: pal.ink, fontVariantNumeric: 'tabular-nums' }}>{m.value}</span>
              {m.unit && <span style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle }}>{m.unit}</span>}
            </div>
            <div style={{ fontFamily: F.body, fontSize: 11, color: C.forest700, marginTop: 2 }}>{m.delta}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════════
// Settings sub-screen — real, navigable, working toggles & segments.
// ════════════════════════════════════════════════════════════════════
function ProfileSettings({ pal, platform, onBack }) {
  const [doseReminders, setDoseReminders] = React.useState(true);
  const [leadTime, setLeadTime] = React.useState('30');
  const [mealReminders, setMealReminders] = React.useState(false);
  const [units, setUnits] = React.useState('kg');
  const [timeFmt, setTimeFmt] = React.useState('24');
  const [weeklyReport, setWeeklyReport] = React.useState(true);
  const [teamMessages, setTeamMessages] = React.useState(true);
  const [reorderAlerts, setReorderAlerts] = React.useState(true);
  const [faceId, setFaceId] = React.useState(true);

  const GroupCard = ({ children }) => (
    <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)', overflow: 'hidden' }}>{children}</div>
  );
  const ToggleRow = ({ title, sub, on, onChange, last }) => (
    <div style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '14px', borderBottom: last ? 'none' : `1px solid ${pal.hairline}` }}>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>{title}</div>
        {sub && <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}>{sub}</div>}
      </div>
      <PrefToggle on={on} onChange={onChange} pal={pal} />
    </div>
  );

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden', paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 48 }}>
        {/* Top bar */}
        <div style={{ padding: '8px 16px 10px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <button onClick={onBack} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 15, fontWeight: 600, color: pal.ink }}>Настройки</div>
        </div>

        {/* Header */}
        <div style={{ padding: '6px 24px 18px' }}>
          <div style={{ fontFamily: F.display, fontSize: 34, color: pal.ink, lineHeight: 1.0, letterSpacing: '-0.018em' }}>
            Ваш <span style={{ fontStyle: 'italic', color: C.forest700 }}>ритм</span>, как удобно.
          </div>
        </div>

        {/* Reminders */}
        <div style={{ padding: '0 16px 18px' }}>
          <SectionHead pal={pal}>Напоминания</SectionHead>
          <GroupCard>
            <ToggleRow title="Напоминания о дозах" sub="Подскажем перед каждой инъекцией" on={doseReminders} onChange={setDoseReminders} />
            <div style={{ padding: '14px', borderBottom: `1px solid ${pal.hairline}`, opacity: doseReminders ? 1 : 0.4, pointerEvents: doseReminders ? 'auto' : 'none', transition: 'opacity 200ms' }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink, marginBottom: 10 }}>За сколько напоминать</div>
              <SegControl pal={pal} value={leadTime} onChange={setLeadTime} options={[
                { id: '15', label: '15 мин' }, { id: '30', label: '30 мин' }, { id: '60', label: '1 час' },
              ]} />
            </div>
            <ToggleRow title="Напоминания о приёме пищи" sub="Тихий нудж, если день без записей" on={mealReminders} onChange={setMealReminders} last />
          </GroupCard>
        </div>

        {/* Units */}
        <div style={{ padding: '0 16px 18px' }}>
          <SectionHead pal={pal}>Единицы измерения</SectionHead>
          <GroupCard>
            <div style={{ padding: '14px', borderBottom: `1px solid ${pal.hairline}` }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink, marginBottom: 10 }}>Вес</div>
              <SegControl pal={pal} value={units} onChange={setUnits} options={[
                { id: 'kg', label: 'Килограммы' }, { id: 'lb', label: 'Фунты' },
              ]} />
            </div>
            <div style={{ padding: '14px' }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink, marginBottom: 10 }}>Время</div>
              <SegControl pal={pal} value={timeFmt} onChange={setTimeFmt} options={[
                { id: '24', label: '24 часа' }, { id: '12', label: 'AM · PM' },
              ]} />
            </div>
          </GroupCard>
        </div>

        {/* Notifications */}
        <div style={{ padding: '0 16px 18px' }}>
          <SectionHead pal={pal}>Уведомления</SectionHead>
          <GroupCard>
            <ToggleRow title="Недельный отчёт" sub="Сводка трендов по воскресеньям" on={weeklyReport} onChange={setWeeklyReport} />
            <ToggleRow title="Сообщения от команды" sub="Доктор, диетолог, медсестра" on={teamMessages} onChange={setTeamMessages} />
            <ToggleRow title="Заказ флаконов" sub="Когда запас подходит к концу" on={reorderAlerts} onChange={setReorderAlerts} last />
          </GroupCard>
        </div>

        {/* Privacy */}
        <div style={{ padding: '0 16px 18px' }}>
          <SectionHead pal={pal}>Приватность и данные</SectionHead>
          <GroupCard>
            <ToggleRow title="Face ID для входа" sub="Блокировать приложение при выходе" on={faceId} onChange={setFaceId} />
            <NavRow pal={pal} iconName="document-text" iconTone="linen" title="Экспорт данных" sub="CSV · все записи курса" onClick={() => {}} />
            <NavRow pal={pal} iconName="trash" iconTone="danger" title="Удалить аккаунт" onClick={() => {}} last />
          </GroupCard>
        </div>
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════════
// Main profile screen.
// ════════════════════════════════════════════════════════════════════
function ProfileScreen({ pal, platform, onBack, onOpenChat, onOpenTrend, onOpenSchedule, onOpenJournal, onOpenBody }) {
  const [settingsOpen, setSettingsOpen] = React.useState(false);
  const gap = 10;

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden', paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 48 }}>
        {/* Top bar */}
        <div style={{ padding: '8px 16px 6px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <button onClick={onBack} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.muted }}>Профиль</div>
          <button onClick={() => setSettingsOpen(true)} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
          }}>
            <Icon name="cog" size={20} />
          </button>
        </div>

        {/* Identity */}
        <div style={{ padding: '10px 20px 18px', display: 'flex', alignItems: 'center', gap: 16 }}>
          <div style={{
            width: 68, height: 68, borderRadius: 999, background: C.forest700, color: C.cream,
            display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
            fontFamily: F.display, fontStyle: 'italic', fontSize: 32,
            boxShadow: '0 4px 14px rgba(45,95,63,.28)',
          }}>М</div>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{ fontFamily: F.display, fontSize: 30, color: pal.ink, lineHeight: 1.02, letterSpacing: '-0.018em' }}>{PROFILE.full}</div>
            <div style={{ fontFamily: F.body, fontSize: 12.5, color: pal.subtle, marginTop: 3 }}>{PROFILE.since}</div>
            <div style={{ marginTop: 8 }}>
              <Pill tone="sand" style={{ fontSize: 11 }}>Цель · {PROFILE.goalPill}</Pill>
            </div>
          </div>
        </div>

        {/* Cycle ring hero */}
        <div style={{ padding: `0 16px ${gap}px` }}>
          <CycleHero pal={pal} onOpenTrend={onOpenTrend} onOpenSchedule={onOpenSchedule} />
        </div>

        {/* Journey */}
        <div style={{ padding: `0 16px ${gap}px` }}>
          <JourneyCard pal={pal} onOpenTrend={onOpenTrend} />
        </div>

        {/* Stats */}
        <div style={{ padding: `0 16px ${gap}px` }}>
          <StatTiles pal={pal} />
        </div>

        {/* Wellbeing journal */}
        <div style={{ padding: `0 16px ${gap}px` }}>
          <button onClick={() => onOpenJournal && onOpenJournal()} className="press" style={{
            display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 14, alignItems: 'center',
            width: '100%', textAlign: 'left', cursor: 'pointer',
            background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 18, padding: 14,
            boxShadow: '0 2px 6px rgba(46,38,24,.05)',
          }}>
            <div style={{ width: 44, height: 44, borderRadius: 12, background: C.sand100, color: '#9a5a3c', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="heart" size={22} />
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>Дневник самочувствия</div>
              <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>Настроение и побочные по курсу</div>
            </div>
            <Icon name="chevron-right" size={18} color={pal.placeholder} />
          </button>
        </div>

        {/* Body & photos */}
        <div style={{ padding: `0 16px ${gap}px` }}>
          <BodyCard pal={pal} onOpenBody={onOpenBody} />
        </div>

        {/* Care team */}
        <div style={{ padding: `0 16px ${gap}px` }}>
          <SectionHead pal={pal}>Ваша команда</SectionHead>
          <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)', overflow: 'hidden' }}>
            {CARE_TEAM.map((m, i) => (
              <div key={m.id} onClick={() => onOpenChat && onOpenChat(m.id)} className="press" style={{
                display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 14, alignItems: 'center',
                padding: '12px 14px', cursor: 'pointer', borderBottom: i < CARE_TEAM.length - 1 ? `1px solid ${pal.hairline}` : 'none',
              }}>
                <div style={{
                  width: 44, height: 44, borderRadius: 999, background: m.avatarBg, color: m.avatarFg,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontFamily: F.display, fontStyle: 'italic', fontSize: 19,
                }}>{m.initial}</div>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{m.name}</div>
                  <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>{m.role}</div>
                </div>
                <Icon name="chat-bubble" size={18} color={C.forest700} />
              </div>
            ))}
          </div>
        </div>

        {/* Settings list */}
        <div style={{ padding: `0 16px ${gap}px` }}>
          <SectionHead pal={pal}>Настройки</SectionHead>
          <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)', overflow: 'hidden' }}>
            <NavRow pal={pal} iconName="bell" iconTone="forest" title="Напоминания" sub="Дозы · приёмы пищи" onClick={() => setSettingsOpen(true)} />
            <NavRow pal={pal} iconName="scale" iconTone="sand" title="Единицы измерения" trail="кг · 24 ч" onClick={() => setSettingsOpen(true)} />
            <NavRow pal={pal} iconName="chat-bubble" iconTone="linen" title="Уведомления" onClick={() => setSettingsOpen(true)} />
            <NavRow pal={pal} iconName="document-text" iconTone="linen" title="Приватность и данные" onClick={() => setSettingsOpen(true)} last />
          </div>
        </div>

        {/* Log out */}
        <div style={{ padding: `6px 16px 4px` }}>
          <button className="press" style={{
            width: '100%', padding: '14px', borderRadius: 999, cursor: 'pointer',
            background: 'transparent', border: `1px solid ${pal.border}`, color: C.danger,
            fontFamily: F.body, fontSize: 14, fontWeight: 500,
          }}>Выйти из аккаунта</button>
        </div>

        <div style={{ textAlign: 'center', fontFamily: F.mono, fontSize: 11, color: pal.placeholder, padding: '14px 0 4px', letterSpacing: '.04em' }}>
          Cadence 2.4.0
        </div>
      </div>

      {/* Settings sub-screen — slides over the profile */}
      <div style={{
        position: 'absolute', inset: 0, zIndex: 10,
        transform: settingsOpen ? 'translateX(0)' : 'translateX(100%)',
        transition: 'transform 360ms cubic-bezier(0.22, 1, 0.36, 1)',
        pointerEvents: settingsOpen ? 'auto' : 'none',
        background: pal.bg,
      }}>
        <ProfileSettings pal={pal} platform={platform} onBack={() => setSettingsOpen(false)} />
      </div>
    </div>
  );
}

Object.assign(window, { ProfileScreen });
