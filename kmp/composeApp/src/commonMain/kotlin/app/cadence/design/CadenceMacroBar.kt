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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import app.cadence.format.formatDecimal

/**
 * One bar's track, keyed by a stable id — three bars coexist on one screen. Not
 * [label]: that's Russian display copy, and a copy edit would silently break
 * every lookup keyed on it (`splitSegmentTag` in `CadenceSplitBar.kt` keys on
 * the same kind of stable, English id for the same reason).
 */
fun macroTrackTag(id: String) = "cadence-macro-track-$id"

/** One bar's fill, keyed the same way as [macroTrackTag] — see that KDoc for why. */
fun macroFillTag(id: String) = "cadence-macro-fill-$id"

/** `height: 5` on the prototype's `MacroBar` (`NutritionScreen.tsx:517-519`). */
private val MACRO_BAR_HEIGHT = 5.dp

/**
 * Value against goal, as a fraction. Clamped at the top like the prototype's
 * own `Math.min(1, v / Math.max(1, goal))` (`NutritionScreen.tsx:115`), so a
 * patient over goal reads a full bar, not one past its track — but not the
 * same function: at `goal = 0` the prototype's guard reads a **full** bar
 * (`v / 1`), while this deliberately returns an **empty** one, since a zero
 * goal means no target set, not a target of zero grams. Kept as a deliberate
 * divergence, not fixed to match the prototype.
 */
fun macroFraction(
    value: Double,
    goal: Double,
): Float = if (goal <= 0.0) 0f else (value / goal).toFloat().coerceIn(0f, 1f)

/**
 * «белок · 35 / 140 г» — one macronutrient's bar on the nutrition screen.
 *
 * Not [gaugeFraction]: that takes `Int` doses with the cabinet's vial
 * semantics — sharing it would couple this screen to dosing rules. `color` is
 * a parameter since the screen draws three of these, each its own colour.
 *
 * [id] is a stable English identifier (`"protein"`, `"carbs"`, `"fat"`),
 * separate from [label]'s Russian display copy: the two were one parameter
 * once, and keying [macroTrackTag]/[macroFillTag] on display copy meant a copy
 * edit could silently break every lookup.
 */
@Composable
fun CadenceMacroBar(
    id: String,
    label: String,
    value: Double,
    goal: Double,
    unit: String,
    color: Color,
    modifier: Modifier = Modifier,
) {
    val fraction = macroFraction(value, goal)

    Column(modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().padding(bottom = CadenceSpacing.xxs),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            CadenceMeta(label, color = Cadence.palette.muted)
            CadenceMeta(
                "${formatDecimal(value, digits = 0)} / ${formatDecimal(goal, digits = 0)} $unit",
                color = Cadence.palette.ink2,
            )
        }

        Box(
            Modifier
                .fillMaxWidth()
                .height(MACRO_BAR_HEIGHT)
                .clip(RoundedCornerShape(CadenceRadius.pill))
                .background(Cadence.palette.sunk)
                .testTag(macroTrackTag(id)),
        ) {
            Box(
                Modifier
                    .fillMaxWidth(fraction)
                    .fillMaxHeight()
                    .background(color)
                    .testTag(macroFillTag(id)),
            )
        }
    }
}
