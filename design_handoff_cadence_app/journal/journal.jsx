// Cadence · Самочувствие (Side-effects journal)
// Two modes: Дневник (timeline, tap to expand a note) · Динамика (mood line +
// calendar heatmap with titration overlay). Plus QuickFeelSheet — a real
// "log how you feel" flow (mood / energy / sleep / tags / note).
//
// Exposes: JournalScreen, QuickFeelSheet
//   JournalScreen props: pal, platform, entries, onBack, onQuickAdd
//   QuickFeelSheet props: pal, platform, onCancel, onSave

const J = window.JOURNAL;

// ── Mood trend line (full 12-week axis, data in elapsed portion) ──
function MoodLine({ entries, pal }) {
  const W = 320, H = 150, padL = 10, padR = 12, padT = 14, padB = 22;
  const xOf = (day) => padL + (day / 84) * (W - padL - padR);
  const yOf = (m) => (H - padB) - ((m - 1) / 4) * (H - padT - padB);
  const pts = J.moodPoints(entries);
  const today = J.TODAY_DAY, titr = J.TITRATION_DAY;
  const line = pts.map((p, i) => `${i ? 'L' : 'M'}${xOf(p.day).toFixed(1)} ${yOf(p.mood).toFixed(1)}`).join(' ');

  return (
    <svg viewBox={`0 0 ${W} ${H}`} width="100%" style={{ display: 'block' }}>
      {/* future shade */}
      <rect x={xOf(today)} y={padT} width={(W - padR) - xOf(today)} height={H - padT - padB} fill={C.sand100} opacity="0.4" />
      {/* mood gridlines */}
      {[1, 2, 3, 4, 5].map(m => (
        <line key={m} x1={padL} x2={W - padR} y1={yOf(m)} y2={yOf(m)} stroke={pal.hairline} strokeWidth="1" />
      ))}
      {/* titration marker */}
      <line x1={xOf(titr)} x2={xOf(titr)} y1={padT} y2={H - padB} stroke={C.sand500} strokeWidth="1.5" strokeDasharray="3 3" />
      <circle cx={xOf(titr)} cy={padT} r="3" fill={C.sand500} />
      {/* today marker */}
      <line x1={xOf(today)} x2={xOf(today)} y1={padT} y2={H - padB} stroke={C.forest700} strokeWidth="1.5" strokeDasharray="2 3" opacity="0.6" />
      {/* line */}
      <path d={line} fill="none" stroke={C.forest700} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
      {/* dots */}
      {pts.map((p, i) => (
        <circle key={i} cx={xOf(p.day)} cy={yOf(p.mood)} r={p.source === 'dose' ? 4.5 : 3.2}
          fill={p.source === 'dose' ? C.sand500 : C.paper} stroke={C.forest700} strokeWidth="2" />
      ))}
      {/* x labels */}
      {[1, 4, 8, 12].map(w => (
        <text key={w} x={xOf((w - 1) * 7)} y={H - 6} fontSize="9" fontFamily="var(--font-mono)" fill={pal.subtle}
          textAnchor={w === 1 ? 'start' : w === 12 ? 'end' : 'middle'}>{`нед ${w}`}</text>
      ))}
    </svg>
  );
}

