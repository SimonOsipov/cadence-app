package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import app.cadence.format.formatDose
import app.cadence.shared.domain.DoseBand
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.MetricAccent
import app.cadence.shared.domain.ProtocolMark
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.TrendSeries
import app.cadence.shared.domain.middleOf

const val CADENCE_SCRUB_CHART_TAG = "cadence-scrub-chart"

/**
 * The scrub cursor is a laid-out node, not a drawing: a dot painted inside a
 * `Canvas` asserts nothing about where it went, the same defect the gauge fill
 * and the syringe both shipped. As a node, «the cursor stands on the reading
 * you touched» becomes a bounds comparison.
 */
const val CADENCE_SCRUB_CURSOR_TAG = "cadence-scrub-cursor"

const val CADENCE_SCRUB_DOSE_ROW_TAG = "cadence-scrub-dose-row"

/** One band per prescribed span, tagged by position so each can be measured. */
fun cadenceScrubBandTag(index: Int): String = "cadence-scrub-band-$index"

/**
 * One node per protocol mark, for the reason the cursor is one. The dashed
 * line itself is painted (a canvas is the only way to dash), but a node stands
 * at the same x, so «the mark is on its day» is a bounds comparison.
 */
fun cadenceScrubMarkTag(index: Int): String = "cadence-scrub-mark-$index"

private val CHART_STROKE = 2.25.dp
private val CURSOR_SIZE = 10.dp
private val DOSE_ROW_HEIGHT = 20.dp
private val DOSE_ROW_GAP = 8.dp
private val BAND_INSET = 2.dp

/** A hairline, not zero: a node with no width lays out with no bounds to read. */
private val MARK_NODE_WIDTH = 1.dp

private const val FILL_ALPHA = 0.18f
private const val MARK_ALPHA = 0.45f

private val HAIRLINE_DASH = floatArrayOf(1f, 4f)
private val MARK_DASH = floatArrayOf(2f, 3f)

/**
 * One metric's window, drawn: the readings, the doses prescribed under them,
 * and the days the protocol changed.
 *
 * [scrubIndex] is hoisted, unlike the prototype's chart-owned state (reset by
 * an effect on data change — the bug that lets scrub survive a metric switch
 * pointing at the other series). Null here means «the latest», so a new series
 * is current by construction, not by remembering to reset.
 *
 * [height] is the plot's requested height; a parent that allows less wins and
 * geometry follows the laid-out box. The dose row adds its own height beneath.
 *
 * [onScrub] reports the index *and* the reading — a caller with only the index
 * would index back into the list this component just walked, and the two
 * would disagree the first time an empty window returned early.
 */
