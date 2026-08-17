/**
 * The three colours a flag pill needs that the token file does not carry.
 *
 * This is the third of the four divergences the reconciliation names — «untokenized hex values in
 * components». The prototype writes these inline in FlagPill (`dd-components.jsx:50-55`); the CSS never
 * gave them a name, so the port cannot take one from `tokens`, and the port may not add one either.
 * They are collected here instead of being scattered through the components, and where the mobile app
 * has a name for the same colour, flag-palette.test.ts requires the two to agree.
 *
 * The other two tones need nothing: the sand pill's text is `tokens.sand900` and the forest pill's is
 * `tokens.forest800`, both already in the CSS.
 */
export const FLAG_INK = {
  /** CadenceColors.dangerFg on mobile. */
  danger: '#6b2818',

  /** CadenceColors.warningFg on mobile. */
  warning: '#7a4a06',

  /** Nobody's but the prototype's — neither surface names it, and this is the only place it lives. */
  info: '#2f4750',
} as const
