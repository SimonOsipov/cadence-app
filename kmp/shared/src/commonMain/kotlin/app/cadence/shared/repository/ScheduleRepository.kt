package app.cadence.shared.repository

import app.cadence.shared.domain.Occurrence
import kotlinx.datetime.LocalDate

/**
 * One cell of the 12-week grid.
 *
 * The dots, not the events: the calendar draws a month at a glance and opens a
 * day for the detail. Both come from the same generator, which is §03's
 * seventh correction — the prototype's Today strip and Schedule screen each
 * carry their own copy and disagree.
 */
data class ScheduleDay(
    val date: LocalDate,
    val cycleWeek: Int?,
    val hasInjection: Boolean,
    val anyPending: Boolean,
    val allDone: Boolean,
)

/** §11: `GET /me/schedule?month`. */
interface ScheduleRepository {
    /** Every day of the month containing [anyDateInMonth], dots included. */
    suspend fun month(anyDateInMonth: LocalDate): List<ScheduleDay>

    /** The occurrences of one day, for the sheet that opens on tap. */
    suspend fun day(date: LocalDate): List<Occurrence>
}
