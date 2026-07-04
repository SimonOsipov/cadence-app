// Cadence — Today-screen meal components.
// Exposes: MealHero, MealTimeline, TodayMeals
//
// MealHero    — sand-warm floating hero, the CTA to log a meal. Mirrors the
//               forest dose hero but in a warmer color register.
// MealTimeline— slim horizontal strip near the top of Today, four slots:
//               breakfast / lunch / snack / dinner — filled as meals are logged.
// TodayMeals  — paper card replacing the old fuel strip. Daily totals + per-meal
//               list, opens Nutrition Today on tap.

// ── Helpers ─────────────────────────────────────────────────────────

function suggestNextMeal(meals, now) {
  // Very loose mapping by clock — purely cosmetic for the demo.
  const hour = parseInt((now || '08:00').split(':')[0], 10) +
    ((now || '').includes('pm') && !(now || '').startsWith('12') ? 12 : 0);
  if (meals.length === 0) return { eyebrow: 'Начнём день', title: 'Завтрак?', meta: 'Целимся в 35 г белка.' };
  if (meals.length === 1) return { eyebrow: 'Следующий приём', title: 'Скоро обед', meta: 'Подсказать, что собрать?' };
  if (meals.length === 2) return { eyebrow: 'Следующий приём', title: 'Перекус?', meta: 'Немного белка, немного фруктов.' };
  return { eyebrow: 'Последний шанс', title: 'Лёгкий ужин?', meta: 'Запас есть — без излишеств.' };
}

// ── MealHero — sand-warm CTA ────────────────────────────────────────

function MealHero({ pal, meals, totals, now, onLogMeal, suggestion, onOpenRecipes }) {
  const t = MEAL_TARGETS;
  const kcalLeft = Math.max(0, t.kcal - totals.kcal);
  const proteinLeft = Math.max(0, t.protein - Math.round(totals.p));
  const s = suggestion || suggestNextMeal(meals, now);

  // Sand-warm background; we keep dark text for legibility.
  const bg   = C.sand500;
  const ink  = C.ink900;
  const sub  = 'rgba(26,26,26,.62)';
  const accentText = '#3a2a14';

  return (
    <div style={{
      background: bg, color: ink,
      borderRadius: 24, padding: 22,
      boxShadow: '0 8px 22px rgba(184,135,76,.22)',
      position: 'relative', overflow: 'hidden',
    }}>
      {/* soft sun highlight */}
      <div aria-hidden style={{
        position: 'absolute', top: -32, right: -32, width: 140, height: 140,
        borderRadius: '50%',
        background: 'radial-gradient(circle, rgba(255,240,210,.55) 0%, rgba(212,165,116,0) 70%)',
      }} />

      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start',
        marginBottom: 12, gap: 12, position: 'relative',
      }}>
        <span style={{
          display: 'inline-flex', alignItems: 'center',
          padding: '4px 10px', borderRadius: 999,
          background: 'rgba(26,26,26,.12)',
          fontFamily: F.body, fontSize: 11, fontWeight: 500,
          color: accentText, letterSpacing: '.02em',
        }}>{s.eyebrow}</span>
        <span style={{ fontFamily: F.mono, fontSize: 11, color: sub }}>{now}</span>
      </div>

      <div style={{
        fontFamily: F.display, fontSize: 36, lineHeight: 1.04,
        letterSpacing: '-0.018em', color: ink, marginBottom: 10,
        position: 'relative',
      }}>
        {(() => {
          const words = s.title.split(' ');
          if (words.length === 1) {
            return <span style={{ fontStyle: 'italic', color: accentText }}>{s.title}</span>;
          }
          const last = words[words.length - 1];
          const head = words.slice(0, -1).join(' ');
          return <>{head}<br/><span style={{ fontStyle: 'italic', color: accentText }}>{last}</span></>;
        })()}
      </div>

      <div style={{
        fontFamily: F.body, fontSize: 13, color: 'rgba(26,26,26,.72)',
        lineHeight: 1.5, marginBottom: 16, position: 'relative',
      }}>
        <span style={{ fontFamily: F.mono, fontWeight: 500, fontVariantNumeric: 'tabular-nums' }}>{kcalLeft.toLocaleString()}</span> ккал
        · <span style={{ fontFamily: F.mono, fontWeight: 500, fontVariantNumeric: 'tabular-nums' }}>{proteinLeft}</span> г белка осталось. {s.meta}
      </div>

      <button
        className="press"
        onClick={onLogMeal}
        style={{
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
          width: '100%', padding: '14px 20px', borderRadius: 999,
          background: C.forest700, color: C.cream,
          border: 'none', fontFamily: F.body, fontSize: 14, fontWeight: 500,
          cursor: 'pointer', position: 'relative',
        }}>
        Записать приём пищи
        <span style={{ fontFamily: F.display, fontStyle: 'italic' }}>→</span>
      </button>
      {onOpenRecipes && (
        <button className="press" onClick={onOpenRecipes} style={{
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7,
          width: '100%', marginTop: 8, padding: '11px 16px', borderRadius: 999,
          background: 'rgba(26,26,26,.08)', border: 'none', color: '#3a2a14',
          fontFamily: F.body, fontSize: 13, fontWeight: 500, cursor: 'pointer', position: 'relative',
        }}>
          <Icon name="book-open" size={15} /> Из рецепта
        </button>
      )}
    </div>
  );
}

