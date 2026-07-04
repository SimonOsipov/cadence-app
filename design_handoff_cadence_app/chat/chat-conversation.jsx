// Cadence — Chat conversation screen.
// Full-screen thread with date-grouped bubbles, inline data/photo cards,
// share chips above the input, and a "+" attach menu.
// Exposes: ChatConversation

function ChatConversation({ pal, platform, threadId, onBack, onBackToList }) {
  const team = CARE_TEAM.find(t => t.id === threadId) || CARE_TEAM[0];
  const initial = CHAT_MESSAGES[threadId] || [];

  const [messages, setMessages] = React.useState(initial);
  const [draft, setDraft] = React.useState('');
  const [attachSheetOpen, setAttachSheetOpen] = React.useState(false);

  const scrollRef = React.useRef(null);

  // Scroll to bottom on mount + whenever a new message lands.
  React.useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages.length]);

  const sendText = () => {
    const text = draft.trim();
    if (!text) return;
    setMessages(prev => [...prev, {
      id: `u-${Date.now()}`, from: 'me', kind: 'text',
      day: 0, time: 'сейчас', text,
    }]);
    setDraft('');
  };

  const insertShare = (opt) => {
    if (opt.id === 'photo') {
      setMessages(prev => [...prev, {
        id: `u-p-${Date.now()}`, from: 'me', kind: 'photo',
        day: 0, time: 'сейчас', photo: opt.data,
      }]);
    } else {
      setMessages(prev => [...prev, {
        id: `u-d-${Date.now()}`, from: 'me', kind: 'data',
        day: 0, time: 'сейчас', data: opt.data,
      }]);
    }
    setAttachSheetOpen(false);
  };

  const groups = groupByDay(messages);

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
      {/* Top bar */}
      <div style={{
        padding: platform === 'ios' ? '48px 12px 8px' : '8px 12px 8px',
        display: 'grid', gridTemplateColumns: 'auto 1fr auto', gap: 10, alignItems: 'center',
        background: pal.bg,
        borderBottom: `1px solid ${pal.hairline}`,
        flexShrink: 0,
      }}>
        <button onClick={onBack} className="press" style={{
          width: 40, height: 40, borderRadius: 999, border: 'none',
          background: 'transparent', color: pal.ink2,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          cursor: 'pointer',
        }}>
          <Icon name="chevron-left" size={22} />
        </button>
        <button
          onClick={onBackToList}
          className="press"
          style={{
            display: 'grid', gridTemplateColumns: 'auto 1fr', gap: 10, alignItems: 'center',
            background: 'transparent', border: 'none', cursor: 'pointer',
            textAlign: 'left',
            minWidth: 0,
          }}>
          <div style={{
            width: 36, height: 36, borderRadius: 999,
            background: team.avatarBg, color: team.avatarFg,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontFamily: F.display, fontStyle: 'italic', fontSize: 17,
          }}>{team.initial}</div>
          <div style={{ minWidth: 0 }}>
            <div style={{
              fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: pal.ink,
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}>{team.name}</div>
            <div style={{
              fontFamily: F.body, fontSize: 11.5, color: C.sand700,
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            }}>{team.role}</div>
          </div>
        </button>
        <button className="press" style={{
          width: 40, height: 40, borderRadius: 999, border: 'none',
          background: 'transparent', color: pal.ink2,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          cursor: 'pointer',
        }}>
          <Icon name="ellipsis-horizontal" size={20} />
        </button>
      </div>

      {/* Message scroll area */}
      <div
        ref={scrollRef}
        className="ds-scroll"
        style={{
          flex: 1, minHeight: 0, overflowY: 'auto', overflowX: 'hidden',
          padding: '16px 14px 10px',
          display: 'flex', flexDirection: 'column', gap: 4,
        }}
      >
        {groups.length === 0 && (
          <div style={{
            margin: 'auto', textAlign: 'center', maxWidth: 240,
            color: pal.muted, fontFamily: F.body, fontSize: 13,
          }}>Начните разговор — напишите сообщение ниже.</div>
        )}
        {groups.map((g, gi) => (
          <React.Fragment key={gi}>
            <DateSeparator day={g.day} pal={pal} />
            {g.messages.map((m, mi) => {
              const prev = g.messages[mi - 1];
              const next = g.messages[mi + 1];
              const showAvatar = m.from === 'doctor' && (!next || next.from !== 'doctor');
              const isFirstInRun = !prev || prev.from !== m.from;
              const isLastInRun  = !next || next.from !== m.from;
              return (
                <MessageRow
                  key={m.id}
                  msg={m}
                  team={team}
                  pal={pal}
                  showAvatar={showAvatar}
                  isFirstInRun={isFirstInRun}
                  isLastInRun={isLastInRun}
                />
              );
            })}
          </React.Fragment>
        ))}
      </div>

      {/* Share chips row — quick data inserts */}
      <div style={{
        padding: '6px 14px 6px',
        display: 'flex', gap: 6, overflowX: 'auto',
        WebkitOverflowScrolling: 'touch',
        flexShrink: 0,
        borderTop: `1px solid ${pal.hairline}`,
      }}>
        {SHARE_OPTIONS.map(opt => (
          <button
            key={opt.id}
            onClick={() => insertShare(opt)}
            className="press"
            style={{
              flexShrink: 0,
              padding: '7px 12px', borderRadius: 999, cursor: 'pointer',
              background: pal.sunk, border: `1px solid ${pal.hairline}`,
              color: pal.ink2,
              fontFamily: F.body, fontSize: 12, fontWeight: 500,
              whiteSpace: 'nowrap',
              display: 'inline-flex', alignItems: 'center', gap: 6,
            }}
          >
            <Icon name={opt.icon} size={14} color={C.forest700} />
            {opt.label}
          </button>
        ))}
      </div>

      {/* Input bar */}
      <div style={{
        padding: '8px 12px 28px',
        display: 'grid', gridTemplateColumns: 'auto 1fr auto', gap: 8, alignItems: 'flex-end',
        background: pal.bg,
        flexShrink: 0,
      }}>
        <button
          onClick={() => setAttachSheetOpen(true)}
          className="press"
          style={{
            width: 40, height: 40, borderRadius: 999,
            background: pal.sunk, color: pal.ink2, border: 'none',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
            flexShrink: 0,
          }}>
          <Icon name="plus" size={20} strokeWidth={1.7} />
        </button>
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendText(); }
          }}
          placeholder="Сообщение…"
          rows={1}
          style={{
            width: '100%', resize: 'none', boxSizing: 'border-box',
            padding: '10px 14px', borderRadius: 20,
            background: pal.paper, border: `1px solid ${pal.border}`,
            outline: 'none',
            fontFamily: F.body, fontSize: 14, color: pal.ink, lineHeight: 1.4,
            minHeight: 40, maxHeight: 96,
          }}
        />
        <button
          onClick={sendText}
          disabled={!draft.trim()}
          className="press"
          style={{
            width: 40, height: 40, borderRadius: 999,
            background: draft.trim() ? C.forest700 : pal.sunk,
            color: draft.trim() ? C.cream : pal.subtle,
            border: 'none',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: draft.trim() ? 'pointer' : 'not-allowed',
            flexShrink: 0,
          }}>
          <Icon name="paper-airplane" size={18} />
        </button>
      </div>

      {/* Attach sheet */}
      {attachSheetOpen && (
        <AttachSheet
          pal={pal}
          options={SHARE_OPTIONS}
          onPick={insertShare}
          onClose={() => setAttachSheetOpen(false)}
        />
      )}
    </div>
  );
}

