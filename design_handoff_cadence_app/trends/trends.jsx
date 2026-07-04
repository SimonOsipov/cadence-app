// Cadence — Trends landing + Trend detail screens.
// Exposes: TrendsLanding, TrendDetail
//
// TrendsLanding — overview of all 7 biomarkers, reachable from the Trends tab.
//                 Tapping a card opens TrendDetail.
// TrendDetail   — single biomarker drill-down. Cream or forest header, scrubbable
//                 chart with protocol annotations, stat strip, correlations,
//                 recent entries.

// ────────────────────────────────────────────────────────────────────
// Shared atoms
// ────────────────────────────────────────────────────────────────────

const TIMEFRAMES = [
  { id: '7d',    short: '7 дней' },
  { id: '4w',    short: '4 недели' },
  { id: '3m',    short: '3 месяца' },
  { id: 'cycle', short: 'Весь цикл' },
];

function TimeframeChips({ value, onChange, pal }) {
  return (
    <div style={{
      display: 'flex', gap: 6, padding: '0 16px 14px',
      overflowX: 'auto', WebkitOverflowScrolling: 'touch',
    }}>
      {TIMEFRAMES.map(tf => {
        const on = value === tf.id;
        return (
          <button
            key={tf.id}
            onClick={() => onChange(tf.id)}
            className="press"
            style={{
              flexShrink: 0,
              padding: '8px 14px', borderRadius: 999, cursor: 'pointer',
              background: on ? C.forest700 : pal.paper,
              color: on ? C.cream : pal.muted,
              border: on ? '1px solid transparent' : `1px solid ${pal.border}`,
              fontFamily: F.body, fontSize: 13, fontWeight: 500,
              whiteSpace: 'nowrap',
              transition: 'background-color 160ms var(--ease-out), color 160ms var(--ease-out), border-color 160ms var(--ease-out)',
            }}
          >
            {tf.short}
          </button>
        );
      })}
    </div>
  );
}

// Small spark for the landing biomarker grid (normalized series → curve)
function MiniSpark({ values, accent, height = 32, pal }) {
  const W = 120, H = height;
  const min = Math.min(...values), max = Math.max(...values);
  const range = max - min || 1;
  const xs = values.map((_, i) => 2 + (i * (W - 4)) / (values.length - 1));
  const ys = values.map(v => 2 + ((max - v) / range) * (H - 4));
  const d = xs.map((x, i) => (i === 0 ? 'M' : 'L') + x + ' ' + ys[i]).join(' ');
  const fillD = d + ` L${xs[xs.length-1]} ${H} L${xs[0]} ${H} Z`;
  const stroke = accent === 'sand' ? '#a5773d' : C.forest700;
  const fill = accent === 'sand' ? 'rgba(212,165,116,0.20)' : 'rgba(45,95,63,0.14)';
  return (
    <svg viewBox={`0 0 ${W} ${H}`} style={{ width: '100%', height: H, display: 'block' }} preserveAspectRatio="none">
      <path d={fillD} fill={fill} />
      <path d={d} stroke={stroke} strokeWidth="1.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={xs[xs.length-1]} cy={ys[ys.length-1]} r="2.5" fill={stroke} />
    </svg>
  );
}

// ────────────────────────────────────────────────────────────────────
// TRENDS LANDING
// ────────────────────────────────────────────────────────────────────

