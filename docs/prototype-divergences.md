# Divergences from the frozen prototype

`mobile/` is the visual specification for the patient app. A difference between
it and `kmp/` is a decision, not an accident — invariant 1 of the `kmp-app`
architecture note. Every one of them is written down here, with the reason.

Step 11 of the port block reviews this document against a side-by-side run of
the two apps. Entries are added as they are made, not reconstructed at the end.

## Body typeface: Golos Text, not DM Sans

**What the prototype does:** `F.body` in `mobile/src/theme/index.ts` is
`DMSans_400Regular`, with `DMSans_500Medium` and `DMSans_600SemiBold` beside it.

**What we do:** Golos Text at the same three weights.

**Why:** DM Sans publishes `latin` and `latin-ext` only. Every string this
product shows a user is Russian, so DM Sans would hand all of it to a system
fallback — the layout would hold and the type would not be the type. The
prototype had already hit exactly this on the display face (Instrument Serif
has no Cyrillic) and fell back to Cormorant Garamond; nobody re-ran the check
on the body face. Golos Text was designed for Cyrillic rather than extended
into it, and sits close to DM Sans in width and tone, so the prototype's
layouts do not shift.

Decision recorded 2026-07-28 in `specs/dashboard-skeleton.md`; it binds both
surfaces, so the dashboard changes `--font-body` in the same way.

Guarded by `BundledFontTest.everyBundledFaceCoversTheCyrillicAlphabet`, which
parses each bundled file's `cmap`. Swapping a Latin-only subset back in fails
it.

## Card shadows: the step survives, the warm tint does not

**What the prototype does:** `SHADOW` in `theme/index.ts` is a warm umbra —
`shadowColor: '#2e2618'` with an explicit opacity, radius and offset per step.

**What we do:** `CadenceElevation` carries the step only; Compose takes an
elevation and derives the shadow per platform.

**Why:** `Modifier.shadow` has no tint parameter that behaves the same on both
platforms. Reproducing the umbra would mean drawing our own shadow under every
card and sheet. Accepted at the design-system port (BST-05) as a visible but
bounded loss: the elevation *order* is what the layouts depend on, and that
survives intact.

## `border` and `ink700`: the mobile theme wins, and the two prototypes disagree

**What the prototype does:** `mobile/src/theme/index.ts` sets `border` to
`#cdc1a8` and defines `ink700` as `#3f3b35`.

**What we do:** exactly the same. `CadenceColors.kt` matches the mobile theme
value for value.

**Why this is listed at all:** the *web* prototype's `colors_and_type.css` sets
`--border` to `#e4dac6` — our `bone` — and has no `ink-700` at all; `#cdc1a8`
is its `--border-strong`. So the divergence is not between us and the
prototype, it is between the two prototypes, and the mobile one is canon for
the mobile app. Nothing to change here; the web side resolves it in its own
token port (`dashboard-skeleton`, step 2).

Verified name by name on 2026-08-03: `CadenceColors.kt` and `theme/index.ts`
agree on every value, and the icon sets match at 41 each.
