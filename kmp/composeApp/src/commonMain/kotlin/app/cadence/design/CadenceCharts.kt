package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// Ported from `Spark` in mobile/src/components/shared.tsx (SVG -> drawing
// commands). Values below are Dp, not raw floats: the prototype's viewBox units
// are density-independent, but a DrawScope measures physical pixels, and raw
// floats would draw a third as thick on a 3x screen.

private val SPARK_PAD = 2.dp
private val SPARK_STROKE = 2.dp
private val SPARK_DOT_RADIUS = 3.dp
private const val SPARK_FILL_ALPHA = 0.4f

/**
 * An inline trend sparkline: a polyline with a dot on the latest point, and an
 * optional area beneath it. Size is parameters, not something the caller wraps,
 * because every call site computes its width from the surrounding layout.
 * [fill] is expected opaque — its alpha multiplies with the area's own 0.4.
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
    val palette = Cadence.palette

    Canvas(modifier = modifier.size(width, height)) {
        val pad = SPARK_PAD.toPx()
        val points = sparkPoints(data, size.width, size.height, pad)

        if (points.isEmpty()) {
            // A baseline, not nothing: an empty box reads as "failed to load"
            // and a patient retries instead of going to measure.
            drawLine(
                color = palette.hairline,
                start = Offset(pad, size.height / 2f),
                end = Offset(size.width - pad, size.height / 2f),
                strokeWidth = SPARK_STROKE.toPx() / 2f,
                cap = StrokeCap.Round,
            )
            return@Canvas
        }

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
                        lineTo(points.last().x, size.height - pad)
                        lineTo(pad, size.height - pad)
                        close()
                    }
                drawPath(area, fill, alpha = SPARK_FILL_ALPHA)
            }

            drawPath(
                line,
                color,
                // Round joins as well as caps: a miter spikes on a sharp
                // reversal at this width (same reason as CadenceIcon).
                style =
                    Stroke(
                        width = SPARK_STROKE.toPx(),
                        cap = StrokeCap.Round,
                        join = StrokeJoin.Round,
                    ),
            )
        }

        drawCircle(color, radius = SPARK_DOT_RADIUS.toPx(), center = points.last())
    }
}

/**
 * Where a sparkline's points land inside a box of [canvasWidth] × [canvasHeight].
 * Pulled out of the drawing because it is the only part with arithmetic that
 * divides, and a NaN coordinate silently paints nothing rather than crashing —
 * invisible to a test that only measures the box.
 *
 * Degenerate cases resolve to the centre, not an edge — a deliberate divergence
 * from the prototype, which pins a flat series to the top (its `|| 1` is
 * faithful to a path it never walks, since its data is hardcoded ascending); see
 * docs/prototype-divergences.md. Pinned-to-top with a fill, a flat series would
 * read as "at maximum". Non-finite values are dropped rather than propagated,
 * since one NaN would make every coordinate NaN and vanish the whole chart.
 */
internal fun sparkPoints(
    data: List<Float>,
    canvasWidth: Float,
    canvasHeight: Float,
    pad: Float,
): List<Offset> {
    val values = data.filter { it.isFinite() }
    if (values.isEmpty()) return emptyList()

    // Clamped: a box smaller than its own padding has no inside, and clamping
    // keeps points within bounds rather than mirroring them negative.
    val innerWidth = (canvasWidth - pad * 2).coerceAtLeast(0f)
    val innerHeight = (canvasHeight - pad * 2).coerceAtLeast(0f)

    val max = values.max()
    val min = values.min()
    val flat = max == min

    // One value has no trend or span to draw against, so it goes in the
    // middle rather than reading as an old high reading in the corner.
    if (values.size == 1) {
        return listOf(Offset(canvasWidth / 2f, canvasHeight / 2f))
    }

    val step = innerWidth / (values.size - 1)

    return values.mapIndexed { index, value ->
        Offset(
            x = pad + index * step,
            y = if (flat) canvasHeight / 2f else pad + (max - value) / (max - min) * innerHeight,
        )
    }
}