function TrendsLanding({ pal, platform, timeframe, setTimeframe, activeBiomarker,
                        onBack, onOpenDetail, onChangeTab, onOpenJournal, onOpenBody }) {
  // Header pieces
  const hero = TREND_DATA[activeBiomarker] || TREND_DATA.weight;
  const heroSeries = hero.series[timeframe];
  const heroDelta = formatDeltaPill(heroSeries, hero);
  const heroProgress = isProgress(heroSeries, hero);

  // Others — exclude hero from grid
  const others = TREND_ORDER.filter(id => id !== hero.id);

  // Coach correlation line — built from hero's strongest correlation
  const topCorr = hero.correlations[0];

  const NavBg = platform === 'android'
    ? <CadenceAndroidNav active="insights" onChange={onChangeTab} pal={pal} />
    : <CadenceTabBar active="insights" onChange={onChangeTab} pal={pal} />;

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 130,
      }}>
        {/* Header */}
        <div style={{
          padding: '12px 20px 14px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 12,
        }}>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginBottom: 4 }}>
              12-я неделя · {TIMEFRAMES.find(t => t.id === timeframe).short.toLowerCase()}
            </div>
            <div style={{
              fontFamily: F.display, fontSize: 36, color: pal.ink,
              lineHeight: 1.0, letterSpacing: '-0.018em',
            }}>
              Ваш <span style={{ fontStyle: 'italic', color: C.forest700 }}>ритм</span>
            </div>
          </div>
          <div style={{
            width: 40, height: 40, borderRadius: 999,
            background: C.forest700, color: C.cream,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontFamily: F.display, fontStyle: 'italic', fontSize: 18,
          }}>M</div>
        </div>

        <TimeframeChips value={timeframe} onChange={setTimeframe} pal={pal} />

        {/* Hero biomarker — featured card with full mini chart */}
        <div style={{ padding: '0 16px 14px' }}>
          <button
            onClick={() => onOpenDetail(hero.id)}
            className="press"
            style={{
              display: 'block', width: '100%', textAlign: 'left',
              background: pal.paper, borderRadius: 20, padding: 18,
              border: `1px solid ${pal.hairline}`,
              boxShadow: '0 2px 8px rgba(46,38,24,.05)',
              cursor: 'pointer',
            }}
          >
            <div style={{
              display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start',
              marginBottom: 12, gap: 12,
            }}>
              <div>
                <Eyebrow style={{ color: pal.subtle, marginBottom: 6 }}>{hero.eyebrow}</Eyebrow>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
                  <span style={{
                    fontFamily: F.mono, fontSize: 36, fontWeight: 500,
                    fontVariantNumeric: 'tabular-nums',
                    color: pal.ink, letterSpacing: '-0.03em', lineHeight: 1,
                  }}>
                    {fmtValue(hero.series.cycle.values[hero.series.cycle.values.length - 1], hero.decimals)}
                  </span>
                  <span style={{
                    fontFamily: F.display, fontStyle: 'italic',
                    fontSize: 18, color: pal.muted,
                  }}>{hero.unit}</span>
                </div>
              </div>
              <div style={{ textAlign: 'right' }}>
                <Pill tone="forest" style={{ fontSize: 11 }}>{heroDelta}</Pill>
                <div style={{
                  marginTop: 6,
                  fontFamily: F.body, fontSize: 11, color: pal.subtle,
                }}>к {timeframe === 'cycle' ? 'старту цикла' : 'началу недели'}</div>
              </div>
            </div>

            <div style={{ marginBottom: 10 }}>
              <MiniSpark values={heroSeries.values} accent={hero.accent} height={60} pal={pal} />
            </div>

            <div style={{
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            }}>
              <span style={{
                fontFamily: F.body, fontSize: 12.5, color: pal.muted,
              }}>Открыть детали</span>
              <Icon name="arrow-right" size={16} color={C.forest700} />
            </div>
          </button>
        </div>

        {/* Other biomarkers — 2-up grid */}
        <div style={{
          padding: '0 16px 14px',
          display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10,
        }}>
          {others.map(id => {
            const b = TREND_DATA[id];
            const s = b.series[timeframe];
            const last = s.values[s.values.length - 1];
            const delta = formatDeltaPill(s, b);
            const prog = isProgress(s, b);
            return (
              <button
                key={id}
                onClick={() => onOpenDetail(id)}
                className="press"
                style={{
                  display: 'block', textAlign: 'left',
                  background: pal.paper, borderRadius: 18, padding: 14,
                  border: `1px solid ${pal.hairline}`,
                  boxShadow: '0 2px 6px rgba(46,38,24,.04)',
                  cursor: 'pointer',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between',
                              alignItems: 'flex-start', marginBottom: 8 }}>
                  <Eyebrow style={{ color: pal.subtle }}>{b.label}</Eyebrow>
                </div>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginBottom: 8 }}>
                  <span style={{
                    fontFamily: F.mono, fontSize: 22, fontWeight: 500,
                    fontVariantNumeric: 'tabular-nums',
                    color: pal.ink, letterSpacing: '-0.025em', lineHeight: 1,
                  }}>{fmtValue(last, b.decimals)}</span>
                  <span style={{
                    fontFamily: F.body, fontSize: 11, color: pal.subtle,
                  }}>{b.unit}</span>
                </div>
                <div style={{ marginBottom: 8 }}>
                  <MiniSpark values={s.values} accent={b.accent} height={28} pal={pal} />
                </div>
                <Pill tone={prog === false ? 'neutral' : 'forest'} style={{ fontSize: 10 }}>
                  {delta}
                </Pill>
              </button>
            );
          })}
        </div>

        {/* Notable shifts — narrative section */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
            padding: '0 4px 10px',
          }}>
            <Eyebrow style={{ color: pal.subtle }}>Заметные сдвиги</Eyebrow>
            <span style={{
              fontFamily: F.mono, fontSize: 10, color: pal.subtle,
              fontVariantNumeric: 'tabular-nums',
            }}>НЕД 12 / 12</span>
          </div>
          <div style={{
            background: pal.paper, borderRadius: 18,
            border: `1px solid ${pal.hairline}`,
          }}>
            {[
              { icon: 'arrow-trending-up',   tone: { bg: C.forest50, fg: C.forest800 },
                title: 'HRV вырос на 17 мс',     sub: 'С момента старта BPC-157',
                trail: '+31%',  trailSub: 'к старту' },
              { icon: 'moon',                 tone: { bg: C.sand100, fg: '#6b4a25' },
                title: 'Глубокий сон +24 мин',  sub: 'В среднем за ночь',
                trail: '1ч 38м', trailSub: 'цель на ночь' },
              { icon: 'fire',                 tone: { bg: C.forest50, fg: C.forest800 },
                title: 'Метаболизм стабилен',   sub: 'Не падает на повышении дозы',
                trail: '1 720',  trailSub: 'ккал / день' },
            ].map((r, i, arr) => (
              <React.Fragment key={i}>
                <div style={{
                  display: 'grid', gridTemplateColumns: '40px 1fr auto',
                  gap: 14, alignItems: 'center', padding: '12px 14px',
                }}>
                  <div style={{
                    width: 40, height: 40, borderRadius: 12,
                    background: r.tone.bg, color: r.tone.fg,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                  }}>
                    <Icon name={r.icon} size={20} />
                  </div>
                  <div>
                    <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>{r.title}</div>
                    <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>{r.sub}</div>
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontFamily: F.mono, fontSize: 13, color: pal.ink2, fontVariantNumeric: 'tabular-nums' }}>{r.trail}</div>
                    <div style={{ fontFamily: F.body, fontSize: 11, color: pal.subtle }}>{r.trailSub}</div>
                  </div>
                </div>
                {i < arr.length - 1 && <div style={{ height: 1, background: pal.hairline, marginLeft: 68 }} />}
              </React.Fragment>
            ))}
          </div>
        </div>

        {/* (Removed: Coach card on Trends landing — quieter feed.) */}

        {/* Самочувствие — opens the side-effects journal */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{ padding: '0 4px 10px' }}>
            <Eyebrow style={{ color: pal.subtle }}>Самочувствие</Eyebrow>
          </div>
          <button onClick={onOpenJournal || (() => {})} className="press" style={{
            display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 14, alignItems: 'center',
            width: '100%', textAlign: 'left', cursor: 'pointer',
            background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 18, padding: 14,
          }}>
            <div style={{ width: 44, height: 44, borderRadius: 12, background: C.sand100, color: '#9a5a3c', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="heart" size={22} />
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>Дневник самочувствия</div>
              <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>Настроение, побочные, заметки по курсу</div>
            </div>
            <Icon name="chevron-right" size={18} color={pal.subtle} />
          </button>
        </div>

        {/* Тело — opens the body-metrics screen */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{ padding: '0 4px 10px' }}>
            <Eyebrow style={{ color: pal.subtle }}>Состав тела</Eyebrow>
          </div>
          <button onClick={onOpenBody || (() => {})} className="press" style={{
            display: 'grid', gridTemplateColumns: '44px 1fr auto', gap: 14, alignItems: 'center',
            width: '100%', textAlign: 'left', cursor: 'pointer',
            background: pal.paper, border: `1px solid ${pal.hairline}`, borderRadius: 18, padding: 14,
          }}>
            <div style={{ width: 44, height: 44, borderRadius: 12, background: C.forest50, color: C.forest700, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="scale" size={22} />
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontFamily: F.body, fontWeight: 500, fontSize: 14, color: pal.ink }}>Тело · замеры и фото</div>
              <div style={{ fontFamily: F.body, fontSize: 12, color: pal.subtle, marginTop: 1 }}>Состав, талия, бёдра, снимки</div>
            </div>
            <Icon name="chevron-right" size={18} color={pal.subtle} />
          </button>
        </div>
      </div>

      {NavBg}
    </div>
  );
}

