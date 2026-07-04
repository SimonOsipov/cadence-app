// Cadence — Doctor dashboard · main page composition

// ── Sidebar ────────────────────────────────────────────────────────────────
function Sidebar({ nav, setNav, stats }) {
  const items = [
    { id: 'overview', icon: 'home',        label: 'Обзор' },
    { id: 'patients', icon: 'user',        label: 'Пациенты', count: stats.total },
    { id: 'schedule', icon: 'calendar',    label: 'Расписание' },
    { id: 'messages', icon: 'chat-bubble', label: 'Сообщения', badge: stats.unread },
    { id: 'analytics', icon: 'chart-bar',  label: 'Аналитика' },
    { id: 'library',  icon: 'book-open',   label: 'Протоколы' },
  ];
  return (
    <aside style={{
      width: 248, flexShrink: 0, height: '100vh', position: 'sticky', top: 0,
      background: C.forest900, color: C.cream,
      display: 'flex', flexDirection: 'column', padding: '22px 16px',
    }}>
      {/* brand */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 11, padding: '4px 8px 22px' }}>
        <svg width="34" height="34" viewBox="0 0 64 64" fill="none">
          <circle cx="32" cy="32" r="30" fill="#2d5f3f" />
          <path d="M14 38 C 20 24, 28 24, 32 32 C 36 40, 44 40, 50 26" stroke="#f6f1ea" strokeWidth="3.5" strokeLinecap="round" fill="none" />
          <circle cx="14" cy="38" r="2.6" fill="#d4a574" />
          <circle cx="50" cy="26" r="2.6" fill="#d4a574" />
        </svg>
        <div>
          <div style={{ fontFamily: F.display, fontSize: 22, lineHeight: 1, color: C.cream }}>Cadence</div>
          <div style={{ fontFamily: F.body, fontSize: 10.5, letterSpacing: '.14em', textTransform: 'uppercase', color: 'rgba(246,241,234,.5)', marginTop: 2 }}>для врача</div>
        </div>
      </div>

      {/* nav */}
      <nav style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {items.map(it => {
          const on = nav === it.id;
          return (
            <button key={it.id} onClick={() => setNav(it.id)} style={{
              display: 'flex', alignItems: 'center', gap: 12, width: '100%',
              padding: '10px 12px', borderRadius: 12, cursor: 'pointer', textAlign: 'left',
              background: on ? 'rgba(212,165,116,.16)' : 'transparent',
              color: on ? C.cream : 'rgba(246,241,234,.7)',
              border: 'none', fontFamily: F.body, fontSize: 14, fontWeight: on ? 600 : 400,
              transition: 'background 140ms',
            }}
            onMouseEnter={e => { if (!on) e.currentTarget.style.background = 'rgba(246,241,234,.06)'; }}
            onMouseLeave={e => { if (!on) e.currentTarget.style.background = 'transparent'; }}>
              <Icon name={it.icon} size={19} strokeWidth={on ? 1.9 : 1.6} color={on ? C.sand500 : 'currentColor'} />
              <span style={{ flex: 1 }}>{it.label}</span>
              {it.badge > 0 && (
                <span style={{ fontFamily: F.mono, fontSize: 11, fontWeight: 500, minWidth: 20, height: 20, padding: '0 6px', borderRadius: 999, background: C.danger, color: C.cream, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>{it.badge}</span>
              )}
              {it.count != null && (
                <span style={{ fontFamily: F.mono, fontSize: 12, color: 'rgba(246,241,234,.45)' }}>{it.count}</span>
              )}
            </button>
          );
        })}
      </nav>

      <div style={{ flex: 1 }} />

      {/* attention nudge */}
      <div style={{ background: 'rgba(184,80,60,.16)', border: '1px solid rgba(184,80,60,.35)', borderRadius: 14, padding: '13px 14px', marginBottom: 14 }}>
        <div style={{ fontFamily: F.body, fontSize: 12.5, color: C.cream, lineHeight: 1.45 }}>
          <span style={{ fontWeight: 600 }}>{stats.attention} пациента</span> ждут вашего внимания сегодня.
        </div>
      </div>

      {/* doctor */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 11, padding: '10px 8px', borderTop: '1px solid rgba(246,241,234,.1)' }}>
        <div style={{ width: 38, height: 38, borderRadius: 999, background: C.sand500, color: C.ink900, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: F.display, fontStyle: 'italic', fontSize: 18, flexShrink: 0 }}>{DOCTOR.initial}</div>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ fontFamily: F.body, fontSize: 13, fontWeight: 600, color: C.cream, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{DOCTOR.name}</div>
          <div style={{ fontFamily: F.body, fontSize: 11.5, color: 'rgba(246,241,234,.55)' }}>{DOCTOR.role}</div>
        </div>
        <button style={{ background: 'none', border: 'none', color: 'rgba(246,241,234,.6)', cursor: 'pointer', padding: 4 }}>
          <Icon name="cog" size={18} />
        </button>
      </div>
    </aside>
  );
}

