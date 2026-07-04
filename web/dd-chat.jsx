// Cadence — Doctor messages view (thread list · conversation · patient context)
// Depends on globals: C, F, Icon, Avatar, FlagPill, StatusTag, DeskSpark,
// PATIENTS/patientById, statusMeta, lost, lostPct,
// THREADS, QUICK_REPLIES, SHARE_ACTIONS, threadByPatient, groupThreadByDay, chatDay.

// ── Shared data card (inside a message bubble) ─────────────────────────────
function MsgDataCard({ kind, data, mine }) {
  const onDark = mine;
  const sub = { background: onDark ? 'rgba(246,241,234,.12)' : C.cream, border: '1px solid ' + (onDark ? 'rgba(246,241,234,.18)' : C.bone) };
  if (kind === 'photo') {
    return (
      <div style={{ ...sub, borderRadius: 14, overflow: 'hidden', width: 200 }}>
        <div style={{ height: 132, background: `repeating-linear-gradient(135deg, ${C.linen}, ${C.linen} 12px, ${C.sand100} 12px, ${C.sand100} 24px)`, display: 'flex', alignItems: 'center', justifyContent: 'center', color: C.sand700 }}>
          <Icon name="camera" size={26} />
        </div>
        <div style={{ padding: '9px 12px', fontFamily: F.body, fontSize: 12, color: onDark ? C.cream : C.ink600 }}>{data.caption}</div>
      </div>
    );
  }
  const iconMap = { weight: 'scale', dose: 'beaker', meal: 'cake' };
  return (
    <div style={{ ...sub, borderRadius: 14, padding: '13px 15px', width: 232 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, marginBottom: 8 }}>
        <Icon name={iconMap[kind] || 'sparkles'} size={14} color={onDark ? C.sand300 : C.forest600} />
        <span style={{ fontFamily: F.body, fontSize: 10.5, fontWeight: 500, letterSpacing: '.12em', textTransform: 'uppercase', color: onDark ? 'rgba(246,241,234,.7)' : C.ink500 }}>{data.eyebrow}</span>
      </div>
      <div style={{ fontFamily: F.display, fontSize: 22, lineHeight: 1.05, color: onDark ? C.cream : C.ink900 }}>{data.title}</div>
      <div style={{ fontFamily: F.body, fontSize: 12.5, color: onDark ? C.sand300 : C.forest600, marginTop: 3 }}>{data.delta}</div>
      <div style={{ fontFamily: F.body, fontSize: 11.5, color: onDark ? 'rgba(246,241,234,.6)' : C.ink500, marginTop: 5 }}>{data.meta}</div>
    </div>
  );
}

// ── Message bubble ─────────────────────────────────────────────────────────
function Bubble({ m, pt }) {
  const mine = m.from === 'me';
  const isData = m.kind !== 'text';
  return (
    <div className="msg-in" style={{ display: 'flex', justifyContent: mine ? 'flex-end' : 'flex-start', gap: 10, alignItems: 'flex-end' }}>
      {!mine && <Avatar pt={pt} size={28} />}
      <div style={{ maxWidth: '64%', display: 'flex', flexDirection: 'column', alignItems: mine ? 'flex-end' : 'flex-start', gap: 4 }}>
        {isData ? (
          <MsgDataCard kind={m.kind} data={m.data} mine={mine} />
        ) : (
          <div style={{
            background: mine ? C.forest700 : C.paper,
            color: mine ? C.cream : C.ink800,
            border: mine ? 'none' : '1px solid ' + C.bone,
            borderRadius: 16, borderBottomRightRadius: mine ? 5 : 16, borderBottomLeftRadius: mine ? 16 : 5,
            padding: '10px 14px', fontFamily: F.body, fontSize: 14, lineHeight: 1.5,
          }}>{m.text}</div>
        )}
        <span style={{ fontFamily: F.mono, fontSize: 10.5, color: C.ink400, padding: '0 4px' }}>{m.time}</span>
      </div>
    </div>
  );
}

