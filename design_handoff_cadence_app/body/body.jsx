// Cadence · Тело (Body metrics detail)
// Composition ring · measurements with inline sparklines (tap for a fuller chart)
// · framed progress photos with a было/стало compare · working add flows.
//
// Exposes: BodyScreen
// Props: pal, platform, state {hist, photos}, onBack, onAddMeasure(id,val), onAddPhoto(), onOpenTrend(id)

const B = window.BODY;

// ── Inline sparkline (auto-scaled) ────────────────────────────
function BodySpark({ values, color, width = 64, height = 26, dot = true }) {
  if (!values || values.length < 2) return <svg width={width} height={height} />;
  const min = Math.min(...values), max = Math.max(...values);
  const span = (max - min) || 1;
  const pad = 3;
  const xOf = (i) => pad + (i / (values.length - 1)) * (width - pad * 2);
  const yOf = (v) => (height - pad) - ((v - min) / span) * (height - pad * 2);
  const d = values.map((v, i) => `${i ? 'L' : 'M'}${xOf(i).toFixed(1)} ${yOf(v).toFixed(1)}`).join(' ');
  const lx = xOf(values.length - 1), ly = yOf(values[values.length - 1]);
  return (
    <svg width={width} height={height} style={{ display: 'block', overflow: 'visible' }}>
      <path d={d} fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      {dot && <circle cx={lx} cy={ly} r="2.6" fill={color} />}
    </svg>
  );
}

// ── Composition ring (fat vs lean) ────────────────────────────
function CompositionRing({ weight, bf, pal }) {
  const lean = B.leanOf(weight, bf);
  const size = 150, stroke = 14, r = (size - stroke) / 2, C0 = 2 * Math.PI * r;
  const frac = bf / 100;
  const fatLen = C0 * frac;
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: 18, alignItems: 'center' }}>
      <div style={{ position: 'relative', width: size, height: size, flexShrink: 0 }}>
        <svg width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
          <circle cx={size/2} cy={size/2} r={r} fill="none" stroke={C.forest700} strokeWidth={stroke} />
          <circle cx={size/2} cy={size/2} r={r} fill="none" stroke={C.sand500} strokeWidth={stroke}
            strokeDasharray={`${fatLen} ${C0 - fatLen}`} strokeDashoffset="0" strokeLinecap="butt" />
        </svg>
        <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
          <span style={{ fontFamily: F.mono, fontSize: 30, fontWeight: 500, color: pal.ink, letterSpacing: '-0.03em', lineHeight: 1 }}>{B.fmt(weight, 1)}</span>
          <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 15, color: pal.muted, marginTop: 2 }}>кг</span>
        </div>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {[
          { c: C.sand500, label: '% жира', v: B.fmt(bf, 1), u: '%' },
          { c: C.forest700, label: 'Сухая масса', v: B.fmt(lean, 1), u: 'кг' },
        ].map((row, i) => (
          <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ width: 12, height: 12, borderRadius: 4, background: row.c, flexShrink: 0 }} />
            <div style={{ minWidth: 0 }}>
              <div style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>{row.label}</div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 3 }}>
                <span style={{ fontFamily: F.mono, fontSize: 19, fontWeight: 500, color: pal.ink, letterSpacing: '-0.02em' }}>{row.v}</span>
                <span style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>{row.u}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Framed progress-photo tile ────────────────────────────────