// ── DateSeparator ───────────────────────────────────────────────────

function DateSeparator({ day, pal }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10,
      padding: '8px 4px', margin: '4px 0',
    }}>
      <div style={{ flex: 1, height: 1, background: pal.hairline }} />
      <span style={{
        fontFamily: F.body, fontSize: 11, fontWeight: 500,
        letterSpacing: '.06em', textTransform: 'uppercase',
        color: pal.subtle,
      }}>{chatDayLabel(day)}</span>
      <div style={{ flex: 1, height: 1, background: pal.hairline }} />
    </div>
  );
}

// ── MessageRow ──────────────────────────────────────────────────────

function MessageRow({ msg, team, pal, showAvatar, isFirstInRun, isLastInRun }) {
  const isMe = msg.from === 'me';
  // iMessage-style corner pinching toward the avatar
  const radii = isMe
    ? { tl: 18, tr: 18, br: isLastInRun ? 4 : 18, bl: 18 }
    : { tl: 18, tr: 18, br: 18, bl: isLastInRun ? 4 : 18 };

  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: isMe ? '1fr auto' : 'auto 1fr',
      gap: 8,
      marginBottom: isLastInRun ? 6 : 2,
    }}>
      {!isMe && (
        <div style={{
          width: 28, height: 28, borderRadius: 999,
          background: showAvatar ? team.avatarBg : 'transparent',
          color: team.avatarFg,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontFamily: F.display, fontStyle: 'italic', fontSize: 13,
          alignSelf: 'flex-end',
          visibility: showAvatar ? 'visible' : 'hidden',
        }}>{team.initial}</div>
      )}
      <div style={{
        display: 'flex', flexDirection: 'column',
        alignItems: isMe ? 'flex-end' : 'flex-start',
        maxWidth: '78%',
        justifySelf: isMe ? 'end' : 'start',
      }}>
        {msg.kind === 'text' && (
          <div style={{
            background: isMe ? C.forest700 : pal.paper,
            color: isMe ? C.cream : pal.ink,
            padding: '10px 14px',
            borderTopLeftRadius: radii.tl, borderTopRightRadius: radii.tr,
            borderBottomRightRadius: radii.br, borderBottomLeftRadius: radii.bl,
            border: isMe ? 'none' : `1px solid ${pal.hairline}`,
            fontFamily: F.body, fontSize: 14, lineHeight: 1.45,
            whiteSpace: 'pre-wrap',
          }}>{msg.text}</div>
        )}
        {msg.kind === 'data' && <DataCard data={msg.data} isMe={isMe} pal={pal} />}
        {msg.kind === 'photo' && <PhotoCard photo={msg.photo} isMe={isMe} pal={pal} />}
        {isLastInRun && (
          <span style={{
            fontFamily: F.mono, fontSize: 10, color: pal.subtle,
            fontVariantNumeric: 'tabular-nums',
            marginTop: 4, padding: '0 4px',
          }}>{msg.time}</span>
        )}
      </div>
    </div>
  );
}

