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
 * The same alpha [CadenceButton] uses for its disabled state, redeclared here
 * since that one is file-private to `CadenceControls.kt`.
 */
private const val DISABLED_ALPHA = 0.65f

/**
 * A row of mutually exclusive options in a pill track — three ways of
 * recording a meal (`MODES` in `LogMealScreen.tsx:157-197`), a reminder's
 * lead time, or a recipe card's per-serving/whole toggle.
 *
 * No prior Kotlin segmented control existed (checked: `Segment*` and
 * `SegmentedControl` in `kmp/`, both empty). The prototype never unified this
 * into one component either — it has **three** separate implementations, not
 * two, each read from its own file:
 *
 * - `LogMealScreen.tsx:157-197`, the recording-mode picker this component's
 *   fixture is drawn from. Inline `View`. Track: `backgroundColor: pal.sunk,
 *   borderRadius: 12, padding: 3, gap: 2`; segments `flex: 1,
 *   paddingVertical: 10`. Active: `backgroundColor: pal.paper` + `SHADOW.xs`,
 *   tint `pal.ink`, `fontSize: 13`; resting: transparent, `pal.muted`.
 * - `SettingsScreen.tsx:58-103`, exported `SegControl` — the one actual
 *   shared component, reused at three call sites (`:358`, `:393`, `:413`).
 *   Track: `backgroundColor: pal.sunk, borderRadius: 999, padding: 3,
 *   gap: 4`; segments `flex: 1, paddingVertical: 8, paddingHorizontal: 10`.
 *   Active: solid `C.forest700`, label `C.cream`, `fontSize: 13`, no
 *   elevation; resting: transparent, `pal.muted`.
 * - `RecipeDetailScreen.tsx:236-272`, the recipe card's `На порцию`/`Всё`
 *   toggle — **not** `SegControl`; imports only `Eyebrow, Press` (`:8`), a
 *   third inline `View`. Track: `gap: 3` (not `SegControl`'s `4`). Segments
 *   carry **no `flex: 1`** — they hug content (`paddingVertical: 6,
 *   paddingHorizontal: 12`), so unlike the other two this row doesn't force
 *   equal widths. Active: same `C.forest700`/`C.cream` fill as `SegControl`
 *   but `fontSize: 12` (not `13`) — arrived at independently, in a different
 *   file. Sharing one colour pair is not the same file.
 *
 * This port follows the brief's own spec — `paper`/`ink`, the
 * `LogMealScreen.tsx` treatment, every segment `weight(1f)` — for both of
 * this component's own call sites, not `SegControl`'s or the recipe toggle's
 * variants. A future port of either should start from the three bullets
 * above, not from re-reading the `.tsx` files again.
 *
 * The label ships at `13.sp` — the `LogMealScreen.tsx`/`SettingsScreen.tsx`
 * size, and the same size sibling [CadenceChip] uses (`CadenceControls.kt:213`)
 * — not `RecipeDetailScreen.tsx`'s `12` or [CadenceMeta]'s default `12.sp`.
 * Set the same way [CadenceChip] does: an inline `fontSize` override via
 * `BasicText` directly, not a second sizing parameter on [CadenceMeta].
 *
 * Generic over `T`, not `List<String>`: a screen selects with its own enum,
 * and [label] is the one place a value becomes Russian copy — the same
 * data/presentation separation every primitive here keeps.
 *
 * Exactly one segment selected is expressed via [selected] semantics, not a
 * tag — the same choice [CadenceWeekBars] makes, since it also proves the
 * *other* segments aren't selected, which an existence check on a tag can't.
 * No fraction function to unit-test here: a segment either has the pill's
 * on-colours or it doesn't, and a semantics query answers that directly.
 *
 * A resting segment's label reads `palette.ink2`, not `palette.muted` — all
 * three prototype implementations above use `pal.muted`, but the brief's own
 * Step 3 prescribes `ink2`. Different tokens in this port's palette (`ink2 =
 * CadenceColors.ink800`, `muted = CadenceColors.ink600`,
 * `CadenceColors.kt:105-107`, mirroring `mobile/src/theme/index.ts:61-62`) —
 * a deliberate divergence from every prototype cited above, kept because the
 * brief is authoritative over them.
 *
 * [enabled] mutes the whole row — track included — blocking every segment's
 * click, not just the one that would fire: the prototype's reminders picker
 * goes fully inert while its parent toggle is off (`opacity: 0.4` on the
 * wrapping `View`, `pointerEvents="none"`, `SettingsScreen.tsx:339-346`), not
 * just the tapped segment. The alpha modifier chains *before*
 * `.background(palette.sunk)` for exactly this reason — the same order
 * [CadenceButton] uses (`CadenceControls.kt:132-138`) — so the track sits
 * inside the dimmed layer, not painted at full strength underneath it.
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
            // testTag before padding: placed after `.padding(...)` it would
            // report the padding-reduced inner box as its bounds, cropping a
            // captured screenshot away from the track's outer edge — exactly
            // where a dimming-order regression needs a test to reach.
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
                BasicText(
                    text = label(option),
                    style =
                        Cadence.typography.meta.copy(
                            color = if (isSelected) palette.ink else palette.ink2,
                            fontSize = 13.sp,
                        ),
                    // maxLines alongside overflow, not overflow alone — measured:
                    // a probe test rendering the same long string at the same
                    // width read 140.dp tall without maxLines, 14.dp with it (see
                    // theSegmentedControlStaysOneLineEvenWithATooLongLabel).
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}