// ── Top bar ──────────────────────────────────────────────────────────────
function TopBar({ query, setQuery }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', gap: 24, marginBottom: 28 }}>
      <div>
        <div style={{ fontFamily: F.body, fontSize: 12.5, fontWeight: 500, letterSpacing: '.14em', textTransform: 'uppercase', color: C.ink500, marginBottom: 8 }}>Четверг, 29 мая · утро</div>
        <h1 style={{ fontFamily: F.display, fontWeight: 400, fontSize: 44, lineHeight: 1, letterSpacing: '-0.02em', color: C.ink900, margin: 0 }}>
          Доброе утро, <span style={{ fontStyle: 'italic' }}>Ксения</span>
        </h1>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 9, background: C.paper, border: '1px solid ' + C.bone, borderRadius: 999, padding: '10px 16px', width: 230 }}>
          <Icon name="magnifying-glass" size={17} color={C.ink400} />
          <input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Поиск пациента"
            style={{ border: 'none', outline: 'none', background: 'transparent', fontFamily: F.body, fontSize: 14, color: C.ink900, width: '100%' }}
          />
        </div>
        <button style={{
          display: 'inline-flex', alignItems: 'center', gap: 8, fontFamily: F.body, fontWeight: 500, fontSize: 14,
          padding: '11px 18px', borderRadius: 999, background: C.forest700, color: C.cream, border: 'none', cursor: 'pointer',
          boxShadow: '0 2px 8px rgba(45,95,63,.25)',
        }}>
          <Icon name="plus" size={17} strokeWidth={2} />Новый пациент
        </button>
      </div>
    </div>
  );
}

// ── Section heading ─────────────────────────────────────────────────────
function SectionHead({ eyebrow, title, action, onAction }) {
  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', marginBottom: 14 }}>
      <div>
        {eyebrow && <div style={{ fontFamily: F.body, fontSize: 11.5, fontWeight: 500, letterSpacing: '.14em', textTransform: 'uppercase', color: C.ink500, marginBottom: 5 }}>{eyebrow}</div>}
        <h2 style={{ fontFamily: F.display, fontWeight: 400, fontSize: 27, lineHeight: 1.05, letterSpacing: '-0.01em', color: C.ink900, margin: 0 }}>{title}</h2>
      </div>
      {action && (
        <button onClick={onAction} style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: C.forest700, display: 'inline-flex', alignItems: 'center', gap: 5 }}>
          {action}<Icon name="arrow-right" size={15} />
        </button>
      )}
    </div>
  );
}

