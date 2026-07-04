// Cadence · Log Dose — V1 Linear wizard
// Five steps: Compound → Dose → Site → Check-in → Review → warm confirmation sheet.

function LogDoseV1({ dark, offProtocol, editPast, platform = 'ios', onCancel, onComplete }) {
  const pal = getPalette(dark);
  const [state, update] = useLogState(
    editPast ? { site: 'r-abdomen', note: 'Лёгкий укол — потом всё ровно.', sides: ['nausea'], mood: 4, time: 'Вс 06:42', date: 'Прошлая неделя' }
    : offProtocol ? { compound: null, dose: '', unit: '', vialId: null, suggested: null } : {}
  );
  const [step, setStep] = React.useState(1);
  const [confirmed, setConfirmed] = React.useState(false);
  const TOTAL = 5;

  const c = compoundById(state.compound || 'sema');

  const next = () => {
    if (step === TOTAL) {
      setConfirmed(true);
      return;
    }
    setStep(s => Math.min(TOTAL, s + 1));
  };
  const prev = () => setStep(s => Math.max(1, s - 1));
  const cancel = () => {
    if (onCancel) { onCancel(); return; }
    setStep(1);
  };
  const closeConfirm = () => {
    setConfirmed(false);
    setStep(1);
    if (onComplete) onComplete();
  };

  // Step config
  const stepDefs = [
    {
      eyebrow: 'Шаг 1 · Препарат',
      title: <>Что вы <em style={{ fontStyle: 'italic', color: C.forest700 }}>приняли</em>?</>,
      sub: editPast ? 'Правим воскресную дозу.' : 'Сегодняшняя доза уже выбрана.',
      nextDisabled: !state.compound,
      body: (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {COMPOUNDS.map(comp => {
            const sel = state.compound === comp.id;
            return (
              <button key={comp.id}
                onClick={() => update({ compound: comp.id, dose: comp.default, unit: comp.unit, vialId: VIALS.find(v => v.compound === comp.id && v.active)?.id || state.vialId })}
                className="press"
                style={{
                  padding: '14px 16px', borderRadius: 14, cursor: 'pointer', textAlign: 'left',
                  background: sel ? pal.paper : 'transparent',
                  border: `1px solid ${sel ? C.forest700 : pal.border}`,
                  display: 'grid', gridTemplateColumns: 'auto 1fr auto', gap: 12, alignItems: 'center',
                  transition: 'all 180ms var(--ease-out)',
                }}>
                <div style={{
                  width: 28, height: 28, borderRadius: 999,
                  border: `2px solid ${sel ? C.forest700 : pal.border}`,
                  background: sel ? C.forest700 : 'transparent', color: C.cream,
                  display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
                }}>
                  {sel && <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"/></svg>}
                </div>
                <div>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
                    <span style={{ fontFamily: F.body, fontWeight: 500, fontSize: 15, color: pal.ink }}>{comp.name}</span>
                    {comp.queued && (
                      <span style={{
                        fontFamily: F.body, fontSize: 10, fontWeight: 500,
                        padding: '2px 8px', borderRadius: 999,
                        background: C.sand100, color: C.sand700,
                      }}>сегодня</span>
                    )}
                  </div>
                  <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 2 }}>
                    {comp.default} {comp.unit} · {comp.mode}
                  </div>
                </div>
              </button>
            );
          })}
        </div>
      ),
    },
    {
      eyebrow: 'Шаг 2 · Доза',
      title: <>Сколько <em style={{ fontStyle: 'italic', color: C.forest700 }}>взять</em>?</>,
      sub: `По умолчанию для ${c.name} — ${c.default} ${c.unit}. Нажмите +/− для подстройки.`,
      body: (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
          {/* Dose stepper */}
          <div style={{ padding: '28px 16px 22px', background: pal.paper, borderRadius: 20, border: `1px solid ${pal.hairline}` }}>
            <DoseStepper
              value={state.dose}
              onChange={(v) => update({ dose: v })}
              step={state.unit === 'mcg' ? 25 : 0.05}
              unit={state.unit}
              pal={pal}
            />
            {/* Syringe */}
            <div style={{ marginTop: 24 }}>
              <div style={{
                fontFamily: F.body, fontSize: 11, fontWeight: 500,
                letterSpacing: '.14em', textTransform: 'uppercase', color: pal.subtle, marginBottom: 10,
              }}>На шприце 100 ед.</div>
              <SyringeBar
                fill={Math.min(100, Math.max(1, (parseFloat(state.dose || 0) / parseFloat(c.default)) * c.syringeFill))}
                max={c.syringeMax}
                pal={pal}
              />
            </div>
          </div>

          {/* Vial picker */}
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px 8px' }}>
              <span style={{
                fontFamily: F.body, fontSize: 11, fontWeight: 500,
                letterSpacing: '.14em', textTransform: 'uppercase', color: pal.subtle,
              }}>Из вашей аптечки</span>
              <button style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500 }}>Добавить</button>
            </div>
            <VialPicker state={state} update={update} pal={pal} compoundFilter={state.compound} />
          </div>
        </div>
      ),
    },
    {
      eyebrow: 'Шаг 3 · Место',
      title: <>Куда на <em style={{ fontStyle: 'italic', color: C.forest700 }}>теле</em>?</>,
      sub: 'Предложим следующую зону по ротации — выберите любую.',
      nextDisabled: !state.site,
      body: (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <BodyDiagram state={state} update={update} pal={pal} size={210} />
        </div>
      ),
    },
    {
      eyebrow: 'Шаг 4 · Контекст',
      title: <>Как вы <em style={{ fontStyle: 'italic', color: C.forest700 }}>себя чувствуете</em>?</>,
      sub: 'Короткая сверка — всё по желанию.',
      body: (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
          {/* Mood */}
          <div>
            <div style={{
              fontFamily: F.body, fontSize: 11, fontWeight: 500,
              letterSpacing: '.14em', textTransform: 'uppercase', color: pal.subtle, marginBottom: 10,
            }}>Энергия · сегодня</div>
            <MoodSlider value={state.mood} onChange={(v) => update({ mood: v })} pal={pal} />
          </div>

          {/* Side effects */}
          <div>
            <div style={{
              fontFamily: F.body, fontSize: 11, fontWeight: 500,
              letterSpacing: '.14em', textTransform: 'uppercase', color: pal.subtle, marginBottom: 10,
            }}>Что-то беспокоит?</div>
            <ChipsRow value={state.sides} onChange={(v) => update({ sides: v })} pal={pal} />
          </div>

          {/* Notes */}
          <div>
            <div style={{
              fontFamily: F.body, fontSize: 11, fontWeight: 500,
              letterSpacing: '.14em', textTransform: 'uppercase', color: pal.subtle, marginBottom: 10,
            }}>Заметки</div>
            <textarea
              value={state.note}
              onChange={(e) => update({ note: e.target.value })}
              placeholder="Что-то важное про эту дозу?"
              style={{
                width: '100%', minHeight: 70, padding: '12px 14px',
                background: pal.paper, border: `1px solid ${pal.border}`, borderRadius: 14,
                fontFamily: F.body, fontSize: 13, color: pal.ink, resize: 'none',
                outline: 'none', boxSizing: 'border-box',
              }}
            />
          </div>

          {/* Photo */}
          <PhotoSlot state={state} update={update} pal={pal} />
        </div>
      ),
    },
    {
      eyebrow: 'Шаг 5 · Проверка',
      title: <>Последний <em style={{ fontStyle: 'italic', color: C.forest700 }}>взгляд</em>.</>,
      sub: null,
      body: <ReviewSummary state={state} pal={pal} />,
    },
  ];

  const cur = stepDefs[step - 1];

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <WizardChrome
        step={step} total={TOTAL}
        onCancel={cancel} onPrev={prev} onNext={next}
        nextLabel={step === TOTAL ? 'Сохранить дозу' : 'Дальше'}
        nextDisabled={cur.nextDisabled}
        platform={platform}
        pal={pal}
        eyebrow={cur.eyebrow}
        title={cur.title}
        sub={cur.sub}
      >
        {cur.body}
      </WizardChrome>

      {/* Confirmation */}
      <WarmConfirm
        open={confirmed}
        onClose={closeConfirm}
        state={state}
        pal={pal}
        variant="sheet"
      />
    </div>
  );
}

