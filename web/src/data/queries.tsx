import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { createContext, useContext, type ReactNode } from 'react'

import type { RosterFilter } from './overview'
import { fixtureTransport, type RosterQuery, type Transport } from './transport'

const TransportContext = createContext<Transport | null>(null)

/**
 * Holds the transport and React Query for everything below it.
 *
 * The transport is injected rather than imported by the screens, which is what lets a test hand them a
 * failing one — an error state nobody can reach is an error state nobody has drawn.
 */
export function DataProvider({
  children,
  transport = fixtureTransport({ latencyMs: 220 }),
  client = defaultClient(),
}: {
  children: ReactNode
  transport?: Transport
  client?: QueryClient
}) {
  return (
    <QueryClientProvider client={client}>
      <TransportContext.Provider value={transport}>{children}</TransportContext.Provider>
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

export type { RosterFilter }