// ── Calendar heatmap ──────────────────────────────────────────
function Heatmap({ entries, pal }) {
  const weeks = J.heatmap(entries);
  return (
    <div>
      <div style={{ display: 'grid', gridTemplateColumns: '30px repeat(7, 1fr)', gap: 4, marginBottom: 6 }}>
        <div />
        {J.WD_FULL ? ['Пн','Вт','Ср','Чт','Пт','Сб','Вс'].map(w => (
          <div key={w} style={{ textAlign: 'center', fontFamily: F.body, fontSize: 9, color: pal.placeholder, textTransform: 'uppercase', letterSpacing: '.04em' }}>{w}</div>
        )) : null}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        {weeks.map((row, wi) => (
          <div key={wi} style={{ display: 'grid', gridTemplateColumns: '30px repeat(7, 1fr)', gap: 4, alignItems: 'center' }}>
            <div style={{ fontFamily: F.mono, fontSize: 9.5, color: pal.subtle }}>{wi + 1}</div>
            {row.map((cell, ci) => {
              if (!cell) return <div key={ci} style={{ aspectRatio: '1', borderRadius: 5, background: 'transparent' }} />;
              const mood = cell.mood ? J.MOOD[cell.mood] : null;
              const isToday = cell.rel === 'today';
              const bg = mood ? mood.color
                : cell.rel === 'future' ? 'transparent'
                : pal.sunk;
              return (
                <div key={ci} style={{
                  aspectRatio: '1', borderRadius: 5, position: 'relative',
                  background: bg,
                  opacity: cell.rel === 'future' ? 1 : (mood ? 1 : 0.7),
                  border: isToday ? `1.5px solid ${C.forest700}`
                    : cell.rel === 'future' ? `1px dashed ${pal.border}`
                    : '1.5px solid transparent',
                }}>
                  {cell.titration && (
                    <span style={{ position: 'absolute', top: -2, right: -2, width: 6, height: 6, borderRadius: 999, background: C.sand500, border: `1px solid ${pal.bg}` }} />
                  )}
                </div>
              );
            })}
          </div>
        ))}
      </div>
      {/* legend */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 12, justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
          <span style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle }}>Тяжело</span>
          {[1,2,3,4,5].map(m => <span key={m} style={{ width: 12, height: 12, borderRadius: 4, background: J.MOOD[m].color }} />)}
          <span style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle }}>Светло</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
          <span style={{ width: 7, height: 7, borderRadius: 999, background: C.sand500 }} />
          <span style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle }}>титрация</span>
        </div>
      </div>
    </div>
  );
}

