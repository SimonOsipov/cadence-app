// Cadence — Nutrition Today screen.
// Reachable from the TodayMeals card or the MealTimeline pills.
// Shows the day's nutrition in detail: ring + macros, meal list, week trend.

function NutritionToday({ pal, platform, meals, totals, onBack, onLogMeal, onChangeTab, onOpenRecipes }) {
  const t = MEAL_TARGETS;

  // 7-day kcal history (cosmetic; mocked, with today computed from real totals).
  const weekHistory = [1685, 1742, 1610, 1820, 1455, 1690, totals.kcal];
  const weekProteinHistory = [128, 142, 118, 145, 102, 138, Math.round(totals.p)];

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 120,
      }}>
        {/* Top bar */}
        <div style={{
          padding: '8px 16px 10px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <button onClick={onBack} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none',
            background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{
            fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.muted,
          }}>Питание</div>
          <button className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none',
            background: pal.sunk, color: pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}>
            <Icon name="ellipsis-horizontal" size={20} />
          </button>
        </div>

        {/* Hero */}
        <div style={{ padding: '6px 24px 18px' }}>
          <Eyebrow style={{ color: pal.subtle, marginBottom: 6 }}>Тарелка сегодня</Eyebrow>
          <div style={{
            fontFamily: F.display, fontSize: 32, color: pal.ink,
            lineHeight: 1.05, letterSpacing: '-0.018em',
          }}>
            {meals.length === 0
              ? <>Пока ничего — начнём, когда <span style={{ fontStyle: 'italic', color: C.sand700 }}>будете готовы</span>.</>
              : <>{meals.length} {meals.length === 1 ? 'приём' : meals.length < 5 ? 'приёма' : 'приёмов'}.</>}
          </div>
        </div>

        {/* Ring + macros block */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            background: pal.paper, borderRadius: 20, padding: 18,
            border: `1px solid ${pal.hairline}`,
            boxShadow: '0 2px 8px rgba(46,38,24,.05)',
            display: 'grid', gridTemplateColumns: '120px 1fr', gap: 18, alignItems: 'center',
          }}>
            <NutritionRing totals={totals} targets={t} pal={pal} />
            <div style={{ minWidth: 0 }}>
              <Eyebrow style={{ color: pal.subtle, marginBottom: 4, fontSize: 10 }}>ккал сегодня</Eyebrow>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginBottom: 14 }}>
                <span style={{
                  fontFamily: F.mono, fontSize: 32, fontWeight: 500,
                  fontVariantNumeric: 'tabular-nums', color: pal.ink,
                  letterSpacing: '-0.03em', lineHeight: 1,
                }}>{totals.kcal.toLocaleString()}</span>
                <span style={{
                  fontFamily: F.body, fontSize: 12, color: pal.subtle,
                }}>/ {t.kcal.toLocaleString()}</span>
              </div>

              <MacroBar label="белок" v={Math.round(totals.p)} goal={t.protein} color={C.forest700} pal={pal} />
              <MacroBar label="углеводы" v={Math.round(totals.c)} goal={t.carbs}   color="#a5773d"    pal={pal} />
              <MacroBar label="жиры"     v={Math.round(totals.f)} goal={t.fat}     color={C.sand700}  pal={pal} last />
            </div>
          </div>
        </div>

        {/* Meals timeline */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
            padding: '0 4px 10px',
          }}>
            <Eyebrow style={{ color: pal.subtle }}>Приёмы сегодня</Eyebrow>
            <span style={{
              fontFamily: F.mono, fontSize: 10.5, color: pal.subtle,
              fontVariantNumeric: 'tabular-nums',
            }}>{meals.length} из 4</span>
          </div>

          {meals.length === 0 ? (
            <div style={{
              background: pal.paper, borderRadius: 18, padding: 18,
              border: `1px solid ${pal.hairline}`,
              fontFamily: F.body, fontSize: 13, color: pal.muted, lineHeight: 1.5,
            }}>
              Сегодня пока ничего. <span onClick={onLogMeal} style={{ color: C.forest700, fontWeight: 500, cursor: 'pointer' }}>Запишите первый приём</span>, чтобы начать ритм.
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {meals.map(m => <MealCard key={m.id} meal={m} pal={pal} />)}
            </div>
          )}
        </div>

        {/* Week trend */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            background: pal.paper, borderRadius: 18, padding: 18,
            border: `1px solid ${pal.hairline}`,
          }}>
            <div style={{
              display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
              marginBottom: 12,
            }}>
              <Eyebrow style={{ color: pal.subtle }}>Прошлая неделя</Eyebrow>
              <span style={{
                fontFamily: F.body, fontSize: 11, color: pal.subtle,
              }}>ккал · по дням</span>
            </div>
            <WeekBars values={weekHistory} target={t.kcal} pal={pal} />
            <div style={{
              marginTop: 14, paddingTop: 12, borderTop: `1px solid ${pal.hairline}`,
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            }}>
              <div>
                <Eyebrow style={{ color: pal.subtle, marginBottom: 2, fontSize: 10 }}>Белок · средн.</Eyebrow>
                <div style={{
                  display: 'flex', alignItems: 'baseline', gap: 3,
                }}>
                  <span style={{
                    fontFamily: F.mono, fontSize: 18, fontWeight: 500,
                    color: pal.ink, letterSpacing: '-0.02em',
                    fontVariantNumeric: 'tabular-nums',
                  }}>{Math.round(weekProteinHistory.reduce((a, b) => a + b) / weekProteinHistory.length)}</span>
                  <span style={{
                    fontFamily: F.body, fontSize: 11, color: pal.subtle,
                  }}>г / день</span>
                </div>
              </div>
              <Pill tone="forest" style={{ fontSize: 11 }}>↑ 6 г к прошлой</Pill>
            </div>
          </div>
        </div>

        {/* (Removed: Coach card on Nutrition — quieter feed.) */}

        {/* Рецепты — opens the recipe library/builder */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{ padding: '0 4px 10px' }}>
            <Eyebrow style={{ color: pal.subtle }}>Рецепты</Eyebrow>
          </div>
          <button onClick={onOpenRecipes || (() => {})} className="press" style={{
            display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 14, alignItems: 'center',
            width: '100%', textAlign: 'left', cursor: 'pointer',
            background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 18, padding: 14,
            boxShadow: '0 2px 6px rgba(46,38,24,.05)',
          }}>
            <div style={{ width: 44, height: 44, borderRadius: 12, background: C.forest50, color: C.forest700, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="book-open" size={22} />
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>Рецепты и конструктор</div>
              <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>Белковые блюда · соберите своё</div>
            </div>
            <Icon name="chevron-right" size={18} color={pal.subtle} />
          </button>
        </div>
      </div>

      {/* Tab bar */}
      {platform === 'android'
        ? <CadenceAndroidNav active="nutrition" onChange={onChangeTab} pal={pal} />
        : <CadenceTabBar     active="nutrition" onChange={onChangeTab} pal={pal} />}
    </div>
  );
}