// ── MealTimeline — slim row of meal slots ───────────────────────────

function MealTimeline({ pal, meals, onLogMeal, onOpenNutrition }) {
  // We map logged meals to canonical slots by time-of-day buckets so the
  // visual stays a fixed 4-slot rhythm regardless of order.
  const slots = [
    { id: 'breakfast', label: 'З', name: 'Завтрак',  defaultTime: '8'  },
    { id: 'lunch',     label: 'О', name: 'Обед',     defaultTime: '12' },
    { id: 'snack',     label: 'П', name: 'Перекус',  defaultTime: '16' },
    { id: 'dinner',    label: 'У', name: 'Ужин',     defaultTime: '19' },
  ];
  // Map meals to slots by mealName (with leniency)
  const fillMap = {};
  for (const m of meals) {
    const n = (m.mealName || '').toLowerCase();
    const id =
      (n.includes('breakfast') || n.includes('завтрак')) ? 'breakfast' :
      (n.includes('lunch')     || n.includes('обед'))    ? 'lunch'     :
      (n.includes('snack')     || n.includes('перекус')) ? 'snack'     :
      (n.includes('dinner')    || n.includes('ужин'))    ? 'dinner'    : null;
    if (id && !fillMap[id]) fillMap[id] = m;
  }

  return (
    <div style={{
      padding: '0 16px 10px',
      display: 'flex', alignItems: 'center', gap: 8, overflowX: 'auto',
      WebkitOverflowScrolling: 'touch',
    }}>
      {slots.map(slot => {
        const filled = fillMap[slot.id];
        return (
          <button
            key={slot.id}
            onClick={() => filled ? onOpenNutrition && onOpenNutrition() : onLogMeal && onLogMeal()}
            className="press"
            style={{
              flexShrink: 0, padding: '6px 12px 6px 8px',
              borderRadius: 999, cursor: 'pointer',
              background: filled ? C.sand500 : 'transparent',
              border: filled ? '1px solid transparent' : `1px solid ${pal.border}`,
              color: filled ? C.ink900 : pal.muted,
              display: 'inline-flex', alignItems: 'center', gap: 8,
            }}>
            <span style={{
              width: 22, height: 22, borderRadius: 999,
              background: filled ? C.cream : pal.sunk,
              color: filled ? C.sand700 : pal.subtle,
              display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
              fontFamily: F.body, fontSize: 10, fontWeight: 600,
              letterSpacing: '.02em',
            }}>
              {filled
                ? <svg width="11" height="11" viewBox="0 0 16 16" fill="none">
                    <path d="M3 8.5L6.5 12L13 4.5" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"/>
                  </svg>
                : slot.label}
            </span>
            <span style={{
              fontFamily: F.body, fontSize: 12, fontWeight: 500,
              whiteSpace: 'nowrap',
            }}>{slot.name}</span>
            <span style={{
              fontFamily: F.mono, fontSize: 10,
              fontVariantNumeric: 'tabular-nums',
              color: filled ? 'rgba(26,26,26,.55)' : pal.subtle,
              whiteSpace: 'nowrap',
            }}>
              {filled ? (filled.time || '').replace(' am','').replace(' pm','') : slot.defaultTime}
            </span>
          </button>
        );
      })}
    </div>
  );
}

// ── TodayMeals — paper card replacing the fuel strip ────────────────