// ── Timeline entry (expandable note) ──────────────────────────
function TimelineEntry({ e, pal, expanded, onToggle, last }) {
  const date = J.dateOf(e.day);
  const wd = J.WD_FULL[date.getDay()];
  const mood = J.MOOD[e.mood];
  const hasNote = !!e.note;
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '52px 1fr', gap: 12, padding: '14px 0',
      borderBottom: last ? 'none' : `1px solid ${pal.hairline}` }}>
      {/* date rail */}
      <div style={{ textAlign: 'center', paddingTop: 2 }}>
        <div style={{ fontFamily: F.mono, fontSize: 18, color: pal.ink, lineHeight: 1, fontVariantNumeric: 'tabular-nums' }}>{date.getDate()}</div>
        <div style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle, marginTop: 2 }}>{wd.slice(0, 2).toLowerCase()}</div>
        <div style={{ width: 11, height: 11, borderRadius: 999, background: mood.color, margin: '8px auto 0' }} />
      </div>
      {/* content */}
      <div onClick={hasNote ? onToggle : undefined} className={hasNote ? 'press' : ''} style={{ cursor: hasNote ? 'pointer' : 'default', minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <span style={{ fontFamily: F.display, fontSize: 19, color: pal.ink, lineHeight: 1.1, letterSpacing: '-0.012em' }}>{mood.label}</span>
          {e.source === 'dose' && (
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontFamily: F.body, fontSize: 10.5, fontWeight: 500, color: C.forest700, background: C.forest50, borderRadius: 999, padding: '2px 8px' }}>
              <Icon name="beaker" size={11} /> с дозой
            </span>
          )}
        </div>
        {/* energy + sleep */}
        <div style={{ display: 'flex', gap: 14, marginTop: 6 }}>
          <span style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>энергия <span style={{ color: pal.ink2 }}>{J.ENERGY_LABEL[e.energy]}</span></span>
          <span style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>сон <span style={{ color: pal.ink2 }}>{J.SLEEP_LABEL[e.sleep]}</span></span>
        </div>
        {/* tags */}
        {e.tags.length > 0 && (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 9 }}>
            {e.tags.map(t => (
              <span key={t} style={{ fontFamily: F.body, fontSize: 11.5, color: '#7a4a06', background: C.sand100, borderRadius: 999, padding: '3px 10px' }}>{J.tagLabel(t)}</span>
            ))}
          </div>
        )}
        {/* note */}
        {hasNote && (
          <div style={{ marginTop: 9 }}>
            <div style={{ fontFamily: F.body, fontSize: 13.5, color: pal.ink2, lineHeight: 1.5,
              display: expanded ? 'block' : '-webkit-box', WebkitLineClamp: expanded ? 'none' : 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{e.note}</div>
          </div>
        )}
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════
// Journal screen
// ════════════════════════════════════════════════════════════════
function JournalScreen({ pal, platform, entries, onBack, onQuickAdd }) {
  const [mode, setMode] = React.useState('diary');     // 'diary' | 'trends'
  const [openId, setOpenId] = React.useState(null);
  const list = J.sortedDesc(entries);
  const st = J.stats(entries);
  const tally = J.tagTally(entries);
  const todayLogged = entries.some(e => e.day === J.TODAY_DAY);
  const fmtAvg = st.avg ? st.avg.toFixed(1).replace('.', ',') : '—';

  const Seg = ({ id, label }) => {
    const on = mode === id;
    return (
      <button onClick={() => setMode(id)} className="press" style={{
        flex: 1, padding: '8px 10px', borderRadius: 999, border: 'none', cursor: 'pointer',
        background: on ? C.forest700 : 'transparent', color: on ? C.cream : pal.muted,
        fontFamily: F.body, fontSize: 13, fontWeight: 500,
        transition: 'background 180ms var(--ease-out), color 180ms var(--ease-out)',
      }}>{label}</button>
    );
  };

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      {/* Top bar */}
      <div style={{
        position: 'absolute', top: 0, left: 0, right: 0, zIndex: 20,
        paddingTop: platform === 'ios' ? 48 : 8, background: pal.bg, borderBottom: `1px solid ${pal.hairline}`,
      }}>
        <div style={{ padding: '8px 16px 6px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <button onClick={onBack} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 15, fontWeight: 600, color: pal.ink }}>Самочувствие</div>
        </div>
        <div style={{ display: 'flex', gap: 4, background: pal.sunk, padding: 3, borderRadius: 999, margin: '4px 16px 12px' }}>
          <Seg id="diary" label="Дневник" />
          <Seg id="trends" label="Динамика" />
        </div>
      </div>

      <div className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 150 : 110, paddingBottom: 96 }}>

        {mode === 'diary' ? (
          <>
            {/* Today prompt */}
            <div style={{ padding: '6px 16px 8px' }}>
              {todayLogged ? (
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 14px', background: C.forest50, borderRadius: 16 }}>
                  <Icon name="check-circle" size={20} color={C.forest700} />
                  <span style={{ fontFamily: F.body, fontSize: 13, color: C.forest800 }}>Сегодня отмечено — спасибо.</span>
                </div>
              ) : (
                <button onClick={onQuickAdd} className="press" style={{
                  display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 12, alignItems: 'center',
                  width: '100%', textAlign: 'left', cursor: 'pointer',
                  background: C.forest800, color: C.cream, border: 'none', borderRadius: 18, padding: 16,
                  boxShadow: '0 8px 24px rgba(20,44,31,.18)',
                }}>
                  <div style={{ width: 44, height: 44, borderRadius: 12, background: 'rgba(212,165,116,.2)', color: C.sand300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Icon name="heart" size={22} />
                  </div>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontFamily: F.display, fontSize: 21, lineHeight: 1.1, letterSpacing: '-0.012em' }}>Как вы себя <span style={{ fontStyle: 'italic', color: C.sand300 }}>чувствуете?</span></div>
                    <div style={{ fontFamily: F.body, fontSize: 12, color: 'rgba(246,241,234,.7)', marginTop: 2 }}>Сегодня ещё не отмечено</div>
                  </div>
                  <Icon name="plus" size={20} color={C.sand300} />
                </button>
              )}
            </div>

            {/* Timeline */}
            <div style={{ padding: '4px 20px 0' }}>
              {list.map((e, i) => (
                <TimelineEntry key={e.id} e={e} pal={pal} expanded={openId === e.id}
                  onToggle={() => setOpenId(openId === e.id ? null : e.id)} last={i === list.length - 1} />
              ))}
            </div>
          </>
        ) : (
          <>
            {/* Stat row */}
            <div style={{ padding: '6px 16px 14px' }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10 }}>
                {[
                  { v: fmtAvg, u: '', l: 'Настроение, ср.' },
                  { v: String(st.count), u: '', l: 'Записей' },
                  { v: tally[0] ? tally[0].label : '—', u: '', l: 'Частая отметка', text: true },
                ].map((s, i) => (
                  <div key={i} style={{ background: pal.paper, borderRadius: 16, padding: '13px 12px', border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)', textAlign: 'center' }}>
                    <div style={{ fontFamily: s.text ? F.display : F.mono, fontSize: s.text ? 18 : 24, fontWeight: s.text ? 400 : 500, color: pal.ink, letterSpacing: '-0.02em', lineHeight: 1.05 }}>{s.v}</div>
                    <div style={{ fontFamily: F.body, fontSize: 10.5, color: pal.subtle, marginTop: 5, lineHeight: 1.2 }}>{s.l}</div>
                  </div>
                ))}
              </div>
            </div>

            {/* Mood line */}
            <div style={{ padding: '0 16px 14px' }}>
              <div style={{ background: pal.paper, borderRadius: 18, padding: 16, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)' }}>
                <Eyebrow style={{ color: pal.subtle, marginBottom: 10 }}>Настроение по неделям</Eyebrow>
                <MoodLine entries={entries} pal={pal} />
              </div>
            </div>

            {/* Heatmap */}
            <div style={{ padding: '0 16px 14px' }}>
              <div style={{ background: pal.paper, borderRadius: 18, padding: 16, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)' }}>
                <Eyebrow style={{ color: pal.subtle, marginBottom: 12 }}>Карта дней · 12 недель</Eyebrow>
                <Heatmap entries={entries} pal={pal} />
              </div>
            </div>

            {/* Tag tally */}
            {tally.length > 0 && (
              <div style={{ padding: '0 16px 14px' }}>
                <Eyebrow style={{ color: pal.subtle, padding: '0 4px 10px' }}>Отметки за курс</Eyebrow>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {tally.map(t => (
                    <span key={t.id} style={{ display: 'inline-flex', alignItems: 'center', gap: 7, fontFamily: F.body, fontSize: 13, color: pal.ink2, background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 999, padding: '7px 13px' }}>
                      {t.label}
                      <span style={{ fontFamily: F.mono, fontSize: 12, color: C.sand700 }}>×{t.n}</span>
                    </span>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Floating quick-add (always available) */}
      <button onClick={onQuickAdd} className="press" aria-label="Отметить самочувствие" style={{
        position: 'absolute', right: 18, bottom: 28, zIndex: 30,
        height: 52, padding: '0 20px', borderRadius: 999, border: 'none', cursor: 'pointer',
        background: C.forest700, color: C.cream, display: 'flex', alignItems: 'center', gap: 8,
        boxShadow: '0 8px 20px rgba(45,95,63,.4)',
        fontFamily: F.body, fontSize: 14, fontWeight: 500,
      }}>
        <Icon name="plus" size={19} strokeWidth={2} /> Запись
      </button>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════
// Quick-add — "log how you feel"
// ════════════════════════════════════════════════════════════════
function StepDots({ value, onChange, pal, accent }) {
  return (
    <div style={{ display: 'flex', gap: 8 }}>
      {[1, 2, 3, 4, 5].map(n => {
        const on = n <= value;
        return (
          <button key={n} onClick={() => onChange(n)} className="press" aria-label={`${n}`} style={{
            flex: 1, height: 38, borderRadius: 12, border: 'none', cursor: 'pointer',
            background: on ? accent : pal.sunk,
            transition: 'background 140ms var(--ease-out)',
          }} />
        );
      })}
    </div>
  );
}

function QuickFeelSheet({ pal, platform, onCancel, onSave }) {
  const [mood, setMood] = React.useState(0);
  const [energy, setEnergy] = React.useState(3);
  const [sleep, setSleep] = React.useState(3);
  const [tags, setTags] = React.useState([]);
  const [note, setNote] = React.useState('');
  const toggleTag = (id) => setTags(t => t.includes(id) ? t.filter(x => x !== id) : [...t, id]);
  const moodMeta = mood ? J.MOOD[mood] : null;

  const Section = ({ title, children, sub }) => (
    <div style={{ marginBottom: 20 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 10 }}>
        <Eyebrow style={{ color: pal.subtle }}>{title}</Eyebrow>
        {sub && <span style={{ fontFamily: F.body, fontSize: 12, color: pal.ink2 }}>{sub}</span>}
      </div>
      {children}
    </div>
  );

  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 95 }}>
      <div className="scrim" onClick={onCancel} style={{ position: 'absolute', inset: 0, background: 'rgba(20,44,31,.35)', backdropFilter: 'blur(4px)' }} />
      <div className="sheet" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0, maxHeight: '92%',
        display: 'flex', flexDirection: 'column',
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28, boxShadow: '0 -18px 40px rgba(0,0,0,.18)',
      }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '12px auto 6px', flexShrink: 0 }} />

        {/* Header */}
        <div style={{ padding: '8px 20px 8px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0 }}>
          <div style={{ fontFamily: F.display, fontSize: 26, color: pal.ink, letterSpacing: '-0.018em' }}>
            Как вы себя <span style={{ fontStyle: 'italic', color: C.forest700 }}>чувствуете?</span>
          </div>
          <button onClick={onCancel} className="press" style={{ width: 34, height: 34, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', flexShrink: 0 }}>
            <Icon name="x-mark" size={17} />
          </button>
        </div>

        <div className="ds-scroll" style={{ overflowY: 'auto', padding: '12px 20px 8px', flex: 1 }}>
          {/* Mood */}
          <Section title="Настроение" sub={moodMeta ? moodMeta.label : ''}>
            <div style={{ display: 'flex', gap: 7 }}>
              {[1, 2, 3, 4, 5].map(m => {
                const meta = J.MOOD[m]; const on = mood === m;
                return (
                  <button key={m} onClick={() => setMood(m)} className="press" style={{
                    flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 7,
                    padding: '12px 4px', borderRadius: 14, cursor: 'pointer',
                    background: on ? meta.soft : pal.sunk,
                    border: on ? `1.5px solid ${meta.color}` : '1.5px solid transparent',
                    transition: 'background 140ms, border-color 140ms',
                  }}>
                    <span style={{ width: 22, height: 22, borderRadius: 999, background: meta.color }} />
                    <span style={{ fontFamily: F.body, fontSize: 10, fontWeight: 500, color: on ? meta.color : pal.subtle, lineHeight: 1 }}>{meta.label}</span>
                  </button>
                );
              })}
            </div>
          </Section>

          {/* Energy */}
          <Section title="Энергия" sub={J.ENERGY_LABEL[energy]}>
            <StepDots value={energy} onChange={setEnergy} pal={pal} accent={C.forest700} />
          </Section>

          {/* Sleep */}
          <Section title="Сон" sub={J.SLEEP_LABEL[sleep]}>
            <StepDots value={sleep} onChange={setSleep} pal={pal} accent={C.sand700} />
          </Section>

          {/* Tags */}
          <Section title="Что отметить">
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {J.TAGS.map(t => {
                const on = tags.includes(t.id);
                return (
                  <button key={t.id} onClick={() => toggleTag(t.id)} className="press" style={{
                    fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer',
                    padding: '8px 14px', borderRadius: 999,
                    background: on ? C.forest700 : 'transparent',
                    color: on ? C.cream : pal.ink2,
                    border: `1px solid ${on ? C.forest700 : pal.border}`,
                    transition: 'background 140ms, color 140ms',
                  }}>{t.label}</button>
                );
              })}
            </div>
          </Section>

          {/* Note */}
          <Section title="Заметка">
            <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3}
              placeholder="Пара слов о дне — по желанию"
              style={{ width: '100%', boxSizing: 'border-box', resize: 'none', border: `1px solid ${pal.border}`, borderRadius: 14, padding: '12px 14px', background: pal.paper, outline: 'none', fontFamily: F.body, fontSize: 14, color: pal.ink, lineHeight: 1.5 }} />
          </Section>
        </div>

        {/* Save */}
        <div style={{ padding: '10px 20px', paddingBottom: 28, flexShrink: 0, borderTop: `1px solid ${pal.hairline}` }}>
          <button
            onClick={() => mood && onSave({ mood, energy, sleep, tags, note: note.trim(), source: 'manual' })}
            disabled={!mood}
            className={mood ? 'press' : ''}
            style={{
              width: '100%', padding: '15px 20px', borderRadius: 999, border: 'none',
              cursor: mood ? 'pointer' : 'default', opacity: mood ? 1 : 0.4,
              background: C.forest700, color: C.cream, fontFamily: F.body, fontSize: 15, fontWeight: 500,
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            }}>
            Сохранить запись
            {mood ? <Icon name="check" size={16} strokeWidth={2} /> : null}
          </button>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { JournalScreen, QuickFeelSheet });
