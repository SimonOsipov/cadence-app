// Cadence · Рецепты — Builder (working) + ingredient picker
// Exposes: RecipeBuilder
//   props: pal, platform, onCancel, onSave(recipe)

const RB = window.RECIPES;

// ── Ingredient picker sheet ───────────────────────────────────
function IngredientPicker({ pal, onClose, onAdd }) {
  const [q, setQ] = React.useState('');
  const [sel, setSel] = React.useState(null);
  const [grams, setGrams] = React.useState(100);
  const list = RB.ING.filter(i => i.name.toLowerCase().includes(q.trim().toLowerCase()));

  const pick = (i) => { setSel(i.id); setGrams(100); };
  const selMeta = sel ? RB.ingMeta(sel) : null;
  const m = sel ? RB.ingMacros(sel, grams) : null;

  return (
    <div style={{ position: 'absolute', inset: 0, zIndex: 96 }}>
      <div className="scrim" onClick={onClose} style={{ position: 'absolute', inset: 0, background: 'rgba(20,44,31,.35)', backdropFilter: 'blur(4px)' }} />
      <div className="sheet" style={{ position: 'absolute', left: 0, right: 0, bottom: 0, height: '78%', display: 'flex', flexDirection: 'column', background: pal.bg, borderTopLeftRadius: 28, borderTopRightRadius: 28, boxShadow: '0 -18px 40px rgba(0,0,0,.18)' }}>
        <div style={{ width: 38, height: 4, borderRadius: 999, background: pal.border, margin: '12px auto 8px', flexShrink: 0 }} />
        <div style={{ padding: '4px 20px 10px', display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0 }}>
          <div style={{ fontFamily: F.display, fontSize: 24, color: pal.ink, letterSpacing: '-0.018em' }}>Ингредиент</div>
          <button onClick={onClose} className="press" style={{ width: 34, height: 34, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' }}>
            <Icon name="x-mark" size={17} />
          </button>
        </div>
        {/* search */}
        <div style={{ padding: '0 16px 10px', flexShrink: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, background: pal.sunk, borderRadius: 14, padding: '11px 14px' }}>
            <Icon name="magnifying-glass" size={18} color={pal.subtle} />
            <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Найти продукт" style={{ flex: 1, border: 'none', background: 'transparent', outline: 'none', fontFamily: F.body, fontSize: 14, color: pal.ink }} />
          </div>
        </div>
        {/* list */}
        <div className="ds-scroll" style={{ overflowY: 'auto', flex: 1, padding: '0 16px' }}>
          {list.map(i => {
            const on = sel === i.id;
            return (
              <button key={i.id} onClick={() => pick(i)} className="press" style={{
                display: 'grid', gridTemplateColumns: '1fr auto', gap: 10, alignItems: 'center', width: '100%', textAlign: 'left',
                padding: '12px 12px', borderRadius: 12, cursor: 'pointer', border: 'none', marginBottom: 4,
                background: on ? C.forest50 : 'transparent',
              }}>
                <div>
                  <div style={{ fontFamily: F.body, fontSize: 14, fontWeight: on ? 600 : 400, color: on ? C.forest800 : pal.ink }}>{i.name}</div>
                  <div style={{ fontFamily: F.mono, fontSize: 11, color: pal.subtle, marginTop: 1 }}>{i.kcal} ккал · {i.p} г белка / 100 г</div>
                </div>
                {on && <Icon name="check-circle" size={20} color={C.forest700} />}
              </button>
            );
          })}
          {list.length === 0 && <div style={{ fontFamily: F.body, fontSize: 13, color: pal.subtle, padding: '12px 4px' }}>Ничего не найдено.</div>}
        </div>
        {/* footer grams + add */}
        {sel && (
          <div style={{ padding: '12px 16px', paddingBottom: 26, flexShrink: 0, borderTop: `1px solid ${pal.hairline}` }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <span style={{ fontFamily: F.body, fontSize: 13, color: pal.muted }}>{selMeta.name}</span>
              <span style={{ fontFamily: F.mono, fontSize: 13, color: C.forest700 }}>{RB.round(m.kcal)} ккал · {RB.round1(m.p)} г белка</span>
            </div>
            <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
              <div style={{ display: 'flex', alignItems: 'center', background: pal.paper, border: `1px solid ${pal.border}`, borderRadius: 999, padding: 3 }}>
                <button onClick={() => setGrams(g => Math.max(5, g - 10))} className="press" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', background: 'transparent', color: pal.ink, cursor: 'pointer', fontSize: 20 }}>−</button>
                <span style={{ minWidth: 64, textAlign: 'center', fontFamily: F.mono, fontSize: 16, color: pal.ink }}>{grams} г</span>
                <button onClick={() => setGrams(g => Math.min(600, g + 10))} className="press" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', background: 'transparent', color: pal.ink, cursor: 'pointer', fontSize: 20 }}>+</button>
              </div>
              <button onClick={() => onAdd(sel, grams)} className="press" style={{ flex: 1, padding: '14px', borderRadius: 999, border: 'none', background: C.forest700, color: C.cream, cursor: 'pointer', fontFamily: F.body, fontSize: 14, fontWeight: 500 }}>Добавить</button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ════════════════════════════════════════════════════════════════
// Builder screen
// ════════════════════════════════════════════════════════════════
function RecipeBuilder({ pal, platform, onCancel, onSave }) {
  const [name, setName] = React.useState('');
  const [mealType, setMealType] = React.useState('lunch');
  const [tags, setTags] = React.useState([]);
  const [servings, setServings] = React.useState(2);
  const [time, setTime] = React.useState(20);
  const [ings, setIngs] = React.useState([]);            // [{id, g}]
  const [steps, setSteps] = React.useState(['']);
  const [pickerOpen, setPickerOpen] = React.useState(false);

  const draft = { ingredients: ings, servings };
  const whole = RB.recipeTotals(draft);
  const ps = RB.perServing(draft);
  const canSave = name.trim() && ings.length > 0;

  const toggleTag = (id) => setTags(t => t.includes(id) ? t.filter(x => x !== id) : [...t, id]);
  const setGrams = (idx, g) => setIngs(arr => arr.map((it, i) => i === idx ? { ...it, g: Math.max(5, g) } : it));
  const removeIng = (idx) => setIngs(arr => arr.filter((_, i) => i !== idx));
  const setStep = (idx, v) => setSteps(arr => arr.map((s, i) => i === idx ? v : s));
  const addStep = () => setSteps(arr => [...arr, '']);
  const removeStep = (idx) => setSteps(arr => arr.length > 1 ? arr.filter((_, i) => i !== idx) : arr);

  const save = () => {
    if (!canSave) return;
    onSave({
      id: `u-${Date.now()}`, name: name.trim(), mealType, tags, servings,
      prepMin: time, cookMin: 0, photo: false, mine: true,
      dek: `${RB.round(ps.p)} г белка · ${RB.round(ps.kcal)} ккал на порцию.`,
      ingredients: ings.map(it => ({ id: it.id, g: it.g })),
      steps: steps.map(s => s.trim()).filter(Boolean),
    });
  };

  const Label = ({ children }) => <Eyebrow style={{ color: pal.subtle, padding: '0 4px 10px' }}>{children}</Eyebrow>;

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      {/* Top bar */}
      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 20, paddingTop: platform === 'ios' ? 48 : 8, background: pal.bg, borderBottom: `1px solid ${pal.hairline}` }}>
        <div style={{ padding: '8px 16px 10px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <button onClick={onCancel} className="press" style={{ width: 40, height: 40, borderRadius: 999, border: 'none', background: pal.sunk, color: pal.ink2, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' }}>
            <Icon name="x-mark" size={20} />
          </button>
          <div style={{ fontFamily: F.body, fontSize: 15, fontWeight: 600, color: pal.ink }}>Новый рецепт</div>
        </div>
      </div>

      <div className="ds-scroll" style={{ height: '100%', overflowY: 'auto', overflowX: 'hidden', paddingTop: platform === 'ios' ? 104 : 64, paddingBottom: 100 }}>
        {/* Name */}
        <div style={{ padding: '8px 16px 18px' }}>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Название рецепта"
            style={{ width: '100%', boxSizing: 'border-box', border: 'none', borderBottom: `1px solid ${pal.border}`, background: 'transparent', outline: 'none', fontFamily: F.display, fontSize: 28, color: pal.ink, letterSpacing: '-0.018em', padding: '0 4px 10px' }} />
        </div>

        {/* Meal type + tags */}
        <div style={{ padding: '0 16px 18px' }}>
          <Label>Тип приёма</Label>
          <div className="ds-scroll" style={{ display: 'flex', gap: 8, overflowX: 'auto', paddingBottom: 4 }}>
            {RB.MEAL_TYPES.map(t => (
              <button key={t.id} onClick={() => setMealType(t.id)} className="press" style={{ flexShrink: 0, fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer', padding: '8px 14px', borderRadius: 999, background: mealType === t.id ? C.forest700 : 'transparent', color: mealType === t.id ? C.cream : pal.ink2, border: `1px solid ${mealType === t.id ? C.forest700 : pal.border}` }}>{t.label}</button>
            ))}
          </div>
          <div style={{ height: 10 }} />
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
            {RB.TAGS.map(t => {
              const on = tags.includes(t.id);
              return <button key={t.id} onClick={() => toggleTag(t.id)} className="press" style={{ fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer', padding: '8px 14px', borderRadius: 999, background: on ? C.sand500 : 'transparent', color: on ? C.ink900 : pal.ink2, border: `1px solid ${on ? C.sand500 : pal.border}` }}>{t.label}</button>;
            })}
          </div>
        </div>

        {/* Servings + time */}
        <div style={{ padding: '0 16px 18px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
          {[{ lbl: 'Порций', val: servings, set: setServings, min: 1, max: 8, step: 1, unit: '' },
            { lbl: 'Время, мин', val: time, set: setTime, min: 5, max: 120, step: 5, unit: '' }].map((row, i) => (
            <div key={i} style={{ background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 16, padding: '12px 14px' }}>
              <div style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginBottom: 8 }}>{row.lbl}</div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <button onClick={() => row.set(v => Math.max(row.min, v - row.step))} className="press" style={{ width: 34, height: 34, borderRadius: 999, border: `1px solid ${pal.border}`, background: 'transparent', color: pal.ink, cursor: 'pointer', fontSize: 18 }}>−</button>
                <span style={{ fontFamily: F.mono, fontSize: 20, color: pal.ink }}>{row.val}</span>
                <button onClick={() => row.set(v => Math.min(row.max, v + row.step))} className="press" style={{ width: 34, height: 34, borderRadius: 999, border: 'none', background: C.forest700, color: C.cream, cursor: 'pointer', fontSize: 18 }}>+</button>
              </div>
            </div>
          ))}
        </div>

        {/* Ingredients */}
        <div style={{ padding: '0 16px 18px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px 10px' }}>
            <Eyebrow style={{ color: pal.subtle }}>Ингредиенты</Eyebrow>
            <button onClick={() => setPickerOpen(true)} style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
              <Icon name="plus" size={13} strokeWidth={2} /> Добавить
            </button>
          </div>
          {ings.length === 0 ? (
            <button onClick={() => setPickerOpen(true)} className="press" style={{ width: '100%', padding: '20px', borderRadius: 16, border: `1.5px dashed ${pal.border}`, background: 'transparent', color: pal.subtle, cursor: 'pointer', fontFamily: F.body, fontSize: 13.5, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8 }}>
              <Icon name="plus" size={18} /> Добавьте первый ингредиент
            </button>
          ) : (
            <div style={{ background: pal.paper, borderRadius: 16, border: `1px solid ${pal.hairline}`, overflow: 'hidden' }}>
              {ings.map((it, idx) => {
                const meta = RB.ingMeta(it.id);
                const m = RB.ingMacros(it.id, it.g);
                return (
                  <div key={idx} style={{ display: 'grid', gridTemplateColumns: '1fr auto auto', gap: 10, alignItems: 'center', padding: '10px 12px', borderBottom: idx < ings.length - 1 ? `1px solid ${pal.hairline}` : 'none' }}>
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontFamily: F.body, fontSize: 13.5, color: pal.ink, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{meta.name}</div>
                      <div style={{ fontFamily: F.mono, fontSize: 11, color: pal.subtle }}>{RB.round(m.kcal)} ккал · {RB.round1(m.p)} б</div>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', background: pal.sunk, borderRadius: 999, padding: 2 }}>
                      <button onClick={() => setGrams(idx, it.g - 10)} className="press" style={{ width: 28, height: 28, borderRadius: 999, border: 'none', background: 'transparent', color: pal.ink2, cursor: 'pointer', fontSize: 16 }}>−</button>
                      <span style={{ minWidth: 44, textAlign: 'center', fontFamily: F.mono, fontSize: 12.5, color: pal.ink }}>{it.g} г</span>
                      <button onClick={() => setGrams(idx, it.g + 10)} className="press" style={{ width: 28, height: 28, borderRadius: 999, border: 'none', background: 'transparent', color: pal.ink2, cursor: 'pointer', fontSize: 16 }}>+</button>
                    </div>
                    <button onClick={() => removeIng(idx)} className="press" style={{ width: 30, height: 30, borderRadius: 999, border: 'none', background: 'transparent', color: pal.placeholder, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <Icon name="x-mark" size={15} />
                    </button>
                  </div>
                );
              })}
            </div>
          )}

          {/* live macro total */}
          {ings.length > 0 && (
            <div style={{ marginTop: 10, background: C.forest800, color: C.cream, borderRadius: 16, padding: 14 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
                <span style={{ fontFamily: F.body, fontSize: 11, fontWeight: 600, letterSpacing: '.1em', textTransform: 'uppercase', color: C.sand300 }}>На порцию</span>
                <span><span style={{ fontFamily: F.mono, fontSize: 22, fontWeight: 500 }}>{RB.round(ps.kcal)}</span> <span style={{ fontFamily: F.display, fontStyle: 'italic', fontSize: 14, color: C.sand300 }}>ккал</span></span>
              </div>
              <RecipeMacroBar macro={ps} pal={{ ...pal, sunk: 'rgba(246,241,234,.14)', subtle: 'rgba(246,241,234,.6)', ink2: C.cream }} />
            </div>
          )}
        </div>

        {/* Steps */}
        <div style={{ padding: '0 16px 8px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px 10px' }}>
            <Eyebrow style={{ color: pal.subtle }}>Приготовление</Eyebrow>
            <button onClick={addStep} style={{ background: 'none', border: 'none', cursor: 'pointer', fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
              <Icon name="plus" size={13} strokeWidth={2} /> Шаг
            </button>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {steps.map((s, idx) => (
              <div key={idx} style={{ display: 'grid', gridTemplateColumns: '30px 1fr auto', gap: 10, alignItems: 'start' }}>
                <div style={{ width: 30, height: 30, borderRadius: 999, background: C.forest50, color: C.forest700, display: 'flex', alignItems: 'center', justifyContent: 'center', fontFamily: F.mono, fontSize: 14, fontWeight: 500, marginTop: 4 }}>{idx + 1}</div>
                <textarea value={s} onChange={(e) => setStep(idx, e.target.value)} rows={2} placeholder="Опишите шаг"
                  style={{ width: '100%', boxSizing: 'border-box', resize: 'none', border: `1px solid ${pal.border}`, borderRadius: 12, padding: '10px 12px', background: pal.paper, outline: 'none', fontFamily: F.body, fontSize: 14, color: pal.ink, lineHeight: 1.45 }} />
                {steps.length > 1 && (
                  <button onClick={() => removeStep(idx)} className="press" style={{ width: 30, height: 30, borderRadius: 999, border: 'none', background: 'transparent', color: pal.placeholder, cursor: 'pointer', marginTop: 6, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Icon name="x-mark" size={15} />
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Save */}
      <div style={{ position: 'absolute', left: 0, right: 0, bottom: 0, zIndex: 20, padding: '12px 16px 28px', background: `linear-gradient(180deg, rgba(0,0,0,0) 0%, ${pal.bg} 30%)` }}>
        <button onClick={save} disabled={!canSave} className={canSave ? 'press' : ''} style={{
          width: '100%', padding: '15px 20px', borderRadius: 999, border: 'none', cursor: canSave ? 'pointer' : 'default', opacity: canSave ? 1 : 0.4,
          background: C.forest700, color: C.cream, fontFamily: F.body, fontSize: 15, fontWeight: 500,
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
        }}>
          Сохранить рецепт <Icon name="check" size={16} strokeWidth={2} />
        </button>
      </div>

      {pickerOpen && (
        <IngredientPicker pal={pal} onClose={() => setPickerOpen(false)}
          onAdd={(id, g) => { setIngs(arr => [...arr, { id, g }]); setPickerOpen(false); }} />
      )}
    </div>
  );
}

Object.assign(window, { RecipeBuilder });
