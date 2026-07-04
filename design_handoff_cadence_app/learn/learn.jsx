// Cadence · Знания (Education library) — Library landing + Article reader
// Exposes: LibraryScreen, ArticleReader
//
// LibraryScreen props: pal, platform, onBack, onOpenArticle
// ArticleReader  props: pal, platform, article, onBack, onOpenArticle

const L = window.LEARN;

// Category tint → swatch
const TINTS = {
  forest: { fg: '#2d5f3f', soft: '#eaf0eb' },
  sand:   { fg: '#6b4a25', soft: '#f3e8d6' },
  clay:   { fg: '#9a5a3c', soft: '#f4e4d8' },
  slate:  { fg: '#41566b', soft: '#e6ecf2' },
};
function tintOf(catId) { const m = L.catMeta(catId); return TINTS[(m && m.tint) || 'forest']; }

// ── Compact typographic article row ───────────────────────────
function ArticleRow({ a, pal, onOpen, last }) {
  const m = L.catMeta(a.cat);
  const tint = tintOf(a.cat);
  return (
    <div onClick={() => onOpen(a.id)} className="press" style={{
      display: 'grid', gridTemplateColumns: '44px 1fr', gap: 14, alignItems: 'start',
      padding: '14px 4px', cursor: 'pointer',
      borderBottom: last ? 'none' : `1px solid ${pal.hairline}`,
    }}>
      <div style={{ width: 44, height: 44, borderRadius: 12, background: tint.soft, color: tint.fg, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Icon name={m.icon} size={20} />
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontFamily: F.display, fontSize: 19, color: pal.ink, lineHeight: 1.12, letterSpacing: '-0.012em' }}>{a.title}</div>
        <div style={{ fontFamily: F.body, fontSize: 12.5, color: pal.subtle, marginTop: 3, lineHeight: 1.45,
          display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{a.dek}</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 7 }}>
          <span style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, color: tint.fg }}>{m.label}</span>
          <span style={{ width: 3, height: 3, borderRadius: 999, background: pal.placeholder }} />
          <span style={{ fontFamily: F.mono, fontSize: 11, color: pal.subtle }}>{a.readMin} мин</span>
        </div>
      </div>
    </div>
  );
}

