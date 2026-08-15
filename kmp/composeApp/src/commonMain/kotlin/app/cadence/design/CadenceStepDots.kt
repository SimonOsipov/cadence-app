package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp

/** One node per bar, so «tapping the third fills three» is a query rather than a look at pixels. */
fun cadenceStepDotTag(step: Int): String = "cadence-step-dot-$step"

/**
 * Which scale a row of bars is reading. A closed type does not stop the sheet handing
 * the sleep row the energy accent — nothing here can — but it makes the mapping a
 * function two literals can pin, which a raw `Color` argument would not be.
 */
enum class CadenceStepAccent {
    ENERGY,
    SLEEP,
}

/** «Энергия» is the green one, «Сон» the sand (`QuickFeelSheet.tsx:191,196`). */
internal fun stepAccentColor(accent: CadenceStepAccent): Color =
    when (accent) {
        CadenceStepAccent.ENERGY -> CadenceColors.forest700
        CadenceStepAccent.SLEEP -> CadenceColors.sand700
    }

/** Cumulative: every bar up to the answer is lit, not the answer's own bar alone. */
internal fun stepDotFilled(
    step: Int,
    value: Int,
): Boolean = step <= value

/**
 * What the bar is painted, from the same predicate its semantics carry. One decision
 * rather than two: written as a second `if` beside the semantics, inverting the paint
 * left every assertion green and every bar drawn backwards — measured, that mutation
 * survived the whole gate.
 */
internal fun stepDotFill(
    step: Int,
    value: Int,
    accent: CadenceStepAccent,
    palette: CadencePalette,
): Color = if (stepDotFilled(step, value)) stepAccentColor(accent) else palette.sunk

internal const val CADENCE_STEP_DOTS = 5

private val DOT_HEIGHT = 38.dp
private val DOT_CORNER = 12.dp
private val DOT_GAP = 8.dp

/**
 * A one-to-five answer, drawn as bars that fill **cumulatively**
 * (`QuickFeelSheet.tsx:54`): three means three bars lit, not the third one lit. A
 * single highlight would read as a choice between five things rather than an amount.
 *
 * [value] `0` is «not answered yet» and lights nothing, which the scale itself cannot
 * say — its lowest answer is one.
 */
@Composable
fun CadenceStepDots(
    value: Int,
    accent: CadenceStepAccent,
    onChange: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Row(modifier, horizontalArrangement = Arrangement.spacedBy(DOT_GAP)) {
        for (step in 1..CADENCE_STEP_DOTS) {
            val filled = stepDotFilled(step, value)

            Box(
                Modifier
                    .weight(1f)
                    .height(DOT_HEIGHT)
                    .background(stepDotFill(step, value, accent, palette), RoundedCornerShape(DOT_CORNER))
                    .clickable(role = Role.Button) { onChange(step) }
                    .semantics {
                        selected = filled
                        contentDescription = step.toString()
                    }.testTag(cadenceStepDotTag(step)),
            )
        }
    }
}