function PhotoTile({ photo, pal, w = 96, h = 132 }) {
  return (
    <div style={{
      flexShrink: 0, width: w, height: h, borderRadius: 14, position: 'relative', overflow: 'hidden',
      background: `linear-gradient(165deg, ${C.sand300} 0%, ${C.linen} 68%, ${C.bone} 100%)`,
      border: `1px solid ${pal.hairline}`,
    }}>
      <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'rgba(107,74,37,.26)' }}>
        <Icon name="user" size={Math.round(h * 0.26)} />
      </div>
      <div style={{ position: 'absolute', left: 0, right: 0, bottom: 0, padding: '16px 9px 8px',
        background: 'linear-gradient(180deg, rgba(26,26,26,0) 0%, rgba(26,26,26,.46) 100%)' }}>
        <div style={{ fontFamily: F.body, fontSize: 10, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: C.cream }}>{photo.week}</div>
        {photo.weight != null && <div style={{ fontFamily: F.mono, fontSize: 12, color: C.cream, fontVariantNumeric: 'tabular-nums' }}>{B.fmt(photo.weight, 1)} кг</div>}
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════
// Stepper used in the add sheet
// ════════════════════════════════════════════════════════════════
function MeasureStepper({ meta, value, onChange, pal }) {
  const bump = (dir) => {
    const next = +(value + dir * meta.step).toFixed(meta.dec);
    onChange(Math.max(meta.min, Math.min(meta.max, next)));
  };
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 14, justifyContent: 'center' }}>
      <button onClick={() => bump(-1)} className="press" style={{ width: 52, height: 52, borderRadius: 999, border: `1px solid ${pal.border}`, background: pal.paper, color: pal.ink, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 24, lineHeight: 1 }}>−</button>
      <div style={{ minWidth: 132, textAlign: 'center' }}>
        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'center', gap: 4 }}>
          <span style={{ fontFamily: F.mono, fontSize: 44, fontWeight: 500, color: pal.ink, letterSpacing: '-0.03em', lineHeight: 1 }}>{B.fmt(value, meta.dec)}</span>
          {meta.unit && <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 18, color: pal.muted }}>{meta.unit}</span>}
        </div>
      </div>
      <button onClick={() => bump(1)} className="press" style={{ width: 52, height: 52, borderRadius: 999, border: 'none', background: C.forest700, color: C.cream, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 24, lineHeight: 1 }}>+</button>
    </div>
  );
}