// ── Quick-peek drawer ──────────────────────────────────────────────────────
function PatientDrawer({ pt, onClose, onMessage }) {
  if (!pt) return null;
  const sm = statusMeta(pt.status);
  const lostKg = lost(pt);
  const pct = lostPct(pt);
  const sparkColor = pt.status === 'attention' ? C.danger : pt.status === 'watch' ? C.warning : C.forest700;
  const M = pt.metrics;
  return (
    <div onClick={onClose} className="scrim" style={{
      position: 'fixed', inset: 0, zIndex: 80, background: 'rgba(20,44,31,.35)', backdropFilter: 'blur(4px)',
      display: 'flex', justifyContent: 'flex-end',
    }}>
      <div onClick={e => e.stopPropagation()} className="drawer" style={{
        width: 460, maxWidth: '92vw', height: '100%', background: C.cream, overflowY: 'auto',
        boxShadow: '-18px 0 50px rgba(20,44,31,.22)', padding: '26px 28px 40px',
      }}>
        {/* header */}
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 22 }}>
          <div style={{ display: 'flex', gap: 15, alignItems: 'center' }}>
            <Avatar pt={pt} size={58} ring={sm.dot} />
            <div>
              <div style={{ fontFamily: F.display, fontSize: 27, lineHeight: 1.05, color: C.ink900 }}>{pt.name}</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 5 }}>
                <StatusTag status={pt.status} />
                <span style={{ fontFamily: F.body, fontSize: 12.5, color: C.ink500 }}>· {pt.age} лет</span>
              </div>
            </div>
          </div>
          <button onClick={onClose} style={{ background: C.linen, border: 'none', borderRadius: 999, width: 38, height: 38, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', color: C.ink700 }}>
            <Icon name="x-mark" size={18} />
          </button>
        </div>

        {/* flags */}
        {pt.flags.length > 0 && (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 7, marginBottom: 18 }}>
            {pt.flags.map(f => <FlagPill key={f} kind={f} />)}
          </div>
        )}
        {pt.note && (
          <div style={{ background: C.paper, border: '1px solid ' + C.bone, borderRadius: 14, padding: '13px 15px', marginBottom: 20, fontFamily: F.body, fontSize: 13.5, lineHeight: 1.5, color: C.ink700 }}>{pt.note}</div>
        )}

        {/* protocol card */}
        <div style={{ background: C.forest800, color: C.cream, borderRadius: 18, padding: 20, marginBottom: 18 }}>
          <div style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em', textTransform: 'uppercase', color: 'rgba(246,241,234,.6)', marginBottom: 8 }}>Текущий протокол</div>
          <div style={{ fontFamily: F.display, fontSize: 30, lineHeight: 1 }}>{pt.compound}</div>
          <div style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 24, color: C.sand300, marginTop: 2 }}>{pt.dose}</div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 16 }}>
            <span style={{ fontFamily: F.body, fontSize: 13, color: 'rgba(246,241,234,.75)' }}>{pt.cadence}</span>
            <span style={{ fontFamily: F.mono, fontSize: 13, color: C.cream }}>неделя {pt.week} из {pt.cycleLen}</span>
          </div>
          <div style={{ height: 5, borderRadius: 999, background: 'rgba(246,241,234,.18)', marginTop: 10, overflow: 'hidden' }}>
            <div style={{ height: '100%', width: (pt.week / pt.cycleLen * 100) + '%', background: C.sand500, borderRadius: 999 }} />
          </div>
        </div>

        {/* weight progress */}
        <div style={{ background: C.paper, border: '1px solid ' + C.bone, borderRadius: 18, padding: 18, marginBottom: 18 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 6 }}>
            <div>
              <div style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em', textTransform: 'uppercase', color: C.ink500, marginBottom: 6 }}>Вес</div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 5 }}>
                <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: 34, letterSpacing: '-0.03em', color: C.ink900, fontVariantNumeric: 'tabular-nums' }}>{pt.weight.toFixed(1).replace('.', ',')}</span>
                <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 16, color: C.ink600 }}>{pt.unit}</span>
              </div>
              <div style={{ fontFamily: F.body, fontSize: 12.5, color: C.forest600, marginTop: 4 }}>↓ {lostKg.toFixed(1).replace('.', ',')} кг с начала · цель {pt.goal} кг</div>
            </div>
            <DeskSpark data={pt.spark} color={sparkColor} width={120} height={48} />
          </div>
          <div style={{ height: 6, borderRadius: 999, background: C.bone, marginTop: 12, overflow: 'hidden' }}>
            <div style={{ height: '100%', width: Math.min(pct, 100) + '%', background: sparkColor, borderRadius: 999 }} />
          </div>
          <div style={{ fontFamily: F.body, fontSize: 11.5, color: C.ink500, marginTop: 6 }}>{pct}% пути к цели</div>
        </div>

        {/* biomarkers */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 12, marginBottom: 22 }}>
          {[
            { k: 'HRV', v: M.hrv, u: 'мс', good: M.hrv >= 58 },
            { k: 'ЧСС покоя', v: M.rhr, u: 'уд/мин', good: M.rhr <= 60 },
            { k: 'Сон', v: M.sleep, u: '/100', good: M.sleep >= 75 },
          ].map(m => (
            <div key={m.k} style={{ background: C.paper, border: '1px solid ' + C.bone, borderRadius: 14, padding: '13px 14px' }}>
              <div style={{ fontFamily: F.body, fontSize: 10.5, fontWeight: 500, letterSpacing: '.1em', textTransform: 'uppercase', color: C.ink500, marginBottom: 7, whiteSpace: 'nowrap' }}>{m.k}</div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 3 }}>
                <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: 21, color: m.good ? C.ink900 : C.warning, fontVariantNumeric: 'tabular-nums' }}>{m.v}</span>
                <span style={{ fontFamily: F.body, fontSize: 11, color: C.ink500 }}>{m.u}</span>
              </div>
            </div>
          ))}
        </div>

        {/* actions */}
        <div style={{ display: 'flex', gap: 10 }}>
          <button style={{ flex: 1, fontFamily: F.body, fontWeight: 500, fontSize: 14, padding: '13px 0', borderRadius: 999, background: C.forest700, color: C.cream, border: 'none', cursor: 'pointer', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 8 }}>
            <Icon name="document-text" size={16} />Полная карта
          </button>
          <button onClick={() => { onMessage && onMessage(pt); onClose(); }} style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, padding: '13px 20px', borderRadius: 999, background: C.linen, color: C.ink900, border: '1px solid ' + C.bone, cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 8 }}>
            <Icon name="paper-airplane" size={16} />Написать
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Main page ──────────────────────────────────────────────────────────────
function Dashboard() {
  const [nav, setNav] = React.useState('overview');
  const [filter, setFilter] = React.useState('all');
  const [query, setQuery] = React.useState('');
  const [selected, setSelected] = React.useState(null);
  const [sort, setSort] = React.useState('status');
  const [threadFor, setThreadFor] = React.useState(null);

  const openThread = (pt) => { setThreadFor(pt && pt.id ? pt.id : pt); setNav('messages'); };

  const attention = PATIENTS.filter(p => p.status === 'attention');

  const FILTERS = [
    { id: 'all', label: 'Все', n: PATIENTS.length },
    { id: 'attention', label: 'Внимание', n: PATIENTS.filter(p => p.status === 'attention').length },
    { id: 'watch', label: 'Наблюдение', n: PATIENTS.filter(p => p.status === 'watch').length },
    { id: 'track', label: 'В норме', n: PATIENTS.filter(p => p.status === 'track').length },
  ];

  const statusRank = { attention: 0, watch: 1, track: 2 };
  let roster = PATIENTS.filter(p => {
    if (filter !== 'all' && p.status !== filter) return false;
    if (query.trim()) {
      const q = query.trim().toLowerCase();
      return p.name.toLowerCase().includes(q) || p.compound.toLowerCase().includes(q);
    }
    return true;
  });
  roster = [...roster].sort((a, b) => {
    if (sort === 'status') return statusRank[a.status] - statusRank[b.status] || b.adherence - a.adherence;
    if (sort === 'adherence') return a.adherence - b.adherence;
    if (sort === 'name') return a.name.localeCompare(b.name, 'ru');
    return 0;
  });

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: C.cream }}>
      <Sidebar nav={nav} setNav={setNav} stats={STATS} />

      {nav === 'messages' ? (
        <Messages initialThreadId={threadFor} onOpenPatient={setSelected} />
      ) : (
      <main style={{ flex: 1, minWidth: 0, padding: '34px 40px 60px', maxWidth: 1320 }}>
        <TopBar query={query} setQuery={setQuery} />

        {/* stat strip */}
        <div className="stat-strip" style={{ marginBottom: 36 }}>
          <StatCard label="Пациентов" value={STATS.total} sub="активных протоколов" icon="user" />
          <StatCard label="Внимание" value={STATS.attention} sub="пациента ожидают" tone="forest" icon="exclamation-circle" />
          <StatCard label="Дозы сегодня" value={`${STATS.dosesDone}/${STATS.dosesToday}`} sub="выполнено" icon="beaker" accent={C.forest600} />
          <StatCard label="Сообщения" value={STATS.unread} sub="без ответа" icon="chat-bubble" accent={C.info} />
          <StatCard label="Регулярность" value={STATS.avgAdh} unit="%" sub="в среднем" icon="check-circle" accent={C.forest600} />
        </div>

        {/* attention queue */}
        <section style={{ marginBottom: 40 }}>
          <SectionHead eyebrow="Триаж" title="Требуют внимания" action="Все флаги" />
          <div className="triage-grid">
            {attention.map(pt => <AttentionCard key={pt.id} pt={pt} onOpen={setSelected} onMessage={openThread} />)}
          </div>
        </section>

        {/* roster */}
        <section style={{ marginBottom: 40 }}>
          <SectionHead eyebrow="Все пациенты" title="Журнал протоколов" />
            {/* controls */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 14, flexWrap: 'wrap' }}>
              <div style={{ display: 'flex', gap: 7, flexWrap: 'wrap' }}>
                {FILTERS.map(f => {
                  const on = filter === f.id;
                  return (
                    <button key={f.id} onClick={() => setFilter(f.id)} style={{
                      fontFamily: F.body, fontSize: 13, fontWeight: 500, padding: '7px 14px', borderRadius: 999, cursor: 'pointer',
                      background: on ? C.ink900 : 'transparent', color: on ? C.cream : C.ink700,
                      border: '1px solid ' + (on ? C.ink900 : C.border), transition: 'all 140ms',
                      display: 'inline-flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
                    }}>
                      {f.label}
                      <span style={{ fontFamily: F.mono, fontSize: 11, color: on ? 'rgba(246,241,234,.7)' : C.ink400 }}>{f.n}</span>
                    </button>
                  );
                })}
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                <span style={{ fontFamily: F.body, fontSize: 12.5, color: C.ink500 }}>Сортировка</span>
                <select value={sort} onChange={e => setSort(e.target.value)} style={{
                  fontFamily: F.body, fontSize: 13, color: C.ink800, padding: '7px 12px', borderRadius: 999,
                  border: '1px solid ' + C.border, background: C.paper, cursor: 'pointer', outline: 'none',
                }}>
                  <option value="status">по статусу</option>
                  <option value="adherence">по регулярности</option>
                  <option value="name">по имени</option>
                </select>
              </div>
            </div>

            {/* table */}
            <div className="roster-scroll" style={{ background: C.paper, border: '1px solid ' + C.bone, borderRadius: 18, boxShadow: '0 2px 6px rgba(46,38,24,.05)' }}>
              <div style={{ minWidth: 560 }}>
                {/* head */}
                <div style={{
                  display: 'grid', gridTemplateColumns: '1.7fr 1.25fr 0.8fr 1.15fr 0.5fr', gap: 16,
                  padding: '12px 18px', borderBottom: '1px solid ' + C.bone, background: C.cream,
                  borderTopLeftRadius: 18, borderTopRightRadius: 18,
                }}>
                  {['Пациент', 'Протокол', 'Цикл', 'Вес', ''].map((h, i) => (
                    <div key={i} style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.1em', textTransform: 'uppercase', color: C.ink500 }}>{h}</div>
                  ))}
                </div>
                {roster.length === 0 ? (
                  <div style={{ padding: '44px 18px', textAlign: 'center', fontFamily: F.body, fontSize: 14, color: C.ink500 }}>
                    Никого не нашлось — попробуйте другой запрос.
                  </div>
                ) : roster.map((pt, i) => (
                  <RosterRow key={pt.id} pt={pt} onOpen={setSelected} last={i === roster.length - 1} />
                ))}
              </div>
            </div>
        </section>

        {/* today's schedule — full width below the roster */}
        <section>
          <SectionHead eyebrow="Сегодня" title="Расписание" action="Открыть календарь" />
          <div style={{ background: C.paper, border: '1px solid ' + C.bone, borderRadius: 18, padding: '20px 24px', boxShadow: '0 2px 6px rgba(46,38,24,.05)' }}>
            <div className="schedule-grid">
              {SCHEDULE.map(s => <ScheduleItem key={s.id} item={s} pt={patientById(s.patientId)} onOpen={setSelected} />)}
            </div>
          </div>
        </section>
      </main>
      )}

      {selected && <PatientDrawer pt={selected} onClose={() => setSelected(null)} onMessage={openThread} />}
    </div>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<Dashboard />);