function TodayMeals({ pal, meals, totals, onLogMeal, onOpenNutrition }) {
  const empty = meals.length === 0;
  const t = MEAL_TARGETS;
  const kcalPct = Math.min(1, totals.kcal / t.kcal);

  return (
    <button
      onClick={onOpenNutrition}
      className="press"
      style={{
        display: 'block', width: '100%', textAlign: 'left',
        background: pal.paper, borderRadius: 18, padding: 16,
        border: `1px solid ${pal.hairline}`,
        cursor: 'pointer',
      }}
    >
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
        marginBottom: 10,
      }}>
        <Eyebrow style={{ color: pal.subtle }}>Приёмы сегодня</Eyebrow>
        <span style={{
          fontFamily: F.body, fontSize: 12, color: pal.subtle,
          display: 'inline-flex', alignItems: 'center', gap: 4,
        }}>
          Питание <Icon name="arrow-right" size={12} />
        </span>
      </div>

      {/* Totals row */}
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
        marginBottom: 8, gap: 12,
      }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
          <span style={{
            fontFamily: F.mono, fontSize: 30, fontWeight: 500,
            color: pal.ink, letterSpacing: '-0.03em',
            fontVariantNumeric: 'tabular-nums', lineHeight: 1,
          }}>{totals.kcal.toLocaleString()}</span>
          <span style={{
            fontFamily: F.body, fontSize: 12, color: pal.subtle,
          }}>/ {t.kcal.toLocaleString()} ккал</span>
        </div>
        <div style={{
          display: 'flex', gap: 10,
          fontFamily: F.mono, fontSize: 10.5, color: pal.muted,
          fontVariantNumeric: 'tabular-nums',
        }}>
          <span><span style={{ color: pal.ink2 }}>{Math.round(totals.p)}</span><span style={{ color: pal.subtle }}>/{t.protein}P</span></span>
          <span><span style={{ color: pal.ink2 }}>{Math.round(totals.c)}</span><span style={{ color: pal.subtle }}>/{t.carbs}C</span></span>
          <span><span style={{ color: pal.ink2 }}>{Math.round(totals.f)}</span><span style={{ color: pal.subtle }}>/{t.fat}F</span></span>
        </div>
      </div>

      {/* kcal progress bar */}
      <div style={{
        height: 6, background: pal.sunk, borderRadius: 999, overflow: 'hidden',
        marginBottom: 14,
      }}>
        <div style={{
          width: `${kcalPct * 100}%`, height: '100%',
          background: `linear-gradient(90deg, ${C.sand500} 0%, ${C.forest700} 100%)`,
          borderRadius: 999,
          transition: 'width 320ms var(--ease-out)',
        }} />
      </div>

      {empty ? (
        <div style={{
          padding: '10px 4px 4px',
          fontFamily: F.body, fontSize: 13, color: pal.muted, lineHeight: 1.5,
        }}>
          Сегодня пока ничего — первый приём приземлится здесь.
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column' }}>
          {meals.slice(-3).map((m, i, arr) => (
            <React.Fragment key={m.id}>
              <div style={{
                display: 'grid', gridTemplateColumns: 'auto 1fr auto',
                gap: 12, alignItems: 'center', padding: '8px 0',
              }}>
                <div style={{
                  width: 32, height: 32, borderRadius: 10,
                  background: C.sand100, color: C.sand700,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontFamily: F.display, fontSize: 16,
                }}>
                  {(m.mealName || 'M').charAt(0).toUpperCase()}
                </div>
                <div style={{ minWidth: 0 }}>
                  <div style={{
                    fontFamily: F.body, fontSize: 13.5, fontWeight: 500, color: pal.ink,
                  }}>{m.mealName}</div>
                  <div style={{
                    fontFamily: F.body, fontSize: 11, color: pal.subtle, marginTop: 1,
                  }}>{m.time} · {m.items.length} {m.items.length === 1 ? 'позиция' : m.items.length < 5 ? 'позиции' : 'позиций'}</div>
                </div>
                <div style={{ textAlign: 'right' }}>
                  <span style={{
                    fontFamily: F.mono, fontSize: 14, fontWeight: 500,
                    color: pal.ink, fontVariantNumeric: 'tabular-nums',
                    letterSpacing: '-0.02em',
                  }}>{m.totals.kcal}</span>
                  <span style={{
                    marginLeft: 2,
                    fontFamily: F.body, fontSize: 10, color: pal.subtle,
                  }}>ккал</span>
                </div>
              </div>
              {i < arr.length - 1 && <div style={{ height: 1, background: pal.hairline, marginLeft: 44 }} />}
            </React.Fragment>
          ))}
        </div>
      )}
    </button>
  );
}

Object.assign(window, { MealHero, MealTimeline, TodayMeals, suggestNextMeal });