@Composable
fun CadenceScrubChart(
    series: TrendSeries,
    modifier: Modifier = Modifier,
    bands: List<DoseBand> = emptyList(),
    marks: List<ProtocolMark> = emptyList(),
    accent: MetricAccent = MetricAccent.FOREST,
    scrubIndex: Int? = null,
    onScrub: (Int, Measurement) -> Unit = { _, _ -> },
    height: Dp = 200.dp,
) {
    val palette = Cadence.palette
    val stroke = accentStroke(accent)

    Column(modifier) {
        BoxWithConstraints(
            Modifier
                .fillMaxWidth()
                .height(height)
                .testTag(CADENCE_SCRUB_CHART_TAG)
                .scrubbable(series) { index -> onScrub(index, series.points[index]) },
        ) {
            val density = LocalDensity.current
            val widthPx = with(density) { maxWidth.toPx() }
            // `maxHeight`, not the parameter: `Modifier.height` is clamped by
            // the parent, and `ScrubPlot` draws against the laid-out
            // `size.height` — reading from two places would put the cursor
            // below the line in a squeezed card.
            val heightPx = with(density) { maxHeight.toPx() }
            val inset = with(density) { chartInset() }
            // Remembered: this recomposes on every pointer event during a
            // drag, and «3 месяца» of a daily metric is 84 conversions a frame.
            val readings = remember(series) { chartReadings(series) }
            val points =
                remember(readings, widthPx, heightPx, inset) { scrubPoints(readings, widthPx, heightPx, inset) }
            // Clamped, not dropped: the window is hoisted and shared with the
            // detail screen, so an index scrubbed on «весь цикл» outlives a
            // switch to «7 дней» (prototype clamps for the same reason) — a
            // stale index with no cursor at all would be the worse failure.
            val cursorAt =
                scrubIndex
                    ?.let { wanted -> readings.indexOfLast { it.index <= wanted }.coerceAtLeast(0) }
                    ?: points.lastIndex
            val cursor = points.getOrNull(cursorAt)

            ScrubPlot(points, marks, series.range, stroke, palette.hairline, inset)

            marks.forEachIndexed { index, mark ->
                Box(
                    Modifier
                        .offset(
                            x =
                                with(density) {
                                    plotX(series.range.middleOf(mark.date), widthPx, inset).toDp()
                                } - MARK_NODE_WIDTH / 2,
                        ).width(MARK_NODE_WIDTH)
                        .height(height)
                        .testTag(cadenceScrubMarkTag(index)),
                )
            }

            if (cursor != null) {
                // Centred on the point: the offset is the top-left corner, so
                // half the cursor comes back off both axes.
                Box(
                    Modifier
                        .offset(
                            x = with(density) { cursor.x.toDp() } - CURSOR_SIZE / 2,
                            y = with(density) { cursor.y.toDp() } - CURSOR_SIZE / 2,
                        ).width(CURSOR_SIZE)
                        .height(CURSOR_SIZE)
                        .testTag(CADENCE_SCRUB_CURSOR_TAG)
                        .background(palette.bg, RoundedCornerShape(CURSOR_SIZE))
                        .border(2.dp, stroke, RoundedCornerShape(CURSOR_SIZE)),
                )
            }
        }

        if (bands.isNotEmpty()) {
            DoseRow(bands, series.range, palette)
        }
    }
}

