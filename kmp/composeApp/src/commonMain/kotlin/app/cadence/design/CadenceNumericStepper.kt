package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.unit.dp
import app.cadence.format.formatDecimal
import kotlin.math.pow
import kotlin.math.round

/**
 * The next value, clamped and rounded.
 *
 * A function beside the composable because the clamping is the whole contract
 * and a drawing asserts nothing. Scaled integer arithmetic rather than raw
 * `Double` addition: a tenth of a gram accumulates binary error the way
 * `CadenceDoseStepper` documents at `DOSE_SCALE`, and the number the patient
 * reads must be the number that is stored.
 *
 * `max = null` is «no ceiling», not «no limit worth writing»: the grams of a
 * parsed meal item have a floor and no top (`LogMealScreen.tsx:880`), while the
 * ingredient sheet caps at 600.
 */
fun steppedValue(
    value: Double,
    delta: Double,
    min: Double,
    max: Double?,
    decimals: Int,
): Double {
    val scale = TEN.pow(decimals)
    val next = round((value + delta) * scale) / scale
    val floored = next.coerceAtLeast(min)

    return if (max == null) floored else floored.coerceAtMost(max)
}

private const val TEN = 10.0

const val CADENCE_STEPPER_MINUS_TAG = "cadence-stepper-minus"
const val CADENCE_STEPPER_PLUS_TAG = "cadence-stepper-plus"

/** The number's own row, so a test can count how many text runs it composed. */
const val CADENCE_STEPPER_VALUE_TAG = "cadence-stepper-value"

/** Same tap target as the dose stepper's own button — one size across both. */
private val STEPPER_BUTTON = 52.dp

/** `strokeWidth={2}` on the prototype's minus. */
private val MINUS_BAR = 2.dp

/**
 * A numeric field with a fractional step, a floor and an optional ceiling.
 *
 * Reports a value, never a delta: the caller stores what it is shown. `unit`
 * is forwarded to [CadenceNumber] unchanged — a caller with nothing to show
 * (a plain count: servings, steps, minutes) passes `null`, and `CadenceNumber`
 * composes no unit run at all rather than an empty one with its own trailing
 * `spacedBy` gap.
 */
@Composable
fun CadenceStepper(
    value: Double,
    onChange: (Double) -> Unit,
    min: Double,
    max: Double?,
    step: Double,
    decimals: Int,
    modifier: Modifier = Modifier,
    unit: String? = null,
) {
    Row(
        modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        StepperButton("Уменьшить", CADENCE_STEPPER_MINUS_TAG) {
            onChange(steppedValue(value, -step, min, max, decimals))
        }

        CadenceNumber(
            value = formatDecimal(value, decimals),
            unit = unit,
            modifier =
                Modifier
                    .padding(horizontal = CadenceSpacing.xl)
                    .testTag(CADENCE_STEPPER_VALUE_TAG),
        )

        StepperButton("Увеличить", CADENCE_STEPPER_PLUS_TAG, CadenceIcons.plus) {
            onChange(steppedValue(value, step, min, max, decimals))
        }
    }
}

/**
 * A private copy of `CadenceStepper.kt`'s button, with a `testTag` and no tie
 * to a [app.cadence.shared.domain.Dose]: the dose stepper stays as it is, and
 * the two coexist rather than share one parametrised button.
 */
@Composable
private fun StepperButton(
    label: String,
    tag: String,
    icon: List<String>? = null,
    onClick: () -> Unit,
) {
    Box(
        Modifier
            .size(STEPPER_BUTTON)
            .clip(CircleShape)
            .background(Cadence.palette.paper)
            .clickable(onClick = onClick)
            .testTag(tag)
            .clearAndSetSemantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        if (icon == null) {
            // A bar rather than an icon: there is no «минус» in `CadenceIcons`,
            // and `CadenceIcon` draws nothing for a name it does not know — the
            // same bug the dose stepper documents at `CadenceStepper.kt`.
            Box(
                Modifier
                    .size(width = CadenceSpacing.xxl, height = MINUS_BAR)
                    .clip(RoundedCornerShape(CadenceRadius.pill))
                    .background(Cadence.palette.ink2),
            )
        } else {
            CadenceIcon(paths = icon, size = CadenceSpacing.xxl, tint = Cadence.palette.ink2)
        }
    }
}
