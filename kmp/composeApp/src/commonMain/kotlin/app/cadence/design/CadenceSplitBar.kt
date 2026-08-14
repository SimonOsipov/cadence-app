package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import app.cadence.format.formatDecimal

/** The bar, so a test can measure a segment's width against the whole of it. */
const val CADENCE_SPLIT_TRACK_TAG = "cadence-split-track"

/** One segment's fill, keyed by its macro — three segments share one row. */
fun splitSegmentTag(macro: String) = "cadence-split-segment-$macro"

/** One legend entry, so a test can bind a macro's grams to *its* label rather than to the row. */
fun splitLegendTag(macro: String) = "cadence-split-legend-$macro"

/**
 * The label and the grams *inside* one entry, separately: a probe that reads the whole
 * entry cannot tell [CadenceSplitBar]'s `labelColor` from its `valueColor` when the two
 * are swapped, since both colours stay somewhere in the entry either way.
 */
fun splitLegendLabelTag(macro: String) = "cadence-split-legend-label-$macro"

fun splitLegendValueTag(macro: String) = "cadence-split-legend-value-$macro"

/** §03's rule of thumb, the prototype's `kp = p*4, kc = c*4, kf = f*9` (`RecipeDetailScreen.tsx:18-20`). */
private const val KCAL_PER_G_PROTEIN = 4.0
private const val KCAL_PER_G_CARBS = 4.0
private const val KCAL_PER_G_FAT = 9.0

/** `height: 8` on the prototype's split bar (`RecipeDetailScreen.tsx:35`). */
private val SPLIT_BAR_HEIGHT = 8.dp

/** `width/height: 8` on the prototype's legend swatch (`RecipeDetailScreen.tsx:47`). */
private val LEGEND_SWATCH_SIZE = 8.dp

/**
 * The three shares of a meal's energy, by calories, not grams — grams would
 * draw three equal segments for equal grams, wrong by a factor of more than
 * two on fat, and right on any fixture whose grams happen to be equal, which
 * is how such a bug survives a suite. An all-zero meal returns three zero
 * shares rather than dividing by zero — nothing logged yet is a real state.
 */
fun splitShares(
    proteinG: Double,
    carbsG: Double,
    fatG: Double,
): Triple<Float, Float, Float> {
    val p = proteinG * KCAL_PER_G_PROTEIN
    val c = carbsG * KCAL_PER_G_CARBS
    val f = fatG * KCAL_PER_G_FAT
    val total = p + c + f

    if (total <= 0.0) return Triple(0f, 0f, 0f)

    return Triple((p / total).toFloat(), (c / total).toFloat(), (f / total).toFloat())
}

/**
 * A meal's energy split into protein / carbs / fat and drawn as one stacked
 * bar with a legend underneath — `RecipeMacroBar` in the prototype
 * (`RecipeDetailScreen.tsx:17-61`; `MacroBarDark` in
 * `RecipeBuilderScreen.tsx:20-75` for the builder card's dark variant). Not
 * [CadenceMacroBar]: that draws one macro against its own goal, this draws
 * three macros against each other's share of one meal.
 *
 * `Row` + `Modifier.weight(share)`, not three `fillMaxWidth(fraction)` boxes:
 * the three segments share one row, so each width is relative to the other
 * two, not to the row alone. A zero share is skipped rather than laid out at
 * `weight(0f)`, which Compose rejects.
 *
 * [trackColor]/[labelColor]/[valueColor] exist for that dark call site: the
 * defaults (`sunk`, `subtle`, `ink2`) are unreadable on `forest800`.
 *
 * Carb colour: the nutrition overview's per-macro `MacroBar` hardcodes carbs
 * to `#a5773d` (`NutritionScreen.tsx:518`), with no matching [CadenceColors]
 * token — but this component's own two call sites map carbs to `C.sand500`
 * (`MACRO_C` in both files), i.e. [CadenceColors.sand500] verbatim. This bar
 * follows its own call sites, not a different screen's literal.
 */
@Composable
fun CadenceSplitBar(
    proteinG: Double,
    carbsG: Double,
    fatG: Double,
    modifier: Modifier = Modifier,
    trackColor: Color = Cadence.palette.sunk,
    labelColor: Color = Cadence.palette.subtle,
    valueColor: Color = Cadence.palette.ink2,
) {
    val (proteinShare, carbsShare, fatShare) = splitShares(proteinG, carbsG, fatG)

    Column(modifier.fillMaxWidth()) {
        Row(
            Modifier
                .fillMaxWidth()
                .height(SPLIT_BAR_HEIGHT)
                .clip(RoundedCornerShape(CadenceRadius.pill))
                .background(trackColor)
                .testTag(CADENCE_SPLIT_TRACK_TAG),
        ) {
            if (proteinShare > 0f) {
                Box(
                    Modifier
                        .weight(proteinShare)
                        .fillMaxHeight()
                        .background(CadenceColors.forest700)
                        .testTag(splitSegmentTag("protein")),
                )
            }
            if (carbsShare > 0f) {
                Box(
                    Modifier
                        .weight(carbsShare)
                        .fillMaxHeight()
                        .background(CadenceColors.sand500)
                        .testTag(splitSegmentTag("carbs")),
                )
            }
            if (fatShare > 0f) {
                Box(
                    Modifier
                        .weight(fatShare)
                        .fillMaxHeight()
                        .background(CadenceColors.sand700)
                        .testTag(splitSegmentTag("fat")),
                )
            }
        }

        Row(
            Modifier.fillMaxWidth().padding(top = CadenceSpacing.sm),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.lg),
        ) {
            SplitLegendItem("Белок", "protein", proteinG, CadenceColors.forest700, labelColor, valueColor)
            SplitLegendItem("Углев", "carbs", carbsG, CadenceColors.sand500, labelColor, valueColor)
            SplitLegendItem("Жиры", "fat", fatG, CadenceColors.sand700, labelColor, valueColor)
        }
    }
}

/** One legend entry: a colour swatch, the macro's short Russian label, its grams. */
@Composable
private fun SplitLegendItem(
    label: String,
    macro: String,
    grams: Double,
    color: Color,
    labelColor: Color,
    valueColor: Color,
) {
    Row(
        Modifier.testTag(splitLegendTag(macro)),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
    ) {
        Box(
            Modifier
                .size(LEGEND_SWATCH_SIZE)
                .clip(RoundedCornerShape(CadenceRadius.xs))
                .background(color),
        )
        CadenceMeta(label, color = labelColor, modifier = Modifier.testTag(splitLegendLabelTag(macro)))
        CadenceMeta(
            "${formatDecimal(grams, digits = 0)} г",
            color = valueColor,
            modifier = Modifier.testTag(splitLegendValueTag(macro)),
        )
    }
}