// ── NutritionRing — concentric arcs for kcal % and protein % ────────

function NutritionRing({ totals, targets, pal }) {
  const size = 120, stroke = 10;
  const r = (size - stroke) / 2;
  const cx = size / 2, cy = size / 2;
  const circ = 2 * Math.PI * r;
  const kcalPct = Math.min(1, totals.kcal / targets.kcal);
  const proteinPct = Math.min(1, totals.p / targets.protein);

  // Inner protein ring
  const innerR = r - (stroke + 4);
  const innerCirc = 2 * Math.PI * innerR;

  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      {/* kcal track */}
      <circle cx={cx} cy={cy} r={r} fill="none" stroke={pal.sunk} strokeWidth={stroke} />
      {/* kcal arc */}
      <circle cx={cx} cy={cy} r={r} fill="none"
        stroke={C.sand500} strokeWidth={stroke}
        strokeLinecap="round"
        strokeDasharray={`${kcalPct * circ} ${circ}`}
        transform={`rotate(-90 ${cx} ${cy})`}
      />
      {/* protein track */}
      <circle cx={cx} cy={cy} r={innerR} fill="none" stroke={pal.sunk} strokeWidth="6" />
      {/* protein arc */}
      <circle cx={cx} cy={cy} r={innerR} fill="none"
        stroke={C.forest700} strokeWidth="6"
        strokeLinecap="round"
        strokeDasharray={`${proteinPct * innerCirc} ${innerCirc}`}
        transform={`rotate(-90 ${cx} ${cy})`}
      />
      <text x={cx} y={cy - 4} textAnchor="middle"
        fontFamily="DM Sans, sans-serif" fontSize="10" fontWeight="500"
        letterSpacing=".06em" fill={pal.subtle} style={{ textTransform: 'uppercase' }}>
        ккал
      </text>
      <text x={cx} y={cy + 14} textAnchor="middle"
        fontFamily="JetBrains Mono, monospace" fontSize="22" fontWeight="500"
        fill={pal.ink} style={{ letterSpacing: '-0.02em' }}>
        {Math.round(kcalPct * 100)}%
      </text>
    </svg>
  );
}

// ── MacroBar — labelled progress bar with goal ──────────────────────

function MacroBar({ label, v, goal, color, pal, last }) {
  const pct = Math.min(1, v / Math.max(1, goal));
  return (
    <div style={{ marginBottom: last ? 0 : 8 }}>
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
        marginBottom: 4,
      }}>
        <span style={{
          fontFamily: F.body, fontSize: 11, color: pal.muted,
          textTransform: 'capitalize',
        }}>{label}</span>
        <span style={{
          fontFamily: F.mono, fontSize: 11, color: pal.ink2,
          fontVariantNumeric: 'tabular-nums',
        }}>
          {v}<span style={{ color: pal.subtle }}>/{goal} г</span>
        </span>
      </div>
      <div style={{
        height: 5, background: pal.sunk, borderRadius: 999, overflow: 'hidden',
      }}>
        <div style={{
          width: `${pct * 100}%`, height: '100%',
          background: color, borderRadius: 999,
          transition: 'width 320ms var(--ease-out)',
        }} />
      </div>
    </div>
  );
}

// ── MealCard — full meal row with items collapsed ───────────────────

