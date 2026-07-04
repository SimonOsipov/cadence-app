// Cadence design tokens — ported from design-system/colors_and_type.css.
// Source of truth for color, type, radius, shadow. Never invent new values.

export const C = {
  paper: '#fbf8f3',
  cream: '#f6f1ea',
  linen: '#ede5d6',
  bone: '#e4dac6',
  border: '#cdc1a8',
  ink900: '#1a1a1a',
  ink800: '#2a2a2a',
  ink700: '#3f3b35',
  ink600: '#5c5852',
  ink500: '#8a857d',
  ink400: '#b3ad9f',
  ink300: '#d6cfbf',
  forest900: '#142c1f',
  forest800: '#1f4530',
  forest700: '#2d5f3f',
  forest600: '#3d7a52',
  forest500: '#4a8161',
  forest300: '#a6c2af',
  forest200: '#cadccf',
  forest100: '#d8e5db',
  forest50: '#eaf0eb',
  sand900: '#6b4a25',
  sand700: '#b8895a',
  sand500: '#d4a574',
  sand300: '#e8d4b8',
  sand100: '#f3e8d6',
  danger: '#b8503c',
  dangerBg: '#f4dfd6',
  warning: '#c2780a',
  warningBg: '#fbeed1',
  info: '#4a6b7a',
  infoBg: '#dfe6e9',
  hairline: 'rgba(26,26,26,0.08)',
} as const;

// Font families. Instrument Serif has no Cyrillic — the product copy is
// Russian, so display type uses the spec'd fallback Cormorant Garamond.
export const F = {
  display: 'CormorantGaramond_500Medium',
  displayItalic: 'CormorantGaramond_500Medium_Italic',
  body: 'DMSans_400Regular',
  bodyMedium: 'DMSans_500Medium',
  bodySemibold: 'DMSans_600SemiBold',
  mono: 'JetBrainsMono_400Regular',
  monoMedium: 'JetBrainsMono_500Medium',
} as const;

// Light "cream" palette — mirrors getPalette(false) in shared.jsx.
// Screens import this directly instead of receiving a `pal` prop.
export const pal = {
  bg: C.cream,
  paper: C.paper,
  sunk: C.linen,
  bone: C.bone,
  border: C.border,
  ink: C.ink900,
  ink2: C.ink800,
  muted: C.ink600,
  subtle: C.ink500,
  placeholder: C.ink400,
  forestBg: C.forest800,
  forestDeep: C.forest900,
  forestFg: C.cream,
  forestPill: { bg: C.forest50, fg: C.forest800 },
  sand: C.sand500,
  sand3: C.sand300,
  sand1: C.sand100,
  sand7: C.sand700,
  glassDark: 'rgba(20,44,31,.35)',
  hairline: 'rgba(26,26,26,0.08)',
  onForest: C.cream,
} as const;

export type Palette = typeof pal;

// Warm umbra shadows (rgba(46,38,24,…)) as RN shadow styles.
export const SHADOW = {
  xs: { shadowColor: '#2e2618', shadowOpacity: 0.05, shadowRadius: 2, shadowOffset: { width: 0, height: 1 }, elevation: 1 },
  sm: { shadowColor: '#2e2618', shadowOpacity: 0.06, shadowRadius: 6, shadowOffset: { width: 0, height: 2 }, elevation: 2 },
  md: { shadowColor: '#2e2618', shadowOpacity: 0.08, shadowRadius: 20, shadowOffset: { width: 0, height: 8 }, elevation: 6 },
  lg: { shadowColor: '#2e2618', shadowOpacity: 0.1, shadowRadius: 40, shadowOffset: { width: 0, height: 18 }, elevation: 12 },
} as const;

export const R = { xs: 4, sm: 8, md: 12, lg: 18, xl: 24, xxl: 32, pill: 999 } as const;
