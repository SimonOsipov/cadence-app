import type { FlagKind, Patient } from '../../data/overview'
import { Icon } from '../../icons/icon'
import { whole } from '../../format'
import { tokens } from '../../tokens/tokens'
import { FLAGS, STATUS_LABEL } from './flags'

export function FlagPill({ kind, small = false }: { kind: FlagKind; small?: boolean }) {
  const flag = FLAGS[kind]

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 5,
        padding: small ? '3px 8px' : '4px 10px',
        borderRadius: tokens.rPill,
        background: flag.bg,
        color: flag.fg,
        fontFamily: tokens.fontBody,
        fontSize: small ? 11 : 12,
        fontWeight: 500,
        whiteSpace: 'nowrap',
      }}
    >
      <Icon name={flag.icon} size={small ? 11 : 12} />
      {flag.label}
    </span>
  )
}

export function StatusDot({ status }: { status: Patient['status'] }) {
  const colour = { attention: tokens.danger, watch: tokens.warning, track: tokens.forest600 }[status]

  return (
    <span
      role="img"
      aria-label={STATUS_LABEL[status]}
      style={{ width: 8, height: 8, borderRadius: tokens.rPill, background: colour, display: 'inline-block' }}
    />
  )
}

/** The weight trend, drawn from points the seam hands down. Geometry, not a conclusion. */
export function Spark({ points, tone }: { points: readonly number[]; tone: string }) {
  const width = 68
  const height = 22
  const step = points.length > 1 ? width / (points.length - 1) : width

  const line = points.map((point, at) => `${at * step},${(1 - point) * height}`).join(' ')

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden="true">
      <polyline points={line} fill="none" stroke={tone} strokeWidth={1.5} strokeLinecap="round" />
    </svg>
  )
}

/** How far along the goal, as a bar. The number arrives computed; this only draws it. */
export function GoalBar({ patient }: { patient: Patient }) {
  return (
    <div>
      <div style={{ height: 6, background: tokens.linen, borderRadius: tokens.rPill, overflow: 'hidden' }}>
        <div
          style={{
            height: '100%',
            width: `${Math.min(patient.goalProgressPct, 100)}%`,
            background: tokens.forest600,
            borderRadius: tokens.rPill,
          }}
        />
      </div>
      <div style={{ fontFamily: tokens.fontBody, fontSize: 11.5, color: tokens.ink500, marginTop: 6 }}>
        {whole(patient.goalProgressPct)}% пути к цели
      </div>
    </div>
  )
}

/**
 * demo marks a section still drawn from fixtures.
 *
 * On the screen and not only in a comment: with the roster live and five sections around it on
 * invented numbers, a doctor reading «Пациентов: 25» beside their own two patients has no way to
 * tell which of the two is their clinic. The mark comes off section by section as M6 answers what
 * each one draws.
 */
export function SectionHead({ eyebrow, title, demo = false }: { eyebrow: string; title: string; demo?: boolean }) {
  return (
    <header style={{ marginBottom: 14 }}>
      <div
        style={{
          fontFamily: tokens.fontBody,
          fontSize: 11,
          fontWeight: 500,
          letterSpacing: '.1em',
          textTransform: 'uppercase',
          color: tokens.ink500,
        }}
      >
        {eyebrow}
      </div>
      <h2 style={{ fontFamily: tokens.fontDisplay, fontSize: 26, color: tokens.ink900, margin: '4px 0 0' }}>
        {title}
        {demo && <DemoMark />}
      </h2>
    </header>
  )
}

/** The words a doctor reads. «Демо-данные» rather than «фикстуры»: the person in front of the screen
 * is not the person who wrote them. */
export function DemoMark() {
  return (
    <span
      style={{
        marginLeft: 10,
        verticalAlign: 'middle',
        fontFamily: tokens.fontBody,
        fontSize: 11,
        fontWeight: 500,
        letterSpacing: '.06em',
        textTransform: 'uppercase',
        color: tokens.ink600,
        background: tokens.linen,
        border: `1px solid ${tokens.bone}`,
        borderRadius: tokens.rPill,
        padding: '3px 9px',
      }}
    >
      Демо-данные · до M6
    </span>
  )
}
