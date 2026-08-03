package app.cadence.format

import app.cadence.shared.domain.PartOfDay
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals

class CadenceDatesTest {
    @Test
    fun everyWeekdayHasARussianName() {
        // getValue throws on a miss, so a short map is a crash on whichever day
        // it forgot — and the greeting is the first line of the app.
        assertEquals(
            7,
            DayOfWeek.entries
                .map { weekdayNominative(it) }
                .toSet()
                .size,
        )
        assertEquals("Воскресенье", weekdayNominative(DayOfWeek.SUNDAY))
        assertEquals("Среда", weekdayNominative(DayOfWeek.WEDNESDAY))
    }

    @Test
    fun theGreetingIsTheProtoypesLineWithItsPartsDerived() {
        assertEquals(
            "Воскресенье, утро · 4-я неделя",
            greeting(LocalDate(2026, 5, 31), PartOfDay.MORNING, cycleWeek = 4),
        )
        assertEquals(
            "Среда, вечер · 12-я неделя",
            greeting(LocalDate(2026, 7, 29), PartOfDay.EVENING, cycleWeek = 12),
        )
    }

    @Test
    fun aDayOutsideTheProtocolDropsTheWeekRatherThanShowingNothing() {
        // Between protocols there is no cycle week, and «Воскресенье, утро ·
        // null-я неделя» is the shape that gets shipped when nobody asks.
        assertEquals("Воскресенье, утро", greeting(LocalDate(2026, 5, 31), PartOfDay.MORNING, cycleWeek = null))
    }
}