// ── Thread list item ───────────────────────────────────────────────────────
function ThreadItem({ thread, pt, active, onClick }) {
  const msgs = thread.messages;
  const lastMsg = msgs[msgs.length - 1];
  const preview = lastMsg.kind === 'text' ? lastMsg.text
    : lastMsg.kind === 'photo' ? '📷 Фото' : `${lastMsg.data.title}`;
  const sm = statusMeta(pt.status);
  return (
    <div onClick={onClick} style={{
      display: 'flex', gap: 12, padding: '13px 16px', cursor: 'pointer', alignItems: 'flex-start',
      background: active ? C.cream : 'transparent',
      borderLeft: '3px solid ' + (active ? C.forest700 : 'transparent'),
      transition: 'background 140ms',
    }}>
      <div style={{ position: 'relative', flexShrink: 0 }}>
        <Avatar pt={pt} size={42} />
        <span style={{ position: 'absolute', right: -1, bottom: -1, width: 12, height: 12, borderRadius: 999, background: sm.dot, border: '2px solid ' + C.paper }} />
      </div>
      <div style={{ minWidth: 0, flex: 1 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, alignItems: 'baseline' }}>
          <span style={{ fontFamily: F.body, fontWeight: 600, fontSize: 14, color: C.ink900, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{pt.name}</span>
          <span style={{ fontFamily: F.mono, fontSize: 11, color: C.ink400, flexShrink: 0 }}>{thread.lastTime}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, alignItems: 'center', marginTop: 2 }}>
          <span style={{ fontFamily: F.body, fontSize: 12.5, color: thread.unread ? C.ink800 : C.ink500, fontWeight: thread.unread ? 500 : 400, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', flex: 1 }}>
            {lastMsg.from === 'me' ? 'Вы: ' : ''}{preview}
          </span>
          {thread.unread > 0 && (
            <span style={{ flexShrink: 0, minWidth: 18, height: 18, padding: '0 5px', borderRadius: 999, background: C.forest700, color: C.cream, fontFamily: F.mono, fontSize: 11, display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>{thread.unread}</span>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Patient context panel (right) ──────────────────────────────────────────
function ContextPanel({ pt, onOpenPatient }) {
  const sm = statusMeta(pt.status);
  const sparkColor = pt.status === 'attention' ? C.danger : pt.status === 'watch' ? C.warning : C.forest700;
  const M = pt.metrics;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center', gap: 8, paddingTop: 4 }}>
        <Avatar pt={pt} size={64} ring={sm.dot} />
        <div>
          <div style={{ fontFamily: F.display, fontSize: 22, lineHeight: 1.05, color: C.ink900 }}>{pt.name}</div>
          <div style={{ fontFamily: F.body, fontSize: 12.5, color: C.ink500, marginTop: 3 }}>{pt.age} лет · {pt.lastSeen}</div>
        </div>
        <div style={{ marginTop: 2 }}><StatusTag status={pt.status} /></div>
      </div>

      {pt.flags.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, justifyContent: 'center' }}>
          {pt.flags.map(f => <FlagPill key={f} kind={f} small />)}
        </div>
      )}

      {/* protocol */}
      <div style={{ background: C.forest800, color: C.cream, borderRadius: 16, padding: 16 }}>
        <div style={{ fontFamily: F.body, fontSize: 10.5, fontWeight: 500, letterSpacing: '.12em', textTransform: 'uppercase', color: 'rgba(246,241,234,.6)', marginBottom: 6 }}>Протокол</div>
        <div style={{ fontFamily: F.display, fontSize: 22, lineHeight: 1 }}>{pt.compound}</div>
        <div style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 18, color: C.sand300 }}>{pt.dose}</div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 10, fontFamily: F.mono, fontSize: 12, color: 'rgba(246,241,234,.8)' }}>
          <span>{pt.cadence}</span><span>нед. {pt.week}/{pt.cycleLen}</span>
        </div>
        <div style={{ height: 4, borderRadius: 999, background: 'rgba(246,241,234,.18)', marginTop: 8, overflow: 'hidden' }}>
          <div style={{ height: '100%', width: (pt.week / pt.cycleLen * 100) + '%', background: C.sand500 }} />
        </div>
      </div>

      {/* weight */}
      <div style={{ background: C.paper, border: '1px solid ' + C.bone, borderRadius: 16, padding: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <div style={{ fontFamily: F.body, fontSize: 10.5, fontWeight: 500, letterSpacing: '.12em', textTransform: 'uppercase', color: C.ink500, marginBottom: 5 }}>Вес</div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
              <span style={{ fontFamily: F.mono, fontWeight: 500, fontSize: 26, color: C.ink900, fontVariantNumeric: 'tabular-nums' }}>{pt.weight.toFixed(1).replace('.', ',')}</span>
              <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 14, color: C.ink600 }}>{pt.unit}</span>
            </div>
            <div style={{ fontFamily: F.body, fontSize: 12, color: C.forest600, marginTop: 3 }}>↓ {lost(pt).toFixed(1).replace('.', ',')} кг</div>
          </div>
          <DeskSpark data={pt.spark} color={sparkColor} width={84} height={40} />
        </div>
      </div>

      {/* biomarkers */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 8 }}>
        {[
          { k: 'HRV', v: M.hrv, good: M.hrv >= 58 },
          { k: 'ЧСС', v: M.rhr, good: M.rhr <= 60 },
          { k: 'Сон', v: M.sleep, good: M.sleep >= 75 },
        ].map(m => (
          <div key={m.k} style={{ background: C.paper, border: '1px solid ' + C.bone, borderRadius: 12, padding: '10px 8px', textAlign: 'center' }}>
            <div style={{ fontFamily: F.body, fontSize: 10, fontWeight: 500, letterSpacing: '.08em', textTransform: 'uppercase', color: C.ink500, marginBottom: 5 }}>{m.k}</div>
            <div style={{ fontFamily: F.mono, fontWeight: 500, fontSize: 18, color: m.good ? C.ink900 : C.warning, fontVariantNumeric: 'tabular-nums' }}>{m.v}</div>
          </div>
        ))}
      </div>

      <button onClick={() => onOpenPatient(pt)} style={{
        fontFamily: F.body, fontWeight: 500, fontSize: 13.5, padding: '12px 0', borderRadius: 999,
        background: C.linen, color: C.ink900, border: '1px solid ' + C.bone, cursor: 'pointer',
        display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 8,
      }}>
        <Icon name="document-text" size={16} />Открыть карту
      </button>
    </div>
  );
}

