// Cadence — Meal-log modal.
// Single screen. Top: segmented mode (Chat / Photo / Voice). Middle: input,
// then parsed-items list once the "AI" returns. Footer: Save.
// Exposes: MealLogScreen

function MealLogScreen({ dark, platform, defaultMode, onCancel, onComplete }) {
  const pal = getPalette(dark);
  const [mode, setMode] = React.useState(defaultMode || 'chat');
  // stage: 'idle' (input awaiting) | 'capturing' (recording/holding) | 'parsing' | 'parsed'
  const [stage, setStage] = React.useState('idle');
  const [transcript, setTranscript] = React.useState('');
  const [items, setItems] = React.useState([]);
  const [mealName, setMealName] = React.useState('');
  const [editingGramsId, setEditingGramsId] = React.useState(null);
  const [sampleIdx, setSampleIdx] = React.useState(0);
  const [chatText, setChatText] = React.useState('');

  // Reset on mode change
  const switchMode = (m) => {
    if (m === mode) return;
    setMode(m);
    setStage('idle');
    setTranscript('');
    setItems([]);
    setChatText('');
  };

  // Common "I've got a parsed sample" handler
  const acceptSample = (sample) => {
    setMealName(sample.mealName);
    setTranscript(sample.transcript);
    setItems(sample.items.map(it => ({ ...it })));
    setStage('parsed');
  };

  // Run a fake parse — show 'parsing' for a beat, then settle
  const runParse = (text) => {
    setStage('parsing');
    const sample = pickSampleForInput(text);
    setTimeout(() => acceptSample(sample), 720);
  };

  const totals = totalMeal(items);

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 110,
      }}>
        {/* Top bar */}
        <div style={{
          padding: '8px 16px 8px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <button onClick={onCancel} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none',
            background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}>
            <Icon name="x-mark" size={18} />
          </button>
          <div style={{
            fontFamily: F.mono, fontSize: 12, color: pal.subtle,
            fontVariantNumeric: 'tabular-nums',
          }}>08:42 · вс 24 мая</div>
          <div style={{ width: 40 }} />
        </div>

        {/* Hero title */}
        <div style={{ padding: '4px 24px 18px' }}>
          <Eyebrow style={{ color: C.sand700, marginBottom: 6 }}>Запись приёма</Eyebrow>
          <div style={{
            fontFamily: F.display, fontSize: 32, color: pal.ink,
            lineHeight: 1.05, letterSpacing: '-0.018em',
          }}>
            Что вы <span style={{ fontStyle: 'italic', color: C.sand700 }}>ели?</span>
          </div>
        </div>

        {/* Segmented mode picker */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            display: 'flex', background: pal.sunk, borderRadius: 12, padding: 3, gap: 2,
          }}>
            {[
              { id: 'chat',  icon: 'paper-airplane', label: 'Текст'  },
              { id: 'photo', icon: 'camera',         label: 'Фото'   },
              { id: 'voice', icon: 'microphone',     label: 'Голос'  },
            ].map(m => {
              const on = mode === m.id;
              return (
                <button key={m.id}
                  onClick={() => switchMode(m.id)}
                  style={{
                    flex: 1, padding: '10px 0', borderRadius: 9, border: 'none', cursor: 'pointer',
                    background: on ? pal.paper : 'transparent',
                    color: on ? pal.ink : pal.muted,
                    fontFamily: F.body, fontSize: 13, fontWeight: 500,
                    boxShadow: on ? '0 1px 2px rgba(46,38,24,.06)' : 'none',
                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 6,
                    transition: 'background-color 160ms var(--ease-out), color 160ms var(--ease-out)',
                  }}>
                  <MealModeIcon name={m.icon} />
                  {m.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Mode-specific input */}
        <div style={{ padding: '0 16px 14px' }}>
          {mode === 'chat'  && <ChatInput
                                  pal={pal}
                                  text={chatText} setText={setChatText}
                                  stage={stage}
                                  onParse={(txt) => runParse(txt)}
                                  sampleIdx={sampleIdx}
                                  onCycleSample={() => {
                                    const next = (sampleIdx + 1) % SAMPLE_PARSES.length;
                                    setSampleIdx(next);
                                    setChatText(SAMPLE_PARSES[next].transcript);
                                  }}
                              />}
          {mode === 'photo' && <PhotoInput pal={pal} stage={stage} setStage={setStage}
                                  onParse={(samp) => acceptSample(samp)} />}
          {mode === 'voice' && <VoiceInput pal={pal} stage={stage} setStage={setStage}
                                  onParse={(samp) => acceptSample(samp)} />}
        </div>

        {/* Parsed transcript chip (after parse) */}
        {stage === 'parsed' && transcript && (
          <div style={{ padding: '0 16px 10px' }}>
            <div style={{
              background: pal.sunk, borderRadius: 14, padding: '10px 12px',
              display: 'flex', alignItems: 'flex-start', gap: 10,
              border: `1px solid ${pal.hairline}`,
            }}>
              <Icon name="sparkles" size={16} color={C.sand700} />
              <div style={{ minWidth: 0, flex: 1 }}>
                <div style={{
                  fontFamily: F.body, fontSize: 11, color: pal.subtle, marginBottom: 2,
                }}>Услышали</div>
                <div style={{
                  fontFamily: F.body, fontSize: 12.5, color: pal.ink2, lineHeight: 1.4,
                }}>{transcript}</div>
              </div>
            </div>
          </div>
        )}

        {/* Parsed items list */}
        {stage === 'parsed' && (
          <div style={{ padding: '0 16px 14px' }}>
            <div style={{
              display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
              padding: '4px 4px 10px',
            }}>
              <Eyebrow style={{ color: pal.subtle }}>{mealName} · {items.length} {items.length === 1 ? 'позиция' : items.length < 5 ? 'позиции' : 'позиций'}</Eyebrow>
              <span style={{
                fontFamily: F.mono, fontSize: 11, color: pal.subtle,
                fontVariantNumeric: 'tabular-nums',
              }}>{totals.kcal} ккал</span>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {items.map((it) => (
                <ParsedItem
                  key={it.id}
                  item={it}
                  pal={pal}
                  editing={editingGramsId === it.id}
                  onStartEdit={() => setEditingGramsId(editingGramsId === it.id ? null : it.id)}
                  onChangeGrams={(g) => {
                    setItems(items.map(x => x.id === it.id ? rescaleItem(x, g) : x));
                  }}
                  onDelete={() => setItems(items.filter(x => x.id !== it.id))}
                />
              ))}
            </div>

            {/* Totals strip */}
            <div style={{ marginTop: 12 }}>
              <MacroStrip totals={totals} pal={pal} compact={false} />
            </div>
          </div>
        )}

        {/* Parsing skeleton */}
        {stage === 'parsing' && (
          <div style={{ padding: '0 16px' }}>
            <div style={{
              padding: 16, borderRadius: 14, background: pal.sunk,
              fontFamily: F.body, fontSize: 13, color: pal.muted,
              display: 'flex', alignItems: 'center', gap: 10,
              border: `1px solid ${pal.hairline}`,
            }}>
              <span style={{
                width: 14, height: 14, borderRadius: 999,
                border: `2px solid ${pal.bone}`, borderTopColor: C.sand700,
                animation: 'spin 800ms linear infinite',
              }} />
              Разбираем, что вы ели…
            </div>
          </div>
        )}
      </div>

      {/* Footer Save button — sticks to bottom */}
      <div style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        padding: '14px 16px 28px',
        background: `linear-gradient(180deg, rgba(246,241,234,0) 0%, ${pal.bg} 40%, ${pal.bg} 100%)`,
      }}>
        <button
          className="press"
          disabled={stage !== 'parsed' || items.length === 0}
          onClick={() => onComplete({
            mealName, items, totals,
            time: '08:42 am',
          })}
          style={{
            width: '100%', padding: '15px 20px', borderRadius: 999,
            background: stage === 'parsed' && items.length > 0 ? C.forest700 : pal.sunk,
            color: stage === 'parsed' && items.length > 0 ? C.cream : pal.subtle,
            border: 'none',
            fontFamily: F.body, fontSize: 14, fontWeight: 500,
            cursor: stage === 'parsed' && items.length > 0 ? 'pointer' : 'not-allowed',
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
          }}
        >
          {stage === 'parsed' && items.length > 0
            ? <>Сохранить · <span style={{ fontFamily: F.mono, fontVariantNumeric: 'tabular-nums' }}>{totals.kcal}</span> ккал</>
            : 'Добавьте что-нибудь'}
        </button>
      </div>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────
// MODE: CHAT
// ────────────────────────────────────────────────────────────────────

function ChatInput({ pal, text, setText, stage, onParse, sampleIdx, onCycleSample }) {
  const ph = SAMPLE_PARSES[sampleIdx].placeholder;
  return (
    <div style={{
      background: pal.paper, borderRadius: 18, padding: 14,
      border: `1px solid ${pal.hairline}`,
      boxShadow: '0 2px 6px rgba(46,38,24,.04)',
    }}>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={ph}
        rows={3}
        style={{
          width: '100%', resize: 'none', border: 'none', outline: 'none', background: 'transparent',
          fontFamily: F.body, fontSize: 15, color: pal.ink, lineHeight: 1.5,
          padding: 0,
          boxSizing: 'border-box',
        }}
      />
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        marginTop: 10, gap: 8,
      }}>
        <button onClick={onCycleSample} className="press" style={{
          padding: '8px 12px', borderRadius: 999,
          background: 'transparent', border: `1px solid ${pal.border}`,
          color: pal.muted, fontFamily: F.body, fontSize: 12, fontWeight: 500,
          cursor: 'pointer',
          display: 'inline-flex', alignItems: 'center', gap: 5,
          whiteSpace: 'nowrap',
        }}>
          <Icon name="sparkles" size={14} />
          Пример
        </button>
        <button
          onClick={() => onParse(text || SAMPLE_PARSES[0].transcript)}
          className="press"
          disabled={stage === 'parsing'}
          style={{
            padding: '8px 16px', borderRadius: 999,
            background: text.trim() ? C.sand500 : pal.sunk,
            color: text.trim() ? C.ink900 : pal.subtle,
            border: 'none', cursor: text.trim() ? 'pointer' : 'not-allowed',
            fontFamily: F.body, fontSize: 13, fontWeight: 500,
            display: 'inline-flex', alignItems: 'center', gap: 6,
          }}
        >
          Разобрать
          <span style={{ fontFamily: F.display, fontStyle: 'italic' }}>→</span>
        </button>
      </div>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────
// MODE: PHOTO
// ────────────────────────────────────────────────────────────────────

function PhotoInput({ pal, stage, setStage, onParse }) {
  const [captured, setCaptured] = React.useState(false);

  const onCapture = () => {
    setCaptured(true);
    setStage('parsing');
    setTimeout(() => onParse(SAMPLE_PARSES[1]), 1100); // photo always parses lunch bowl
  };

  if (!captured) {
    return (
      <button
        onClick={onCapture}
        className="press"
        style={{
          width: '100%', aspectRatio: '4 / 3',
          background: pal.sunk,
          border: `1.5px dashed ${pal.border}`,
          borderRadius: 18, padding: 0, cursor: 'pointer',
          display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
          gap: 10,
        }}
      >
        <div style={{
          width: 56, height: 56, borderRadius: 999,
          background: C.sand300, color: C.sand700,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <Icon name="camera" size={26} />
        </div>
        <div style={{
          fontFamily: F.display, fontSize: 20, color: pal.ink,
          letterSpacing: '-0.012em',
        }}>
          Снимок <span style={{ fontStyle: 'italic', color: C.sand700 }}>тарелки</span>
        </div>
        <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, textAlign: 'center', maxWidth: 240 }}>
          Нажмите — мы распознаем, что на ней
        </div>
      </button>
    );
  }

  // Captured: show a soft sand placeholder + small thumbnail header
  return (
    <div style={{
      width: '100%', aspectRatio: '16 / 9',
      borderRadius: 18, overflow: 'hidden', position: 'relative',
      background: `linear-gradient(135deg, ${C.sand500} 0%, ${C.sand700} 100%)`,
      border: `1px solid ${pal.hairline}`,
    }}>
      <div style={{
        position: 'absolute', inset: 0,
        background: 'radial-gradient(circle at 30% 40%, rgba(255,240,210,.35) 0%, transparent 50%)',
      }} />
      <div style={{
        position: 'absolute', inset: 0,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        flexDirection: 'column', gap: 8, color: C.cream,
      }}>
        <Icon name="camera" size={28} />
        <div style={{ fontFamily: F.body, fontSize: 12, opacity: 0.85, letterSpacing: '.04em' }}>
          ФОТО СДЕЛАНО
        </div>
      </div>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────
// MODE: VOICE
// ────────────────────────────────────────────────────────────────────

function VoiceInput({ pal, stage, setStage, onParse }) {
  const [holding, setHolding] = React.useState(false);
  const [held, setHeld] = React.useState(false);   // has been released once

  const onStart = () => {
    if (held) return;
    setHolding(true);
    setStage('capturing');
  };
  const onEnd = () => {
    if (!holding) return;
    setHolding(false);
    setHeld(true);
    setStage('parsing');
    setTimeout(() => onParse(SAMPLE_PARSES[2]), 1100);
  };

  return (
    <div style={{
      background: pal.paper, borderRadius: 18, padding: 22,
      border: `1px solid ${pal.hairline}`,
      display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 14,
    }}>
      <div style={{
        fontFamily: F.body, fontSize: 12.5, color: pal.muted, textAlign: 'center',
      }}>
        {held
          ? 'Поняли — секунду.'
          : holding
            ? 'Слушаем… отпустите, когда закончите.'
            : 'Зажмите кнопку и говорите.'}
      </div>

      <button
        onMouseDown={onStart}
        onMouseUp={onEnd}
        onMouseLeave={() => holding && onEnd()}
        onTouchStart={(e) => { e.preventDefault(); onStart(); }}
        onTouchEnd={(e) => { e.preventDefault(); onEnd(); }}
        disabled={held}
        style={{
          width: 96, height: 96, borderRadius: 999,
          background: holding ? C.forest800 : held ? pal.sunk : C.sand500,
          color: holding ? C.cream : held ? pal.subtle : C.ink900,
          border: 'none',
          cursor: held ? 'not-allowed' : 'pointer',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          boxShadow: holding ? '0 0 0 14px rgba(45,95,63,.12)' : '0 4px 14px rgba(212,165,116,.36)',
          transition: 'background-color 160ms var(--ease-out), box-shadow 160ms var(--ease-out)',
        }}
      >
        <Icon name="microphone" size={36} />
      </button>

      {/* Waveform bars */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 4,
        height: 24,
      }}>
        {Array.from({ length: 9 }).map((_, i) => (
          <span key={i} style={{
            width: 3, borderRadius: 2,
            background: holding ? C.forest700 : pal.bone,
            height: holding
              ? (8 + Math.abs(Math.sin((i + 1) * 1.3)) * 16)
              : 6,
            animation: holding ? `wavebar 700ms ease-in-out ${i * 70}ms infinite` : 'none',
          }} />
        ))}
      </div>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────
// PARSED ITEM ROW
// ────────────────────────────────────────────────────────────────────

function ParsedItem({ item, pal, editing, onStartEdit, onChangeGrams, onDelete }) {
  const macroBadge = (label, value, color) => (
    <span style={{
      display: 'inline-flex', alignItems: 'baseline', gap: 3,
      fontFamily: F.mono, fontSize: 10.5, fontVariantNumeric: 'tabular-nums',
      color: pal.muted,
    }}>
      <span style={{ width: 5, height: 5, borderRadius: 999, background: color, alignSelf: 'center' }} />
      {label}
      <span style={{ color: pal.ink2 }}>{value}</span>
    </span>
  );

  return (
    <div style={{
      background: pal.paper, borderRadius: 14, padding: '12px 14px',
      border: `1px solid ${pal.hairline}`,
    }}>
      <div style={{
        display: 'grid', gridTemplateColumns: '1fr auto auto',
        alignItems: 'center', gap: 10,
      }}>
        <div style={{ minWidth: 0 }}>
          <div style={{
            fontFamily: F.body, fontSize: 14, fontWeight: 500, color: pal.ink,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}>{item.name}</div>
          <div style={{
            display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 4,
          }}>
            {macroBadge('Белок', item.p + 'г', C.forest700)}
            {macroBadge('Углеводы', item.c + 'г', '#a5773d')}
            {macroBadge('Жиры', item.f + 'г', C.sand700)}
          </div>
        </div>
        <button onClick={onStartEdit} className="press" style={{
          padding: '4px 10px', borderRadius: 999,
          background: editing ? C.forest700 : pal.sunk,
          color: editing ? C.cream : pal.ink2,
          border: 'none', cursor: 'pointer',
          fontFamily: F.mono, fontSize: 12, fontVariantNumeric: 'tabular-nums',
          letterSpacing: '-0.01em', whiteSpace: 'nowrap',
        }}>
          {item.grams} g
        </button>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end' }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 2 }}>
            <span style={{
              fontFamily: F.mono, fontSize: 16, fontWeight: 500,
              color: pal.ink, fontVariantNumeric: 'tabular-nums',
              letterSpacing: '-0.02em',
            }}>{item.kcal}</span>
            <span style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle }}>kcal</span>
          </div>
          <button onClick={onDelete} className="press" style={{
            marginTop: 2, padding: 4, background: 'transparent', border: 'none', cursor: 'pointer',
            color: pal.subtle, display: 'inline-flex',
          }}>
            <Icon name="trash" size={14} />
          </button>
        </div>
      </div>

      {/* Inline grams stepper */}
      {editing && (
        <div style={{
          marginTop: 12, paddingTop: 12, borderTop: `1px dashed ${pal.hairline}`,
          display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12,
        }}>
          <Meta style={{ color: pal.subtle }}>Поправить порцию</Meta>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <button onClick={() => onChangeGrams(Math.max(5, item.grams - 10))}
              className="press" style={{
                width: 32, height: 32, borderRadius: 999, border: 'none',
                background: pal.sunk, color: pal.ink, fontSize: 18, cursor: 'pointer',
              }}>−</button>
            <span style={{
              fontFamily: F.mono, fontSize: 18, fontWeight: 500,
              fontVariantNumeric: 'tabular-nums',
              color: pal.ink, minWidth: 56, textAlign: 'center',
            }}>{item.grams} g</span>
            <button onClick={() => onChangeGrams(item.grams + 10)}
              className="press" style={{
                width: 32, height: 32, borderRadius: 999, border: 'none',
                background: C.sand500, color: C.ink900, fontSize: 18, cursor: 'pointer',
              }}>+</button>
          </div>
        </div>
      )}
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────
// MACRO TOTALS STRIP — reusable
// ────────────────────────────────────────────────────────────────────

function MacroStrip({ totals, pal, compact, targets }) {
  const t = targets || MEAL_TARGETS;
  const rows = [
    { lbl: 'ккал',  v: totals.kcal, goal: t.kcal,    color: C.forest700 },
    { lbl: 'белок', v: Math.round(totals.p),    goal: t.protein, color: C.forest700 },
    { lbl: 'углев', v: Math.round(totals.c),    goal: t.carbs,   color: '#a5773d' },
    { lbl: 'жиры',  v: Math.round(totals.f),    goal: t.fat,     color: C.sand700 },
  ];
  return (
    <div style={{
      background: pal.paper, borderRadius: 14, padding: 12,
      border: `1px solid ${pal.hairline}`,
      display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10,
    }}>
      {rows.map((r, i) => (
        <div key={r.lbl} style={{
          borderLeft: i === 0 ? 'none' : `1px solid ${pal.hairline}`,
          paddingLeft: i === 0 ? 0 : 10,
        }}>
          <Eyebrow style={{ color: pal.subtle, marginBottom: 4, fontSize: 10 }}>{r.lbl}</Eyebrow>
          <div style={{
            fontFamily: F.mono, fontSize: 16, fontWeight: 500,
            color: pal.ink, letterSpacing: '-0.02em',
            fontVariantNumeric: 'tabular-nums', lineHeight: 1,
          }}>{r.v}</div>
          {!compact && (
            <div style={{
              fontFamily: F.body, fontSize: 10, color: pal.subtle, marginTop: 2,
            }}>/ {r.goal}{r.lbl === 'ккал' ? '' : ' г'}</div>
          )}
        </div>
      ))}
    </div>
  );
}

// Tiny icon wrapper — uses Heroicons set; falls back to a glyph for 'microphone'
function MealModeIcon({ name }) {
  if (name === 'microphone') {
    return (
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
        <path d="M12 18.75v3M12 18.75a6 6 0 0 0 6-6V9a6 6 0 1 0-12 0v3.75a6 6 0 0 0 6 6Z M12 4.5v8.25"
          stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    );
  }
  return <Icon name={name} size={14} />;
}

Object.assign(window, { MealLogScreen, MacroStrip });
