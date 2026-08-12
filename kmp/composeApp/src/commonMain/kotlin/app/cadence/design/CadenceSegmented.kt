package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.disabled
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.sp

/** The track, so a test can capture a pixel of it that no segment draws over. */
const val CADENCE_SEGMENTED_TRACK_TAG = "cadence-segmented-track"

/**
 * A dimmed disabled control still has to be legible next to a live one — the
 * same alpha [CadenceButton] uses for its own disabled state, redeclared here
 * because that one is file-private to `CadenceControls.kt`.
 */
private const val DISABLED_ALPHA = 0.65f

/**
 * A row of mutually exclusive options in a pill track — three ways of
 * recording a meal (`MODES` in `LogMealScreen.tsx:157-197`), a reminder's
 * lead time, or a recipe card's per-serving/whole toggle.
 *
 * No Kotlin segmented control existed before this one — checked by searching
 * `kmp/` for `Segment*` and `SegmentedControl` before writing this file, both
 * empty. The prototype itself never unified the idea into one component
 * either; it has **three** separate implementations — not two — and a later
 * port of any of them should read *this* paragraph rather than rediscover
 * the split by diffing pixels. Each entry below was read from its own file,
 * not inferred from another:
 *
 * - `LogMealScreen.tsx:157-197`, the recording-mode picker this component's
 *   own first test fixture is drawn from. One inline `View`, not a shared
 *   component. Track: `backgroundColor: pal.sunk, borderRadius: 12,
 *   padding: 3, gap: 2`. Segments are `flex: 1`, `paddingVertical: 10`.
 *   Active segment: `backgroundColor: pal.paper` plus `SHADOW.xs`,
 *   icon/label tinted `pal.ink`, `fontSize: 13`; resting segments are
 *   `'transparent'` with `pal.muted`.
 * - `SettingsScreen.tsx:58-103`, exported as `SegControl` — the one actual
 *   shared component, imported and reused at three call sites in that same
 *   file: the reminders lead-time picker (`:358`), the weight-unit picker
 *   (`:393`), and the time-format picker (`:413`). Track:
 *   `backgroundColor: pal.sunk, borderRadius: 999, padding: 3, gap: 4`.
 *   Segments are `flex: 1`, `paddingVertical: 8, paddingHorizontal: 10`.
 *   Active segment: solid `backgroundColor: C.forest700`, label
 *   `color: C.cream`, `fontSize: 13`, no elevation; resting segments are
 *   `'transparent'` with `pal.muted`.
 * - `RecipeDetailScreen.tsx:236-272`, the recipe card's `На порцию`/`Всё`
 *   toggle — **not** `SegControl`. This file imports only `Eyebrow, Press`
 *   from `../../components/primitives` (`:8`); the toggle is a third,
 *   inline `View`, distinct from both of the above. Track:
 *   `backgroundColor: pal.sunk, borderRadius: 999, padding: 3, gap: 3` (not
 *   `SegControl`'s `4`). Segments carry **no `flex: 1`** — they hug their
 *   own content (`paddingVertical: 6, paddingHorizontal: 12`), so unlike
 *   the other two this row does not force equal-width segments. Active
 *   segment: solid `backgroundColor: C.forest700`, label `color: C.cream`,
 *   `fontSize: 12` (not `13`) — the same fill as `SegControl`, but arrived
 *   at independently, in a different file, with a different track gap, a
 *   different segment shape and a different font size. Sharing one colour
 *   pair is not the same file.
 *
 * This port follows the brief's own spec — `paper`/`ink`, the
 * `LogMealScreen.tsx` treatment, every segment `weight(1f)` — for both of
 * this component's own call sites (a recording mode and a per-serving
 * toggle), rather than inventing a `kind` parameter for a distinction the
 * brief does not ask for. A future port of `SettingsScreen.tsx`'s
 * `SegControl` (three call sites) or of `RecipeDetailScreen.tsx`'s inline
 * toggle (one call site, hug-content segments, `fontSize: 12`) will need to
 * decide whether either is a genuine second and third visual variant of
 * this component or a divergence a port should normalise; either way, that
 * decision starts from the three bullets above, not from re-reading three
 * `.tsx` files again.
 *
 * The label ships at `13.sp` — the `LogMealScreen.tsx` and `SettingsScreen.tsx`
 * size, two of the three bullets above, and the same size the sibling
 * [CadenceChip] uses for its own label (`CadenceControls.kt:213`) — rather
 * than `RecipeDetailScreen.tsx`'s `12` or [CadenceMeta]'s own default `12.sp`.
 * Set the same way [CadenceChip] sets it: an inline `fontSize` override on
 * the shared style, via `BasicText` directly, not a second sizing parameter
 * grafted onto [CadenceMeta].
 *
 * Generic over `T`, not `List<String>`: a screen selects with its own enum
 * (a meal mode, a lead time, a portion mode) and [label] is the one place
 * that turns a value into Russian copy, the same separation of data from
 * presentation every other primitive in this file keeps.
 *
 * Exactly one segment is selected at a time, and that fact is expressed with
 * [selected] semantics rather than a tag — the same choice
 * [CadenceWeekBars] makes for its highlighted day, because it also proves
 * the *other* segments are not selected, which an existence check on a tag
 * cannot. No geometry beyond spacing/radius tokens is needed here, so unlike
 * the bar and ring primitives in this file there is no pure fraction
 * function to unit-test: a segment either has the pill's on-colours or it
 * does not, and a semantics query answers that directly.
 *
 * A resting segment's label reads `palette.ink2`, not `palette.muted` —
 * every one of the three prototype implementations above uses `pal.muted`
 * for this role, but the brief's own Step 3 prescribes `ink2`. `ink2` and
 * `muted` are two different tokens in this port's own palette (`ink2 =
 * CadenceColors.ink800`, `muted = CadenceColors.ink600` —
 * `CadenceColors.kt:105-107`, mirroring `mobile/src/theme/index.ts:61-62`),
 * so this is a real, deliberate divergence from every prototype source
 * cited above, kept because the brief is authoritative over them — not an
 * accident of this port matching its source.
 *
 * [enabled] mutes the whole row — track included — and blocks every
 * segment's click, rather than only the one that would otherwise fire: the
 * prototype's own reminders lead-time picker goes fully inert while its
 * parent toggle is off (`opacity: 0.4` on the whole wrapping `View`,
 * `pointerEvents="none"`, `SettingsScreen.tsx:339-346`), not just the
 * segment a patient happens to tap, and not just the segments' own content.
 * The alpha modifier is chained *before* `.background(palette.sunk)` for
 * exactly this reason — the same order [CadenceButton] uses
 * (`CadenceControls.kt:132-138`) — so the track is inside the dimmed layer
 * rather than painted at full strength underneath it.
 */