// ── Messages view ──────────────────────────────────────────────────────────
function Messages({ initialThreadId, onOpenPatient }) {
  // local mutable copy so the composer can append
  const [convos, setConvos] = React.useState(() => {
    const o = {};
    THREADS.forEach(t => { o[t.patientId] = { messages: [...t.messages], unread: t.unread, lastTime: t.lastTime }; });
    return o;
  });
  const order = THREADS.map(t => t.patientId);
  const [activeId, setActiveId] = React.useState(initialThreadId && convos[initialThreadId] ? initialThreadId : order[0]);
  const [input, setInput] = React.useState('');
  const [showShare, setShowShare] = React.useState(false);
  const scrollRef = React.useRef(null);

  React.useEffect(() => {
    if (initialThreadId && convos[initialThreadId]) {
      setActiveId(initialThreadId);
      markRead(initialThreadId);
    }
  }, [initialThreadId]);

  React.useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [activeId, convos]);

  function markRead(id) {
    setConvos(prev => ({ ...prev, [id]: { ...prev[id], unread: 0 } }));
  }
  function openThread(id) { setActiveId(id); markRead(id); setShowShare(false); }

  function nowTime() {
    const d = new Date();
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  }
  function appendMine(text) {
    if (!text.trim()) return;
    const msg = { id: 'x' + Date.now(), from: 'me', kind: 'text', day: 0, time: nowTime(), text: text.trim() };
    setConvos(prev => ({ ...prev, [activeId]: { ...prev[activeId], messages: [...prev[activeId].messages, msg], lastTime: 'сейчас' } }));
  }
  function send() { if (input.trim()) { appendMine(input); setInput(''); } }

  const pt = patientById(activeId);
  const conv = convos[activeId];
  const groups = groupThreadByDay(conv.messages);

  // sort: unread first, keep authored order otherwise
  const sortedIds = [...order].sort((a, b) => (convos[b].unread > 0) - (convos[a].unread > 0));

  return (
    <main className="msg-main" style={{ flex: 1, minWidth: 0, height: '100vh', display: 'flex' }}>
      {/* thread list */}
      <div className="msg-list" style={{ width: 320, flexShrink: 0, borderRight: '1px solid ' + C.bone, background: C.paper, display: 'flex', flexDirection: 'column', height: '100vh' }}>
        <div style={{ padding: '22px 20px 14px' }}>
          <div style={{ fontFamily: F.body, fontSize: 11.5, fontWeight: 500, letterSpacing: '.14em', textTransform: 'uppercase', color: C.ink500, marginBottom: 5 }}>Входящие</div>
          <h2 style={{ fontFamily: F.display, fontWeight: 400, fontSize: 28, color: C.ink900, margin: 0 }}>Сообщения</h2>
          <div style={{ display: 'flex', alignItems: 'center', gap: 9, background: C.cream, border: '1px solid ' + C.bone, borderRadius: 999, padding: '9px 14px', marginTop: 14 }}>
            <Icon name="magnifying-glass" size={16} color={C.ink400} />
            <span style={{ fontFamily: F.body, fontSize: 13.5, color: C.ink400 }}>Поиск по пациентам</span>
          </div>
        </div>
        <div className="ds-scroll" style={{ flex: 1, overflowY: 'auto', paddingBottom: 12 }}>
          {sortedIds.map(id => (
            <ThreadItem key={id} thread={convos[id]} pt={patientById(id)} active={id === activeId} onClick={() => openThread(id)} />
          ))}
        </div>
      </div>

      {/* conversation */}
      <div className="msg-convo" style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', height: '100vh', background: C.cream }}>
        {/* header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 13, padding: '16px 24px', borderBottom: '1px solid ' + C.bone, background: C.paper, flexShrink: 0 }}>
          <Avatar pt={pt} size={42} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontFamily: F.body, fontWeight: 600, fontSize: 15.5, color: C.ink900 }}>{pt.name}</div>
            <div style={{ fontFamily: F.body, fontSize: 12.5, color: C.ink500 }}>{pt.compound} · {pt.dose} · нед. {pt.week}/{pt.cycleLen}</div>
          </div>
          <button onClick={() => onOpenPatient(pt)} style={{ display: 'inline-flex', alignItems: 'center', gap: 7, fontFamily: F.body, fontWeight: 500, fontSize: 13, padding: '9px 15px', borderRadius: 999, background: 'transparent', border: '1px solid ' + C.border, color: C.ink800, cursor: 'pointer' }}>
            <Icon name="document-text" size={15} />Карта
          </button>
          <button style={{ width: 38, height: 38, borderRadius: 999, border: '1px solid ' + C.border, background: 'transparent', color: C.ink600, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Icon name="ellipsis-horizontal" size={18} />
          </button>
        </div>

        {/* messages */}
        <div ref={scrollRef} className="ds-scroll" style={{ flex: 1, overflowY: 'auto', padding: '22px 24px' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14, maxWidth: 720, margin: '0 auto' }}>
            {groups.map((g, gi) => (
              <React.Fragment key={gi}>
                <div style={{ textAlign: 'center', margin: '6px 0' }}>
                  <span style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.1em', textTransform: 'uppercase', color: C.ink400, background: C.linen, padding: '4px 12px', borderRadius: 999 }}>{chatDay(g.day)}</span>
                </div>
                {g.items.map(m => <Bubble key={m.id} m={m} pt={pt} />)}
              </React.Fragment>
            ))}
            {conv.unread === 0 && conv.messages[conv.messages.length - 1].from === 'them' && (
              <div style={{ fontFamily: F.body, fontStyle: 'italic', fontSize: 12.5, color: C.ink400, textAlign: 'center', marginTop: 4 }}>Ждёт вашего ответа</div>
            )}
          </div>
        </div>

        {/* composer */}
        <div style={{ borderTop: '1px solid ' + C.bone, background: C.paper, padding: '12px 24px 16px', flexShrink: 0 }}>
          {/* quick replies */}
          <div className="ds-scroll" style={{ display: 'flex', gap: 8, overflowX: 'auto', paddingBottom: 10 }}>
            {QUICK_REPLIES.map((q, i) => (
              <button key={i} onClick={() => appendMine(q)} style={{
                flexShrink: 0, fontFamily: F.body, fontSize: 12.5, color: C.forest800, background: C.forest50,
                border: '1px solid ' + C.forest100, borderRadius: 999, padding: '7px 13px', cursor: 'pointer', whiteSpace: 'nowrap',
              }}>{q}</button>
            ))}
          </div>
          {/* share actions */}
          {showShare && (
            <div className="fade-in" style={{ display: 'flex', gap: 8, marginBottom: 10, flexWrap: 'wrap' }}>
              {SHARE_ACTIONS.map(a => (
                <button key={a.id} onClick={() => { appendMine(a.text); setShowShare(false); }} style={{
                  display: 'inline-flex', alignItems: 'center', gap: 7, fontFamily: F.body, fontSize: 12.5, fontWeight: 500,
                  color: C.ink800, background: C.cream, border: '1px solid ' + C.border, borderRadius: 12, padding: '9px 13px', cursor: 'pointer',
                }}>
                  <Icon name={a.icon} size={15} color={C.forest600} />{a.label}
                </button>
              ))}
            </div>
          )}
          {/* input row */}
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 10 }}>
            <button onClick={() => setShowShare(s => !s)} title="Запросить или прикрепить данные" style={{
              width: 44, height: 44, flexShrink: 0, borderRadius: 999, border: '1px solid ' + C.border,
              background: showShare ? C.forest50 : 'transparent', color: showShare ? C.forest700 : C.ink600, cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center', transition: 'all 140ms',
            }}>
              <Icon name="plus" size={20} strokeWidth={1.8} />
            </button>
            <textarea
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }}
              placeholder={`Ответить ${pt.name.split(' ')[0]}…`}
              rows={1}
              style={{
                flex: 1, resize: 'none', fontFamily: F.body, fontSize: 14, lineHeight: 1.5, color: C.ink900,
                background: C.cream, border: '1px solid ' + C.border, borderRadius: 22, padding: '11px 16px', outline: 'none',
                maxHeight: 120,
              }}
            />
            <button onClick={send} disabled={!input.trim()} style={{
              width: 44, height: 44, flexShrink: 0, borderRadius: 999, border: 'none',
              background: input.trim() ? C.forest700 : C.bone, color: input.trim() ? C.cream : C.ink400,
              cursor: input.trim() ? 'pointer' : 'default', display: 'flex', alignItems: 'center', justifyContent: 'center',
              transition: 'all 140ms',
            }}>
              <Icon name="paper-airplane" size={19} />
            </button>
          </div>
        </div>
      </div>

      {/* context panel */}
      <div className="msg-context ds-scroll" style={{ width: 296, flexShrink: 0, borderLeft: '1px solid ' + C.bone, background: C.cream, height: '100vh', overflowY: 'auto', padding: '22px 18px' }}>
        <ContextPanel pt={pt} onOpenPatient={onOpenPatient} />
      </div>
    </main>
  );
}

Object.assign(window, { Messages });
