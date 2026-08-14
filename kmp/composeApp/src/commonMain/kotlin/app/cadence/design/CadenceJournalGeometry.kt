package app.cadence.design

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import app.cadence.format.leadingBlanks
import app.cadence.shared.domain.MoodLevel
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.middleOf
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil
import kotlinx.datetime.plus

/*
 * The journal's two charts reduced to arithmetic, on `CadenceScrubGeometry`'s
 * precedent: everything here is a function of its inputs, asserted against
 * literals, and the files beside it hold only what needs a canvas.
 */

private const val DAYS_IN_WEEK = 7

/**
 * Enough rows to hold the lead-in and every day of the course — computed, not the
 * prototype's twelve (`journal/data.ts:126-135`). Twelve rows hold exactly 84 cells,
 * but the grid opens on the Monday of the starting week, so a course beginning on any
 * other day spends some of them before it starts and runs out early: the prototype's
 * own seed opens on a Sunday and loses days 78–83 off the bottom.
 *
 * Counted rather than fixed at thirteen, because a course is not always twelve weeks
 * (`Protocol.weeks` is data), and a fixed thirteen would truncate a longer one exactly
 * the way twelve truncates this one — while drawing an empty row for a Monday start.
 */
internal fun heatmapRows(
    lead: Int,
    days: Int,
): Int = (lead + days + DAYS_IN_WEEK - 1) / DAYS_IN_WEEK

/** Three states, not two: an unwritten past day and a day still ahead are not the same absence. */
internal enum class DayStanding {
    PAST,
    TODAY,
    FUTURE,
}

/**
 * One square of the heatmap. [week] is the calendar week the row draws, counted
 * from the Monday the grid opens on (`JournalScreen.tsx:160` labels the row with
 * its own index). That is not days-since-the-start divided by seven: on a Sunday
 * start the two disagree by one from the second row down, and the last course day
 * sits in week 13 of a course twelve weeks long — because it does.
 */
internal data class HeatmapCell(
    val date: LocalDate,
    val week: Int,
    val mood: MoodLevel?,
    val standing: DayStanding,
    val titration: Boolean,
)

/**
 * The grid, Monday first, one row per week and `null` where a cell falls
 * outside the course — the lead-in before it opens and the tail after it ends.
 */
internal fun heatmapWeeks(
    course: TrendRange,
    today: LocalDate,
    moodByDate: Map<LocalDate, MoodLevel>,
    titrations: Set<LocalDate>,
): List<List<HeatmapCell?>> {
    val lead = leadingBlanks(course.from)
    val gridStart = course.from.plus(DatePeriod(days = -lead))

    return List(heatmapRows(lead, course.days)) { row ->
        List(DAYS_IN_WEEK) { column ->
            val date = gridStart.plus(DatePeriod(days = row * DAYS_IN_WEEK + column))

            if (date !in course) {
                null
            } else {
                HeatmapCell(
                    date = date,
                    week = row + 1,
                    mood = moodByDate[date],
                    standing =
                        when {
                            date < today -> DayStanding.PAST
                            date == today -> DayStanding.TODAY
                            else -> DayStanding.FUTURE
                        },
                    titration = date in titrations,
                )
            }
        }
    }
}

/**
 * How a cell is filled. Named rather than decided inside the modifier chain, for
 * [accentStroke]'s reason: a colour reaches only a canvas and a background, so an
 * inverted branch is invisible to every assertion about layout — and «past, today and
 * still to come are told apart» is this step's acceptance, not a matter of taste.
 */
internal fun heatmapFill(
    cell: HeatmapCell,
    palette: CadencePalette,
): Color =
    when {
        cell.mood != null -> CadenceMoodTone.of(cell.mood)?.color ?: palette.sunk
        cell.standing == DayStanding.FUTURE -> Color.Transparent
        else -> palette.sunk.copy(alpha = UNWRITTEN_ALPHA)
    }

/** An unwritten past day is drawn faint rather than absent — «I did not write» is itself a reading. */
private const val UNWRITTEN_ALPHA = 0.7f

/** The ring around a cell, or none. Today's is the heavier one: it has to win against a filled neighbour. */
internal fun heatmapBorder(
    standing: DayStanding,
    palette: CadencePalette,
): HeatmapBorder? =
    when (standing) {
        DayStanding.TODAY -> HeatmapBorder(TODAY_RING, CadenceColors.forest700)
        DayStanding.FUTURE -> HeatmapBorder(FUTURE_BORDER, palette.border)
        DayStanding.PAST -> null
    }

internal data class HeatmapBorder(
    val width: Dp,
    val color: Color,
)

private val TODAY_RING = 1.5.dp
private val FUTURE_BORDER = 1.dp

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