@Composable
fun <T> CadenceSegmented(
    options: List<T>,
    selected: T,
    onSelect: (T) -> Unit,
    label: (T) -> String,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    val palette = Cadence.palette

    Row(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.pill))
            // testTag before padding: a modifier placed after `.padding(...)`
            // reports the padding-reduced inner box as its own bounds, which
            // would crop a captured screenshot to the content area and hide
            // the track's own outer edge — exactly the region a dimming-order
            // regression needs a test to reach.
            .testTag(CADENCE_SEGMENTED_TRACK_TAG)
            .then(if (enabled) Modifier else Modifier.alpha(DISABLED_ALPHA))
            .background(palette.sunk)
            .padding(CadenceSpacing.xxs),
    ) {
        options.forEach { option ->
            val isSelected = option == selected

            Box(
                Modifier
                    .weight(1f)
                    .clip(RoundedCornerShape(CadenceRadius.pill))
                    .background(if (isSelected) palette.paper else Color.Transparent)
                    .clickable(enabled = enabled) { onSelect(option) }
                    .semantics(mergeDescendants = true) {
                        this.selected = isSelected
                        if (!enabled) disabled()
                    }.padding(vertical = CadenceSpacing.sm, horizontal = CadenceSpacing.sm),
                contentAlignment = Alignment.Center,
            ) {
                // Same mechanism as CadenceChip (CadenceControls.kt:213): an inline
                // fontSize override on the shared style, via BasicText directly,
                // rather than a second sizing parameter on CadenceMeta.
                BasicText(
                    text = label(option),
                    style =
                        Cadence.typography.meta.copy(
                            color = if (isSelected) palette.ink else palette.ink2,
                            fontSize = 13.sp,
                        ),
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}
