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

    @Test
    fun theHeadingsAreMondayFirstInOrder() {
        // Asserted as a sequence. The screen test checked that all seven labels
        // are displayed, in any order — a Sunday-first calendar, wrong for a
        // Russian product, would have shipped green.
        assertEquals(listOf("Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"), weekdayHeadings())
    }

    @Test
    fun theBlanksBeforeTheFirstOfTheMonthMatchItsWeekday() {
        // Zero on a Monday, six on a Sunday, and the two are what a grid that
        // is off by a column gets wrong. `leadingBlanks` had no direct test at
        // all, and the screen's bounds comparison held for three of the seven
        // possible offsets.
        assertEquals(0, leadingBlanks(LocalDate(2026, 6, 1)), "1 June 2026 is a Monday")
        assertEquals(4, leadingBlanks(LocalDate(2026, 5, 1)), "1 May 2026 is a Friday")
        assertEquals(6, leadingBlanks(LocalDate(2026, 11, 1)), "1 November 2026 is a Sunday")
    }

    @Test
    fun allTwelveMonthsHaveTheirGenitiveForm() {
        // The plan asked for a table test and got «мая» and «июня» by accident,
        // through the schedule screen. A transposed pair would surface only
        // when the calendar reached it.
        assertEquals(
            listOf(
                "1 января",
                "1 февраля",
                "1 марта",
                "1 апреля",
                "1 мая",
                "1 июня",
                "1 июля",
                "1 августа",
                "1 сентября",
                "1 октября",
                "1 ноября",
                "1 декабря",
            ),
            (1..12).map { dayAndMonth(LocalDate(2026, it, 1)) },
        )
    }
}
