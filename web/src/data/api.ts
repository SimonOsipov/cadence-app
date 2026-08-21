import type { CreatedPatient, DashboardOverviewData, Me, NewPatientBody, Problem, RosterPage, StaffPage } from '../api'

/**
 * The route the roster is read from. Hand-written — the generated types carry every shape the contract
 * has and no path — and reconciled against the same document by api.test.ts.
 */
export const OVERVIEW_PATH = '/v1/dashboard/overview'

/** The route that says who is signed in. The dashboard greets them by the name it answers. */
export const ME_PATH = '/v1/me'

/** Where a patient is created, and where the clinic's staff is read from — POST and GET of one path. */
export const PATIENTS_PATH = '/v1/patients'
export const PROVIDERS_PATH = '/v1/providers'

/**
 * What this client needs of the session: the token to present, and a way to get a fresh one.
 *
 * Declared here by the consumer so that the refresh lives in one place — see src/auth/session.ts.
 * A refresh token may be spent once, so a client refreshing on its own would end the session of
 * whoever happened to have two requests in flight.
 */
export type SessionAccess = {
  token(): string
  refresh(): Promise<string>
}

export type ApiOptions = {
  /** Where the API listens. No trailing slash. */
  baseUrl: string

  session: SessionAccess

  /** Injected so a test can answer without a server, and never used to reach anything else. */
  fetcher?: typeof fetch | undefined
}

/** What the dashboard asks the roster for. Both are the contract's own, and both are optional there. */
export type RosterQuery = NonNullable<DashboardOverviewData['query']>

export type ApiClient = {
  roster(query: RosterQuery): Promise<RosterPage>

  /** Who the caller is, as the API describes them: their role, and the name to greet them by. */
  me(): Promise<Me>

  /** Everyone a care team may name. */
  staff(): Promise<StaffPage>

  /** Creates the patient and sends their invitation, which the API does as one action. */
  createPatient(patient: NewPatientBody): Promise<CreatedPatient>
}

export function apiClient({ baseUrl, session, fetcher = fetch }: ApiOptions): ApiClient {
  /**
   * Sends the request, and once — only once — sends it again with a token the provider has just
   * minted.
   *
   * A 401 here is an access token that has run out, which happens to every session that is open for
   * an hour. Retrying without refreshing would be a loop; retrying twice would be two loops.
   */
  async function ask(url: URL, body?: unknown): Promise<Response> {
    const send = (token: string) =>
      fetcher(url.toString(), {
        ...(body === undefined
          ? {}
          : { method: 'POST', body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } }),
        headers: {
          Accept: 'application/json',
          Authorization: `Bearer ${token}`,
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
        },
      })

    const answer = await send(session.token())
    if (answer.status !== 401) return answer

    return send(await session.refresh())
  }

  return {
    async roster({ cursor, limit }) {
      const url = new URL(OVERVIEW_PATH, baseUrl)

      // Set only when asked for. An empty cursor is not «the first page» to this API — it is a page
      // marker it did not issue, and it answers 400 for one.
      if (cursor !== undefined) url.searchParams.set('cursor', cursor)
      if (limit !== undefined) url.searchParams.set('limit', String(limit))

      const answer = await ask(url)

      if (!answer.ok) throw await refusal(answer)

      return (await answer.json()) as RosterPage
    },

    async me() {
      const answer = await ask(new URL(ME_PATH, baseUrl))

      if (!answer.ok) throw await refusal(answer)

      return (await answer.json()) as Me
    },

    async staff() {
      const answer = await ask(new URL(PROVIDERS_PATH, baseUrl))

      if (!answer.ok) throw await refusal(answer)

      return (await answer.json()) as StaffPage
    },

    async createPatient(patient) {
      const answer = await ask(new URL(PATIENTS_PATH, baseUrl), patient)

      if (!answer.ok) throw await refusal(answer)

      return (await answer.json()) as CreatedPatient
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
/**
 * A refusal, carrying the status as well as the sentence.
 *
 * The status is on the error because a screen sometimes has a better sentence than the API's: «this
 * address is already somebody» and «the invitation was not sent, and the patient was not created» are
 * things the person filling a form needs told, and both arrive as one shape with one detail.
 */
export class ApiRefusal extends Error {
  readonly status: number
  readonly problem: Problem | undefined

  constructor(status: number, message: string, problem?: Problem) {
    super(message)
    this.name = 'ApiRefusal'
    this.status = status
    this.problem = problem
  }
}

async function refusal(answer: Response): Promise<Error> {
  const said = await answer.text()

  try {
    const problem = JSON.parse(said) as Problem

    if (typeof problem.detail === 'string' && problem.detail !== '') {
      return new ApiRefusal(answer.status, problem.detail, problem)
    }
  } catch {
    // Not a problem document: a proxy's HTML, an empty body, a connection cut mid-answer. The status
    // is then the whole of what is known, and it is better than an exception about JSON.
  }

  return new ApiRefusal(answer.status, `the API answered ${answer.status}`)
}
