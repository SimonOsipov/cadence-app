import type { DashboardOverviewData, Problem, RosterPage } from '../api'

/**
 * The route the roster is read from. Hand-written — the generated types carry every shape the contract
 * has and no path — and reconciled against the same document by api.test.ts.
 */
export const OVERVIEW_PATH = '/v1/dashboard/overview'

export type ApiOptions = {
  /** Where the API listens. No trailing slash. */
  baseUrl: string

  /** The session token, as the identity provider minted it. */
  token: string

  /** Injected so a test can answer without a server, and never used to reach anything else. */
  fetcher?: typeof fetch
}

/** What the dashboard asks the roster for. Both are the contract's own, and both are optional there. */
export type RosterQuery = NonNullable<DashboardOverviewData['query']>

export type ApiClient = {
  roster(query: RosterQuery): Promise<RosterPage>
}

export function apiClient({ baseUrl, token, fetcher = fetch }: ApiOptions): ApiClient {
  return {
    async roster({ cursor, limit }) {
      const url = new URL(OVERVIEW_PATH, baseUrl)

      // Set only when asked for. An empty cursor is not «the first page» to this API — it is a page
      // marker it did not issue, and it answers 400 for one.
      if (cursor !== undefined) url.searchParams.set('cursor', cursor)
      if (limit !== undefined) url.searchParams.set('limit', String(limit))

      const answer = await fetcher(url.toString(), {
        headers: { Accept: 'application/json', Authorization: `Bearer ${token}` },
      })

      if (!answer.ok) throw await refusal(answer)

      return (await answer.json()) as RosterPage
    },
  }
}

/**
 * Turns a refusal into the sentence the API wrote for the person reading it.
 *
 * Every non-2xx of this API is one shape — RFC 9457 problem details — and below 500 its `detail` is
 * Russian written for a human. A client that throws the status alone throws that away and leaves the
 * screen with nothing to show but a number.
 */
async function refusal(answer: Response): Promise<Error> {
  const said = await answer.text()

  try {
    const problem = JSON.parse(said) as Problem

    if (typeof problem.detail === 'string' && problem.detail !== '') {
      return new Error(problem.detail, { cause: problem })
    }
  } catch {
    // Not a problem document: a proxy's HTML, an empty body, a connection cut mid-answer. The status
    // is then the whole of what is known, and it is better than an exception about JSON.
  }

  return new Error(`the API answered ${answer.status}`)
}
