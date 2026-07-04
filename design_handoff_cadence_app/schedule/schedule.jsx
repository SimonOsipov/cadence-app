// Cadence · График (Injection schedule) — calendar screen
// Vertically-scrolling month grids across the 12-week cycle, dose dots per day,
// a titration callout, and a tappable day-detail sheet that hands off to Log dose.
//
// Exposes: ScheduleScreen
// Props: pal, platform, doseLogged, todayMeals, todayKcal, onBack, onLogDose

const S = window.SCHED;

// ── Status pill copy + tone ───────────────────────────────────
function statusMeta(status) {
  return {
    done:      { label: 'Записано',      tone: 'forest' },
    pending:   { label: 'Ждёт',          tone: 'sand'   },
    scheduled: { label: 'Запланировано', tone: 'neutral'},
    skipped:   { label: 'Пропущено',     tone: 'danger' },
    logged:    { label: 'Записано',      tone: 'forest' },
    open:      { label: 'Открыто',       tone: 'sand'   },
  }[status] || { label: '', tone: 'neutral' };
}

// ── One day cell in the month grid ────────────────────────────
function DayCell({ date, pal, ctx, onPick }) {
  if (!date) return <div style={{ aspectRatio: '1 / 1.05' }} />;
  const inCyc = S.inCycle(date);
  const isToday = S.sameDay(date, S.TODAY);
  const info = inCyc ? S.dotsForDate(date, ctx) : null;
  const r = S.rel(date);
  const dim = !inCyc;

  const ringColor = isToday ? C.forest700 : (info && info.step ? C.sand500 : 'transparent');
  const anchorBg = info && info.injection && !isToday
    ? (info.step ? 'rgba(212,165,116,.16)' : 'rgba(45,95,63,.07)')
    : 'transparent';

  return (
    <button
      onClick={() => inCyc && onPick(date)}
      className={inCyc ? 'press' : ''}
      style={{
        aspectRatio: '1 / 1.05', border: 'none', background: 'transparent',
        cursor: inCyc ? 'pointer' : 'default', padding: 0,
        display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4,
        opacity: dim ? 0.32 : 1,
      }}
    >
      <div style={{
        width: 34, height: 34, borderRadius: 999, marginTop: 3,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        background: isToday ? C.forest700 : anchorBg,
        border: !isToday && ringColor !== 'transparent' ? `1.5px solid ${ringColor}` : '1.5px solid transparent',
        boxShadow: isToday ? '0 2px 8px rgba(45,95,63,.35)' : 'none',
      }}>
        <span style={{
          fontFamily: F.mono, fontSize: 14, fontVariantNumeric: 'tabular-nums',
          fontWeight: info && info.injection ? 600 : 400,
          color: isToday ? C.cream : (info && info.injection ? pal.ink : pal.muted),
          letterSpacing: '-0.02em',
        }}>{date.getDate()}</span>
      </div>
      {/* dots */}
      <div style={{ display: 'flex', gap: 3, height: 6, alignItems: 'center' }}>
        {info && info.cats.slice(0, 4).map(c => (
          <span key={c} style={{
            width: 5, height: 5, borderRadius: 999, background: S.CATS[c].dot,
            opacity: r === 'past' && c !== 'meal' ? 0.5 : 1,
          }} />
        ))}
      </div>
    </button>
  );
}

// ── A single month block ──────────────────────────────────────
function MonthBlock({ m, pal, ctx, onPick, todayRef }) {
  return (
    <div style={{ padding: '0 16px 18px' }} ref={m.isTodayMonth ? todayRef : null}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, padding: '4px 4px 8px' }}>
        <span style={{ fontFamily: F.display, fontSize: 24, color: pal.ink, letterSpacing: '-0.018em' }}>{m.label}</span>
        <span style={{ fontFamily: F.mono, fontSize: 12, color: pal.placeholder }}>{m.year}</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', columnGap: 0, rowGap: 2 }}>
        {m.cells.map((d, i) => (
          <DayCell key={i} date={d} pal={pal} ctx={ctx} onPick={onPick} />
        ))}
      </div>
    </div>
  );
}

