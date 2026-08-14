package app.cadence.design

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import app.cadence.shared.domain.MoodLevel
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.middleOf
import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil

/*
 * The mood chart's arithmetic, split from the heatmap's for the same reason the two
 * are separate primitives — and because one file holding both crossed detekt's
 * function ceiling, which is the ceiling asking the same question.
 */

/** One day's entry. [dosed] is drawn larger and solid; a hand-written one is hollow. */
data class MoodReading(
    val date: LocalDate,
    val level: MoodLevel,
    val dosed: Boolean,
)

/**
 * Where the readings land. Placed by date through [middleOf], the same mapping
 * the scrub chart uses: placed by position in the list instead, a week with one
 * entry and a week with five would draw the same width, and the titration
 * hairline would stand under the wrong day.
 */
internal fun moodPoints(
    readings: List<MoodReading>,
    course: TrendRange,
    canvasWidth: Float,
    canvasHeight: Float,
    inset: ChartInset,
): List<Offset> =
    readings.map { reading ->
        Offset(
            x = plotX(course.middleOf(reading.date), canvasWidth, inset),
            y = moodY(reading.level, canvasHeight, inset),
        )
    }

/**
 * How a reading's dot is drawn (`JournalScreen.tsx:91-102`). A dose's dot is filled
 * sand and larger; a hand-written one is hollow on the paper. Both keep the same
 * outline, so the difference a reader has to catch is fill and size, not colour —
 * the mood's own colour stays in the heatmap, where a cell has nothing else to say it.
 */
internal fun moodDot(
    reading: MoodReading,
    palette: CadencePalette,
): MoodDot =
    if (reading.dosed) {
        MoodDot(DOSED_DOT_RADIUS, CadenceColors.sand500, CadenceColors.forest700)
    } else {
        MoodDot(DOT_RADIUS, palette.paper, CadenceColors.forest700)
    }

internal data class MoodDot(
    val radius: Dp,
    val fill: Color,
    val stroke: Color,
)

private val DOT_RADIUS = 3.2.dp
private val DOSED_DOT_RADIUS = 4.5.dp

/** Which hairline: a dose change, or the day the patient is standing on. */
internal enum class MoodMarkKind {
    TITRATION,
    TODAY,
}

internal data class MoodMark(
    val date: LocalDate,
    val kind: MoodMarkKind,
)

/**
 * Which day gets which hairline, decided here rather than at the two call sites that
 * would otherwise each pass a kind alongside a list. Passed as an argument, asking for
 * the titration style while drawing today is a one-word slip that paints correctly-
 * shaped output and survives every assertion — measured: that mutation was the one
 * survivor of the round that extracted [moodMarkStyle].
 */
internal fun moodMarks(
    titrations: List<LocalDate>,
    today: LocalDate,
): List<MoodMark> = titrations.map { MoodMark(it, MoodMarkKind.TITRATION) } + MoodMark(today, MoodMarkKind.TODAY)

/**
 * The two hairlines drawn differently on purpose (`JournalScreen.tsx:61-81`): the
 * titration carries sand and a cap, today is a fainter forest with a tighter dash and
 * none. Named rather than left inside the canvas for [heatmapFill]'s reason — drawn
 * alike, three hairlines on a seeded course read as three dose changes, and no
 * assertion about layout would notice.
 */
internal fun moodMarkStyle(kind: MoodMarkKind): MoodMarkStyle =
    when (kind) {
        MoodMarkKind.TITRATION -> MoodMarkStyle(CadenceColors.sand500, TITRATION_DASH_ON, MARK_CAP_RADIUS)
        MoodMarkKind.TODAY -> MoodMarkStyle(CadenceColors.forest700.copy(alpha = TODAY_ALPHA), TODAY_DASH_ON, null)
    }

internal data class MoodMarkStyle(
    val color: Color,
    val dashOn: Dp,
    val cap: Dp?,
)

private val TITRATION_DASH_ON = 3.dp
private val TODAY_DASH_ON = 2.dp
private val MARK_CAP_RADIUS = 3.dp

/** Today is the fainter of the two — the prototype's own 0.6. */
private const val TODAY_ALPHA = 0.6f

/**
 * The five level lines, brightest first — that is, top to bottom on a canvas.
 * The outer two carry readings, so they are the plot's own borders rather than
 * decoration inside it.
 */
internal fun moodGridLines(
    canvasHeight: Float,
    inset: ChartInset,
): List<Float> = MoodLevel.entries.sortedByDescending { it.value }.map { moodY(it, canvasHeight, inset) }

private fun moodY(
    level: MoodLevel,
    canvasHeight: Float,
    inset: ChartInset,
): Float {
    val innerHeight = (canvasHeight - inset.top - inset.bottom).coerceAtLeast(0f)
    val steps = (MoodLevel.entries.size - 1).toFloat()
    val fromTheTop = (MoodLevel.entries.size - level.value) / steps
    return inset.top + fromTheTop * innerHeight
}

/**
 * How much of the axis is still ahead, `0f..1f`. The journal's chart draws the
 * whole course rather than stopping at today, so without the shading a course
 * in its second week looks like one that ended with nothing recorded.
 */
internal fun futureFraction(
    course: TrendRange,
    today: LocalDate,
): Float {
    val remaining = today.daysUntil(course.through)
    return (remaining.toFloat() / course.days).coerceIn(0f, 1f)
}
