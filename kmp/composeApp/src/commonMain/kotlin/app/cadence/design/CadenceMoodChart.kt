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
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.middleOf
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.plus

const val CADENCE_MOOD_CHART_TAG = "cadence-mood-chart"

/**
 * A node at each titration's x, for [CADENCE_SCRUB_CURSOR_TAG]'s reason: the
 * dashed line has to be painted, but «the mark stands on its day» is only an
 * assertion if something laid out stands there too.
 */
fun cadenceMoodMarkTag(date: LocalDate): String = "cadence-mood-mark-$date"

private val CHART_HEIGHT = 132.dp
private val LINE_STROKE = 2.dp
private val GRID_STROKE = 1.dp
private val DOT_RADIUS = 3.dp
private val DOSED_DOT_RADIUS = 4.5.dp
private val MARK_CAP_RADIUS = 2.5.dp
private val MARK_WIDTH = 1.dp

private val DASH_ON = 4.dp
private val DASH_OFF = 3.dp

/** The four the prototype labels — «нед 1 / нед 4 / нед 8 / нед 12» (`JournalScreen.tsx:104`). */
private val LABELLED_WEEKS = listOf(1, 4, 8, 12)

private const val DAYS_IN_WEEK = 7

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
        val heightPx = with(density) { maxHeight.toPx() }

        Canvas(Modifier.fillMaxSize()) {
            drawFuture(course, today, inset, palette.sunk)
            drawGrid(heightPx, inset, palette.hairline)
            drawMarks(titrations + today, course, inset, palette.border)
            drawMoodLine(readings, course, widthPx, heightPx, inset)
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
        val day = course.from.plusDays((week - 1) * DAYS_IN_WEEK)
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
                    Modifier.offset(
                        x =
                            when (week) {
                                LABELLED_WEEKS.first() -> 0.dp
                                LABELLED_WEEKS.last() -> -LABEL_WIDTH
                                else -> -LABEL_WIDTH / 2
                            },
                    ),
            )
        }
    }
}

/** Wide enough for «нед 12» at the meta size — the anchor rule needs a width before the text is measured. */
private val LABEL_WIDTH = 34.dp

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
    canvasHeight: Float,
    inset: ChartInset,
    color: Color,
) {
    moodGridLines(canvasHeight, inset).forEach { y ->
        drawLine(
            color = color,
            start = Offset(inset.left, y),
            end = Offset(size.width - inset.right, y),
            strokeWidth = GRID_STROKE.toPx(),
        )
    }
}

private fun DrawScope.drawMarks(
    dates: List<LocalDate>,
    course: TrendRange,
    inset: ChartInset,
    color: Color,
) {
    val dash = PathEffect.dashPathEffect(floatArrayOf(DASH_ON.toPx(), DASH_OFF.toPx()))

    dates.filter { it in course }.forEach { date ->
        val x = plotX(course.middleOf(date), size.width, inset)

        drawLine(
            color = color,
            start = Offset(x, inset.top),
            end = Offset(x, size.height - inset.bottom),
            strokeWidth = MARK_WIDTH.toPx(),
            pathEffect = dash,
        )
        // The cap tells a mark from the grid it crosses at a glance.
        drawCircle(color = color, radius = MARK_CAP_RADIUS.toPx(), center = Offset(x, inset.top))
    }
}

private fun DrawScope.drawMoodLine(
    readings: List<MoodReading>,
    course: TrendRange,
    canvasWidth: Float,
    canvasHeight: Float,
    inset: ChartInset,
) {
    val points = moodPoints(readings, course, canvasWidth, canvasHeight, inset)
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
        val tone = CadenceMoodTone.of(reading.level) ?: return@forEach
        val radius = if (reading.dosed) DOSED_DOT_RADIUS else DOT_RADIUS
        drawCircle(color = tone.color, radius = radius.toPx(), center = point)
    }
}

private fun LocalDate.plusDays(days: Int): LocalDate = plus(DatePeriod(days = days))
