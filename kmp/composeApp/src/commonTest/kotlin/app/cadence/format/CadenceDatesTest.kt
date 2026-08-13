package app.cadence.format

import app.cadence.shared.domain.PartOfDay
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlin.test.Test
import kotlin.test.assertEquals

class CadenceDatesTest {
    @Test
    fun everyWeekdayHasARussianName() {
        // getValue throws on a miss, so a short map crashes on whichever day it forgot.
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
        // No cycle week between protocols — «Воскресенье, утро · null-я неделя» ships when nobody asks.
        assertEquals("Воскресенье, утро", greeting(LocalDate(2026, 5, 31), PartOfDay.MORNING, cycleWeek = null))
    }

    @Test
    fun theHeadingsAreMondayFirstInOrder() {
        // As a sequence: the screen test only checked all seven labels exist, so a wrong-for-RU
        // Sunday-first calendar would still have shipped green.
        assertEquals(listOf("Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"), weekdayHeadings())
    }

    @Test
    fun theBlanksBeforeTheFirstOfTheMonthMatchItsWeekday() {
        // Zero on Monday, six on Sunday — what an off-by-one-column grid gets wrong.
        // `leadingBlanks` had no direct test before this.
        assertEquals(0, leadingBlanks(LocalDate(2026, 6, 1)), "1 June 2026 is a Monday")
        assertEquals(4, leadingBlanks(LocalDate(2026, 5, 1)), "1 May 2026 is a Friday")
        assertEquals(6, leadingBlanks(LocalDate(2026, 11, 1)), "1 November 2026 is a Sunday")
    }

    @Test
    fun clockTimeZeroPadsBothHalves() {
        // The prototype's «08:42» pads both halves; nothing else here proves this pads rather
        // than being ported as a literal.
        assertEquals("08:05", clockTime(LocalDateTime(2026, 5, 31, 8, 5)))
        assertEquals("23:59", clockTime(LocalDateTime(2026, 5, 31, 23, 59)))
    }

    @Test
    fun allTwelveMonthsHaveTheirGenitiveForm() {
        // A table test: «мая»/«июня» were previously covered only by accident through the
        // schedule screen, where a transposed pair would surface only if the calendar reached it.
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
