// Typechecked with everything else — the type-aware rules need it in the project — and shipped by
// nothing, since no module imports it. The gate lints this file and requires every line below to be refused.
// A rule this load-bearing with no test of its own goes green for ever on a typo in a selector.
// Every name in `rowsNamed` appears, not just one: with only `items` exercised, narrowing the
// alternation to /^(items)$/ kept all the refusals while `patients.filter(…)` — the shape the rule's
// own message quotes — stopped being caught.
import type { Patient, RosterPage, ScheduleEntry } from '../../data/overview'

export function Probe({
  page,
  items,
  patients,
  groups,
}: {
  page: RosterPage
  items: readonly Patient[]
  patients: readonly Patient[]
  groups: { roster: readonly Patient[]; triage: readonly Patient[]; schedule: readonly ScheduleEntry[] }
}) {
  const a = items.filter((p) => p.status === 'attention').length
  const b = page.items.filter((p) => p.status === 'attention').length
  const c = page.items.map((p) => p.adherence).reduce((x, y) => x + y, 0)
  const d = [...page.items].sort((x, y) => x.adherence - y.adherence)
  let e = 0
  for (const p of page.items) if (p.status === 'watch') e += 1
  let f = 0
  for (const p of items) if (p.status === 'watch') f += 1
  const g = items.map((p) => p.name)
  const h = patients.filter((p) => p.status === 'attention').length
  const i = groups.roster.some((p) => p.status === 'attention')
  const j = groups.schedule.filter((entry) => entry.state === 'done').length
  let k = 0
  for (const p of groups.triage) if (p.status === 'attention') k += 1

  return <div>{[a, b, c, d.length, e, f, g.length, h, i, j, k].join(' ')}</div>
}
