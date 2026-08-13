package app.cadence.shared.domain

import kotlinx.datetime.LocalTime
import kotlin.test.Test
import kotlin.test.assertEquals

class PartOfDayTest {
    @Test
    fun eachBoundaryFallsOnTheSideRussianPutsIt() {
        // Both sides of each boundary: that's the only place an off-by-one can hide.
        assertEquals(PartOfDay.NIGHT, partOfDay(LocalTime(0, 0)))
        assertEquals(PartOfDay.NIGHT, partOfDay(LocalTime(4, 59)))
        assertEquals(PartOfDay.MORNING, partOfDay(LocalTime(5, 0)))
        assertEquals(PartOfDay.MORNING, partOfDay(LocalTime(11, 59)))
        assertEquals(PartOfDay.AFTERNOON, partOfDay(LocalTime(12, 0)))
        assertEquals(PartOfDay.AFTERNOON, partOfDay(LocalTime(17, 59)))
        assertEquals(PartOfDay.EVENING, partOfDay(LocalTime(18, 0)))
        assertEquals(PartOfDay.EVENING, partOfDay(LocalTime(23, 59)))
    }

    @Test
    fun theDayIsDividedAndNotJustLabelled() {
        // An earlier version asserted 24 hours yield 24 results, true of any function — a
        // stub returning NIGHT for every hour passed it.
        val hours = (0..23).map { partOfDay(LocalTime(it, 0)) }

        assertEquals(5, hours.count { it == PartOfDay.NIGHT }, "00:00–04:59")
        assertEquals(7, hours.count { it == PartOfDay.MORNING }, "05:00–11:59")
        assertEquals(6, hours.count { it == PartOfDay.AFTERNOON }, "12:00–17:59")
        assertEquals(6, hours.count { it == PartOfDay.EVENING }, "18:00–23:59")
    }
}