// ────────────────────────────────────────────────────────────────────
// TREND DETAIL
// ────────────────────────────────────────────────────────────────────

function TrendDetail({ pal, platform, biomarkerId, timeframe, setTimeframe,
                      headerVariant, showAnnotations, onBack, onSwitchBiomarker }) {
  const biom = TREND_DATA[biomarkerId] || TREND_DATA.weight;
  const seriesAtTf = biom.series[timeframe];
  const last = seriesAtTf.values[seriesAtTf.values.length - 1];
  const delta = formatDeltaPill(seriesAtTf, biom);
  const prog = isProgress(seriesAtTf, biom);
  const stats = seriesStats(seriesAtTf, biom);
  const baselineDiff = last - biom.baseline;
  const baselineDelta = (baselineDiff > 0 ? '↑ ' : '↓ ') + fmtValue(Math.abs(baselineDiff), biom.decimals) + ' ' + (biom.unit.startsWith('/') ? '' : biom.unit);

  const isForest = headerVariant === 'forest';

  // Hero block contents (used in both variants, different colors)
  const heroEyebrow = isForest ? C.sand300 : pal.subtle;
  const heroInk     = isForest ? C.cream    : pal.ink;
  const heroMuted   = isForest ? 'rgba(246,241,234,.65)' : pal.muted;
  const heroAccent  = isForest ? C.sand300  : C.forest700;

  return (
    <div style={{ background: pal.bg, height: '100%', position: 'relative', overflow: 'hidden' }}>
      <div className="ds-scroll" style={{
        height: '100%', overflowY: 'auto', overflowX: 'hidden',
        paddingTop: platform === 'ios' ? 48 : 8, paddingBottom: 40,
      }}>
        {/* Top bar */}
        <div style={{
          padding: '8px 16px 10px',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
          background: isForest ? C.forest800 : 'transparent',
        }}>
          <button onClick={onBack} className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none',
            background: isForest ? 'rgba(246,241,234,.12)' : pal.sunk,
            color: isForest ? C.cream : pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}>
            <Icon name="chevron-left" size={20} />
          </button>
          <div style={{
            fontFamily: F.body, fontSize: 13, fontWeight: 500,
            color: isForest ? 'rgba(246,241,234,.7)' : pal.muted,
          }}>{biom.label}</div>
          <button className="press" style={{
            width: 40, height: 40, borderRadius: 999, border: 'none',
            background: isForest ? 'rgba(246,241,234,.12)' : pal.sunk,
            color: isForest ? C.cream : pal.ink2,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
          }}>
            <Icon name="ellipsis-horizontal" size={20} />
          </button>
        </div>

        {/* Hero */}
        <div style={{
          background: isForest ? C.forest800 : 'transparent',
          padding: isForest ? '8px 24px 28px' : '6px 24px 22px',
          borderBottomLeftRadius: isForest ? 28 : 0,
          borderBottomRightRadius: isForest ? 28 : 0,
          marginBottom: isForest ? 16 : 4,
          color: isForest ? C.cream : pal.ink,
        }}>
          <div style={{
            fontFamily: F.body, fontSize: 11, fontWeight: 500, letterSpacing: '.14em',
            textTransform: 'uppercase', color: heroEyebrow, marginBottom: 8,
          }}>{biom.eyebrow}</div>

          <div style={{
            fontFamily: F.display, fontSize: 36, lineHeight: 1.04,
            letterSpacing: '-0.018em', color: heroInk, marginBottom: 18,
          }}>
            Your <span style={{ fontStyle: 'italic', color: heroAccent }}>{biom.coach.emph}</span>
          </div>

          <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 10 }}>
            <span style={{
              fontFamily: F.mono, fontSize: 64, fontWeight: 500,
              fontVariantNumeric: 'tabular-nums',
              color: heroInk, letterSpacing: '-0.03em', lineHeight: 1,
            }}>{fmtValue(last, biom.decimals)}</span>
            <span style={{
              fontFamily: F.display, fontStyle: 'italic',
              fontSize: 22, color: heroMuted,
            }}>{biom.unit}</span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <span style={{
              display: 'inline-flex', alignItems: 'center',
              padding: '3px 10px', borderRadius: 999,
              fontFamily: F.body, fontSize: 11, fontWeight: 500,
              background: isForest ? 'rgba(246,241,234,.14)' : C.forest50,
              color: isForest ? C.cream : C.forest800,
              whiteSpace: 'nowrap',
            }}>{delta}</span>
            <span style={{
              fontFamily: F.body, fontSize: 12, color: heroMuted,
              whiteSpace: 'nowrap',
            }}>за {timeframe === '7d' ? 'неделю' : timeframe === '4w' ? 'месяц' : timeframe === '3m' ? '3 месяца' : 'цикл'}</span>
            {timeframe !== 'cycle' && (
              <>
                <span style={{ color: heroMuted, opacity: 0.4 }}>·</span>
                <span style={{
                  fontFamily: F.body, fontSize: 12, color: heroMuted,
                  whiteSpace: 'nowrap',
                }}>{baselineDelta} к старту</span>
              </>
            )}
          </div>
        </div>

        {/* Timeframe chips */}
        <TimeframeChips value={timeframe} onChange={setTimeframe} pal={pal} />

        {/* Chart card */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            background: pal.paper, borderRadius: 18, padding: 16,
            border: `1px solid ${pal.hairline}`,
            boxShadow: '0 2px 8px rgba(46,38,24,.05)',
          }}>
            <ScrubChart
              data={seriesAtTf}
              accent={biom.accent}
              pal={pal}
              biom={biom}
              annotations={showAnnotations}
              showDoseRow={showAnnotations}
              height={220}
            />
            <div style={{
              marginTop: 8, fontFamily: F.body, fontSize: 11.5, color: pal.subtle,
              display: 'flex', alignItems: 'center', gap: 6,
            }}>
              <span style={{
                width: 18, height: 1, background: C.sand700, opacity: 0.6,
                display: 'inline-block',
                borderTop: '1px dashed ' + C.sand700,
              }} />
              {showAnnotations
                ? 'Пунктир — изменения протокола. Тяните по графику, чтобы посмотреть любой день.'
                : 'Тяните по графику, чтобы посмотреть значение в любой день.'}
            </div>
          </div>
        </div>

        {/* Stat strip */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            background: pal.paper, borderRadius: 18,
            border: `1px solid ${pal.hairline}`,
            padding: 14,
            display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12,
          }}>
            {[
              { lbl: 'Сред.', v: stats.avg },
              { lbl: 'Мин',   v: stats.min },
              { lbl: 'Макс',  v: stats.max },
              { lbl: 'Δ',     v: delta.replace(/^[↑↓]\s*/, ''),
                prefix: delta.match(/^[↑↓]/)?.[0] || '' },
            ].map((s, i) => (
              <div key={i} style={{
                borderLeft: i === 0 ? 'none' : `1px solid ${pal.hairline}`,
                paddingLeft: i === 0 ? 0 : 12,
              }}>
                <Eyebrow style={{ color: pal.subtle, marginBottom: 4, fontSize: 10 }}>{s.lbl}</Eyebrow>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 2 }}>
                  {s.prefix && (
                    <span style={{
                      fontFamily: F.mono, fontSize: 14, color: C.forest700,
                      fontVariantNumeric: 'tabular-nums',
                    }}>{s.prefix}</span>
                  )}
                  <span style={{
                    fontFamily: F.mono, fontSize: 18, fontWeight: 500,
                    color: pal.ink, letterSpacing: '-0.02em',
                    fontVariantNumeric: 'tabular-nums', lineHeight: 1,
                  }}>{s.v}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Narrative + Correlations card */}
        <div style={{ padding: '0 16px 14px' }}>
          <div style={{
            background: pal.paper, borderRadius: 18, padding: 18,
            border: `1px solid ${pal.hairline}`,
          }}>
            <Eyebrow style={{ color: pal.subtle, marginBottom: 8 }}>Что связано</Eyebrow>
            <div style={{
              fontFamily: F.display, fontSize: 19, color: pal.ink,
              lineHeight: 1.25, letterSpacing: '-0.012em', marginBottom: 14,
            }}>
              {biom.narrative}
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {biom.correlations.map((c, i) => (
                <div key={i}
                  onClick={() => onSwitchBiomarker && onSwitchBiomarker(c.withId)}
                  className="press"
                  style={{
                    display: 'grid', gridTemplateColumns: '1fr auto', gap: 10,
                    alignItems: 'center', cursor: 'pointer',
                    padding: '10px 12px',
                    background: pal.sunk, borderRadius: 12,
                  }}>
                  <div style={{ minWidth: 0 }}>
                    <div style={{
                      display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4,
                    }}>
                      <span style={{
                        fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.ink,
                      }}>{biom.label}</span>
                      <span style={{ color: pal.subtle, fontSize: 13 }}>↔</span>
                      <span style={{
                        fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.ink,
                      }}>{c.label}</span>
                    </div>
                    <div style={{ fontFamily: F.body, fontSize: 12, color: pal.muted, lineHeight: 1.4 }}>{c.note}</div>
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <CorrelationBar strength={c.strength} pal={pal} />
                    <div style={{
                      marginTop: 4,
                      fontFamily: F.mono, fontSize: 10,
                      color: pal.subtle, fontVariantNumeric: 'tabular-nums',
                    }}>r = {c.strength > 0 ? '+' : ''}{c.strength.toFixed(2)}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Recent entries */}
        <div style={{ padding: '0 16px' }}>
          <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'baseline',
            padding: '0 4px 10px',
          }}>
            <Eyebrow style={{ color: pal.subtle }}>Последние записи</Eyebrow>
            <button style={{
              background: 'none', border: 'none', cursor: 'pointer',
              fontFamily: F.body, fontSize: 12, color: C.forest700, fontWeight: 500,
            }}>Все</button>
          </div>
          <div style={{
            background: pal.paper, borderRadius: 18,
            border: `1px solid ${pal.hairline}`,
          }}>
            {biom.recent.map((r, i, arr) => (
              <React.Fragment key={i}>
                <div style={{
                  display: 'grid', gridTemplateColumns: '1fr auto',
                  gap: 12, alignItems: 'center', padding: '12px 16px',
                }}>
                  <div>
                    <div style={{ fontFamily: F.body, fontSize: 13, fontWeight: 500, color: pal.ink }}>
                      {dayLabel(r.day)}
                    </div>
                    {r.note && (
                      <div style={{ fontFamily: F.body, fontSize: 11.5, color: pal.subtle, marginTop: 1 }}>{r.note}</div>
                    )}
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <span style={{
                      fontFamily: F.mono, fontSize: 16, fontWeight: 500,
                      color: pal.ink, fontVariantNumeric: 'tabular-nums',
                      letterSpacing: '-0.02em',
                    }}>{fmtValue(r.value, biom.decimals)}</span>
                    <span style={{
                      marginLeft: 3,
                      fontFamily: F.body, fontSize: 11, color: pal.subtle,
                    }}>{biom.unit}</span>
                  </div>
                </div>
                {i < arr.length - 1 && <div style={{ height: 1, background: pal.hairline, marginLeft: 16 }} />}
              </React.Fragment>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// Tiny correlation bar — bipolar -1..+1
function CorrelationBar({ strength, pal }) {
  const W = 56, H = 6;
  const mid = W / 2;
  const mag = Math.min(1, Math.abs(strength));
  const w = mag * (W / 2 - 1);
  const x = strength >= 0 ? mid : mid - w;
  const color = strength >= 0 ? C.forest700 : C.sand700;
  return (
    <svg width={W} height={H} viewBox={`0 0 ${W} ${H}`} style={{ display: 'inline-block' }}>
      <rect x="0" y={H/2 - 0.5} width={W} height="1" fill={pal.hairline} />
      <line x1={mid} y1="0" x2={mid} y2={H} stroke={pal.border} strokeWidth="1" />
      <rect x={x} y="1" width={w} height={H - 2} rx={(H-2)/2} fill={color} />
    </svg>
  );
}

function dayLabel(day) {
  const ago = TODAY_DAY - day;
  if (ago < 1) return 'Сегодня';
  if (ago === 1) return 'Вчера';
  if (ago < 7) return `${ago} дн назад`;
  const w = Math.round(ago / 7);
  return `${w} нед назад`;
}

Object.assign(window, { TrendsLanding, TrendDetail, TIMEFRAMES });
