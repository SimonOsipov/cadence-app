/**
 * How numbers become text on this surface, and the only place it happens.
 *
 * The project rule is that numbers are data and formatting is presentation: a weight is 104.2 and
 * «104,2 кг» is a render concern. Russian writes the decimal with a comma, so a component that
 * interpolates a number directly is a component showing «104.2» to a doctor — which is what the first
 * screenshot comparison against the prototype caught.
 *
 * One module per surface, never two. The mobile app has its own; that is the same rule, not a
 * duplicate of this one.
 */
const RU = 'ru-RU'

/** A weight, a dose, any measured quantity: one decimal at most, and never a trailing «,0» that the
 * source did not have. */
export function decimal(value: number, places = 1): string {
  return new Intl.NumberFormat(RU, { maximumFractionDigits: places }).format(value)
}

/** A quantity with its unit, spaced as Russian typography spaces it. */
export function quantity(value: number, unit: string, places = 1): string {
  return `${decimal(value, places)} ${unit}`
}

/** A whole number: counts, percentages, weeks. */
export function whole(value: number): string {
  return new Intl.NumberFormat(RU, { maximumFractionDigits: 0 }).format(value)
}
