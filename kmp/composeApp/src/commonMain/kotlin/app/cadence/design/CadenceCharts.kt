package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// Ported from `Spark` in mobile/src/components/shared.tsx. The prototype draws
// it in SVG, which Compose has no equivalent for, so the geometry moves to
// drawing commands — the numbers are the prototype's, not re-derived.

private const val SPARK_PAD = 2f
private const val SPARK_STROKE = 2f
private const val SPARK_DOT_RADIUS = 3f
private const val SPARK_FILL_ALPHA = 0.4f

/**
 * An inline trend sparkline: a polyline with a dot on the latest point, and an
 * optional area under it.
 *
 * Every call site passes a width computed from the layout it sits in, which is
 * why the size is parameters rather than something the caller wraps.
 *
 * Degenerate series are the normal case, not an edge one — a patient in their
 * first week has one measurement, and three identical readings are common —
 * and both make the prototype's formula divide by zero. Neither may crash and
 * neither may draw nothing where a value exists.
 */
@Composable
fun CadenceSpark(
    data: List<Float>,
    modifier: Modifier = Modifier,
    color: Color = CadenceColors.forest700,
    fill: Color? = null,
    width: Dp = 120.dp,
    height: Dp = 36.dp,
) {
    Canvas(modifier = modifier.size(width, height)) {
        val points = sparkPoints(data, size.width, size.height)
        if (points.isEmpty()) return@Canvas

        if (points.size > 1) {
            val line =
                Path().apply {
                    moveTo(points.first().x, points.first().y)
                    points.drop(1).forEach { lineTo(it.x, it.y) }
                }

            if (fill != null) {
                val area =
                    Path().apply {
                        addPath(line)
                        lineTo(points.last().x, size.height - SPARK_PAD)
                        lineTo(SPARK_PAD, size.height - SPARK_PAD)
                        close()
                    }
                drawPath(area, fill, alpha = SPARK_FILL_ALPHA)
            }

            drawPath(line, color, style = Stroke(width = SPARK_STROKE, cap = StrokeCap.Round))
        }

        drawCircle(color, radius = SPARK_DOT_RADIUS, center = points.last())
    }
}

/**
 * Where a sparkline's points land inside a box of [canvasWidth] × [canvasHeight].
 *
 * Pulled out of the drawing because it is the only part with arithmetic, and
 * arithmetic that divides needs to be checkable without a Canvas: a NaN
 * coordinate does not crash a draw, it silently paints nothing, so a test that
 * only measures the box would pass over a broken chart.
 *
 * Both divisions in the prototype's formula have a zero case, and both are
 * ordinary data rather than corruption — see [CadenceSpark].
 */
internal fun sparkPoints(
    data: List<Float>,
    canvasWidth: Float,
    canvasHeight: Float,
): List<Offset> {
    if (data.isEmpty()) return emptyList()

    val max = data.max()
    val min = data.min()
    // A flat series has no span to scale against; the prototype writes `|| 1`
    // for the same reason. Every point then lands on the top edge.
    val span = if (max == min) 1f else max - min
    val innerWidth = canvasWidth - SPARK_PAD * 2
    val innerHeight = canvasHeight - SPARK_PAD * 2
    // One point has no step. It draws as the dot alone, which is the honest
    // rendering of a single measurement.
    val step = if (data.size == 1) 0f else innerWidth / (data.size - 1)

    return data.mapIndexed { index, value ->
        Offset(
            x = SPARK_PAD + index * step,
            y = SPARK_PAD + (max - value) / span * innerHeight,
        )
    }
}
