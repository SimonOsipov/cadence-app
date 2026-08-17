// The shapes the Overview reads. Fixtures fill them today and `/v1` will fill them later; the seam is
// transport.ts, and nothing above it knows which.
//
// Every number a screen shows arrives here already worked out. That is invariant 2 of the component
// note, and it is not a style preference: a client that derives adherence is a second source of truth
// for it, and the two disagree at the worst possible moment — in front of a doctor, about a patient.
// The prototype derived all of them, which is why porting it means moving the arithmetic rather than
// copying it.

export type PatientStatus = 'attention' | 'watch' | 'track'

export type FlagKind = 'missed' | 'side' | 'biomarker' | 'message' | 'titration' | 'cycle'

/**
 * One biomarker as the doctor reads it.
 *
 * `withinRange` is the verdict and `range` is what it was reached against. Both travel because the two
 * invariants in play pull different ways — the component note says thresholds arrive in the response,
 * the overview note says the client derives nothing — and carrying both satisfies each. They cannot
 * drift apart in silence: a test requires the verdict to agree with the range for every row.
 */
export type Biomarker = {
  key: string
  label: string
  value: number
  unit: string
  range: { min?: number; max?: number }
  withinRange: boolean
}

export type Patient = {
  id: string
  name: string
  age: number
  initial: string

  compound: string
  dose: string
  cadence: string
  week: number
  cycleLength: number

  status: PatientStatus
  adherence: number
  lastSeen: string
  flags: readonly FlagKind[]
  note: string

  weight: number
  weightStart: number
  goal: number
  unit: string

  /** Worked out here rather than in the card: weightStart − weight, and the way to the goal. */
  lostKg: number
  goalProgressPct: number

  /** The weight trend the sparkline draws. Points, not a conclusion — the chart is geometry. */
  spark: readonly number[]

  biomarkers: readonly Biomarker[]
}

/**
 * Every number the stats strip and the roster tabs show.
 *
 * A type of its own, so that a component wanting one of them imports this rather than reaching into
 * the roster. The lint rule in eslint.config.js closes the other road.
 */
export type OverviewAggregates = {
  patients: number
  attention: number
  watch: number
  dosesToday: number
  dosesDone: number
  unread: number
  averageAdherence: number

  /** The roster tab counters, in the order the tabs are drawn. */
  byStatus: { all: number; attention: number; watch: number; track: number }
}

export type ScheduleEntry = {
  id: string
  patientId: string

  /**
   * Carried rather than looked up. The schedule names patients from anywhere in the clinic and the
   * roster on screen is one page of it, so a screen resolving this itself shows an id to the doctor
   * for everyone it has not loaded — measured against the prototype, two of eight entries.
   */
  patientName: string
  /** Wall-clock, as the clinic reads it: the schedule is one day and the doctor's own timezone. */
  at: string
  kind: 'dose' | 'checkin'
  label: string
  state: 'done' | 'now' | 'due'
}

/**
 * A page of the roster, and the shape is the point: `items`, a total and a cursor rather than a bare
 * array. The server will filter, sort and paginate, and a client holding the whole list would keep its
 * own filtering the day it stops holding it.
 */
export type RosterPage = {
  items: readonly Patient[]
  total: number
  cursor: string | null
}

export type RosterFilter = 'all' | PatientStatus

export type Overview = {
  doctor: { name: string; initial: string }
  aggregates: OverviewAggregates
  /** The patients the triage row shows, already chosen. */
  triage: readonly Patient[]
  schedule: readonly ScheduleEntry[]
}