// ── DataCard — shared app data inside a message ─────────────────────

function DataCard({ data, isMe, pal }) {
  const sandTone = data.kind === 'meal';
  const accent = sandTone ? C.sand700 : C.forest700;
  const accentBg = sandTone ? C.sand100 : C.forest50;

  // On user-side bubble, keep the card paper-toned (cards always read as content,
  // not as conversational text); just nudge the corner to match the chat tail.
  return (
    <div style={{
      background: pal.paper,
      border: `1px solid ${pal.hairline}`,
      borderRadius: 16, padding: 14,
      boxShadow: '0 1px 3px rgba(46,38,24,.05)',
      minWidth: 220, maxWidth: '100%',
    }}>
      <div style={{
        display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6,
      }}>
        <span style={{
          padding: '2px 7px', borderRadius: 999,
          fontFamily: F.body, fontSize: 9.5, fontWeight: 600,
          letterSpacing: '.08em', textTransform: 'uppercase',
          background: accentBg, color: accent,
        }}>Поделились</span>
        <span style={{
          fontFamily: F.body, fontSize: 11, color: pal.subtle,
        }}>{data.eyebrow}</span>
      </div>
      <div style={{
        fontFamily: F.display, fontSize: 22, color: pal.ink,
        lineHeight: 1.1, letterSpacing: '-0.014em', marginBottom: 4,
      }}>{data.title}</div>
      <div style={{
        fontFamily: F.body, fontSize: 12.5, color: pal.ink2, marginBottom: 2,
      }}>{data.delta}</div>
      <div style={{
        fontFamily: F.body, fontSize: 11, color: pal.subtle,
      }}>{data.meta}</div>
    </div>
  );
}

