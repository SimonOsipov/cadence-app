package app.cadence.shared.mock

import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.OccurrenceStatus
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The mock wound to a moment, so «today» is the test's choice and not the machine's. */
private fun mocks(iso: String = "2026-05-31T09:00:00Z") = CadenceMocks(clock = FixedCadenceClock.at(iso), zone = ZONE)

class MockRepositoryTest {
    @Test
    fun todayReportsTheNextDoseAsDataRatherThanAsAString() =
        runTest {
            val summary = mocks().today.today()

            val next = assertNotNull(summary.nextDose, "the seeded protocol has a dose due today")
            assertEquals(0.25, next.dose?.value)
            assertEquals(DoseUnit.MG, next.dose?.unit)
        }

    @Test
    fun theCycleWeekMovesWhenTheClockDoes() =
        runTest {
            // The whole point of the clock seam: nothing here is pinned to
            // 31 May the way the prototype's three «todays» are.
            assertEquals(4, mocks("2026-05-31T09:00:00Z").today.today().cycleWeek)
            assertEquals(5, mocks("2026-06-07T09:00:00Z").today.today().cycleWeek)
        }

    @Test
    fun loggingADoseThroughTheInterfaceIsVisibleThroughTheInterface() =
        runTest {
            // The block's acceptance criterion in one test: a write goes in
            // through the repository and comes back out of it, with no screen
            // involved and nothing shared but the interface.
            val m = mocks()
            val before = m.today.today()
            assertFalse(before.doseLoggedToday, "the seeded day starts unlogged")

            m.dosing.logDose(before.nextDose!!.itemId, site = null)

            assertTrue(m.today.today().doseLoggedToday)
        }

    @Test
    fun aLoggedDoseComesOutOfTheVialItWasDrawnFrom() =
        runTest {
            // §03's third correction, end to end through the seam rather than
            // in the arithmetic alone.
            val m = mocks()
            val before = m.today.today().vialDosesLeft

            m.dosing.logDose(
                m.today
                    .today()
                    .nextDose!!
                    .itemId,
                site = null,
            )

            assertEquals(before - 1, m.today.today().vialDosesLeft)
        }

    @Test
    fun theScheduleAndTodayAgreeAboutTheSameDay() =
        runTest {
            // §03's seventh correction: the Today strip and the Schedule screen
            // render the same generated occurrences. The prototype's two
            // disagree because each has its own hardcoded copy.
            val m = mocks()

            val fromSchedule = m.schedule.day(LocalDate(2026, 5, 31))
            val fromToday = m.today.today().nextDose

            assertEquals(
                fromSchedule.first { it.status == OccurrenceStatus.PENDING }.itemId,
                fromToday?.itemId,
            )
        }

    @Test
    fun aMonthOfDaysCarriesTheDotsTheCalendarDraws() =
        runTest {
            val days = mocks().schedule.month(LocalDate(2026, 5, 1))

            assertEquals(31, days.size, "May has 31 days")
            assertTrue(days.any { it.hasInjection }, "the seeded protocol injects in May")
            assertTrue(days.none { it.date.month != LocalDate(2026, 5, 1).month })
        }

    @Test
    fun daysBeforeTheProtocolBeganCarryNothing() =
        runTest {
            // The calendar is drawn for whole months, and the cycle starts on
            // the 10th — the first nine days must come back empty rather than
            // inheriting the protocol.
            val days = mocks().schedule.month(LocalDate(2026, 5, 1))

            assertTrue(days.take(9).none { it.hasInjection })
            assertTrue(days.drop(9).any { it.hasInjection })
        }

    @Test
    fun theDaySummaryCountsMealsAndTheirEnergy() =
        runTest {
            val summary = mocks().today.today()

            // Numbers, not «1 240 ккал» — the formatting lives on the UI side
            // and this is what it is handed.
            assertTrue(summary.mealCount > 0)
            assertTrue(summary.mealKcal > 0)
            assertTrue(summary.targets.kcal > summary.mealKcal)
        }

    @Test
    fun anotherDayHasItsOwnMealsAndNotTheSeededDays() =
        runTest {
            // A day the seed has no meals for. Without a date filter the app
            // reports a breakfast eaten three weeks earlier as eaten today,
            // and «сегодня» stops meaning anything.
            val summary = mocks("2026-06-07T09:00:00Z").today.today()

            assertEquals(0, summary.mealCount)
            assertEquals(0, summary.mealKcal)
        }
}