function MealCard({ meal, pal }) {
  const [open, setOpen] = React.useState(false);
  return (
    <div style={{
      background: pal.paper, borderRadius: 16,
      border: `1px solid ${pal.hairline}`,
      overflow: 'hidden',
    }}>
      <button onClick={() => setOpen(o => !o)} className="press" style={{
        display: 'grid', gridTemplateColumns: 'auto 1fr auto auto',
        gap: 12, alignItems: 'center', padding: '12px 14px',
        width: '100%', background: 'transparent', border: 'none', cursor: 'pointer',
        textAlign: 'left',
      }}>
        <div style={{
          width: 36, height: 36, borderRadius: 10,
          background: C.sand100, color: C.sand700,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontFamily: F.display, fontSize: 18, fontStyle: 'italic',
        }}>{(meal.mealName || 'M').charAt(0).toUpperCase()}</div>
        <div style={{ minWidth: 0 }}>
          <div style={{ fontFamily: F.body, fontSize: 14, fontWeight: 500, color: pal.ink }}>{meal.mealName}</div>
          <div style={{
            fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 1,
            fontVariantNumeric: 'tabular-nums',
          }}>{meal.time} · {meal.items.length} {meal.items.length === 1 ? 'позиция' : meal.items.length < 5 ? 'позиции' : 'позиций'} · {Math.round(meal.totals.p)} г белка</div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{
            fontFamily: F.mono, fontSize: 16, fontWeight: 500,
            color: pal.ink, letterSpacing: '-0.02em',
            fontVariantNumeric: 'tabular-nums', lineHeight: 1,
          }}>{meal.totals.kcal}</div>
          <div style={{ fontFamily: F.body, fontSize: 10, color: pal.subtle, marginTop: 2 }}>ккал</div>
        </div>
        <Icon name={open ? 'chevron-up' : 'chevron-down'} size={16} color={pal.subtle} />
      </button>
      {open && (
        <div style={{
          padding: '4px 14px 14px',
          borderTop: `1px dashed ${pal.hairline}`,
          background: pal.bg,
        }}>
          {meal.items.map((it, i) => (
            <div key={it.id} style={{
              display: 'grid', gridTemplateColumns: '1fr auto auto',
              gap: 10, alignItems: 'center', padding: '8px 0',
              borderBottom: i < meal.items.length - 1 ? `1px solid ${pal.hairline}` : 'none',
            }}>
              <div style={{ minWidth: 0 }}>
                <div style={{ fontFamily: F.body, fontSize: 12.5, color: pal.ink2, fontWeight: 500 }}>{it.name}</div>
                <div style={{
                  fontFamily: F.mono, fontSize: 10, color: pal.subtle, marginTop: 1,
                  fontVariantNumeric: 'tabular-nums',
                }}>Белок {it.p}г · Углеводы {it.c}г · Жиры {it.f}г</div>
              </div>
              <div style={{
                fontFamily: F.mono, fontSize: 11, color: pal.subtle,
                fontVariantNumeric: 'tabular-nums',
              }}>{it.grams} g</div>
              <div style={{
                fontFamily: F.mono, fontSize: 12, color: pal.ink2,
                fontVariantNumeric: 'tabular-nums', minWidth: 36, textAlign: 'right',
              }}>{it.kcal}</div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── WeekBars — 7 bars, target line ──────────────────────────────────

function WeekBars({ values, target, pal }) {
  const labels = ['Пн','Вт','Ср','Чт','Пт','Сб','Сег'];
  const max = Math.max(...values, target) * 1.05;
  const W = 320, H = 90, padL = 8, padR = 8, padT = 4, padB = 18;
  const bw = (W - padL - padR) / values.length;
  const yFor = (v) => padT + ((max - v) / max) * (H - padT - padB);

  return (
    <svg viewBox={`0 0 ${W} ${H}`} style={{ width: '100%', height: 'auto', display: 'block' }}>
      {/* target line */}
      <line x1={padL} y1={yFor(target)} x2={W - padR} y2={yFor(target)}
        stroke={C.sand700} strokeOpacity="0.4" strokeWidth="1" strokeDasharray="2 3" />
      <text x={W - padR} y={yFor(target) - 4} textAnchor="end"
        fontFamily="JetBrains Mono, monospace" fontSize="9" fill={C.sand700} opacity="0.75">
        цель {target.toLocaleString()}
      </text>
      {/* bars */}
      {values.map((v, i) => {
        const isToday = i === values.length - 1;
        const y = yFor(v);
        const h = H - padB - y;
        const x = padL + i * bw + 3;
        const w = bw - 6;
        return (
          <g key={i}>
            <rect x={x} y={y} width={w} height={h} rx="3"
              fill={isToday ? C.forest700 : C.sand500} opacity={isToday ? 1 : 0.7} />
            <text x={x + w / 2} y={H - 4} textAnchor="middle"
              fontFamily="DM Sans, sans-serif" fontSize="9"
              fill={isToday ? pal.ink : pal.subtle}
              fontWeight={isToday ? 500 : 400}>
              {labels[i]}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

Object.assign(window, { NutritionToday });