// ─────────────────────────────────────────────────────────────
// Review summary — used in step 5
// ─────────────────────────────────────────────────────────────
function ReviewSummary({ state, pal }) {
  const c = compoundById(state.compound);
  const moodLabels = ['Никак', 'Слабо', 'Ровно', 'Хорошо', 'Светло'];
  const sideLabels = state.sides.length === 0
    ? '—'
    : state.sides.includes('none')
      ? 'Без замечаний'
      : state.sides.map(id => SIDE_EFFECTS.find(s => s.id === id)?.label).filter(Boolean).join(' · ');
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {/* Hero summary card */}
      <div style={{
        background: pal.forestBg, color: C.cream, borderRadius: 22, padding: 22,
        position: 'relative', overflow: 'hidden',
      }}>
        <div style={{
          position: 'absolute', inset: 8, borderRadius: 16,
          border: '1px solid rgba(212,165,116,.22)', pointerEvents: 'none',
        }} />
        <div style={{ position: 'relative' }}>
          <div style={{
            fontFamily: F.body, fontSize: 10, fontWeight: 500,
            letterSpacing: '.14em', textTransform: 'uppercase',
            color: C.sand300, marginBottom: 10,
          }}>{state.date} · {state.time}</div>
          <div style={{ fontFamily: F.display, fontSize: 30, color: C.cream, lineHeight: 1.04, letterSpacing: '-0.018em' }}>
            {c.name}
          </div>
          <div style={{ fontFamily: F.display, fontSize: 30, color: C.sand300, fontStyle: 'italic', lineHeight: 1.04, letterSpacing: '-0.018em' }}>
            {state.dose} {state.unit}
          </div>
          <div style={{ fontFamily: F.body, fontSize: 13, color: 'rgba(246,241,234,.7)', marginTop: 10 }}>
            {zoneLabel(state.site)}
          </div>
        </div>
      </div>

      {/* Detail rows */}
      <div style={{ background: pal.paper, borderRadius: 16, border: `1px solid ${pal.hairline}`, overflow: 'hidden' }}>
        {[
          { icon: 'beaker', label: 'Флакон', value: `${(VIALS.find(v => v.id === state.vialId) || {}).dose || '—'} · ${(VIALS.find(v => v.id === state.vialId) || {}).remaining || '—'} доз` },
          { icon: 'bolt',   label: 'Энергия', value: moodLabels[state.mood - 1] || '—' },
          { icon: 'exclamation-circle', label: 'Заметки', value: sideLabels },
          ...(state.note ? [{ icon: 'pencil', label: 'Заметка', value: state.note }] : []),
          ...(state.photo === 'attached' ? [{ icon: 'camera', label: 'Фото', value: 'Прикреплено' }] : []),
        ].map((r, i, arr) => (
          <div key={i} style={{
            display: 'grid', gridTemplateColumns: '32px 1fr', gap: 12, alignItems: 'flex-start',
            padding: '12px 14px',
            borderBottom: i < arr.length - 1 ? `1px solid ${pal.hairline}` : 'none',
          }}>
            <div style={{ width: 28, height: 28, borderRadius: 8, background: pal.sunk, color: pal.muted, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name={r.icon} size={16} />
            </div>
            <div>
              <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginBottom: 2 }}>{r.label}</div>
              <div style={{ fontFamily: F.body, fontSize: 13, color: pal.ink, lineHeight: 1.4 }}>{r.value}</div>
            </div>
          </div>
        ))}
      </div>

      <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, textAlign: 'center', padding: '6px 16px' }}>
        Нажмите <span style={{ fontStyle: 'italic' }}>Сохранить дозу</span>, чтобы подтвердить. Позже можно поправить на «Сегодня».
      </div>
    </div>
  );
}

Object.assign(window, { LogDoseV1, ReviewSummary });
