package app.cadence.shared.domain

import kotlinx.datetime.LocalTime
import kotlin.test.Test
import kotlin.test.assertEquals

class PartOfDayTest {
    @Test
    fun eachBoundaryFallsOnTheSideRussianPutsIt() {
        // The prototype freezes «утро» into the greeting; this is the rule it
        // stands for. Boundaries are asserted on both sides, because that is
        // the only place an off-by-one can hide.
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
    fun everyHourOfTheDayHasAnAnswer() {
        // A `when` over hours with a gap would throw on the hour it forgot,
        // and the greeting is the first thing on the screen.
        assertEquals(24, (0..23).map { partOfDay(LocalTime(it, 0)) }.size)
    }
}
