// Cadence Dashboard — V1 Refined Today (mid-cycle, balanced)
// Forest hero, two-up biomarkers, schedule, coach, fuel strip.
// Interactivity: log-dose, time-of-day chips, biomarker sheet, pull-to-refresh, tab nav.

function V1Refined({ dark, doseLogged, setDoseLogged, coachIndex, platform, onLogDose, onPlusTap, onOpenTrends, onOpenTrend, onOpenVials, onOpenChat, onOpenProfile, onOpenSchedule, onOpenLearn, onOpenJournal, onQuickFeel, doseJustLogged,
                    meals, mealTotals, mealCoach, onLogMeal, onOpenNutrition, mealHeroSuggestion,
                    reorderHint, onOpenRecipes }) {
  const pal = getPalette(dark);
  const coach = useCoach(coachIndex);
  const [tod, setTod] = React.useState('morning');
  const [tab, setTab] = React.useState('today');
  const [sheet, setSheet] = React.useState(null);

  // When wired into a prototype, the hero + the "+" tab open the Log Dose wizard.
  // Stand-alone (canvas previews), they fall back to toggling local state.
  const heroAction = () => {
    if (onLogDose && !doseLogged) { onLogDose(); return; }
    setDoseLogged(!doseLogged);
  };
  const handleTab = (id) => {
    if (id === 'log' && onPlusTap) { onPlusTap(); return; }
    if (id === 'insights' && onOpenTrends) { onOpenTrends(); return; }
    if (id === 'inventory' && onOpenVials) { onOpenVials(); return; }
    if (id === 'nutrition' && onOpenNutrition) { onOpenNutrition(); return; }
    setTab(id);
  };

  const BiomCard = ({ eyebrow, value, unit, delta, sparkData, onClick }) => (
    <div onClick={onClick} className="press" style={{
      background: pal.paper, borderRadius: 18, padding: 14, cursor: 'pointer',
      boxShadow: '0 2px 6px rgba(46,38,24,.05)',
      border: `1px solid ${pal.hairline}`,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
        <Eyebrow style={{ color: pal.subtle }}>{eyebrow}</Eyebrow>
        {sparkData && <Spark data={sparkData} color={dark ? '#a6c2af' : C.forest700} width={50} height={20} />}
      </div>
      <Num value={value} unit={unit} size={28} color={pal.ink} unitColor={pal.muted} />
      <div style={{ marginTop: 8 }}>
        <Pill tone="forest" style={{ fontSize: 10, background: pal.forestPill.bg, color: pal.forestPill.fg }}>{delta}</Pill>
      </div>
    </div>
  );

  const NavBg = platform === 'android' ? null : <CadenceTabBar active={tab} onChange={handleTab} pal={pal} />;
  const navBg2 = platform === 'android' ? <CadenceAndroidNav active={tab} onChange={handleTab} pal={pal} /> : null;

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <PullToRefresh pal={pal}>
        <div style={{ paddingBottom: 120, paddingTop: platform === 'ios' ? 48 : 8 }}>
          {/* Header */}
          <div style={{ padding: '12px 20px 14px' }}>
            <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginBottom: 8 }}>Воскресенье, утро · 4-я неделя</div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 12 }}>
              <div style={{ fontFamily: F.display, fontSize: 32, color: pal.ink, lineHeight: 1.05, letterSpacing: '-0.018em', minWidth: 0, flex: 1 }}>Марина</div>
            <div style={{ display: 'flex', gap: 8 }}>
              <button className="press" onClick={onOpenChat || (() => {})} style={{
                width: 40, height: 40, borderRadius: 999, border: 'none', cursor: 'pointer',
                background: C.forest50, color: C.forest700,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                position: 'relative',
              }}>
                <Icon name="chat-bubble" size={20} />
                {/* Tiny presence dot to hint "doctor is online" */}
                <span style={{
                  position: 'absolute', top: 8, right: 9,
                  width: 7, height: 7, borderRadius: 999,
                  background: C.forest700,
                  border: `1.5px solid ${C.forest50}`,
                }} />
              </button>
              <button className="press" onClick={onOpenSchedule || (() => {})} aria-label="График" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', cursor: 'pointer', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Icon name="calendar" size={20} />
              </button>
              <button className="press" onClick={onOpenLearn || (() => {})} aria-label="Знания" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', cursor: 'pointer', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Icon name="book-open" size={20} />
              </button>
              <button className="press" onClick={onOpenProfile || (() => {})} aria-label="Профиль" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', cursor: 'pointer', background: C.forest700, color: C.cream, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: F.display, fontStyle: 'italic', fontSize: 18 }}>М</button>
              </div>
            </div>
          </div>

          {/* (Removed: TimeOfDayChips + MealTimeline — user prefers a quieter top.) */}

          {/* Hero dose card */}
          <div style={{ padding: '0 16px 14px' }}>
            <div style={{
              background: pal.forestBg, color: pal.forestFg, borderRadius: 24, padding: 22,
              boxShadow: dark ? 'none' : '0 8px 24px rgba(20,44,31,.18)',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 12 }}>
                <Pill tone="dark" style={{ background: 'rgba(246,241,234,.12)', color: C.sand300 }}>
                  {tod === 'morning' ? 'Сегодня утром' : tod === 'afternoon' ? 'Записано ранее' : 'Сегодня вечером'}
                </Pill>
                <span style={{ fontFamily: F.mono, fontSize: 11, color: 'rgba(246,241,234,.6)' }}>
                  {TIMES.find(t => t.id === tod).sub.replace(' am','').replace(' pm','')}
                </span>
              </div>
              <div style={{ fontFamily: F.display, fontSize: 36, color: C.cream, lineHeight: 1.04, letterSpacing: '-0.018em', marginBottom: 10 }}>
                Семаглутид<br/>
                <span style={{ fontStyle: 'italic', color: C.sand300 }}>0,25 мг</span>
              </div>
              <div style={{ fontFamily: F.body, fontSize: 13, color: 'rgba(246,241,234,.75)', lineHeight: 1.5, marginBottom: 16 }}>
                {doseLogged
                  ? 'Записано · правый живот, ротация.'
                  : 'Недельная инъекция, запланирована на сегодня.'}
              </div>
              <button
                className="press"
                onClick={heroAction}
                style={{
                  display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                  width: '100%', padding: '14px 20px', borderRadius: 999,
                  background: doseLogged ? 'rgba(246,241,234,.12)' : C.sand500,
                  color: doseLogged ? C.cream : C.ink900,
                  border: 'none', fontFamily: F.body, fontSize: 14, fontWeight: 500, cursor: 'pointer',
                }}>
                {doseLogged ? <>
                  <svg className="tick" width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                  Открыть детали
                </> : <>Записать →</>}
              </button>
              {doseLogged && (
                <button onClick={onQuickFeel || (() => {})} className="press" style={{
                  display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7,
                  width: '100%', marginTop: 10, padding: '11px 16px', borderRadius: 999,
                  background: 'transparent', border: '1px solid rgba(246,241,234,.2)', color: C.sand300,
                  fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer',
                }}>
                  <Icon name="heart" size={15} /> Как перенесли дозу? Отметить
                </button>
              )}
            </div>
          </div>

          {/* Headline biomarker — full width with chart on the right half */}
          <div style={{ padding: '0 16px 14px' }}>
            <div
              className="press"
              onClick={() => setSheet({
                trendId: 'weight',
                eyebrow: 'Вес · за неделю', title: 'Вы плавно идёте вниз.',
                value: '110,0', unit: 'кг', delta: '↓ 0,6 кг', trend: 'down',
                series: [0.7,0.65,0.6,0.55,0.5,0.48,0.4],
                note: 'Килограмм в неделю — золотая середина: достаточно быстро, чтобы заметить, и медленно, чтобы удержать. Если ускорится — подскажем.',
              })}
              style={{
                background: pal.paper, borderRadius: 18, padding: 16, cursor: 'pointer',
                boxShadow: '0 2px 6px rgba(46,38,24,.05)',
                border: `1px solid ${pal.hairline}`,
                display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14, alignItems: 'center',
              }}
            >
              <div>
                <Eyebrow style={{ color: pal.subtle, marginBottom: 10 }}>Вес · 7 дней</Eyebrow>
                <Num value="110,0" unit="кг" size={34} color={pal.ink} unitColor={pal.muted} />
                <div style={{ marginTop: 10 }}>
                  <Pill tone="forest" style={{ fontSize: 11, background: pal.forestPill.bg, color: pal.forestPill.fg }}>↓ 0,6 кг</Pill>
                </div>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'stretch', gap: 4 }}>
                <Spark
                  data={[0.7,0.65,0.6,0.55,0.5,0.48,0.4]}
                  color={dark ? '#a6c2af' : C.forest700}
                  fill={dark ? 'rgba(166,194,175,0.2)' : C.forest50}
                  width={150}
                  height={70}
                />
                <div style={{
                  display: 'flex', justifyContent: 'space-between',
                  fontFamily: F.mono, fontSize: 10, color: pal.subtle,
                  fontVariantNumeric: 'tabular-nums',
                  paddingTop: 2,
                }}>
                  <span>Пн</span><span>Ср</span><span>Пт</span><span>Сег</span>
                </div>
              </div>
            </div>
          </div>

          {/* Wellbeing nudge — opens the side-effects journal */}
          <div style={{ padding: '0 16px 14px' }}>
            <button onClick={onOpenJournal || (() => {})} className="press" style={{
              display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 12, alignItems: 'center',
              width: '100%', textAlign: 'left', cursor: 'pointer',
              background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 18, padding: 14,
              boxShadow: '0 2px 6px rgba(46,38,24,.05)',
            }}>
              <div style={{ width: 44, height: 44, borderRadius: 12, background: C.sand100, color: '#9a5a3c', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Icon name="heart" size={22} />
              </div>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontFamily: F.display, fontSize: 20, color: pal.ink, lineHeight: 1.12, letterSpacing: '-0.012em' }}>Как вы себя чувствуете?</div>
                <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}>Дневник самочувствия · сегодня не отмечено</div>
              </div>
              <Icon name="chevron-right" size={18} color={pal.subtle} />
            </button>
          </div>

          {/* Reorder reminder — surfaced when inventory has a low-stock compound */}
          {reorderHint && (
            <div style={{ padding: '0 16px 14px' }}>
              <button
                onClick={onOpenVials}
                className="press"
                style={{
                  display: 'grid', gridTemplateColumns: '40px 1fr auto',
                  gap: 12, alignItems: 'center',
                  width: '100%', textAlign: 'left',
                  background: '#fbeed1', border: '1px solid rgba(194,120,10,.18)',
                  borderRadius: 16, padding: 14, cursor: 'pointer',
                }}>
                <div style={{
                  width: 40, height: 40, borderRadius: 12,
                  background: 'rgba(194,120,10,.18)', color: '#c2780a',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <Icon name="information-circle" size={20} />
                </div>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: '#7a4a06' }}>
                    {reorderHint.meta.name} закончится через ~{reorderHint.weeksLeft} {reorderHint.weeksLeft === 1 ? 'неделю' : reorderHint.weeksLeft < 5 ? 'недели' : 'недель'}
                  </div>
                  <div style={{ fontFamily: F.body, fontSize: 11.5, color: '#7a4a06', opacity: 0.7, marginTop: 1 }}>
                    Запасного флакона нет
                  </div>
                </div>
                <div style={{
                  display: 'inline-flex', alignItems: 'center', gap: 4,
                  fontFamily: F.body, fontSize: 12, fontWeight: 500, color: '#c2780a',
                }}>
                  В аптечку
                  <Icon name="arrow-right" size={14} />
                </div>
              </button>
            </div>
          )}

          {/* This week's protocol */}
          <div style={{ padding: '0 16px 14px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px 10px' }}>
              <Eyebrow style={{ color: pal.subtle }}>Протокол этой недели</Eyebrow>
              <button onClick={onOpenSchedule || (() => {})} style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 12, color: pal.forestBg === C.forest800 ? C.forest700 : C.sand500, fontWeight: 500 }}>Весь график</button>
            </div>
            <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}` }}>
              {[
                { icon: 'check-circle', tone: { bg: pal.forestPill.bg, fg: pal.forestPill.fg }, title: 'Семаглутид · 0,25 мг', sub: 'Воскресенье 07:00 · еженедельно', trail: 'Сегодня', trailSub: doseLogged ? 'записано' : 'ждёт' },
                { icon: 'beaker',       tone: { bg: dark ? 'rgba(232,212,184,.14)' : C.sand100, fg: dark ? C.sand300 : '#6b4a25' }, title: 'BPC-157 · 250 мкг', sub: 'Ежедневно · подкожно', trail: '2× в день', trailSub: 'восстановление' },
                { icon: 'moon',         tone: { bg: pal.sunk, fg: pal.ink2 }, title: 'Глицин + магний', sub: 'На ночь · за 30 мин до сна', trail: '21:30', trailSub: 'вечером' },
              ].map((r, i, arr) => (
                <React.Fragment key={i}>
                  <div style={{ display: 'grid', gridTemplateColumns: '40px 1fr auto', gap: 14, alignItems: 'center', padding: '12px 14px' }}>
                    <div style={{ width: 40, height: 40, borderRadius: 12, background: r.tone.bg, color: r.tone.fg, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <Icon name={r.icon} size={20} />
                    </div>
                    <div>
                      <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>{r.title}</div>
                      <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>{r.sub}</div>
                    </div>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{ fontFamily: F.mono, fontSize: 13, color: pal.ink2 }}>{r.trail}</div>
                      <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>{r.trailSub}</div>
                    </div>
                  </div>
                  {i < arr.length - 1 && <div style={{ height: 1, background: pal.hairline, marginLeft: 68 }} />}
                </React.Fragment>
              ))}
            </div>
          </div>

          {/* Sand meal hero — second floating hero, calling for the next meal */}
          {meals && (
            <div style={{ padding: '0 16px 14px' }}>
              <MealHero
                pal={pal}
                meals={meals}
                totals={mealTotals}
                now={mealHeroSuggestion?.now || '08:42 am'}
                suggestion={mealHeroSuggestion}
                onLogMeal={onLogMeal}
                onOpenRecipes={onOpenRecipes}
              />
            </div>
          )}

          {/* Today's meals card — replaces the old fuel strip */}
          {meals && (
            <div style={{ padding: '0 16px 14px' }}>
              <TodayMeals
                pal={pal}
                meals={meals}
                totals={mealTotals}
                onLogMeal={onLogMeal}
                onOpenNutrition={onOpenNutrition}
              />
            </div>
          )}

          {/* (Removed: Coach card on Today — user prefers a quieter feed.) */}
        </div>
      </PullToRefresh>

      {NavBg}
      {navBg2}

      <BiomarkerSheet open={!!sheet} onClose={() => setSheet(null)} biomarker={sheet} pal={pal} onOpenTrend={onOpenTrend} />
    </div>
  );
}

Object.assign(window, { V1Refined });
