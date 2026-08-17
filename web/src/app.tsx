import { DataProvider } from './data/queries'
import { OverviewPage } from './features/overview/overview-page'

export function App() {
  return (
    <DataProvider>
      <OverviewPage />
    </DataProvider>
  )
}
