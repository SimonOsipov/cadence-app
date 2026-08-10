package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import kotlin.math.round

/** The track, so a test can measure the fill against it. */
const val CADENCE_GAUGE_TAG = "cadence-gauge"

/** The fill itself, whose width is the only honest evidence of the fraction. */
const val CADENCE_GAUGE_FILL_TAG = "cadence-gauge-fill"

/** The row, so a bar can be measured against the height it is scaled to. */
const val CADENCE_USAGE_ROW_TAG = "cadence-usage-row"

/** One bar of the usage row, by week. */
fun usageBarTag(week: Int) = "cadence-usage-$week"

private const val PERCENT = 100f

private val GAUGE_HEIGHT = 6.dp
private val USAGE_HEIGHT = 32.dp
private val USAGE_BAR_WIDTH = 8.dp

/**
 * How full a vial is, as a fraction.
 *
 * A function beside the composable rather than arithmetic inside it, because a
 * drawing asserts nothing: the clamping and the zero-total case are testable
 * only if they exist somewhere a test can call.
 */
fun gaugeFraction(
    left: Int,
    total: Int,
): Float = if (total <= 0) 0f else (left.toFloat() / total).coerceIn(0f, 1f)

/**
 * «Доз осталось» — the meter on a vial card.
 *
 * The fill is a laid-out box rather than a canvas: a canvas's pixels are
 * asserted nowhere in this codebase, and the syringe bar shipped a barrel
 * painted full regardless of the dose until its fill had bounds to measure.
 */
@Composable
fun CadenceGauge(
    left: Int,
    total: Int,
    modifier: Modifier = Modifier,
) {
    val fraction = gaugeFraction(left, total)

    Box(
        modifier
            .fillMaxWidth()
            .height(GAUGE_HEIGHT)
            .clip(RoundedCornerShape(CadenceRadius.pill))
            .background(Cadence.palette.sunk)
            .testTag(CADENCE_GAUGE_TAG)
            .semantics {
                contentDescription = "$left из $total доз · ${round(fraction * PERCENT).toInt()} %"
            },
    ) {
        Box(
            Modifier
                .fillMaxWidth(fraction)
                .fillMaxHeight()
                .background(if (fraction < LOW_FRACTION) CadenceColors.danger else CadenceColors.forest700)
                .testTag(CADENCE_GAUGE_FILL_TAG),
        )
    }
}

/** §03's «low <25%», the same line `vialStatus` draws. */
private const val LOW_FRACTION = 0.25f

/**
 * Weekly usage, as bars.
 *
 * Scaled against the busiest week rather than against a fixed maximum: a
 * patient on one dose a week would otherwise read a row of slivers, and the
 * shape of their own rhythm is the only thing this row is for.
 */
@Composable
fun CadenceUsageRow(
    dosesPerWeek: List<Int>,
    modifier: Modifier = Modifier,
) {
    val tallest = dosesPerWeek.maxOrNull() ?: return

    Row(
        modifier.height(USAGE_HEIGHT).testTag(CADENCE_USAGE_ROW_TAG),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
        verticalAlignment = Alignment.Bottom,
    ) {
        dosesPerWeek.forEachIndexed { week, doses ->
            Box(
                Modifier
                    .width(USAGE_BAR_WIDTH)
                    // Floored, so a week with nothing in it is still a bar: a
                    // zero-height one is invisible, and a gap in a row reads as
                    // missing data rather than as a week off.
                    .fillMaxHeight(barFraction(doses, tallest))
                    .clip(RoundedCornerShape(CadenceRadius.xs))
                    .background(if (doses == 0) Cadence.palette.bone else CadenceColors.forest300)
                    .testTag(usageBarTag(week)),
            )
        }
    }
}

/** The floor a bar never goes below, so an empty week is still drawn. */
private const val MIN_BAR = 0.08f

private fun barFraction(
    doses: Int,
    tallest: Int,
): Float = if (tallest <= 0) MIN_BAR else (doses.toFloat() / tallest).coerceIn(MIN_BAR, 1f)