// ── Personalized horizontal card ──────────────────────────────
function ForYouCard({ a, pal, onOpen }) {
  const m = L.catMeta(a.cat);
  const tint = tintOf(a.cat);
  return (
    <button onClick={() => onOpen(a.id)} className="press" style={{
      flexShrink: 0, width: 220, textAlign: 'left', cursor: 'pointer',
      background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 18, padding: 16,
      boxShadow: '0 2px 6px rgba(46,38,24,.05)',
      display: 'flex', flexDirection: 'column', gap: 10, minHeight: 168,
    }}>
      <div style={{ width: 40, height: 40, borderRadius: 11, background: tint.soft, color: tint.fg, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Icon name={m.icon} size={20} />
      </div>
      <div style={{ fontFamily: F.display, fontSize: 20, color: pal.ink, lineHeight: 1.1, letterSpacing: '-0.012em', flex: 1 }}>{a.title}</div>
      <div style={{ fontFamily: F.mono, fontSize: 11, color: pal.subtle }}>{a.readMin} мин · {m.short}</div>
    </button>
  );
}

// ════════════════════════════════════════════════════════════════
// Library landing
// ════════════════════════════════════════════════════════════════
function LibraryScreen({ pal, platform, onBack, onOpenArticle }) {
  const [q, setQ] = React.useState('');
  const [cat, setCat] = React.useState(null);     // active category filter
  const feat = L.featured();
  const forYou = L.FOR_YOU.map(L.byId).filter(a => a && a.id !== feat.id);
  const results = q.trim() ? L.search(q) : null;
  const catArticles = cat ? L.byCat(cat) : null;

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
            display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer', flexShrink: 0,
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 15, fontWeight: 600, color: pal.ink }}>Знания</div>
        </div>
        {/* Search */}
        <div style={{ padding: '4px 16px 12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, background: pal.sunk, borderRadius: 14, padding: '11px 14px' }}>
            <Icon name="magnifying-glass" size={18} color={pal.subtle} />
            <input
              value={q} onChange={(e) => setQ(e.target.value)}
              placeholder="Поиск по библиотеке"
              style={{ flex: 1, border: 'none', background: 'transparent', outline: 'none',
                fontFamily: F.body, fontSize: 14, color: pal.ink }}
            />
            {q && (
              <button onClick={() => setQ('')} style={{ border: 'none', background: 'transparent', color: pal.subtle, cursor: 'pointer', display: 'flex', padding: 0 }}>
                <Icon name="x-mark" size={16} />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Body */}
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 150 : 110, paddingBottom: 40,
      }}>
        {/* SEARCH RESULTS */}
        {results ? (
          <div style={{ padding: '6px 16px 0' }}>
            <Eyebrow style={{ color: pal.subtle, padding: '6px 4px 8px' }}>
              {results.length ? `Найдено · ${results.length}` : 'Ничего не найдено'}
            </Eyebrow>
            {results.length === 0 ? (
              <div style={{ fontFamily: F.body, fontSize: 13.5, color: pal.subtle, padding: '8px 4px', lineHeight: 1.5 }}>
                Попробуйте «семаглутид», «тошнота» или «белок».
              </div>
            ) : results.map((a, i) => (
              <ArticleRow key={a.id} a={a} pal={pal} onOpen={onOpenArticle} last={i === results.length - 1} />
            ))}
          </div>
        ) : cat ? (
          /* CATEGORY FILTER */
          <div style={{ padding: '6px 16px 0' }}>
            <button onClick={() => setCat(null)} className="press" style={{
              display: 'inline-flex', alignItems: 'center', gap: 6, marginBottom: 6,
              background: pal.sunk, border: 'none', borderRadius: 999, padding: '7px 14px', cursor: 'pointer',
              fontFamily: F.body, fontSize: 12.5, fontWeight: 500, color: pal.ink2,
            }}>
              <Icon name="chevron-left" size={14} /> Все темы
            </button>
            <div style={{ fontFamily: F.display, fontSize: 30, color: pal.ink, letterSpacing: '-0.018em', padding: '6px 4px 12px' }}>
              {L.catMeta(cat).label}
            </div>
            {catArticles.map((a, i) => (
              <ArticleRow key={a.id} a={a} pal={pal} onOpen={onOpenArticle} last={i === catArticles.length - 1} />
            ))}
          </div>
        ) : (
          /* BROWSE */
          <>
            {/* Lead */}
            <div style={{ padding: '8px 20px 14px' }}>
              <div style={{ fontFamily: F.display, fontSize: 32, color: pal.ink, lineHeight: 1.04, letterSpacing: '-0.018em' }}>
                Понимать свой <span style={{ fontStyle: 'italic', color: C.forest700 }}>ритм</span>.
              </div>
            </div>

            {/* Featured (personalized, photo) */}
            <div style={{ padding: '0 16px 18px' }}>
              <button onClick={() => onOpenArticle(feat.id)} className="press" style={{
                display: 'block', width: '100%', textAlign: 'left', cursor: 'pointer', border: 'none', padding: 0,
                borderRadius: 24, overflow: 'hidden', background: C.forest800,
                boxShadow: '0 8px 24px rgba(20,44,31,.18)',
              }}>
                <image-slot id={feat.photoSlot} shape="rect"
                  placeholder="Тёплое фото — перетащите"
                  style={{ display: 'block', width: '100%', height: '156px' }}></image-slot>
                <div style={{ padding: 20 }}>
                  <div style={{ display: 'inline-flex', alignItems: 'center', gap: 6, marginBottom: 10,
                    background: 'rgba(212,165,116,.2)', color: C.sand300, borderRadius: 999, padding: '5px 11px',
                    fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.04em' }}>
                    <Icon name="sparkles" size={13} /> {feat.eyebrow}
                  </div>
                  <div style={{ fontFamily: F.display, fontSize: 28, color: C.cream, lineHeight: 1.06, letterSpacing: '-0.018em' }}>{feat.title}</div>
                  <div style={{ fontFamily: F.body, fontSize: 13, color: 'rgba(246,241,234,.74)', lineHeight: 1.5, marginTop: 9 }}>{feat.dek}</div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 16,
                    fontFamily: F.body, fontSize: 13, fontWeight: 500, color: C.sand300 }}>
                    Читать <Icon name="arrow-right" size={15} />
                    <span style={{ marginLeft: 'auto', fontFamily: F.mono, fontSize: 11, color: 'rgba(246,241,234,.55)' }}>{feat.readMin} мин</span>
                  </div>
                </div>
              </button>
            </div>

            {/* For you rail */}
            <div style={{ paddingBottom: 18 }}>
              <div style={{ padding: '0 20px 10px' }}>
                <Eyebrow style={{ color: pal.subtle }}>Готовимся к 0,5 мг</Eyebrow>
              </div>
              <div className="ds-scroll" style={{ display: 'flex', gap: 12, overflowX: 'auto', padding: '0 16px 4px' }}>
                {forYou.map(a => <ForYouCard key={a.id} a={a} pal={pal} onOpen={onOpenArticle} />)}
              </div>
            </div>

            {/* Topic grid */}
            <div style={{ padding: '0 16px 18px' }}>
              <Eyebrow style={{ color: pal.subtle, padding: '0 4px 10px' }}>Темы</Eyebrow>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                {L.CATEGORIES.map(c => {
                  const tint = TINTS[c.tint];
                  const n = L.byCat(c.id).length;
                  return (
                    <button key={c.id} onClick={() => setCat(c.id)} className="press" style={{
                      display: 'flex', flexDirection: 'column', gap: 10, alignItems: 'flex-start',
                      background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 16, padding: 14,
                      cursor: 'pointer', textAlign: 'left', boxShadow: '0 2px 6px rgba(46,38,24,.05)',
                    }}>
                      <div style={{ width: 40, height: 40, borderRadius: 11, background: tint.soft, color: tint.fg, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                        <Icon name={c.icon} size={20} />
                      </div>
                      <div style={{ fontFamily: F.body, fontWeight: 600, fontSize: 14, color: pal.ink, lineHeight: 1.2 }}>{c.label}</div>
                      <div style={{ fontFamily: F.mono, fontSize: 11, color: pal.subtle }}>{n} {n === 1 ? 'материал' : n < 5 ? 'материала' : 'материалов'}</div>
                    </button>
                  );
                })}
              </div>
            </div>

            {/* All articles */}
            <div style={{ padding: '0 16px' }}>
              <Eyebrow style={{ color: pal.subtle, padding: '0 4px 6px' }}>Все материалы</Eyebrow>
              {L.ARTICLES.map((a, i) => (
                <ArticleRow key={a.id} a={a} pal={pal} onOpen={onOpenArticle} last={i === L.ARTICLES.length - 1} />
              ))}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════
// Article reader
// ════════════════════════════════════════════════════════════════
function ArticleReader({ pal, platform, article, onBack, onOpenArticle }) {
  const scrollRef = React.useRef(null);
  React.useEffect(() => { if (scrollRef.current) scrollRef.current.scrollTop = 0; }, [article && article.id]);
  if (!article) return null;
  const a = article;
  const m = L.catMeta(a.cat);
  const tint = tintOf(a.cat);

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      {/* Floating back */}
      <div style={{ position: 'absolute', top: platform === 'ios' ? 48 : 8, left: 12, zIndex: 30 }}>
        <button onClick={onBack} className="press" style={{
          width: 40, height: 40, borderRadius: 999, border: 'none', cursor: 'pointer',
          background: pal.bg, color: pal.ink2, boxShadow: '0 2px 8px rgba(46,38,24,.16)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <Icon name="chevron-left" size={20} />
        </button>
      </div>

      <div ref={scrollRef} className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden', paddingBottom: 48 }}>
        {/* Editorial hero — tinted band */}
        <div style={{ background: tint.soft, padding: platform === 'ios' ? '102px 20px 26px' : '64px 20px 26px' }}>
          <div style={{ display: 'inline-flex', alignItems: 'center', gap: 7, marginBottom: 14,
            color: tint.fg, fontFamily: F.body, fontSize: 11, fontWeight: 600, letterSpacing: '.1em', textTransform: 'uppercase' }}>
            <Icon name={m.icon} size={14} /> {a.eyebrow || m.label}
          </div>
          <div style={{ fontFamily: F.display, fontSize: 36, color: C.ink900, lineHeight: 1.06, letterSpacing: '-0.02em' }}>{a.title}</div>
          <div style={{ fontFamily: F.body, fontSize: 15, color: C.ink600, lineHeight: 1.5, marginTop: 12 }}>{a.dek}</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 16 }}>
            <span style={{ fontFamily: F.mono, fontSize: 12, color: tint.fg }}>{a.readMin} мин чтения</span>
          </div>
        </div>

        {/* Body */}
        <div style={{ padding: '6px 22px 8px' }}>
          {a.blocks.map((b, i) => {
            if (b.t === 'p') return (
              <p key={i} style={{ fontFamily: F.body, fontSize: 16, color: pal.ink2, lineHeight: 1.62, margin: '18px 0', textWrap: 'pretty' }}>{b.x}</p>
            );
            if (b.t === 'h') return (
              <h3 key={i} style={{ fontFamily: F.display, fontSize: 25, color: pal.ink, lineHeight: 1.12, letterSpacing: '-0.016em', margin: '30px 0 4px' }}>{b.x}</h3>
            );
            if (b.t === 'quote') return (
              <div key={i} style={{ margin: '28px 0 28px', paddingLeft: 18, borderLeft: `3px solid ${C.sand500}` }}>
                <div style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 24, color: C.forest700, lineHeight: 1.28, letterSpacing: '-0.01em' }}>{b.x}</div>
              </div>
            );
            if (b.t === 'key') return (
              <div key={i} style={{ margin: '26px 0', background: C.forest800, color: C.cream, borderRadius: 20, padding: 20, boxShadow: '0 8px 24px rgba(20,44,31,.18)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 9, marginBottom: 14 }}>
                  <div style={{ width: 30, height: 30, borderRadius: 9, background: 'rgba(212,165,116,.2)', color: C.sand300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Icon name="sparkles" size={16} />
                  </div>
                  <div style={{ fontFamily: F.body, fontSize: 11, fontWeight: 600, letterSpacing: '.12em', textTransform: 'uppercase', color: C.sand300 }}>{b.title}</div>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 11 }}>
                  {b.items.map((it, j) => (
                    <div key={j} style={{ display: 'grid', gridTemplateColumns: '20px 1fr', gap: 10, alignItems: 'start' }}>
                      <div style={{ marginTop: 2, color: C.sand300, display: 'flex' }}><Icon name="check" size={16} strokeWidth={2} /></div>
                      <div style={{ fontFamily: F.body, fontSize: 14.5, color: 'rgba(246,241,234,.92)', lineHeight: 1.45 }}>{it}</div>
                    </div>
                  ))}
                </div>
              </div>
            );
            return null;
          })}
        </div>

        {/* End rule */}
        <div style={{ textAlign: 'center', color: pal.placeholder, fontFamily: F.display, fontStyle: 'italic', fontSize: 22, padding: '8px 0 20px' }}>—</div>

        {/* Keep reading */}
        <div style={{ padding: '0 16px' }}>
          <Eyebrow style={{ color: pal.subtle, padding: '0 4px 6px' }}>Читать дальше</Eyebrow>
          <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, overflow: 'hidden', padding: '0 14px' }}>
            {L.ARTICLES.filter(x => x.id !== a.id).slice(0, 3).map((x, i, arr) => (
              <ArticleRow key={x.id} a={x} pal={pal} onOpen={onOpenArticle} last={i === arr.length - 1} />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { LibraryScreen, ArticleReader });