function SheetShell({ pal, onClose, children, title }) {
  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 95 }}>
      <div className="scrim" onClick={onClose} style={{ position: 'absolute', inset: 0, background: 'rgba(20,44,31,.35)', backdropFilter: 'blur(4px)' }} />
      <div className="sheet" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0, maxHeight: '88%',
        display: 'flex', flexDirection: 'column',
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28, boxShadow: '0 -18px 40px rgba(0,0,0,.18)',
      }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '12px auto 6px', flexShrink: 0 }} />
        <div style={{ padding: '6px 20px 8px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0 }}>
          <div style={{ fontFamily: F.display, fontSize: 26, color: pal.ink, letterSpacing: '-0.018em' }}>{title}</div>
          <button onClick={onClose} className="press" style={{ width: 34, height: 34, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' }}>
            <Icon name="x-mark" size={17} />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

// ── Add measurement (and quick weight) ────────────────────────
function AddMeasureSheet({ pal, state, presetId, onClose, onSave }) {
  const editable = B.METRICS.filter(m => m.editable);
  const [id, setId] = React.useState(presetId || 'weight');
  const meta = B.metricMeta(id);
  const [val, setVal] = React.useState(() => B.latest(B.histOf(state, id)));
  const pick = (nid) => { setId(nid); setVal(B.latest(B.histOf(state, nid))); };

  return (
    <SheetShell pal={pal} onClose={onClose} title={<>Добавить <span style={{ fontStyle: 'italic', color: C.forest700 }}>замер</span></>}>
      <div className="ds-scroll" style={{ overflowY: 'auto', padding: '14px 20px 8px', flex: 1 }}>
        <Eyebrow style={{ color: pal.subtle, marginBottom: 10 }}>Что измеряем</Eyebrow>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 26 }}>
          {editable.map(m => {
            const on = m.id === id;
            return (
              <button key={m.id} onClick={() => pick(m.id)} className="press" style={{
                fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer',
                padding: '8px 14px', borderRadius: 999,
                background: on ? C.forest700 : 'transparent', color: on ? C.cream : pal.ink2,
                border: `1px solid ${on ? C.forest700 : pal.border}`,
              }}>{m.label}</button>
            );
          })}
        </div>
        <div style={{ paddingBottom: 8 }}>
          <MeasureStepper meta={meta} value={val} onChange={setVal} pal={pal} />
        </div>
        <div style={{ textAlign: 'center', fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 14 }}>
          Сейчас в журнале · {B.fmt(B.latest(B.histOf(state, id)), meta.dec)} {meta.unit}
        </div>
      </div>
      <div style={{ padding: '10px 20px', paddingBottom: 28, flexShrink: 0, borderTop: `1px solid ${pal.hairline}` }}>
        <button onClick={() => onSave(id, val)} className="press" style={{
          width: '100%', padding: '15px 20px', borderRadius: 999, border: 'none', cursor: 'pointer',
          background: C.forest700, color: C.cream, fontFamily: F.body, fontSize: 15, fontWeight: 500,
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
        }}>
          Сохранить <Icon name="check" size={16} strokeWidth={2} />
        </button>
      </div>
    </SheetShell>
  );
}

// ── Add photo ─────────────────────────────────────────────────
function AddPhotoSheet({ pal, onClose, onCapture }) {
  return (
    <SheetShell pal={pal} onClose={onClose} title={<>Новый <span style={{ fontStyle: 'italic', color: C.forest700 }}>снимок</span></>}>
      <div style={{ padding: '14px 20px 8px' }}>
        <div style={{
          width: '100%', aspectRatio: '3 / 4', maxHeight: 320, borderRadius: 18, margin: '0 auto',
          background: `linear-gradient(165deg, ${C.sand300} 0%, ${C.linen} 70%, ${C.bone} 100%)`,
          border: `1px dashed ${pal.border}`, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 10, color: '#9a5a3c',
        }}>
          <Icon name="camera" size={40} />
          <div style={{ fontFamily: F.body, fontSize: 13, color: '#7a4a06' }}>Анфас · ровный свет</div>
        </div>
        <div style={{ fontFamily: F.body, fontSize: 12.5, color: pal.subtle, textAlign: 'center', marginTop: 14, lineHeight: 1.5 }}>
          Снимок добавится к этой неделе и встанет рядом с прошлыми.
        </div>
      </div>
      <div style={{ padding: '12px 20px', paddingBottom: 28, flexShrink: 0 }}>
        <button onClick={onCapture} className="press" style={{
          width: '100%', padding: '15px 20px', borderRadius: 999, border: 'none', cursor: 'pointer',
          background: C.forest700, color: C.cream, fontFamily: F.body, fontSize: 15, fontWeight: 500,
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
        }}>
          <Icon name="camera" size={17} /> Сделать снимок
        </button>
      </div>
    </SheetShell>
  );
}

// ── Measurement detail sheet (fuller chart + history) ─────────
function MeasureDetailSheet({ pal, state, metricId, onClose, onUpdate, onOpenTrend }) {
  if (!metricId) return null;
  const meta = B.metricMeta(metricId);
  const hist = B.histOf(state, metricId);
  const d = B.delta(hist);
  const W = 300, H = 90, min = Math.min(...hist), max = Math.max(...hist), span = (max - min) || 1, pad = 8;
  const xOf = (i) => pad + (i / (hist.length - 1)) * (W - pad * 2);
  const yOf = (v) => (H - pad) - ((v - min) / span) * (H - pad * 2);
  const path = hist.map((v, i) => `${i ? 'L' : 'M'}${xOf(i).toFixed(1)} ${yOf(v).toFixed(1)}`).join(' ');

  return (
    <SheetShell pal={pal} onClose={onClose} title={meta.label}>
      <div className="ds-scroll" style={{ overflowY: 'auto', padding: '8px 20px 8px', flex: 1 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, marginBottom: 14 }}>
          <span style={{ fontFamily: F.mono, fontSize: 40, fontWeight: 500, color: pal.ink, letterSpacing: '-0.03em', lineHeight: 1 }}>{B.fmt(B.latest(hist), meta.dec)}</span>
          {meta.unit && <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 18, color: pal.muted }}>{meta.unit}</span>}
          <span style={{ marginLeft: 'auto' }}><Pill tone={d.down ? 'forest' : 'neutral'} style={{ fontSize: 11 }}>{B.fmtDelta(hist, meta.dec, meta.unit)} за курс</Pill></span>
        </div>
        <div style={{ background: pal.paper, borderRadius: 16, border: `1px solid ${pal.hairline}`, padding: 14, marginBottom: 14 }}>
          <svg viewBox={`0 0 ${W} ${H}`} width="100%" style={{ display: 'block' }}>
            <path d={path} fill="none" stroke={C.forest700} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
            {hist.map((v, i) => <circle key={i} cx={xOf(i)} cy={yOf(v)} r={i === hist.length - 1 ? 4 : 2.6} fill={i === hist.length - 1 ? C.sand500 : C.paper} stroke={C.forest700} strokeWidth="2" />)}
          </svg>
        </div>
        {/* history rows */}
        <div style={{ background: pal.paper, borderRadius: 16, border: `1px solid ${pal.hairline}`, overflow: 'hidden' }}>
          {hist.map((v, i) => (
            <div key={i} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '11px 14px', borderBottom: i < hist.length - 1 ? `1px solid ${pal.hairline}` : 'none' }}>
              <span style={{ fontFamily: F.body, fontSize: 13, color: pal.muted }}>{B.WEEK_LABELS[i] || `Точка ${i + 1}`}</span>
              <span style={{ fontFamily: F.mono, fontSize: 14, color: pal.ink, fontVariantNumeric: 'tabular-nums' }}>{B.fmt(v, meta.dec)} {meta.unit}</span>
            </div>
          ))}
        </div>
      </div>
      <div style={{ padding: '10px 20px', paddingBottom: 28, flexShrink: 0, borderTop: `1px solid ${pal.hairline}`, display: 'flex', gap: 10 }}>
        {meta.trendId && (
          <button onClick={() => onOpenTrend(meta.trendId)} className="press" style={{
            flex: 1, padding: '13px 16px', borderRadius: 999, cursor: 'pointer',
            background: 'transparent', border: `1px solid ${pal.border}`, color: pal.ink2,
            fontFamily: F.body, fontSize: 14, fontWeight: 500, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
          }}>Открыть тренд <Icon name="arrow-right" size={15} /></button>
        )}
        <button onClick={() => onUpdate(metricId)} className="press" style={{
          flex: 1, padding: '13px 16px', borderRadius: 999, border: 'none', cursor: 'pointer',
          background: C.forest700, color: C.cream, fontFamily: F.body, fontSize: 14, fontWeight: 500,
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
        }}>Обновить</button>
      </div>
    </SheetShell>
  );
}

