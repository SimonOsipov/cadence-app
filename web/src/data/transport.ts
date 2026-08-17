import { OVERVIEW, PATIENTS, TRIAGE_IDS } from './fixtures/overview'
import type { Overview, RosterFilter, RosterPage } from './overview'

/**
 * What the Overview asks for, and the only thing it knows about where the answer comes from.
 *
 * Declared by the consumer so that swapping fixtures for `/v1` is one implementation and no component
 * edit. It is asynchronous from the first day for the same reason it is paginated from the first day:
 * a screen written against a synchronous list has no loading state and no error state, and adding them
 * later means opening every component again.
 */
export type Transport = {
  overview(): Promise<Overview>
  roster(query: RosterQuery): Promise<RosterPage>
}

export type RosterQuery = {
  filter: RosterFilter
  /** The id to continue after; null asks for the first page. */
  cursor?: string | null
}

/** How many rows a page carries. Small enough that the fixture has several — a paging bug needs pages. */
export const PAGE_SIZE = 8

export type FixtureOptions = {
  /** Milliseconds before answering. Zero in tests; a beat in the browser, so loading is visible. */
  latencyMs?: number

  /** When set, every call rejects with it. The error state has to be reachable to be drawn. */
  failWith?: Error
}

export function fixtureTransport({ latencyMs = 0, failWith }: FixtureOptions = {}): Transport {
  const answer = <T>(value: () => T): Promise<T> =>
    new Promise((resolve, reject) => {
      // try/catch and not a bare call: a throw inside a setTimeout callback escapes the executor, so
      // the promise never settles and the caller waits for ever. Measured — two tests hung five
      // seconds each before this.
      setTimeout(() => {
        if (failWith) return reject(failWith)

        try {
          resolve(value())
        } catch (error) {
          reject(error instanceof Error ? error : new Error(String(error)))
        }
      }, latencyMs)
    })

  return {
    overview: () =>
      answer(() => ({
        ...OVERVIEW,
        triage: TRIAGE_IDS.map(byId),
      })),

    roster: ({ filter, cursor = null }) =>
      answer(() => {
        // Filtered here, on the far side of the seam, because this is where the server will do it. A
        // component filtering a page it was handed would be filtering one page of many.
        const matching = PATIENTS.filter((patient) => filter === 'all' || patient.status === filter)

        const from = cursor === null ? 0 : matching.findIndex((patient) => patient.id === cursor) + 1
        if (cursor !== null && from === 0) {
          throw new Error(`no patient ${cursor} in this filter, so there is no page after them`)
        }

        const items = matching.slice(from, from + PAGE_SIZE)
        const next = matching[from + PAGE_SIZE]

        return { items, total: matching.length, cursor: next === undefined ? null : items.at(-1)?.id ?? null }
      }),
  }
}

function byId(id: string) {
  const patient = PATIENTS.find((candidate) => candidate.id === id)
  if (patient === undefined) {
    throw new Error(`the fixture names ${id} in triage and carries no such patient`)
  }

  return patient
}