/** Where the readings land, and the lines drawn through and behind them. */
@Composable
private fun ScrubPlot(
    points: List<Offset>,
    marks: List<ProtocolMark>,
    range: TrendRange,
    stroke: Color,
    hairline: Color,
    inset: ChartInset,
) {
    Canvas(Modifier.fillMaxSize()) {
        val plotTop = inset.top
        val plotBottom = size.height - inset.bottom

        drawHairlines(hairline, inset, plotTop, plotBottom)
        drawProtocolMarks(marks, range, inset, plotTop, plotBottom)

        if (points.isEmpty()) {
            // Same answer `CadenceSpark` gives an empty series: a baseline, not
            // an empty box that reads as «failed to load».
            drawLine(
                color = hairline,
                start = Offset(inset.left, (plotTop + plotBottom) / 2f),
                end = Offset(size.width - inset.right, (plotTop + plotBottom) / 2f),
                strokeWidth = 1f,
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
            val area =
                Path().apply {
                    addPath(line)
                    lineTo(points.last().x, plotBottom)
                    lineTo(points.first().x, plotBottom)
                    close()
                }
            drawPath(area, stroke, alpha = FILL_ALPHA)
            drawPath(
                line,
                stroke,
                style =
                    Stroke(
                        width = CHART_STROKE.toPx(),
                        cap = StrokeCap.Round,
                        join = StrokeJoin.Round,
                    ),
            )
        }
    }
}

/** The prescribed doses, laid out under the axis they belong to. */
@Composable
private fun DoseRow(
    bands: List<DoseBand>,
    range: TrendRange,
    palette: CadencePalette,
) {
    // Inset like the plot above it: laid out against the full width, a band
    // edge would drift from its opening dashed mark by up to the inset,
    // growing across the chart, and the strip would claim a dose change
    // slightly before the hairline did.
    BoxWithConstraints(
        Modifier
            .fillMaxWidth()
            .padding(top = DOSE_ROW_GAP, start = CHART_INSET_X, end = CHART_INSET_X)
            .height(DOSE_ROW_HEIGHT)
            .testTag(CADENCE_SCRUB_DOSE_ROW_TAG),
    ) {
        val full = maxWidth
        bands.forEachIndexed { index, band ->
            val start = range.fractionOf(band.range.from).start
            val end = range.fractionOf(band.range.through).endInclusive
            val (value, unit) = formatDose(band.dose)

            Box(
                Modifier
                    .offset(x = full * start + BAND_INSET)
                    .width((full * (end - start) - BAND_INSET * 2).coerceAtLeast(0.dp))
                    .height(DOSE_ROW_HEIGHT)
                    .testTag(cadenceScrubBandTag(index))
                    .background(palette.sunk, RoundedCornerShape(6.dp))
                    .border(1.dp, palette.hairline, RoundedCornerShape(6.dp)),
                contentAlignment = Alignment.Center,
            ) {
                CadenceMeta(text = "$value $unit", color = palette.muted)
            }
        }
    }
}

/** Three rules, so the eye has a scale to read height against. */
private fun DrawScope.drawHairlines(
    hairline: Color,
    inset: ChartInset,
    plotTop: Float,
    plotBottom: Float,
) {
    listOf(0f, 0.5f, 1f).forEach { fraction ->
        val y = plotTop + (plotBottom - plotTop) * fraction
        drawLine(
            color = hairline,
            start = Offset(inset.left, y),
            end = Offset(size.width - inset.right, y),
            strokeWidth = 1f,
            pathEffect = PathEffect.dashPathEffect(HAIRLINE_DASH),
        )
    }
}

/** A dashed vertical on each day the protocol changed, drawn under the line. */
private fun DrawScope.drawProtocolMarks(
    marks: List<ProtocolMark>,
    range: TrendRange,
    inset: ChartInset,
    plotTop: Float,
    plotBottom: Float,
) {
    marks.forEach { mark ->
        val x = plotX(range.middleOf(mark.date), size.width, inset)
        drawLine(
            color = CadenceColors.sand700.copy(alpha = MARK_ALPHA),
            start = Offset(x, plotTop - inset.top / 2f),
            end = Offset(x, plotBottom),
            strokeWidth = 1f,
            pathEffect = PathEffect.dashPathEffect(MARK_DASH),
        )
    }
}

/**
 * Touch and drag both report; the chart is scrubbed, not clicked. Written as a
 * modifier so the geometry above stays a function of its inputs — the gesture
 * needs the laid-out width, which only the pointer scope knows.
 */
private fun Modifier.scrubbable(
    series: TrendSeries,
    onIndex: (Int) -> Unit,
): Modifier =
    this.pointerInput(series) {
        if (series.points.isEmpty()) return@pointerInput
        val readings = chartReadings(series)

        // One handler for both, not `detectTapGestures` then
        // `detectDragGestures`: the first suspends until cancellation, so the
        // second would never run, and a chart with taps only can't be dragged.
        awaitEachGesture {
            // Laid out per gesture, not once per series: `pointerInput` isn't
            // restarted by a remeasure, so a rotation would leave the touch
            // mapping on the old width. `size` is live on this scope.
            val points = scrubPoints(readings, size.width.toFloat(), size.height.toFloat(), chartInset())

            val down = awaitFirstDown(requireUnconsumed = false)
            nearestIndex(down.position.x, points)?.let { onIndex(readings[it].index) }

            var pressed = true
            while (pressed) {
                val event = awaitPointerEvent()
                // One finger, not every pressed pointer: a second thumb would
                // otherwise fire twice per event, the later report winning.
                event.changes.firstOrNull { it.pressed }?.let { change ->
                    nearestIndex(change.position.x, points)?.let { onIndex(readings[it].index) }
                }
                pressed = event.changes.any { it.pressed }
            }
        }
    }
