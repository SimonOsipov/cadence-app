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

## The sparkline is drawn, not vectored

**What the prototype does:** `Spark` in `shared.tsx` is an SVG `<Path>`.
**What we do:** a Compose `Canvas` with the same geometry — the padding, the
2dp stroke, the 3dp trailing dot, the 0.4 fill alpha are the prototype's values.
**Why:** Compose has no SVG. The numbers moved across as `Dp` rather than raw
floats, because a `DrawScope` measures in physical pixels and the prototype's
viewBox units are density-independent; passed straight through, the line would
be a third as thick on a 3x screen and different on each platform.

## A degenerate series is centred, not pinned to the top

**What the prototype does:** `(max - v) / (max - min || 1)` — with no span, the
`|| 1` sends every point to `y = pad`, the top edge. A single point lands at the
top-left corner.
**What we do:** a flat series sits on the centre line; a single measurement sits
in the middle of the box.
**Why:** the prototype never renders either, because all of its data is
hardcoded ascending — its `|| 1` is faithful to a path it never walks. Pinned to
the top with a fill, a flat series fills ~86% of the box and reads as "at
maximum", and «110, 110, 110» draws identically to «0,5 · 0,5 · 0,5»: the
vertical axis stops meaning anything while still looking like it does. A single
point in the top-left reads as an old, high reading, when the trailing dot marks
"now" at the right everywhere else. For adherence and biomarkers this is read
clinically, so "no change" is drawn as no change.

## An empty series draws a baseline, not nothing

**What the prototype does:** nothing — an empty `data` array yields an empty
`<Svg>`.
**What we do:** a hairline rule across the centre.
**Why:** an empty box beside a caption and an axis reads as "failed to load",
and a patient waits and retries instead of going and measuring. A rule says
"there is a scale here and no values on it".

## Non-finite values are dropped

**What the prototype does:** nothing — JavaScript propagates `NaN` silently.
**What we do:** filter them out of the series before any arithmetic.
**Why:** one `NaN` makes `max`, `min` and every coordinate `NaN`, and the whole
chart vanishes with nothing raised. A measurement pipeline that divides — a
ratio with a zero denominator, a unit conversion — produces them.

## The tab bar has four destinations and one action, not five tabs

**What the prototype does:** `TABS` is one list of five, with the centre entry
flagged `primary`, and `active` typed `TabId | null`.
**What we do:** `CadenceDestination` holds the four places; the centre action is
separate, with its own callback.
**Why:** an action is not a place. Keeping them in one set made "which
destination is current" answerable with the action, which draws a bar with
nothing highlighted — the state `TabId | null` expressed and this port had
claimed impossible while leaving it reachable. The showcase reached it on the
first tap of the centre button.

Also renamed: the prototype's `insights` is `TRENDS` here, matching the screen's
own name («Тренды») rather than its route key.
