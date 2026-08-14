package app.cadence.design

import androidx.compose.ui.geometry.Offset
import app.cadence.shared.domain.MoodLevel
import app.cadence.shared.domain.TrendRange
import app.cadence.shared.domain.middleOf
import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil
import kotlinx.datetime.isoDayNumber
import kotlinx.datetime.plus

/*
 * The journal's two charts reduced to arithmetic, on `CadenceScrubGeometry`'s
 * precedent: everything here is a function of its inputs, asserted against
 * literals, and the files beside it hold only what needs a canvas.
 */

private const val DAYS_IN_WEEK = 7

/** The last day of a twelve-week course, counted from zero. */
internal const val COURSE_LAST_DAY = 83

/**
 * Thirteen rows, not the prototype's twelve (`journal/data.ts:126-135`). Twelve
 * rows hold 84 cells, which is exactly the course — but the grid opens on the
 * Monday of the starting week, so a course beginning any day but Monday spends
 * some of them on the lead-in and runs out early. A Sunday start, which the
 * prototype's own seed uses, loses days 78–83 off the bottom. Thirteen is the
 * smallest number that covers the worst lead-in of six.
 */
internal const val HEATMAP_ROWS = 13

/** Three states, not two: an unwritten past day and a day still ahead are not the same absence. */
internal enum class DayStanding {
    PAST,
    TODAY,
    FUTURE,
}

/**
 * One square of the heatmap. [week] is the calendar week the row draws, counted
 * from the Monday the grid opens on (`JournalScreen.tsx:158` labels the row with
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
    val lead = course.from.dayOfWeek.isoDayNumber - 1
    val gridStart = course.from.plus(DatePeriod(days = -lead))

    return List(HEATMAP_ROWS) { row ->
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

/** One day's entry. [dosed] draws larger and filled: a mood beside a dose is the reading the doctor reads for. */
internal data class MoodReading(
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
