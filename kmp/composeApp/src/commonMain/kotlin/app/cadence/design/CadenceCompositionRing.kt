package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.format.formatDecimal

const val CADENCE_COMPOSITION_RING_TAG = "cadence-composition-ring"

internal const val FULL_TURN = 360f

private const val PERCENT_WHOLE = 100.0

private val RING_SIZE = 150.dp
private val RING_STROKE = 14.dp

/** Twelve o'clock, not three: an arc that starts on the right reads as a gauge rather than a share. */
private const val ARC_START = -90f

private val WEIGHT_SIZE = 30.sp
private const val WEIGHT_DIGITS = 1

/**
 * How far round the fat share reaches, in degrees. A reading outside 0..100 is a bad
 * measurement rather than a bad drawing: 140% would otherwise wind the arc past its
 * own start and draw as though it were 40%. `NaN` is named separately because
 * `coerceIn` hands it straight back — every comparison against it is false — and a
 * sweep is a number. Step-11's acceptance allows an undefined reading to reach here.
 */
internal fun compositionSweep(fatPercent: Double): Float {
    if (fatPercent.isNaN()) return 0f

    return (fatPercent.coerceIn(0.0, PERCENT_WHOLE) / PERCENT_WHOLE * FULL_TURN).toFloat()
}

/**
 * Body composition as one circle: the whole ring is the body, the sand arc is its fat
 * share, and what is left showing underneath is the lean mass. Lean is never computed
 * to be drawn — it is the part of the ring the arc does not cover, so the two halves
 * cannot disagree by a rounding.
 */
@Composable
fun CadenceCompositionRing(
    weightKg: Double,
    fatPercent: Double,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Box(
        modifier.size(RING_SIZE).testTag(CADENCE_COMPOSITION_RING_TAG),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(Modifier.fillMaxSize()) {
            val stroke = RING_STROKE.toPx()
            val inset = stroke / 2f
            val arcSize = Size(size.width - stroke, size.height - stroke)

            drawArc(
                color = CadenceColors.forest700,
                startAngle = ARC_START,
                sweepAngle = FULL_TURN,
                useCenter = false,
                topLeft = Offset(inset, inset),
                size = arcSize,
                style = Stroke(width = stroke),
            )
            drawArc(
                color = CadenceColors.sand500,
                startAngle = ARC_START,
                sweepAngle = compositionSweep(fatPercent),
                useCenter = false,
                topLeft = Offset(inset, inset),
                size = arcSize,
                // Butt, not round: a rounded end overshoots its own share at both ends.
                style = Stroke(width = stroke, cap = StrokeCap.Butt),
            )
        }

        CadenceNumber(
            value = formatDecimal(weightKg, digits = WEIGHT_DIGITS),
            unit = "кг",
            size = WEIGHT_SIZE,
            color = palette.ink,
        )
    }
}