// ── PhotoCard — image placeholder ───────────────────────────────────

function PhotoCard({ photo, isMe, pal }) {
  return (
    <div style={{
      width: 200, maxWidth: '100%',
      borderRadius: 16, overflow: 'hidden',
      border: `1px solid ${pal.hairline}`,
      background: pal.paper,
    }}>
      <div style={{
        width: '100%', aspectRatio: '4 / 3',
        background: `linear-gradient(135deg, ${C.sand500} 0%, ${C.sand700} 100%)`,
        position: 'relative',
      }}>
        <div style={{
          position: 'absolute', inset: 0,
          background: 'radial-gradient(circle at 30% 40%, rgba(255,240,210,.35) 0%, transparent 50%)',
        }} />
        <div style={{
          position: 'absolute', inset: 0,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          color: C.cream,
        }}>
          <Icon name="camera" size={28} />
        </div>
      </div>
      <div style={{
        padding: '8px 12px',
        fontFamily: F.body, fontSize: 12, color: pal.ink2,
      }}>{photo.caption}</div>
    </div>
  );
}

// ── AttachSheet — bottom sheet for full attachment menu ────────────

function AttachSheet({ pal, options, onPick, onClose }) {
  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 80 }}>
      <div className="scrim" onClick={onClose} style={{
        position: 'absolute', inset: 0,
        background: 'rgba(20,44,31,.35)',
        backdropFilter: 'blur(4px)',
      }} />
      <div className="sheet" style={{
        position: 'absolute', left: 0, right: 0, bottom: 0,
        background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28,
        padding: '12px 20px 28px',
        boxShadow: '0 -18px 40px rgba(0,0,0,.18)',
      }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '0 auto 16px' }} />
        <div style={{ marginBottom: 14 }}>
          <Eyebrow style={{ color: pal.subtle, marginBottom: 6 }}>Прикрепить</Eyebrow>
          <div style={{
            fontFamily: F.display, fontSize: 24, color: pal.ink,
            lineHeight: 1.05, letterSpacing: '-0.018em',
          }}>Что <span style={{ fontStyle: 'italic', color: C.forest700 }}>отправляем</span>?</div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {options.map(opt => (
            <button
              key={opt.id}
              onClick={() => onPick(opt)}
              className="press"
              style={{
                display: 'grid', gridTemplateColumns: '40px 1fr auto',
                gap: 12, alignItems: 'center',
                width: '100%', textAlign: 'left',
                background: pal.paper, border: `1px solid ${pal.hairline}`,
                borderRadius: 14, padding: 12, cursor: 'pointer',
              }}
            >
              <div style={{
                width: 40, height: 40, borderRadius: 12,
                background: opt.id === 'photo' ? C.sand100 : C.forest50,
                color: opt.id === 'photo' ? C.sand700 : C.forest700,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
                <Icon name={opt.icon} size={20} />
              </div>
              <div style={{ minWidth: 0 }}>
                <div style={{
                  fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: pal.ink,
                }}>{opt.label}</div>
                {opt.data && opt.data.title && (
                  <div style={{
                    fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 2,
                  }}>{opt.data.title}</div>
                )}
              </div>
              <Icon name="arrow-right" size={16} color={pal.subtle} />
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { ChatConversation });
