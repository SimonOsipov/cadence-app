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

// Ported from `Spark` in mobile/src/components/shared.tsx. The prototype draws
// it in SVG, which Compose has no equivalent for, so the geometry moves to
// drawing commands.
//
// These are Dp, not raw floats. The prototype's numbers live in a viewBox whose
// units equal its dp size, so they are density-independent there; a DrawScope
// measures in physical pixels, and passing them straight through would draw the
// line a third as thick on a 3x screen — and differently on each platform.

private val SPARK_PAD = 2.dp
private val SPARK_STROKE = 2.dp
private val SPARK_DOT_RADIUS = 3.dp
private const val SPARK_FILL_ALPHA = 0.4f

/**
 * An inline trend sparkline: a polyline with a dot on the latest point, and an
 * optional area beneath it.
 *
 * Every call site passes a width computed from the layout it sits in, which is
 * why the size is parameters rather than something the caller wraps.
 *
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
            // A baseline rather than nothing. An empty box next to a caption and
            // an axis reads as "failed to load", and a patient waits and retries
            // instead of going and measuring. A rule says "there is a scale here
            // and no values on it".
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
                // Round joins as well as caps: a sparkline is all corners, and
                // a miter spikes on a sharp reversal at this width. CadenceIcon
                // sets both for the same reason.
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
 *
 * Pulled out of the drawing because it is the only part with arithmetic, and
 * arithmetic that divides needs to be checkable without a Canvas: a NaN
 * coordinate does not crash a draw, it silently paints nothing, so a test that
 * only measured the box would pass over a broken chart.
 *
 * Everything degenerate resolves to the centre rather than to an edge. That is
 * a deliberate divergence from the prototype, which pins a flat series to the
 * top — recorded in docs/prototype-divergences.md. The prototype never renders
 * one, because all of its data is hardcoded ascending, so its `|| 1` is
 * faithful to a path it never walks. Pinned to the top with a fill, a flat
 * series reads as "at maximum", and «110, 110, 110» draws identically to
 * «0,5 · 0,5 · 0,5».
 *
 * Non-finite values are dropped rather than propagated: one NaN would make
 * every coordinate NaN and the whole chart vanish, and a measurement pipeline
 * that divides can produce one.
 */
internal fun sparkPoints(
    data: List<Float>,
    canvasWidth: Float,
    canvasHeight: Float,
    pad: Float,
): List<Offset> {
    val values = data.filter { it.isFinite() }
    if (values.isEmpty()) return emptyList()

    // A box smaller than its own padding has no inside; clamping keeps every
    // point within the bounds this function promises rather than mirroring them.
    val innerWidth = (canvasWidth - pad * 2).coerceAtLeast(0f)
    val innerHeight = (canvasHeight - pad * 2).coerceAtLeast(0f)

    val max = values.max()
    val min = values.min()
    val flat = max == min

    // One value has no trend to draw and no span to draw it against; it belongs
    // in the middle, where it reads as "one measurement" rather than as an old
    // high reading in the corner. The trailing dot marks "now" everywhere else.
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
