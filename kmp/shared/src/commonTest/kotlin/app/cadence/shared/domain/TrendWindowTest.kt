package app.cadence.shared.domain

import app.cadence.shared.domain.TrendFixture.PLAN
import app.cadence.shared.domain.TrendFixture.START
import app.cadence.shared.domain.TrendFixture.TODAY
import app.cadence.shared.domain.TrendFixture.planStartingOn
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * What span of days each window covers.
 *
 * Kept apart from what a metric looks like inside one: three of these windows
 * are arithmetic on `today`, and the fourth is a fact about the patient's
 * course.
 */
class TrendWindowTest {
    @Test
    fun aWeekIsSevenDaysCountingTodayAsOneOfThem() {
        val range = assertNotNull(TrendWindow.WEEK.rangeOn(PLAN, TODAY))

        // 25 May, not 24: a «7 days» window that ran to today from seven days
        // back would cover eight of them.
        assertEquals(LocalDate(2026, 5, 25), range.from)
        assertEquals(TODAY, range.through)
        assertEquals(7, range.days)
    }

    @Test
    fun fourWeeksAndThreeMonthsAreTheirOwnLengths() {
        val fourWeeks = assertNotNull(TrendWindow.FOUR_WEEKS.rangeOn(PLAN, TODAY))
        assertEquals(28, fourWeeks.days)
        assertEquals(LocalDate(2026, 5, 4), fourWeeks.from)

        val threeMonths = assertNotNull(TrendWindow.THREE_MONTHS.rangeOn(PLAN, TODAY))
        assertEquals(84, threeMonths.days)
        assertEquals(LocalDate(2026, 3, 9), threeMonths.from)
    }

    @Test
    fun theCycleIsAnchoredOnTheProtocolRatherThanCountedBackFromToday() {
        // The same day, two protocols. A cycle window that was secretly a
        // length would answer identically for both.
        val early = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(START), TODAY))
        val late = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(LocalDate(2026, 5, 20)), TODAY))

        assertEquals(START, early.from)
        assertEquals(22, early.days)
        assertEquals(LocalDate(2026, 5, 20), late.from)
        assertEquals(12, late.days)
    }

    @Test
    fun aLengthWindowReachesBackPastTheProtocolItselfAndStillEndsToday() {
        // «3 месяца» asks for 84 days of a course that started 21 days ago. The
        // left edge is deliberately *not* clipped to the start — that is what
        // makes the window visibly partial rather than quietly full — while the
        // right edge never runs past today.
        val range = assertNotNull(TrendWindow.THREE_MONTHS.rangeOn(PLAN, TODAY))

        assertTrue(range.from < PLAN.protocol.startDate, "the window opens before the course did")
        assertEquals(TODAY, range.through)
        assertFalse(LocalDate(2026, 6, 1) in range, "tomorrow is not part of any window")
    }

    @Test
    fun aCycleThatHasEndedStopsOnItsLastPrescribedDayRatherThanGrowingForever() {
        // Twelve weeks from 5 January ran out on 29 March. Left unbounded, «весь
        // цикл» would be a five-month window on a twelve-week course, and every
        // date past the 29th would be one `cycleWeek` calls outside the protocol.
        val range = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(LocalDate(2026, 1, 5)), TODAY))

        assertEquals(LocalDate(2026, 1, 5), range.from)
        assertEquals(LocalDate(2026, 3, 29), range.through)
        assertEquals(84, range.days)
        assertNull(cycleWeek(planStartingOn(LocalDate(2026, 1, 5)).protocol, LocalDate(2026, 3, 30)))
    }

    @Test
    fun aCycleThatHasNotBegunHasNoWindowAtAll() {
        // A doctor may prescribe ahead. Null rather than a range with its ends
        // swapped: `from` and `through` are what the axis and the dose bands
        // read, and an inverted pair would draw both backwards over the eleven
        // days between today and a start date that has not arrived.
        assertNull(TrendWindow.CYCLE.rangeOn(planStartingOn(LocalDate(2026, 6, 10)), TODAY))
    }

    @Test
    fun aCycleIsAtLeastItsFirstDayOnTheDayItBegins() {
        val range = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(TODAY), TODAY))

        assertEquals(1, range.days)
        assertTrue(TODAY in range)
    }

    @Test
    fun aRangeRunsForwardsOrItIsNotARange() {
        // The guard the null above exists to preserve. Without it every future
        // consumer of `from`/`through` inherits the inverted case by default.
        assertFailsWith<IllegalArgumentException> {
            TrendRange(LocalDate(2026, 6, 10), LocalDate(2026, 5, 31))
        }
    }
}
