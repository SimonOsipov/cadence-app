package app.cadence.shared.mock

import app.cadence.shared.domain.DoseUnit
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.PartOfDay
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
    fun theSummaryCarriesTheDayItIsAboutAndTheHourOfIt() =
        runTest {
            // The greeting is «Воскресенье, утро · 4-я неделя» in the prototype,
            // frozen. Both halves are facts about the clock, and the zone is
            // the patient's: 09:00 UTC is noon in Moscow, which is «день».
            val morning = mocks("2026-05-31T04:00:00Z").today.today()
            val afternoon = mocks("2026-05-31T09:00:00Z").today.today()

            assertEquals(LocalDate(2026, 5, 31), morning.date)
            assertEquals(PartOfDay.MORNING, morning.partOfDay)
            assertEquals(PartOfDay.AFTERNOON, afternoon.partOfDay)
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
    fun theDayDotsSayWhichDayIsWhich() =
        runTest {
            // Three of ScheduleDay's five fields had no assertion anywhere, and
            // dropping the emptiness guard from `allDone` painted «всё
            // выполнено» over the nine days before the protocol began.
            val m = mocks()
            val days = m.schedule.month(LocalDate(2026, 5, 1))
            val beforeTheCycle = days.first { it.date == LocalDate(2026, 5, 1) }
            val today = days.first { it.date == LocalDate(2026, 5, 31) }

            assertEquals(null, beforeTheCycle.cycleWeek)
            assertFalse(beforeTheCycle.allDone, "a day outside the protocol cannot be done")
            assertFalse(beforeTheCycle.anyPending)

            assertEquals(4, today.cycleWeek)
            assertTrue(today.anyPending, "today has occurrences nobody has logged")
        }

    @Test
    fun aSeriesIsOldestFirstAndKeepsTheMostRecentPoints() =
        runTest {
            // Ordered because a chart is drawn left to right, and truncated
            // from the *end*: `take` instead of `takeLast` would draw the
            // patient's first seven weeks forever.
            val series = mocks().measurements.series(Metric.WEIGHT, points = 3)

            assertEquals(3, series.points.size)
            assertEquals(series.points.sortedBy { it.measuredAt }, series.points)
            assertEquals(series.points.last(), series.latest)
            assertEquals(98.4, series.latest?.value)
        }

    @Test
    fun aMetricWithNoReadingsIsEmptyRatherThanAbsent() =
        runTest {
            // A blank chart beside a caption reads as «failed to load», so the
            // caller has to be able to tell «nothing measured» apart from an
            // error — which means not throwing.
            val series = mocks().measurements.series(Metric.CHEST)

            assertTrue(series.points.isEmpty())
            assertEquals(null, series.latest)
        }

    @Test
    fun theWeightSeriesOnTodayIsTheSameSevenPoints() =
        runTest {
            val m = mocks()

            assertEquals(
                m.measurements
                    .series(Metric.WEIGHT)
                    .points
                    .map { it.value },
                m.today.today().weightSeries,
            )
        }

    @Test
    fun theReorderHintReachesTheScreenPayload() =
        runTest {
            // The only thing wiring `dosesPerWeek()` to what a patient sees.
            // Nothing asserted `reorder` at all, so the hint could disappear
            // for every patient with the gate green.
            val hint = mocks().today.today().reorder

            assertEquals(MockSeed.semaglutide.id, hint?.compoundId)
            assertEquals(4, hint?.weeksLeft, "four doses at one a week")
        }

    @Test
    fun theLatestWeightIsTheLatestWeightAndNotTheLatestReading() =
        runTest {
            // The seed's most recent measurement is an HRV of 58. «Latest
            // reading wins» is per metric, and a lookup that took the last row
            // would put 58 kg on the Today screen.
            assertEquals(98.4, mocks().today.today().weightKg)
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