// ════════════════════════════════════════════════════════════════
// Body screen
// ════════════════════════════════════════════════════════════════
function BodyScreen({ pal, platform, state, onBack, onAddMeasure, onAddPhoto, onOpenTrend }) {
  const [addPreset, setAddPreset] = React.useState(null);   // metric id or null
  const [addOpen, setAddOpen] = React.useState(false);
  const [detail, setDetail] = React.useState(null);         // metric id
  const [photoOpen, setPhotoOpen] = React.useState(false);

  const weight = B.latest(state.hist.weight);
  const bf = B.latest(state.hist.bodyfat);
  const toTarget = weight - B.TARGET.weight;
  const photos = state.photos;
  const firstPhoto = photos[0], lastPhoto = photos[photos.length - 1];
  const wDelta = (lastPhoto.weight != null && firstPhoto.weight != null) ? (lastPhoto.weight - firstPhoto.weight) : null;

  const openAdd = (preset) => { setAddPreset(preset || null); setAddOpen(true); };
  const measureRows = ['waist', 'hip', 'chest', 'bmi'];

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      {/* Top bar */}
      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 20, paddingTop: platform === 'ios' ? 48 : 8, background: pal.bg, borderBottom: `1px solid ${pal.hairline}` }}>
        <div style={{ padding: '8px 16px 10px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <button onClick={onBack} className="press" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 15, fontWeight: 600, color: pal.ink }}>Тело</div>
        </div>
      </div>

      <div className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden', paddingTop: platform === 'ios' ? 104 : 64, paddingBottom: 96 }}>
        {/* Composition */}
        <div style={{ padding: '6px 16px 14px' }}>
          <div style={{ background: pal.paper, borderRadius: 18, padding: 18, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 16 }}>
              <Eyebrow style={{ color: pal.subtle }}>Состав тела</Eyebrow>
              <button onClick={() => onOpenTrend('weight')} style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                Вес · тренд <Icon name="arrow-right" size={13} />
              </button>
            </div>
            <CompositionRing weight={weight} bf={bf} pal={pal} />
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 16, paddingTop: 14, borderTop: `1px solid ${pal.hairline}` }}>
              <Icon name="scale" size={16} color={pal.subtle} />
              <span style={{ fontFamily: F.body, fontSize: 12.5, color: pal.muted }}>
                Цель <span style={{ fontFamily: F.mono, color: pal.ink2 }}>{B.fmt(B.TARGET.weight, 1)} кг</span> · осталось <span style={{ fontFamily: F.mono, color: C.forest700 }}>{B.fmt(toTarget, 1)} кг</span>
              </span>
              <button onClick={() => openAdd('weight')} className="press" style={{ marginLeft: 'auto', padding: '8px 14px', borderRadius: 999, border: `1px solid ${pal.border}`, background: 'transparent', color: pal.ink2, fontFamily: F.body, fontSize: 12.5, fontWeight: 500, cursor: 'pointer' }}>
                Обновить вес
              </button>
            </div>
          </div>
        </div>

        {/* Measurements */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px 10px' }}>
            <Eyebrow style={{ color: pal.subtle }}>Замеры</Eyebrow>
            <button onClick={() => openAdd(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
              <Icon name="plus" size={13} strokeWidth={2} /> Добавить
            </button>
          </div>
          <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, overflow: 'hidden' }}>
            {measureRows.map((mid, i) => {
              const meta = B.metricMeta(mid);
              const hist = B.histOf(state, mid);
              const d = B.delta(hist);
              return (
                <div key={mid} onClick={() => setDetail(mid)} className="press" style={{
                  display: 'grid', gridTemplateColumns: '1fr auto auto', gap: 14, alignItems: 'center',
                  padding: '13px 14px', cursor: 'pointer',
                  borderBottom: i < measureRows.length - 1 ? `1px solid ${pal.hairline}` : 'none',
                }}>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>{meta.label}</div>
                    <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginTop: 2 }}>
                      <span style={{ fontFamily: F.mono, fontSize: 18, fontWeight: 500, color: pal.ink, letterSpacing: '-0.02em' }}>{B.fmt(B.latest(hist), meta.dec)}</span>
                      {meta.unit && <span style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>{meta.unit}</span>}
                      <span style={{ fontFamily: F.body, fontSize: 11.5, color: d.down ? C.forest700 : pal.subtle, marginLeft: 4 }}>{B.fmtDelta(hist, meta.dec)}</span>
                    </div>
                  </div>
                  <BodySpark values={hist} color={C.forest700} width={60} height={26} />
                  <Icon name="chevron-right" size={16} color={pal.placeholder} />
                </div>
              );
            })}
          </div>
        </div>

        {/* Before / after */}
        {wDelta != null && (
          <div style={{ padding: '0 16px 14px' }}>
            <Eyebrow style={{ color: pal.subtle, padding: '0 4px 10px' }}>До и после</Eyebrow>
            <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, padding: 16, boxShadow: '0 2px 6px rgba(46,38,24,.05)' }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', gap: 12, alignItems: 'center' }}>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
                  <PhotoTile photo={firstPhoto} pal={pal} w={104} h={142} />
                  <span style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>было · {firstPhoto.week}</span>
                </div>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 30, color: C.forest700, lineHeight: 1 }}>−{B.fmt(Math.abs(wDelta), 1)}</div>
                  <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 2 }}>кг</div>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
                  <PhotoTile photo={lastPhoto} pal={pal} w={104} h={142} />
                  <span style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>стало · {lastPhoto.week}</span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Gallery */}
        <div style={{ paddingBottom: 8 }}>
          <div style={{ padding: '0 20px 10px' }}>
            <Eyebrow style={{ color: pal.subtle }}>Все снимки</Eyebrow>
          </div>
          <div className="ds-scroll" style={{ display: 'flex', gap: 10, overflowX: 'auto', padding: '0 16px 4px' }}>
            {photos.map(p => <PhotoTile key={p.id} photo={p} pal={pal} />)}
            <button onClick={() => setPhotoOpen(true)} className="press" style={{
              flexShrink: 0, width: 96, height: 132, borderRadius: 14, cursor: 'pointer',
              background: 'transparent', border: `1.5px dashed ${pal.border}`, color: pal.muted,
              display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 6,
            }}>
              <Icon name="camera" size={22} />
              <span style={{ fontFamily: F.body, fontSize: 11 }}>Снимок</span>
            </button>
          </div>
        </div>
      </div>

      {/* Sheets */}
      {addOpen && (
        <AddMeasureSheet pal={pal} state={state} presetId={addPreset}
          onClose={() => setAddOpen(false)}
          onSave={(id, val) => { onAddMeasure(id, val); setAddOpen(false); }} />
      )}
      {photoOpen && (
        <AddPhotoSheet pal={pal} onClose={() => setPhotoOpen(false)}
          onCapture={() => { onAddPhoto(); setPhotoOpen(false); }} />
      )}
      <MeasureDetailSheet pal={pal} state={state} metricId={detail}
        onClose={() => setDetail(null)}
        onUpdate={(id) => { setDetail(null); openAdd(id); }}
        onOpenTrend={(tid) => { setDetail(null); onOpenTrend(tid); }} />
    </div>
  );
}

Object.assign(window, { BodyScreen });
