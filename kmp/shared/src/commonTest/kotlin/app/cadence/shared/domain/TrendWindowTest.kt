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

/** Floats built by division do not land on the literal they are compared to. */
private const val TOLERANCE = 1e-6f

/**
 * Kept apart from what a metric looks like inside one: three windows are arithmetic on
 * `today`, the fourth a fact about the patient's course.
 */
class TrendWindowTest {
    @Test
    fun aWeekIsSevenDaysCountingTodayAsOneOfThem() {
        val range = assertNotNull(TrendWindow.WEEK.rangeOn(PLAN, TODAY))

        // 25 May, not 24: a "7 days" window running back seven from today covers eight.
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
        // Same day, two protocols: a cycle window secretly a length would answer identically.
        val early = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(START), TODAY))
        val late = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(LocalDate(2026, 5, 20)), TODAY))

        assertEquals(START, early.from)
        assertEquals(22, early.days)
        assertEquals(LocalDate(2026, 5, 20), late.from)
        assertEquals(12, late.days)
    }

    @Test
    fun aLengthWindowReachesBackPastTheProtocolItselfAndStillEndsToday() {
        // Left edge deliberately *not* clipped to the start, so the window reads visibly
        // partial rather than quietly full; right edge never runs past today.
        val range = assertNotNull(TrendWindow.THREE_MONTHS.rangeOn(PLAN, TODAY))

        assertTrue(range.from < PLAN.protocol.startDate, "the window opens before the course did")
        assertEquals(TODAY, range.through)
        assertFalse(LocalDate(2026, 6, 1) in range, "tomorrow is not part of any window")
    }

    @Test
    fun aCycleThatHasEndedStopsOnItsLastPrescribedDayRatherThanGrowingForever() {
        // Left unbounded, «весь цикл» would be a five-month window on a twelve-week course.
        val range = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(LocalDate(2026, 1, 5)), TODAY))

        assertEquals(LocalDate(2026, 1, 5), range.from)
        assertEquals(LocalDate(2026, 3, 29), range.through)
        assertEquals(84, range.days)
        assertNull(cycleWeek(planStartingOn(LocalDate(2026, 1, 5)).protocol, LocalDate(2026, 3, 30)))
    }

    @Test
    fun aCycleThatHasNotBegunHasNoWindowAtAll() {
        // Null, not a range with swapped ends: an inverted pair would draw the axis and dose
        // bands backwards.
        assertNull(TrendWindow.CYCLE.rangeOn(planStartingOn(LocalDate(2026, 6, 10)), TODAY))
    }

    @Test
    fun aCycleIsAtLeastItsFirstDayOnTheDayItBegins() {
        val range = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(TODAY), TODAY))

        assertEquals(1, range.days)
        assertTrue(TODAY in range)
    }

    @Test
    fun aDayIsASpanAcrossTheWindowAndNotAPoint() {
        // Each of seven days owns a seventh: a band covering the window fills it, rather
        // than stopping six-sevenths along as treating a day like a point would.
        val week = assertNotNull(TrendWindow.WEEK.rangeOn(PLAN, TODAY))

        assertEquals(0f, week.fractionOf(LocalDate(2026, 5, 25)).start)
        assertEquals(1f / 7f, week.fractionOf(LocalDate(2026, 5, 25)).endInclusive, TOLERANCE)
        assertEquals(6f / 7f, week.fractionOf(TODAY).start, TOLERANCE)
        assertEquals(1f, week.fractionOf(TODAY).endInclusive)
    }

    @Test
    fun aMarkStandsInTheMiddleOfItsOwnDay() {
        // Not on the seam: drawn on the boundary it would visually claim the day before.
        val week = assertNotNull(TrendWindow.WEEK.rangeOn(PLAN, TODAY))

        assertEquals(0.5f / 7f, week.middleOf(LocalDate(2026, 5, 25)), TOLERANCE)
        assertEquals(6.5f / 7f, week.middleOf(TODAY), TOLERANCE)
    }

    @Test
    fun aSingleDayWindowIsFilledByItsOneDay() {
        val oneDay = assertNotNull(TrendWindow.CYCLE.rangeOn(planStartingOn(TODAY), TODAY))

        assertEquals(0f..1f, oneDay.fractionOf(TODAY))
        assertEquals(0.5f, oneDay.middleOf(TODAY))
    }

    @Test
    fun aDayOutsideTheWindowIsHeldAtTheEdgeRatherThanDrawnOffIt() {
        val week = assertNotNull(TrendWindow.WEEK.rangeOn(PLAN, TODAY))

        assertEquals(0f..0f, week.fractionOf(LocalDate(2026, 5, 1)))
        assertEquals(1f..1f, week.fractionOf(LocalDate(2026, 6, 30)))
    }

    @Test
    fun aRangeRunsForwardsOrItIsNotARange() {
        assertFailsWith<IllegalArgumentException> {
            TrendRange(LocalDate(2026, 6, 10), LocalDate(2026, 5, 31))
        }
    }
}
