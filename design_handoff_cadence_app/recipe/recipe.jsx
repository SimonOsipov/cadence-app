// Cadence · Рецепты — Library + Detail
// Exposes: RecipeLibrary, RecipeDetail (+ shared atoms on window)
//   RecipeLibrary props: pal, platform, recipes, todayTotals, onBack, onOpen, onCreate
//   RecipeDetail  props: pal, platform, recipe, onBack, onAddToDay

const R = window.RECIPES;

const TYPE_TINT = {
  breakfast: { soft: '#f3e8d6', fg: '#6b4a25' },
  lunch:     { soft: '#eaf0eb', fg: '#2d5f3f' },
  dinner:    { soft: '#e6ecf2', fg: '#41566b' },
  snack:     { soft: '#f4e4d8', fg: '#9a5a3c' },
};
const MACRO_C = { p: '#2d5f3f', c: '#d4a574', f: '#b8895a' };

// ── Macro split bar (by kcal contribution) ────────────────────
function RecipeMacroBar({ macro, pal, showGrams = true }) {
  const kp = macro.p * 4, kc = macro.c * 4, kf = macro.f * 9;
  const tot = (kp + kc + kf) || 1;
  const seg = [['p', kp], ['c', kc], ['f', kf]];
  return (
    <div>
      <div style={{ display: 'flex', height: 8, borderRadius: 999, overflow: 'hidden', background: pal.sunk }}>
        {seg.map(([k, v]) => <div key={k} style={{ width: `${(v / tot) * 100}%`, background: MACRO_C[k] }} />)}
      </div>
      {showGrams && (
        <div style={{ display: 'flex', gap: 16, marginTop: 9 }}>
          {[['p', 'Белок'], ['c', 'Углев'], ['f', 'Жиры']].map(([k, lbl]) => (
            <div key={k} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span style={{ width: 8, height: 8, borderRadius: 3, background: MACRO_C[k] }} />
              <span style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>{lbl}</span>
              <span style={{ fontFamily: F.mono, fontSize: 12.5, color: pal.ink2, fontVariantNumeric: 'tabular-nums' }}>{R.round(macro[k])} г</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Typographic recipe row ────────────────────────────────────
function RecipeRow({ r, pal, onOpen, last }) {
  const ps = R.perServing(r);
  const tint = TYPE_TINT[r.mealType];
  return (
    <div onClick={() => onOpen(r.id)} className="press" style={{
      display: 'grid', gridTemplateColumns: '1fr', gap: 0, padding: '14px 4px', cursor: 'pointer',
      borderBottom: last ? 'none' : `1px solid ${pal.hairline}`,
    }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, flexWrap: 'wrap' }}>
        <span style={{ fontFamily: F.display, fontSize: 20, color: pal.ink, lineHeight: 1.1, letterSpacing: '-0.012em' }}>{r.name}</span>
        {r.mine && <span style={{ fontFamily: F.body, fontSize: 10, fontWeight: 600, color: C.forest700, background: C.forest50, borderRadius: 999, padding: '2px 8px' }}>Моё</span>}
      </div>
      <div style={{ fontFamily: F.body, fontSize: 12.5, color: pal.subtle, marginTop: 3, lineHeight: 1.45,
        display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{r.dek}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 9, flexWrap: 'wrap' }}>
        <span style={{ fontFamily: F.body, fontSize: 11, fontWeight: 500, color: tint.fg, background: tint.soft, borderRadius: 999, padding: '3px 9px' }}>{R.typeLabel(r.mealType)}</span>
        <span style={{ fontFamily: F.mono, fontSize: 11.5, color: pal.ink2 }}>{R.round(ps.kcal)} ккал</span>
        <span style={{ width: 3, height: 3, borderRadius: 999, background: pal.placeholder }} />
        <span style={{ fontFamily: F.mono, fontSize: 11.5, color: C.forest700 }}>{R.round(ps.p)} г белка</span>
        <span style={{ marginLeft: 'auto', fontFamily: F.body, fontSize: 11.5, color: pal.subtle }}>{R.totalTime(r)} мин</span>
      </div>
    </div>
  );
}

function FilterChip({ label, on, onClick, pal }) {
  return (
    <button onClick={onClick} className="press" style={{
      flexShrink: 0, fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer',
      padding: '7px 13px', borderRadius: 999,
      background: on ? C.forest700 : 'transparent', color: on ? C.cream : pal.ink2,
      border: `1px solid ${on ? C.forest700 : pal.border}`,
    }}>{label}</button>
  );
}

// ════════════════════════════════════════════════════════════════
// Library
// ════════════════════════════════════════════════════════════════
function RecipeLibrary({ pal, platform, recipes, todayTotals, onBack, onOpen, onCreate }) {
  const [tag, setTag] = React.useState(null);
  const [mealType, setMealType] = React.useState(null);
  const { pick, rem } = R.suggest(recipes, todayTotals);
  const pickPs = R.perServing(pick);
  const list = R.filter(recipes, { tag, mealType });
  const mine = list.filter(r => r.mine);
  const rest = list.filter(r => !r.mine);

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      {/* Top bar */}
      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 20, paddingTop: platform === 'ios' ? 48 : 8, background: pal.bg, borderBottom: `1px solid ${pal.hairline}` }}>
        <div style={{ padding: '8px 16px 10px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <button onClick={onBack} className="press" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 15, fontWeight: 600, color: pal.ink }}>Рецепты</div>
          <button onClick={onCreate} className="press" style={{ marginLeft: 'auto', height: 40, padding: '0 14px', borderRadius: 999, border: 'none', background: C.forest700, color: C.cream, display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer', fontFamily: F.body, fontSize: 13, fontWeight: 500 }}>
            <Icon name="plus" size={16} strokeWidth={2} /> Создать
          </button>
        </div>
      </div>

      <div className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden', paddingTop: platform === 'ios' ? 106 : 66, paddingBottom: 40 }}>
        {/* Personalized featured */}
        <div style={{ padding: '8px 16px 14px' }}>
          <button onClick={() => onOpen(pick.id)} className="press" style={{
            display: 'block', width: '100%', textAlign: 'left', cursor: 'pointer', border: 'none', padding: 0,
            borderRadius: 24, overflow: 'hidden', background: C.forest800, boxShadow: '0 8px 24px rgba(20,44,31,.18)',
          }}>
            <image-slot id={pick.photoSlot || 'recipe-feat'} shape="rect" placeholder="Тёплое фото блюда — перетащите"
              style={{ display: 'block', width: '100%', height: '150px' }}></image-slot>
            <div style={{ padding: 18 }}>
              <div style={{ display: 'inline-flex', alignItems: 'center', gap: 6, marginBottom: 10, background: 'rgba(212,165,116,.2)', color: C.sand300, borderRadius: 999, padding: '5px 11px', fontFamily: F.body, fontSize: 11, fontWeight: 500 }}>
                <Icon name="sparkles" size={13} /> Для вас сегодня
              </div>
              <div style={{ fontFamily: F.display, fontSize: 27, color: C.cream, lineHeight: 1.06, letterSpacing: '-0.018em' }}>{pick.name}</div>
              <div style={{ fontFamily: F.body, fontSize: 13, color: 'rgba(246,241,234,.74)', lineHeight: 1.5, marginTop: 8 }}>
                Осталось <span style={{ fontFamily: F.mono, color: C.sand300 }}>{R.round(rem.p)} г</span> белка сегодня — порция закроет <span style={{ fontFamily: F.mono, color: C.sand300 }}>{R.round(pickPs.p)} г</span>.
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 14, fontFamily: F.body, fontSize: 13, fontWeight: 500, color: C.sand300 }}>
                Открыть рецепт <Icon name="arrow-right" size={15} />
                <span style={{ marginLeft: 'auto', fontFamily: F.mono, fontSize: 11, color: 'rgba(246,241,234,.55)' }}>{R.round(pickPs.kcal)} ккал · {R.totalTime(pick)} мин</span>
              </div>
            </div>
          </button>
        </div>

        {/* Filters */}
        <div className="ds-scroll" style={{ display: 'flex', gap: 8, overflowX: 'auto', padding: '0 16px 10px' }}>
          <FilterChip label="Все" on={!mealType} onClick={() => setMealType(null)} pal={pal} />
          {R.MEAL_TYPES.map(t => <FilterChip key={t.id} label={t.label} on={mealType === t.id} onClick={() => setMealType(mealType === t.id ? null : t.id)} pal={pal} />)}
        </div>
        <div className="ds-scroll" style={{ display: 'flex', gap: 8, overflowX: 'auto', padding: '0 16px 14px' }}>
          <FilterChip label="Любые" on={!tag} onClick={() => setTag(null)} pal={pal} />
          {R.TAGS.map(t => <FilterChip key={t.id} label={t.label} on={tag === t.id} onClick={() => setTag(tag === t.id ? null : t.id)} pal={pal} />)}
        </div>

        {/* Mine */}
        {mine.length > 0 && (
          <div style={{ padding: '0 16px 8px' }}>
            <Eyebrow style={{ color: pal.subtle, padding: '0 4px 4px' }}>Мои рецепты</Eyebrow>
            {mine.map((r, i) => <RecipeRow key={r.id} r={r} pal={pal} onOpen={onOpen} last={i === mine.length - 1} />)}
          </div>
        )}

        {/* Library */}
        <div style={{ padding: '0 16px' }}>
          <Eyebrow style={{ color: pal.subtle, padding: '0 4px 4px' }}>{mine.length ? 'Готовые рецепты' : 'Все рецепты'}</Eyebrow>
          {rest.length === 0 ? (
            <div style={{ fontFamily: F.body, fontSize: 13, color: pal.subtle, padding: '12px 4px' }}>Под фильтр ничего не подошло.</div>
          ) : rest.map((r, i) => <RecipeRow key={r.id} r={r} pal={pal} onOpen={onOpen} last={i === rest.length - 1} />)}
        </div>
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════
// Detail
// ════════════════════════════════════════════════════════════════
function RecipeDetail({ pal, platform, recipe, onBack, onAddToDay }) {
  const [mode, setMode] = React.useState('serving');   // 'serving' | 'whole'
  const [portions, setPortions] = React.useState(1);
  if (!recipe) return null;
  const tint = TYPE_TINT[recipe.mealType];
  const whole = R.recipeTotals(recipe);
  const ps = R.perServing(recipe);
  const macro = mode === 'serving' ? ps : whole;

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div style={{ position: 'absolute', top: platform === 'ios' ? 48 : 8, left: 12, zIndex: 30 }}>
        <button onClick={onBack} className="press" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', cursor: 'pointer', background: pal.bg, color: pal.ink2, boxShadow: '0 2px 8px rgba(46,38,24,.16)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Icon name="chevron-left" size={20} />
        </button>
      </div>

      <div className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden', paddingBottom: 110 }}>
        {/* Hero */}
        <div style={{ background: tint.soft, padding: platform === 'ios' ? '100px 20px 24px' : '62px 20px 24px' }}>
          <div style={{ display: 'inline-flex', alignItems: 'center', gap: 7, marginBottom: 12, color: tint.fg, fontFamily: F.body, fontSize: 11, fontWeight: 600, letterSpacing: '.1em', textTransform: 'uppercase' }}>
            {R.typeLabel(recipe.mealType)}{recipe.mine ? ' · моё' : ''}
          </div>
          <div style={{ fontFamily: F.display, fontSize: 34, color: C.ink900, lineHeight: 1.06, letterSpacing: '-0.02em' }}>{recipe.name}</div>
          <div style={{ fontFamily: F.body, fontSize: 14.5, color: C.ink600, lineHeight: 1.5, marginTop: 10 }}>{recipe.dek}</div>
          <div style={{ display: 'flex', gap: 16, marginTop: 16, flexWrap: 'wrap' }}>
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontFamily: F.body, fontSize: 12.5, color: tint.fg }}><Icon name="clock" size={15} /> {R.totalTime(recipe)} мин</span>
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontFamily: F.body, fontSize: 12.5, color: tint.fg }}><Icon name="cake" size={15} /> {recipe.servings} порц.</span>
            {(recipe.tags || []).map(t => <span key={t} style={{ fontFamily: F.body, fontSize: 11.5, color: tint.fg, background: 'rgba(255,255,255,.5)', borderRadius: 999, padding: '3px 10px' }}>{R.tagLabel(t)}</span>)}
          </div>
        </div>

        {/* Macros */}
        <div style={{ padding: '16px 16px 6px' }}>
          <div style={{ background: pal.paper, borderRadius: 18, padding: 16, border: `1px solid ${pal.hairline}`, boxShadow: '0 2px 6px rgba(46,38,24,.05)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
              <Eyebrow style={{ color: pal.subtle }}>Макросы</Eyebrow>
              <div style={{ display: 'flex', gap: 3, background: pal.sunk, padding: 3, borderRadius: 999 }}>
                {[['serving', 'На порцию'], ['whole', 'Всё']].map(([id, lbl]) => (
                  <button key={id} onClick={() => setMode(id)} className="press" style={{ padding: '6px 12px', borderRadius: 999, border: 'none', cursor: 'pointer', background: mode === id ? C.forest700 : 'transparent', color: mode === id ? C.cream : pal.muted, fontFamily: F.body, fontSize: 12, fontWeight: 500 }}>{lbl}</button>
                ))}
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, marginBottom: 14 }}>
              <span style={{ fontFamily: F.mono, fontSize: 34, fontWeight: 500, color: pal.ink, letterSpacing: '-0.03em' }}>{R.round(macro.kcal)}</span>
              <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 16, color: pal.muted }}>ккал</span>
              <span style={{ marginLeft: 'auto', fontFamily: F.body, fontSize: 12, color: pal.subtle }}>{mode === 'serving' ? '1 порция' : `${recipe.servings} порц.`}</span>
            </div>
            <RecipeMacroBar macro={macro} pal={pal} />
          </div>
        </div>

        {/* Ingredients */}
        <div style={{ padding: '10px 16px 6px' }}>
          <Eyebrow style={{ color: pal.subtle, padding: '0 4px 8px' }}>Ингредиенты · {recipe.servings} порц.</Eyebrow>
          <div style={{ background: pal.paper, borderRadius: 18, border: `1px solid ${pal.hairline}`, overflow: 'hidden' }}>
            {recipe.ingredients.map((it, i) => {
              const m = R.ingMacros(it.id, it.g);
              return (
                <div key={i} style={{ display: 'grid', gridTemplateColumns: '1fr auto auto', gap: 12, alignItems: 'center', padding: '12px 14px', borderBottom: i < recipe.ingredients.length - 1 ? `1px solid ${pal.hairline}` : 'none' }}>
                  <span style={{ fontFamily: F.body, fontSize: 14, color: pal.ink }}>{R.ingMeta(it.id).name}</span>
                  <span style={{ fontFamily: F.mono, fontSize: 13, color: pal.ink2, fontVariantNumeric: 'tabular-nums' }}>{it.g} г</span>
                  <span style={{ fontFamily: F.mono, fontSize: 12, color: pal.subtle, minWidth: 54, textAlign: 'right' }}>{R.round(m.kcal)} ккал</span>
                </div>
              );
            })}
          </div>
        </div>

        {/* Steps */}
        <div style={{ padding: '12px 16px 6px' }}>
          <Eyebrow style={{ color: pal.subtle, padding: '0 4px 10px' }}>Приготовление</Eyebrow>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            {recipe.steps.map((s, i) => (
              <div key={i} style={{ display: 'grid', gridTemplateColumns: '30px 1fr', gap: 12, alignItems: 'start' }}>
                <div style={{ width: 30, height: 30, borderRadius: 999, background: C.forest50, color: C.forest700, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: F.mono, fontSize: 14, fontWeight: 500 }}>{i + 1}</div>
                <div style={{ fontFamily: F.body, fontSize: 14.5, color: pal.ink2, lineHeight: 1.55, paddingTop: 3, textWrap: 'pretty' }}>{s}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Add to day */}
      <div style={{ position: 'absolute', left: 0, right: 0, bottom: 0, zIndex: 20, padding: '12px 16px 28px', background: `linear-gradient(180deg, rgba(0,0,0,0) 0%, ${pal.bg} 28%)` }}>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 0, background: pal.paper, border: `1px solid ${pal.border}`, borderRadius: 999, padding: 3 }}>
            <button onClick={() => setPortions(p => Math.max(1, p - 1))} className="press" style={{ width: 38, height: 38, borderRadius: 999, border: 'none', background: 'transparent', color: pal.ink, cursor: 'pointer', fontSize: 20 }}>−</button>
            <span style={{ minWidth: 34, textAlign: 'center', fontFamily: F.mono, fontSize: 16, color: pal.ink }}>{portions}</span>
            <button onClick={() => setPortions(p => Math.min(6, p + 1))} className="press" style={{ width: 38, height: 38, borderRadius: 999, border: 'none', background: 'transparent', color: pal.ink, cursor: 'pointer', fontSize: 20 }}>+</button>
          </div>
          <button onClick={() => onAddToDay(recipe, portions)} className="press" style={{
            flex: 1, padding: '15px 18px', borderRadius: 999, border: 'none', cursor: 'pointer',
            background: C.forest700, color: C.cream, fontFamily: F.body, fontSize: 15, fontWeight: 500,
            display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
          }}>
            Добавить в день <Icon name="plus" size={16} strokeWidth={2} />
          </button>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { RecipeLibrary, RecipeDetail, RecipeMacroBar, RECIPE_TYPE_TINT: TYPE_TINT });
