import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ReactNode } from 'react'

import type { ApiClient } from './api'
import { DataProvider, defaultClient, useLiveRoster } from './queries'

function reading(api: ApiClient) {
  return ({ children }: { children: ReactNode }) => (
    <DataProvider api={api} client={defaultClient()}>
      {children}
    </DataProvider>
  )
}

const noIdentity = () => Promise.reject(new Error('these tests do not ask who is signed in'))

const onePage: ApiClient = {
  me: noIdentity,
  roster: () =>
    Promise.resolve({
      patients: [{ user_id: '3f2a', full_name: 'Марина Волкова', age: 38, invite_state: 'accepted' }],
    }),
}

describe('the live roster', () => {
  it('answers the page the API gave it', async () => {
    const { result } = renderHook(() => useLiveRoster(), { wrapper: reading(onePage) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.patients[0]?.full_name).toBe('Марина Волкова')
  })

  it('asks again for a different page rather than answering the first from cache', async () => {
    const asked: (string | undefined)[] = []
    const paging: ApiClient = {
      me: noIdentity,
      roster: ({ cursor }) => {
        asked.push(cursor)

        return Promise.resolve({ patients: [] })
      },
    }

    const { rerender } = renderHook((cursor?: string) => useLiveRoster(cursor === undefined ? {} : { cursor }), {
      wrapper: reading(paging),
      initialProps: undefined,
    })

    await waitFor(() => expect(asked).toEqual([undefined]))

    rerender('after-marina')
    await waitFor(() => expect(asked).toEqual([undefined, 'after-marina']))
  })

  // The refusal reaches the screen: an error state nobody can produce is an error state nobody drew.
  it('reports the refusal the API wrote', async () => {
    const refusing: ApiClient = {
      me: noIdentity,
      roster: () => Promise.reject(new Error('Страница не найдена. Откройте реестр заново.')),
    }

    const { result } = renderHook(() => useLiveRoster(), { wrapper: reading(refusing) })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error?.message).toContain('Страница не найдена')
  })

  // The dashboard runs on fixtures until step 7, so a screen reaching for live data outside a provider
  // that has an API has to say so rather than render an empty roster.
  it('refuses to read outside a provider that was given an API', () => {
    const withoutApi = ({ children }: { children: ReactNode }) => (
      <DataProvider client={defaultClient()}>{children}</DataProvider>
    )

    expect(() => renderHook(() => useLiveRoster(), { wrapper: withoutApi })).toThrow(/no API/)
  })
})
