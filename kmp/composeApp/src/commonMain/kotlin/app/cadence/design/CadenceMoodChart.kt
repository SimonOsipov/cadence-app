package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.middleOf
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.plus
import kotlin.math.roundToInt

const val CADENCE_MOOD_CHART_TAG = "cadence-mood-chart"

/**
 * A node at each titration's x, for [CADENCE_SCRUB_CURSOR_TAG]'s reason: the
 * dashed line has to be painted, but «the mark stands on its day» is only an
 * assertion if something laid out stands there too.
 */
fun cadenceMoodMarkTag(date: LocalDate): String = "cadence-mood-mark-$date"

private val CHART_HEIGHT = 132.dp
private val LINE_STROKE = 2.5.dp
private val GRID_STROKE = 1.dp
private val DOT_STROKE = 2.dp
private val MARK_WIDTH = 1.5.dp

private val DASH_OFF = 3.dp

/** The four the prototype labels — «нед 1 / нед 4 / нед 8 / нед 12» (`JournalScreen.tsx:104`). */
private val LABELLED_WEEKS = listOf(1, 4, 8, 12)

private const val WEEK_LENGTH = 7

/** How faint the days still to come are drawn behind the grid. */
private const val FUTURE_ALPHA = 0.35f

/**
 * Mood across the whole course — the days ahead included, shaded rather than
 * cut off. Stopping the axis at today would draw a course in its second week
 * exactly like one that ran its twelve and recorded nothing.
 */
@Composable
fun CadenceMoodChart(
    readings: List<MoodReading>,
    course: TrendRange,
    today: LocalDate,
    titrations: List<LocalDate>,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val density = LocalDensity.current
    val inset = with(density) { chartInset() }

    BoxWithConstraints(modifier.fillMaxWidth().height(CHART_HEIGHT).testTag(CADENCE_MOOD_CHART_TAG)) {
        val widthPx = with(density) { maxWidth.toPx() }

        Canvas(Modifier.fillMaxSize()) {
            drawFuture(course, today, inset, palette.sunk)
            drawGrid(inset, palette.hairline)
            drawMarks(moodMarks(titrations, today), course, inset)
            drawMoodLine(readings, course, inset, palette)
        }

        titrations.forEach { date ->
            Box(
                Modifier
                    .offset(x = with(density) { plotX(course.middleOf(date), widthPx, inset).toDp() })
                    .testTag(cadenceMoodMarkTag(date)),
            )
        }

        MoodChartWeekLabels(course, widthPx, inset)
    }
}

@Composable
private fun BoxScope.MoodChartWeekLabels(
    course: TrendRange,
    widthPx: Float,
    inset: ChartInset,
) {
    val density = LocalDensity.current
    val palette = Cadence.palette

    LABELLED_WEEKS.forEach { week ->
        val day = course.from.plusDays((week - 1) * WEEK_LENGTH)
        val x = plotX(course.middleOf(day), widthPx, inset)

        Box(
            Modifier
                .align(Alignment.BottomStart)
                .offset(x = with(density) { x.toDp() }),
        ) {
            // The first label is anchored at its left edge and the last at its
            // right, or they hang off the plot; the middle two are centred.
            CadenceMeta(
                text = "нед $week",
                color = palette.subtle,
                modifier =
                    Modifier.anchoredAt(
                        when (week) {
                            LABELLED_WEEKS.first() -> 0f
                            LABELLED_WEEKS.last() -> 1f
                            else -> 0.5f
                        },
                    ),
            )
        }
    }
}

/**
 * Shifts a label left by [fraction] of its own **measured** width, so the first sits
 * on its tick, the last ends on it and the middle two straddle it. Measured rather
 * than assumed: a guessed width that runs short pushes the last label past the plot,
 * and one that runs long pulls it off its own tick with nothing to notice.
 */
private fun Modifier.anchoredAt(fraction: Float): Modifier =
    layout { measurable, constraints ->
        val placeable = measurable.measure(constraints)
        layout(placeable.width, placeable.height) {
            placeable.place(-(placeable.width * fraction).roundToInt(), 0)
        }
    }

private fun DrawScope.drawFuture(
    course: TrendRange,
    today: LocalDate,
    inset: ChartInset,
    color: Color,
) {
    val shaded = futureFraction(course, today)
    if (shaded <= 0f) return

    val left = size.width - (size.width - inset.left - inset.right) * shaded - inset.right
    drawRect(
        color = color.copy(alpha = FUTURE_ALPHA),
        topLeft = Offset(left, inset.top),
        size = Size(size.width - inset.right - left, size.height - inset.top - inset.bottom),
    )
}

private fun DrawScope.drawGrid(
    inset: ChartInset,
    color: Color,
) {
    moodGridLines(size.height, inset).forEach { y ->
        drawLine(
            color = color,
            start = Offset(inset.left, y),
            end = Offset(size.width - inset.right, y),
            strokeWidth = GRID_STROKE.toPx(),
        )
    }
}

private fun DrawScope.drawMarks(
    marks: List<MoodMark>,
    course: TrendRange,
    inset: ChartInset,
) {
    marks.filter { it.date in course }.forEach { mark ->
        val style = moodMarkStyle(mark.kind)
        val dash = PathEffect.dashPathEffect(floatArrayOf(style.dashOn.toPx(), DASH_OFF.toPx()))
        val x = plotX(course.middleOf(mark.date), size.width, inset)

        drawLine(
            color = style.color,
            start = Offset(x, inset.top),
            end = Offset(x, size.height - inset.bottom),
            strokeWidth = MARK_WIDTH.toPx(),
            pathEffect = dash,
        )
        style.cap?.let {
            drawCircle(color = style.color, radius = it.toPx(), center = Offset(x, inset.top))
        }
    }
}

private fun DrawScope.drawMoodLine(
    readings: List<MoodReading>,
    course: TrendRange,
    inset: ChartInset,
    palette: CadencePalette,
) {
    val points = moodPoints(readings, course, size.width, size.height, inset)
    if (points.isEmpty()) return

    if (points.size > 1) {
        val path =
            Path().apply {
                moveTo(points.first().x, points.first().y)
                points.drop(1).forEach { lineTo(it.x, it.y) }
            }
        drawPath(
            path = path,
            color = CadenceColors.forest700,
            style = Stroke(width = LINE_STROKE.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round),
        )
    }

    readings.zip(points).forEach { (reading, point) ->
        val dot = moodDot(reading, palette)

        drawCircle(color = dot.fill, radius = dot.radius.toPx(), center = point)
        drawCircle(color = dot.stroke, radius = dot.radius.toPx(), center = point, style = Stroke(DOT_STROKE.toPx()))
    }
}

private fun LocalDate.plusDays(days: Int): LocalDate = plus(DatePeriod(days = days))
