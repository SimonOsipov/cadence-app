import { icons, type IconName } from './icons'

type IconProps = {
  name: IconName
  size?: number

  /** Given only when the icon stands alone: with a label it is announced, without one it is skipped. */
  label?: string
}

/** One heroicon, drawn from the derived subset. A name outside that subset does not compile. */
export function Icon({ name, size = 20, label }: IconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      // currentColor rather than a prop with a default: an icon takes the colour of the text it sits
      // in, so a caller that styles the row has styled the icon.
      stroke="currentColor"
      strokeWidth={1.5}
      aria-hidden={label === undefined ? true : undefined}
      role={label === undefined ? undefined : 'img'}
    >
      {label !== undefined && <title>{label}</title>}
      {icons[name].map((d) => (
        <path key={d} d={d} strokeLinecap="round" strokeLinejoin="round" />
      ))}
    </svg>
  )
}
