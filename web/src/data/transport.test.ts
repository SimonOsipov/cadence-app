import { describe, expect, it } from 'vitest'

import { PATIENTS } from './fixtures/overview'
import { PAGE_SIZE, fixtureTransport } from './transport'

const transport = fixtureTransport()

describe('the roster page', () => {
  it('is a page and not the list', async () => {
    const first = await transport.roster({ filter: 'all' })

    expect(first.items).toHaveLength(PAGE_SIZE)
    expect(first.total).toBe(PATIENTS.length)
    expect(first.total).toBeGreaterThan(first.items.length)
    expect(first.cursor).not.toBeNull()
  })

  // Walked to the end rather than checked at the first hop: an off-by-one in the cursor shows up as a
  // repeated row or a skipped one, and neither is visible in a single page.
  it('walks the whole roster once, in order, with no repeats', async () => {
    const seen: string[] = []
    let cursor: string | null = null

    do {
      const page: Awaited<ReturnType<typeof transport.roster>> = await transport.roster({
        filter: 'all',
        cursor,
      })
      seen.push(...page.items.map((patient) => patient.id))
      cursor = page.cursor
    } while (cursor !== null)

    expect(seen).toEqual(PATIENTS.map((patient) => patient.id))
    expect(new Set(seen).size).toBe(seen.length)
  })

  it('filters on the far side of the seam', async () => {
    const attention = await transport.roster({ filter: 'attention' })

    expect(attention.items.every((patient) => patient.status === 'attention')).toBe(true)
    expect(attention.total).toBe(PATIENTS.filter((patient) => patient.status === 'attention').length)
    expect(attention.total).toBeLessThan(PATIENTS.length)
  })

  // The empty state has to be reachable, or it is drawn against nothing and stays wrong until a real
  // clinic has no patients of some kind. Reached past the last page rather than by a filter nobody
  // matches, because every status has patients in this fixture.
  it('answers an empty page at the end rather than refusing', async () => {
    const attention = PATIENTS.filter((patient) => patient.status === 'attention')
    const last = attention.at(-1)
    // Asserted rather than optional-chained: an undefined cursor asks for the first page, which is a
    // different question and would pass for the wrong reason.
    expect(last).toBeDefined()

    const page = await transport.roster({ filter: 'attention', cursor: last!.id })

    expect(page.items).toEqual([])
    expect(page.total).toBe(attention.length)
    expect(page.cursor).toBeNull()
  })

  it('refuses a cursor that is not in the filter rather than silently starting over', async () => {
    // Real, and the shape a filter change produces: the cursor is a patient of another status.
    const outsider = PATIENTS.find((patient) => patient.status === 'track')
    expect(outsider).toBeDefined()

    await expect(transport.roster({ filter: 'attention', cursor: outsider!.id })).rejects.toThrow(
      /no patient/,
    )
  })
})

describe('the overview', () => {
  it('carries the triage patients themselves, not their ids', async () => {
    const overview = await transport.overview()

    expect(overview.triage.length).toBeGreaterThan(0)
    expect(overview.triage.every((patient) => patient.status === 'attention')).toBe(true)
    expect(overview.triage[0]).toHaveProperty('name')
  })

  it('carries the aggregates the screen shows', async () => {
    const { aggregates } = await transport.overview()

    expect(aggregates.patients).toBe(PATIENTS.length)
    expect(aggregates.averageAdherence).toBeGreaterThan(0)
  })
})

// Both states the components have to draw, and neither is reachable without the transport being able
// to produce it.
describe('what the seam can be made to do', () => {
  it('fails when it is told to', async () => {
    const broken = fixtureTransport({ failWith: new Error('the dashboard could not be reached') })

    await expect(broken.overview()).rejects.toThrow('could not be reached')
    await expect(broken.roster({ filter: 'all' })).rejects.toThrow('could not be reached')
  })

  it('answers later than it is asked, so there is a loading state at all', async () => {
    const slow = fixtureTransport({ latencyMs: 20 })
    let answered = false

    const pending = slow.overview().then(() => {
      answered = true
    })

    expect(answered).toBe(false)
    await pending
    expect(answered).toBe(true)
  })
})
