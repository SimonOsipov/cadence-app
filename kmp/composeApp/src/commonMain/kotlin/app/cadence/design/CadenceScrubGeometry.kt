package app.cadence.design

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.MetricAccent
import app.cadence.shared.domain.TrendSeries
import app.cadence.shared.domain.middleOf
import kotlin.math.abs

/*
 * The scrub chart's arithmetic, kept out of the drawing.
 *
 * `sparkPoints` set the precedent and gave the reason: a NaN coordinate paints
 * nothing rather than crashing, so a test that only measured the box would pass
 * over a chart that had silently vanished. Everything here is a function of its
 * inputs and is asserted against literals; the file beside it holds only the
 * parts that need a canvas or a layout.
 */

/** The plot's borders. Shared with the drawing, which insets its canvas by them. */
internal val CHART_INSET_X = 8.dp
internal val CHART_INSET_TOP = 20.dp
internal val CHART_INSET_BOTTOM = 16.dp

/**
 * How much room is left above and below the readings.
 *
 * The prototype's twelve percent. Without it a peak touches the top border and
 * reads as clipped — as though the chart had more to show and ran out of room.
 */
private const val HEADROOM = 0.12f

/**
 * Every reading of a series, placed on the axis its window describes.
 *
 * The bands and the marks below are laid out by date; if the readings were laid
 * out by ordinal position instead, the two would agree only where a series has
 * exactly one reading per day. Weight is weekly and the tape is measured on
 * Sundays, so on «4 недели» four readings spread evenly across the plot while
 * the titration hairline stands at its true date — and the chart would put a
 * dose change under the wrong dot. §11 asks for «protocol event overlays» on
 * the same axis, and this is what makes it one axis.
 */
internal fun chartReadings(series: TrendSeries): List<ChartReading> =
    series.points
        .mapIndexed { index, point ->
            ChartReading(index, series.range.middleOf(series.dayOf(point)), point.value.toFloat())
        }.filter { it.value.isFinite() }

/**
 * One reading: which of the series' points it is, where its day falls across
 * the window, and what it read.
 *
 * [index] is kept because the non-finite ones are dropped here. A measurement
 * pipeline that divides can produce a NaN, and one of them makes every
 * coordinate NaN — a chart that vanishes without an error. Dropping them
 * renumbers what is left, so the position on the canvas and the position in
 * `series.points` stop being the same number, and a scrub would report the
 * reading beside the one the cursor stands on.
 */
internal data class ChartReading(
    val index: Int,
    val fraction: Float,
    val value: Float,
)

/**
 * Where a `0f..1f` position across the window lands on the canvas.
 *
 * The same mapping `scrubPoints` uses for its readings, named so that the marks
 * cannot drift from the line they stand against: a mark placed against the full
 * width while the readings are inset would lean a little further right on every
 * chart, and no assertion about `middleOf` would notice.
 */
internal fun plotX(
    fraction: Float,
    canvasWidth: Float,
    inset: ChartInset,
): Float = inset.left + fraction * (canvasWidth - inset.left - inset.right).coerceAtLeast(0f)

/**
 * The reading nearest [x], or null when there are none.
 *
 * Nearest rather than «the one to the left»: a finger between two points is
 * asking about whichever it is closer to, and rounding one way makes the last
 * reading unreachable on the right edge.
 */
internal fun nearestIndex(
    x: Float,
    points: List<Offset>,
): Int? =
    points
        .withIndex()
        .minByOrNull { abs(it.value.x - x) }
        ?.index

/**
 * The line's colour for a metric's family.
 *
 * A function rather than a `when` inside the composable: a colour reaches the
 * canvas and a border, and neither lands in semantics, so an inverted branch is
 * invisible to a UI test. Named, it is two inputs and two answers.
 */
internal fun accentStroke(accent: MetricAccent): Color =
    when (accent) {
        MetricAccent.FOREST -> CadenceColors.forest700
        MetricAccent.SAND -> CadenceColors.sand700
    }

/** The one place the plot's borders turn into pixels, so the cursor and the gesture agree. */
internal fun androidx.compose.ui.unit.Density.chartInset(): ChartInset =
    ChartInset(
        left = CHART_INSET_X.toPx(),
        right = CHART_INSET_X.toPx(),
        top = CHART_INSET_TOP.toPx(),
        bottom = CHART_INSET_BOTTOM.toPx(),
    )

/**
 * The plot's borders, in pixels.
 *
 * Asymmetric: the top leaves room for a protocol mark to stand above the line
 * it annotates, and the bottom for the day labels the prototype draws under the
 * axis. Those labels are not ported yet — step-9 collects what the three screens
 * do not draw — so today the bottom inset is headroom the chart does not spend.
 */
internal data class ChartInset(
    val left: Float,
    val right: Float,
    val top: Float,
    val bottom: Float,
)

/**
 * Where the readings land inside a box of [canvasWidth] × [canvasHeight].
 *
 * Pulled out of the drawing for `sparkPoints`' reason: this is the only part
 * with arithmetic, a NaN coordinate paints nothing rather than crashing, and a
 * test that only measured the box would pass over a chart that had silently
 * vanished.
 *
 * A flat series sits in the middle rather than at an edge, and one reading sits
 * in the middle of both axes — the same answers `sparkPoints` gives, for the
 * same reason: pinned to the top with a fill, «98,4 · 98,4 · 98,4» draws
 * identically to a series that climbed.
 */
internal fun scrubPoints(
    readings: List<ChartReading>,
    canvasWidth: Float,
    canvasHeight: Float,
    inset: ChartInset,
): List<Offset> {
    val drawable = readings
    val innerHeight = (canvasHeight - inset.top - inset.bottom).coerceAtLeast(0f)
    val middleY = inset.top + innerHeight / 2f

    return when {
        drawable.isEmpty() -> {
            emptyList()
        }

        drawable.size == 1 -> {
            listOf(Offset(plotX(drawable.single().fraction, canvasWidth, inset), middleY))
        }

        else -> {
            val max = drawable.maxOf { it.value }
            val min = drawable.minOf { it.value }
            val span = max - min
            // Headroom at both ends, so neither a peak nor a trough touches a
            // border and reads as clipped.
            val top = max + span * HEADROOM
            val bottom = min - span * HEADROOM

            drawable.map { reading ->
                Offset(
                    x = plotX(reading.fraction, canvasWidth, inset),
                    y =
                        if (span == 0f) {
                            middleY
                        } else {
                            inset.top + (top - reading.value) / (top - bottom) * innerHeight
                        },
                )
            }
        }
    }
}
