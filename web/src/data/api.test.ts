import { readFileSync } from 'node:fs'

import { describe, expect, it } from 'vitest'

import { apiClient, OVERVIEW_PATH, type SessionAccess } from './api'

type Contract = { paths: Record<string, Record<string, unknown> | undefined> }

/** The document the types in src/api are generated from, read as the client's other half. */
function contract(): Contract {
  return JSON.parse(readFileSync('../api/openapi.json', 'utf8')) as Contract
}

type Call = { url: string; init: RequestInit | undefined }

/** A fetcher that answers one body and keeps what it was asked for. */
function answering(body: unknown, status = 200): { fetcher: typeof fetch; calls: Call[] } {
  const calls: Call[] = []

  const fetcher = (url: string, init?: RequestInit) => {
    calls.push({ url, init })

    return Promise.resolve(
      new Response(typeof body === 'string' ? body : JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  }

  return { fetcher: fetcher as unknown as typeof fetch, calls }
}

/** A session that never expires, for the calls that are not about expiry. */
function staticSession(token = 'a-session-token'): SessionAccess {
  return { token: () => token, refresh: () => Promise.reject(new Error('this session does not refresh')) }
}

/** Answers each call with the next of the given answers, so a retry can be told from a first try. */
function answeringInTurn(...answers: { status: number; body: unknown }[]): { fetcher: typeof fetch; calls: Call[] } {
  const calls: Call[] = []
  let served = 0

  const fetcher = (url: string, init?: RequestInit) => {
    calls.push({ url, init })

    const answer = answers[Math.min(served, answers.length - 1)]
    served += 1

    return Promise.resolve(
      new Response(JSON.stringify(answer?.body), {
        status: answer?.status ?? 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  }

  return { fetcher: fetcher as unknown as typeof fetch, calls }
}

function client(fetcher: typeof fetch, session: SessionAccess = staticSession()) {
  return apiClient({ baseUrl: 'https://api.example', session, fetcher })
}

// The generated types carry every shape the contract has and no path: a route renamed under an
// unchanged response is a change nothing in src/api can show, and the dashboard would find it as a
// 404 in a browser. So the one hand-written half is read back against the same document.
describe('the path this client calls', () => {
  it('is one the contract serves, by the method it is asked for', () => {
    const operations = contract().paths[OVERVIEW_PATH]

    expect(operations, `${OVERVIEW_PATH} is not in the contract`).toBeDefined()
    expect(Object.keys(operations ?? {})).toContain('get')
  })
})

describe('the roster call', () => {
  it('answers what the contract describes', async () => {
    const { fetcher } = answering({
      patients: [{ user_id: '3f2a', full_name: 'Марина Волкова', age: 38, invite_state: 'accepted' }],
      next_cursor: 'the-next-page',
    })

    const page = await client(fetcher).roster({})

    expect(page.patients[0]?.invite_state).toBe('accepted')
    expect(page.next_cursor).toBe('the-next-page')
  })

  it('sends the session token, or every request is answered 401', async () => {
    const { fetcher, calls } = answering({ patients: [] })

    await client(fetcher).roster({})

    expect(new Headers(calls[0]?.init?.headers).get('authorization')).toBe('Bearer a-session-token')
  })

  it('carries the cursor and the page size the caller asked for', async () => {
    const { fetcher, calls } = answering({ patients: [] })

    await client(fetcher).roster({ cursor: 'after-marina', limit: 25 })

    const asked = new URL(calls[0]?.url ?? '')

    expect(asked.pathname).toBe(OVERVIEW_PATH)
    expect(asked.searchParams.get('cursor')).toBe('after-marina')
    expect(asked.searchParams.get('limit')).toBe('25')
  })

  it('asks for the first page by leaving the cursor out, not by sending an empty one', async () => {
    const { fetcher, calls } = answering({ patients: [] })

    await client(fetcher).roster({})

    expect(new URL(calls[0]?.url ?? '').searchParams.has('cursor')).toBe(false)
  })

  // Every refusal of this API is one shape, and the detail below 500 is written for the person
  // reading it. A client that throws the status alone throws away the sentence.
  it('raises the refusal the API wrote, not the status code alone', async () => {
    const { fetcher } = answering(
      {
        type: '/problems/validation',
        title: 'Bad Request',
        status: 400,
        detail: 'Страница не найдена. Откройте реестр заново.',
      },
      400,
    )

    await expect(client(fetcher).roster({ cursor: 'nonsense' })).rejects.toThrow('Страница не найдена')
  })

  it('still raises something when the refusal carries no document at all', async () => {
    const { fetcher } = answering('<html>gateway</html>', 502)

    await expect(client(fetcher).roster({})).rejects.toThrow(/502/)
  })
})

// An access token runs out after an hour of a dashboard being open, and a doctor mid-page should not
// be sent back to a sign-in form for it.
describe('a token that has run out', () => {
  it('is refreshed once and the request is sent again', async () => {
    const { fetcher, calls } = answeringInTurn(
      { status: 401, body: { status: 401, title: 'Unauthorized' } },
      { status: 200, body: { patients: [] } },
    )

    let refreshed = 0
    const session: SessionAccess = {
      token: () => 'the-stale-token',
      refresh: () => {
        refreshed += 1

        return Promise.resolve('the-fresh-token')
      },
    }

    await client(fetcher, session).roster({})

    expect(refreshed).toBe(1)
    expect(new Headers(calls[0]?.init?.headers).get('authorization')).toBe('Bearer the-stale-token')
    expect(new Headers(calls[1]?.init?.headers).get('authorization')).toBe('Bearer the-fresh-token')
  })

  // Twice would be a loop: a token the API refuses twice is a session that has ended, and the screen
  // has to say so rather than hold the person in a retry nobody can see.
  it('is not refreshed a second time when the fresh one is refused too', async () => {
    const { fetcher, calls } = answeringInTurn(
      { status: 401, body: { status: 401 } },
      { status: 401, body: { status: 401 } },
    )

    let refreshed = 0
    const session: SessionAccess = {
      token: () => 'the-stale-token',
      refresh: () => {
        refreshed += 1

        return Promise.resolve('the-fresh-token')
      },
    }

    await expect(client(fetcher, session).roster({})).rejects.toThrow(/401/)
    expect(refreshed).toBe(1)
    expect(calls.length).toBe(2)
  })
})

// The sentence is not always the whole of it: a form has a better one for «this address is already
// somebody» than the API's, and it can only choose it by the status.
describe('a refusal', () => {
  it('carries the status it came with', async () => {
    const { fetcher } = answering({ status: 409, title: 'Conflict', detail: 'Адрес занят.' }, 409)

    await expect(client(fetcher).createPatient({ full_name: 'Марина', email: 'm@clinic.example', specialists: [] }))
      .rejects.toMatchObject({ status: 409, message: 'Адрес занят.' })
  })

  it('carries it even when there was no document to read', async () => {
    const { fetcher } = answering('<html>gateway</html>', 502)

    await expect(client(fetcher).staff()).rejects.toMatchObject({ status: 502 })
  })
})
