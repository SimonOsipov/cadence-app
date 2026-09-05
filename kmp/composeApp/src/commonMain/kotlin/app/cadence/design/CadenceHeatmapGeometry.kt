package app.cadence.design

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import app.cadence.format.leadingBlanks
import app.cadence.shared.domain.MoodLevel
import app.cadence.shared.domain.TrendRange
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.plus

/*
 * The heatmap reduced to arithmetic, on `CadenceScrubGeometry`'s precedent:
 * everything here is a function of its inputs, asserted against literals, and
 * `CadenceHeatmap.kt` beside it holds only what needs a layout.
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
