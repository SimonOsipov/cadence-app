import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { createContext, useContext, type ReactNode } from 'react'

import type { ApiClient, RosterQuery as LiveRosterQuery } from './api'
import type { RosterFilter } from './overview'
import { fixtureTransport, type RosterQuery, type Transport } from './transport'

const TransportContext = createContext<Transport | null>(null)

/**
 * The generated contract's client, when this dashboard has one.
 *
 * Null by default and not a fixture: the screens read the fixture transport above until step 7 moves
 * them across, and a stand-in here would let a screen read live data that is not live.
 */
const ApiContext = createContext<ApiClient | null>(null)

/**
 * Holds the transport and React Query for everything below it.
 *
 * The transport is injected rather than imported by the screens, which is what lets a test hand them a
 * failing one — an error state nobody can reach is an error state nobody has drawn.
 */
export function DataProvider({
  children,
  transport = fixtureTransport({ latencyMs: 220 }),
  api = null,
  client = defaultClient(),
}: {
  children: ReactNode
  transport?: Transport
  api?: ApiClient | null
  client?: QueryClient
}) {
  return (
    <QueryClientProvider client={client}>
      <ApiContext.Provider value={api}>
        <TransportContext.Provider value={transport}>{children}</TransportContext.Provider>
      </ApiContext.Provider>
    </QueryClientProvider>
  )
}

/** No retries and no refetch on focus: a test would wait out the former, and a doctor watching a
 * dashboard reload itself on every window switch is a doctor reading a flicker. */
export function defaultClient(): QueryClient {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  })
}

function useTransport(): Transport {
  const transport = useContext(TransportContext)
  if (transport === null) {
    throw new Error('this screen is outside a DataProvider, so it has nothing to read')
  }

  return transport
}

export function useOverview() {
  const transport = useTransport()

  return useQuery({ queryKey: ['overview'], queryFn: () => transport.overview() })
}

export function useRoster(query: RosterQuery) {
  const transport = useTransport()

  return useQuery({
    // The filter and the cursor are both in the key: a page of «attention» is not a page of «all», and
    // caching them together is how a tab switch shows the wrong rows for a frame.
    queryKey: ['roster', query.filter, query.cursor ?? null],
    queryFn: () => transport.roster(query),
    // The previous page stays on screen while the next one loads, so paging does not blink the table
    // away and back.
    placeholderData: (previous) => previous,
  })
}

/** Reads the roster from the API rather than from the fixtures. */
export function useLiveRoster(query: LiveRosterQuery = {}) {
  const api = useContext(ApiContext)
  if (api === null) {
    throw new Error('this screen is inside a DataProvider with no API, so there is nothing live to read')
  }

  return useQuery({
    // Both bounds in the key, for the reason useRoster states: a page is not the page before it, and
    // one cached under the other shows the wrong rows for a frame.
    queryKey: ['live-roster', query.cursor ?? null, query.limit ?? null],
    queryFn: () => api.roster(query),
    placeholderData: (previous) => previous,
  })
}

export type { RosterFilter }
