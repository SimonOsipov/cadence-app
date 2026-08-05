package app.cadence.shared.domain

import app.cadence.shared.domain.TrendFixture.PLAN
import app.cadence.shared.domain.TrendFixture.TODAY
import app.cadence.shared.domain.TrendFixture.ZONE
import app.cadence.shared.domain.TrendFixture.reading
import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

private fun window() = requireNotNull(TrendWindow.FOUR_WEEKS.rangeOn(PLAN, TODAY))

/** The whole set, resolved the way the repository will hand it over. */
private fun overview(
    measurements: List<Measurement> = MockSeed.measurements,
    window: TrendWindow = TrendWindow.FOUR_WEEKS,
): TrendsOverview {
    val range = requireNotNull(window.rangeOn(MockSeed.plan, TODAY))
    return TrendsOverview(
        window = window,
        range = range,
        metrics = Metric.entries.map { MetricTrend(it.meta, trendSeries(measurements, it, range, ZONE)) },
    )
}

class MetricTrendTest {
    @Test
    fun movementIsMeasuredAgainstWhereTheWindowOpenedAndNotInTheMetricsOwnUnit() {
        // The point of it is to rank metrics against each other. Eight
        // milliseconds of HRV on a base of fifty is a bigger change than two
        // kilograms on a base of a hundred, and comparing the raw numbers would
        // sort by unit.
        val hrv =
            MetricTrend(
                Metric.HRV.meta,
                trendSeries(
                    listOf(
                        reading(Metric.HRV, LocalDate(2026, 5, 10), 50.0),
                        reading(Metric.HRV, LocalDate(2026, 5, 31), 58.0),
                    ),
                    Metric.HRV,
                    window(),
                    ZONE,
                ),
            )
        val weight =
            MetricTrend(
                Metric.WEIGHT.meta,
                trendSeries(
                    listOf(
                        reading(Metric.WEIGHT, LocalDate(2026, 5, 10), 100.0),
                        reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.0),
                    ),
                    Metric.WEIGHT,
                    window(),
                    ZONE,
                ),
            )

        val hrvMoved = requireNotNull(hrv.movement)
        val weightMoved = requireNotNull(weight.movement)
        assertEquals(0.16, hrvMoved, 1e-9)
        assertEquals(0.02, weightMoved, 1e-9)
        assertTrue(hrvMoved > weightMoved, "the smaller number moved the patient further")
    }

    @Test
    fun aMetricWithNothingToCompareHasNoMovement() {
        val once =
            MetricTrend(
                Metric.WEIGHT.meta,
                trendSeries(listOf(reading(Metric.WEIGHT, TODAY, 98.0)), Metric.WEIGHT, window(), ZONE),
            )
        val never = MetricTrend(Metric.CHEST.meta, trendSeries(emptyList(), Metric.CHEST, window(), ZONE))

        assertNull(once.movement, "one reading has moved from nothing")
        assertNull(never.movement)
    }
}

class TrendsOverviewTest {
    @Test
    fun theListHoldsEveryMetricIncludingTheOnesWithNothingToShow() {
        // The set, not the count — and the unmeasured one has to be in it, or a
        // patient cannot find out that it is unmeasured.
        assertEquals(Metric.entries.toSet(), overview().metrics.map { it.meta.metric }.toSet())
        assertTrue(
            overview()
                .metrics
                .single { it.meta.metric == Metric.CHEST }
                .series.points
                .isEmpty(),
            "chest is in the list and empty, not absent",
        )
    }

    @Test
    fun theHeroIsTheFirstMetricOfTheSetAndTheRestFollowIt() {
        // «What I looked at last time» is a second kind of memory, and there is
        // nowhere to keep it until the profile block. The prototype features
        // whichever biomarker was opened last; this screen stays a function of
        // its data.
        val all = overview()

        assertEquals(Metric.WEIGHT, all.hero?.meta?.metric)
        assertEquals(Metric.entries.drop(1), all.rest.map { it.meta.metric })
        assertEquals(all.metrics.size, 1 + all.rest.size, "the hero is not also in the grid")
    }

    @Test
    fun theShiftsAreRankedByHowFarTheMetricMovedNotByTheOrderOfTheSet() {
        val shifts = overview().notableShifts()

        assertEquals(NOTABLE_SHIFTS, shifts.size)
        val movements = shifts.map { requireNotNull(it.movement) }
        assertEquals(movements.sortedDescending(), movements, "largest first")
        // And they really are the largest three of the whole set, not the
        // first three that happened to have a delta.
        val everyMovement = overview().metrics.mapNotNull { it.movement }.sortedDescending()
        assertEquals(everyMovement.take(NOTABLE_SHIFTS), movements)
        // Named, so a reader can see what the seed actually ranks — and so a
        // change to the seed shows up here rather than passing as «still three».
        assertEquals(listOf(Metric.SLEEP, Metric.HRV, Metric.RHR), shifts.map { it.meta.metric })
    }

    @Test
    fun aMetricThatHasNotMovedTwiceIsNotAShift() {
        // A section called «заметные сдвиги» listing a metric that has not been
        // measured twice is naming a shift that did not happen. Chest is
        // unmeasured in the seed on purpose, so it is the one to check.
        val shifts = overview().notableShifts(limit = Metric.entries.size)

        assertTrue(shifts.none { it.meta.metric == Metric.CHEST })
        assertTrue(shifts.all { it.movement != null })
    }

    @Test
    fun aMetricThatOpenedOnZeroHasNoMovementRatherThanAnInfiniteOne() {
        // A flaky import can produce it, and without the guard the share is
        // `Infinity` — which sorts above every real metric and makes that
        // reading the headline shift for as long as it is in the window.
        val fromNothing =
            MetricTrend(
                Metric.SLEEP.meta,
                trendSeries(
                    listOf(
                        reading(Metric.SLEEP, LocalDate(2026, 5, 10), 0.0),
                        reading(Metric.SLEEP, LocalDate(2026, 5, 31), 5.0),
                    ),
                    Metric.SLEEP,
                    window(),
                    ZONE,
                ),
            )

        assertNull(fromNothing.movement)
    }

    @Test
    fun twoMetricsThatMovedTheSameShareKeepAFixedOrder() {
        // The tie-break, not order-independence: `trendSeries` sorts and the
        // overview is built in `Metric.entries` order, so reversing the input
        // is the same computation. What this measures is that a stable sort
        // with no `thenBy` would put WAIST first.
        val tied =
            listOf(
                reading(Metric.WAIST, LocalDate(2026, 5, 10), 100.0),
                reading(Metric.WAIST, LocalDate(2026, 5, 31), 90.0),
                reading(Metric.HIP, LocalDate(2026, 5, 10), 110.0),
                reading(Metric.HIP, LocalDate(2026, 5, 31), 99.0),
            )

        val once = overview(tied).notableShifts()
        val twice = overview(tied.reversed()).notableShifts()

        assertEquals(listOf(Metric.HIP, Metric.WAIST), once.map { it.meta.metric }, "by code when the share ties")
        assertEquals(once.map { it.meta.metric }, twice.map { it.meta.metric })
    }

    @Test
    fun aWindowWithNoMovementAnywhereHasNoShiftsRatherThanThreeEmptyOnes() {
        val silent = overview(emptyList())

        assertEquals(emptyList(), silent.notableShifts())
        // The metrics are still all there, so the grid still says «нет данных»
        // for each of them.
        assertEquals(Metric.entries.size, silent.metrics.size)
    }
}
