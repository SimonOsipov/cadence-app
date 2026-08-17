// Not compiled and not shipped: the gate lints this file and requires every line below to be refused.
// A rule this load-bearing with no test of its own goes green for ever on a typo in a selector.
import type { Patient, RosterPage } from '../../data/overview'

export function Probe({ page, items }: { page: RosterPage; items: readonly Patient[] }) {
  const a = items.filter((p) => p.status === 'attention').length
  const b = page.items.filter((p) => p.status === 'attention').length
  const c = page.items.map((p) => p.adherence).reduce((x, y) => x + y, 0)
  const d = [...page.items].sort((x, y) => x.adherence - y.adherence)
  let e = 0
  for (const p of page.items) if (p.status === 'watch') e += 1
  let f = 0
  for (const p of items) if (p.status === 'watch') f += 1
  const g = items.map((p) => p.name)

  return <div>{[a, b, c, d.length, e, f, g.length].join(' ')}</div>
}