// ── Day-detail bottom sheet ───────────────────────────────────
function DaySheet({ date, pal, ctx, onClose, onLogDose }) {
  if (!date) return null;
  const evs = S.eventsForDate(date, ctx);
  const week = S.weekOfCycle(date);
  const step = S.isTitrationDay(date);
  const r = S.rel(date);
  const wd = S.WD_FULL[date.getDay()];
  const pendingSema = evs.find(e => e.id === 'sema' && e.status === 'pending');

  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 90 }}>
      <div className="scrim" onClick={onClose} style={{
        position: 'absolute', inset: 0, background: 'rgba(20,44,31,.35)', backdropFilter: 'blur(4px)',
      }} />
      <div className="sheet" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0, maxHeight: '82%',
        display: 'flex', flexDirection: 'column',
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28,
        boxShadow: '0 -18px 40px rgba(0,0,0,.18)',
      }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '12px auto 8px', flexShrink: 0 }} />

        {/* Header */}
        <div style={{ padding: '6px 20px 14px', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12 }}>
          <div style={{ minWidth: 0 }}>
            <Eyebrow style={{ color: pal.subtle, marginBottom: 6 }}>{r === 'today' ? 'Сегодня' : r === 'past' ? 'Прошедший день' : 'Впереди'}</Eyebrow>
            <div style={{ fontFamily: F.display, fontSize: 27, color: pal.ink, lineHeight: 1.04, letterSpacing: '-0.018em' }}>
              {wd}, <span style={{ fontStyle: 'italic', color: C.forest700 }}>{S.longDate(date)}</span>
            </div>
            <div style={{ marginTop: 8 }}><Pill tone="neutral" style={{ fontSize: 11 }}>{week}-я неделя курса</Pill></div>
          </div>
          <button onClick={onClose} className="press" style={{
            width: 36, height: 36, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', flexShrink: 0,
          }}>
            <Icon name="x-mark" size={18} />
          </button>
        </div>

        {/* Titration callout */}
        {step && (
          <div style={{ padding: '0 16px 14px' }}>
            <div style={{
              display: 'grid', gridTemplateColumns: '40px 1fr', gap: 12, alignItems: 'center',
              background: C.sand100, border: '1px solid rgba(184,137,90,.28)', borderRadius: 16, padding: 14,
            }}>
              <div style={{ width: 40, height: 40, borderRadius: 12, background: C.sand500, color: C.ink900, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Icon name="arrow-trending-up" size={20} />
              </div>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: '#6b4a25' }}>Шаг титрации</div>
                <div style={{ fontFamily: F.body, fontSize: 12, color: '#6b4a25', opacity: 0.78, marginTop: 1 }}>
                  Доза растёт: {step.from} мг → <span style={{ fontWeight: 600 }}>{step.to} мг</span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Events */}
        <div className="ds-scroll" style={{ overflowY: 'auto', padding: '0 16px 8px', flex: 1 }}>
          <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, overflow: 'hidden' }}>
            {evs.map((e, i) => {
              const cat = S.CATS[e.cat];
              const sm = statusMeta(e.status);
              const tappable = e.loggable && (e.status === 'pending' || e.status === 'scheduled') && r !== 'past';
              return (
                <div key={e.id} onClick={tappable ? onLogDose : undefined}
                  className={tappable ? 'press' : ''}
                  style={{
                    display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 13, alignItems: 'center',
                    padding: '13px 14px', cursor: tappable ? 'pointer' : 'default',
                    borderBottom: i < evs.length - 1 ? `1px solid ${pal.hairline}` : 'none',
                  }}>
                  <div style={{ width: 44, height: 44, borderRadius: 12, background: cat.soft, color: cat.fg, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Icon name={cat.icon} size={21} />
                  </div>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'baseline', gap: 7, flexWrap: 'wrap' }}>
                      <span style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14.5, color: pal.ink }}>{e.title}</span>
                      {e.dose && <span style={{ fontFamily: F.mono, fontSize: 12.5, color: pal.ink2, fontVariantNumeric: 'tabular-nums' }}>{e.dose}</span>}
                    </div>
                    <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}>
                      {e.time ? <span style={{ fontFamily: F.mono, color: pal.muted }}>{e.time}</span> : null}
                      {e.time && e.sub ? ' · ' : ''}{e.sub}
                    </div>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    {sm.label && <Pill tone={sm.tone} style={{ fontSize: 10 }}>{sm.label}</Pill>}
                    {tappable && <Icon name="chevron-right" size={15} color={pal.placeholder} />}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Primary CTA — today with a queued dose */}
        <div style={{ padding: '12px 16px', paddingBottom: 28, flexShrink: 0 }}>
          {pendingSema ? (
            <button onClick={onLogDose} className="press" style={{
              width: '100%', padding: '15px 20px', borderRadius: 999, border: 'none', cursor: 'pointer',
              background: C.forest700, color: C.cream, fontFamily: F.body, fontSize: 15, fontWeight: 500,
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            }}>
              Записать дозу
              <Icon name="arrow-right" size={16} />
            </button>
          ) : (
            <button onClick={onClose} className="press" style={{
              width: '100%', padding: '14px 20px', borderRadius: 999, cursor: 'pointer',
              background: 'transparent', border: `1px solid ${pal.border}`, color: pal.muted,
              fontFamily: F.body, fontSize: 14, fontWeight: 500,
            }}>
              Закрыть
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════
// Main schedule screen
// ════════════════════════════════════════════════════════════════
function ScheduleScreen({ pal, platform, doseLogged, todayMeals, todayKcal, onBack, onLogDose }) {
  const ctx = { doseLogged, todayMeals, todayKcal };
  const [pick, setPick] = React.useState(null);
  const scrollRef = React.useRef(null);
  const todayRef = React.useRef(null);

  const months = React.useMemo(() => S.monthsInCycle(), []);
  const week = S.weekOfCycle(S.TODAY);
  const pct = Math.max(0, Math.min(1, (week - 1 + 0.5) / S.CYCLE_WEEKS));
  const titr = S.nextTitration();

  const jumpToToday = () => {
    const el = todayRef.current, box = scrollRef.current;
    if (el && box) box.scrollTo({ top: el.offsetTop - 120, behavior: 'smooth' });
  };
  React.useEffect(() => {
    const el = todayRef.current, box = scrollRef.current;
    if (el && box) box.scrollTop = Math.max(0, el.offsetTop - 120);
  }, []);

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      {/* Top bar */}
      <div style={{
        position: 'absolute', top: 0, left: 0, right: 0, zIndex: 20,
        paddingTop: platform === 'ios' ? 48 : 8,
        background: pal.bg, borderBottom: `1px solid ${pal.hairline}`,
      }}>
        <div style={{ padding: '8px 16px 10px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <button onClick={onBack} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 15, fontWeight: 600, color: pal.ink }}>График</div>
          <button onClick={jumpToToday} className="press" style={{
            height: 40, padding: '0 14px', borderRadius: 999, border: `1px solid ${pal.border}`, background: 'transparent',
            color: C.forest700, fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer',
          }}>
            Сегодня
          </button>
        </div>

        {/* Weekday header */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', padding: '2px 16px 8px' }}>
          {S.WD_SHORT.map((w, i) => (
            <div key={w} style={{
              textAlign: 'center', fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.06em',
              color: i === 6 ? C.forest700 : pal.subtle, textTransform: 'uppercase',
            }}>{w}</div>
          ))}
        </div>
      </div>

      {/* Scroll body */}
      <div ref={scrollRef} className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 138 : 98, paddingBottom: 40,
      }}>
        {/* Cycle band + titration callout */}
        <div style={{ padding: '6px 16px 16px' }}>
          <div style={{ background: C.forest800, color: C.cream, borderRadius: 22, padding: 18, boxShadow: '0 8px 24px rgba(20,44,31,.18)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 12 }}>
              <div style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em', textTransform: 'uppercase', color: C.sand300 }}>Текущий курс</div>
              <span style={{ fontFamily: F.mono, fontSize: 11, color: 'rgba(246,241,234,.6)' }}>неделя {week} из {S.CYCLE_WEEKS}</span>
            </div>
            <div style={{ fontFamily: F.display, fontSize: 26, lineHeight: 1.06, letterSpacing: '-0.018em', marginBottom: 14 }}>
              Семаглутид · <span style={{ fontStyle: 'italic', color: C.sand300 }}>{S.semaDose(week).value} мг</span> еженедельно
            </div>
            {/* progress track */}
            <div style={{ position: 'relative', height: 6, borderRadius: 999, background: 'rgba(246,241,234,.16)', marginBottom: 8 }}>
              <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${pct * 100}%`, borderRadius: 999, background: C.sand500 }} />
              <div style={{ position: 'absolute', top: '50%', left: `${pct * 100}%`, transform: 'translate(-50%,-50%)', width: 12, height: 12, borderRadius: 999, background: C.cream, border: `2px solid ${C.forest800}` }} />
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontFamily: F.body, fontSize: 10.5, color: 'rgba(246,241,234,.55)' }}>
              <span>10 мая · старт</span>
              <span>26 июля · финиш</span>
            </div>

            {titr && (
              <div style={{ marginTop: 16, paddingTop: 14, borderTop: '1px solid rgba(246,241,234,.14)', display: 'flex', alignItems: 'center', gap: 12 }}>
                <div style={{ width: 38, height: 38, borderRadius: 11, background: 'rgba(212,165,116,.2)', color: C.sand300, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  <Icon name="arrow-trending-up" size={19} />
                </div>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontFamily: F.body, fontSize: 13, color: C.cream }}>
                    Следующий шаг — <span style={{ fontWeight: 600, color: C.sand300 }}>{titr.to} мг</span> с {titr.week}-й недели
                  </div>
                  <div style={{ fontFamily: F.body, fontSize: 11.5, color: 'rgba(246,241,234,.6)', marginTop: 1 }}>
                    Через {S.daysBetween(S.TODAY, titr.date)} дн. · {S.longDate(titr.date)}
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Legend */}
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px', padding: '0 20px 16px' }}>
          {['inj', 'supp', 'weigh', 'meal'].map(c => (
            <div key={c} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span style={{ width: 7, height: 7, borderRadius: 999, background: S.CATS[c].dot }} />
              <span style={{ fontFamily: F.body, fontSize: 11.5, color: pal.muted }}>{S.CATS[c].label}</span>
            </div>
          ))}
        </div>

        {/* Months */}
        {months.map(m => (
          <MonthBlock key={m.key} m={m} pal={pal} ctx={ctx} onPick={setPick} todayRef={todayRef} />
        ))}

        <div style={{ textAlign: 'center', fontFamily: F.mono, fontSize: 11, color: pal.placeholder, padding: '6px 0 8px', letterSpacing: '.04em' }}>
          Курс · 12 недель
        </div>
      </div>

      <DaySheet
        date={pick}
        pal={pal}
        ctx={ctx}
        onClose={() => setPick(null)}
        onLogDose={() => { setPick(null); onLogDose && onLogDose(); }}
      />
    </div>
  );
}

Object.assign(window, { ScheduleScreen });
