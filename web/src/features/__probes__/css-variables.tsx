// Same purpose, the other rule. See aggregates.tsx.
export function Probe() {
  const inline = { color: 'var(--paper)' }
  const templated = `1px solid var( --bone )`

  return <div style={inline} data-border={templated} />
}
