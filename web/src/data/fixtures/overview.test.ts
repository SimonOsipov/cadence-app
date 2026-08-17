import { describe, expect, it } from 'vitest'

import { OVERVIEW, PATIENTS, TRIAGE_IDS } from './overview'

// The aggregates are literals in a generated file, which is the point — no component derives them. The
// cost of that is a second copy of the truth, and this is what stops the two from drifting: the numbers
// the screen shows are checked against the rows they describe, here, on the far side of the seam where
// a real API will do the same sum.
describe('the aggregates', () => {
  const { aggregates } = OVERVIEW
  const withStatus = (status: string) => PATIENTS.filter((patient) => patient.status === status).length

  it('count the patients there are', () => {
    expect(PATIENTS.length).toBeGreaterThan(0)
    expect(aggregates.patients).toBe(PATIENTS.length)
    expect(aggregates.byStatus.all).toBe(PATIENTS.length)
  })

  it.each(['attention', 'watch', 'track'] as const)('count the %s patients', (status) => {
    expect(withStatus(status)).toBeGreaterThan(0)
    expect(aggregates.byStatus[status]).toBe(withStatus(status))
  })

  it('agree with the tab counters about attention and watch', () => {
    expect(aggregates.attention).toBe(aggregates.byStatus.attention)
    expect(aggregates.watch).toBe(aggregates.byStatus.watch)
  })

  it('average the adherence the patients carry', () => {
    const mean = PATIENTS.reduce((total, patient) => total + patient.adherence, 0) / PATIENTS.length

    expect(aggregates.averageAdherence).toBe(Math.round(mean))
  })

  it('count the doses of the day and the ones already given', () => {
    const doses = OVERVIEW.schedule.filter((entry) => entry.kind === 'dose')

    expect(aggregates.dosesToday).toBe(doses.length)
    expect(aggregates.dosesDone).toBe(doses.filter((entry) => entry.state === 'done').length)
    expect(aggregates.dosesDone).toBeLessThanOrEqual(aggregates.dosesToday)
  })

  it('count the patients waiting for an answer', () => {
    expect(aggregates.unread).toBe(PATIENTS.filter((patient) => patient.flags.includes('message')).length)
  })
})

describe('the triage row', () => {
  it('names patients this fixture carries, and only the ones needing attention', () => {
    expect(TRIAGE_IDS.length).toBeGreaterThan(0)

    for (const id of TRIAGE_IDS) {
      const patient = PATIENTS.find((candidate) => candidate.id === id)

      expect(patient, `triage names ${id}`).toBeDefined()
      expect(patient?.status).toBe('attention')
    }
  })

  it('leaves nobody needing attention out of it', () => {
    const needing = PATIENTS.filter((patient) => patient.status === 'attention').map((p) => p.id)

    expect([...TRIAGE_IDS].sort()).toEqual(needing.sort())
  })
})

describe('what each patient carries already worked out', () => {
  it.each(PATIENTS.map((patient) => [patient.name, patient] as const))(
    '%s has lost what their weights say',
    (_, patient) => {
      expect(patient.lostKg).toBe(Number((patient.weightStart - patient.weight).toFixed(1)))
    },
  )

  it.each(PATIENTS.map((patient) => [patient.name, patient] as const))(
    '%s is as far along the goal as their weights say',
    (_, patient) => {
      const whole = patient.weightStart - patient.goal
      const expected = whole <= 0 ? 100 : Math.round(((patient.weightStart - patient.weight) / whole) * 100)

      expect(patient.goalProgressPct).toBe(expected)
    },
  )

  // The two invariants in play pull different ways — the component note says thresholds arrive in the
  // response, the overview note says the client derives nothing — so both the range and the verdict
  // travel. This is what keeps them one fact rather than two.
  it.each(PATIENTS.flatMap((patient) => patient.biomarkers.map((marker) => [patient.name, marker] as const)))(
    '%s: the %s verdict agrees with its range',
    (_, marker) => {
      const inside =
        (marker.range.min === undefined || marker.value >= marker.range.min) &&
        (marker.range.max === undefined || marker.value <= marker.range.max)

      expect(marker.withinRange).toBe(inside)
    },
  )

  it('carries a range for every biomarker, so no verdict rests on nothing', () => {
    for (const patient of PATIENTS) {
      expect(patient.biomarkers.length).toBeGreaterThan(0)

      for (const marker of patient.biomarkers) {
        expect(marker.range.min ?? marker.range.max).toBeDefined()
      }
    }
  })

  // Both outcomes have to be in the fixture or the card's two appearances are drawn against one.
  it('carries biomarkers on both sides of their range', () => {
    const verdicts = PATIENTS.flatMap((patient) => patient.biomarkers.map((marker) => marker.withinRange))

    expect(new Set(verdicts)).toEqual(new Set([true, false]))
  })
})
