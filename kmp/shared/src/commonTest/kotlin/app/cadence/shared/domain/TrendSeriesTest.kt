package app.cadence.shared.domain

import app.cadence.shared.domain.TrendFixture.PATIENT
import app.cadence.shared.domain.TrendFixture.PLAN
import app.cadence.shared.domain.TrendFixture.TODAY
import app.cadence.shared.domain.TrendFixture.ZONE
import app.cadence.shared.domain.TrendFixture.reading
import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toInstant
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

private fun week() = requireNotNull(TrendWindow.WEEK.rangeOn(PLAN, TODAY))

class TrendSeriesTest {
    @Test
    fun theSeriesHoldsOneMetricAndKeepsItsOwnIdentityWhenEmpty() {
        val readings =
            listOf(
                reading(Metric.WEIGHT, LocalDate(2026, 5, 28), 98.8),
                reading(Metric.HRV, LocalDate(2026, 5, 28), 57.0),
            )

        val weight = trendSeries(readings, Metric.WEIGHT, week(), ZONE)
        val chest = trendSeries(readings, Metric.CHEST, week(), ZONE)

        assertEquals(listOf(98.8), weight.points.map { it.value })
        assertEquals(Metric.CHEST, chest.metric, "an empty series still says which metric it is about")
        assertTrue(chest.points.isEmpty())
        // The window travels with the result: the chart's axis and the dose
        // bands under it are both drawn from these two dates, and a series that
        // dropped them would have them derived a second time.
        assertEquals(week(), chest.range)
    }

    @Test
    fun theSeriesIsOrderedOldestFirstEvenWhenTheReadingsAreNot() {
        // The seed puts one of its weights out of list order on purpose. A
        // chart is drawn left to right, and a base taken off an unsorted list
        // is whichever row the server happened to send first.
        //
        // The middle reading is the lowest of the three, so ordering by date
        // and ordering by value disagree — a sort on the wrong key would still
        // pass against a series that only ever fell.
        //
        // The ids run backwards against the dates for the same reason. Every id
        // this codebase generates carries an ISO date, so id order and time
        // order normally agree, and a tie-break promoted to the primary key
        // would sort correctly by accident on every other fixture here.
        val readings =
            listOf(
                reading(Metric.WEIGHT, LocalDate(2026, 5, 29), 98.4, id = "m-m"),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 26), 99.1, id = "m-z"),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.9, id = "m-a"),
            )

        val series = trendSeries(readings, Metric.WEIGHT, week(), ZONE)

        assertEquals(listOf(99.1, 98.4, 98.9), series.points.map { it.value })
    }

    @Test
    fun twoReadingsOnOneDayAreOrderedWithinIt() {
        // §03: «latest reading wins, whatever the source». A patient who
        // weighed in before breakfast and whose watch synced that evening has
        // two readings on one day, and the evening one is the later one.
        val readings =
            listOf(
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.9, at = LocalTime(20, 30)),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.4, at = LocalTime(6, 0)),
            )

        val series = trendSeries(readings, Metric.WEIGHT, week(), ZONE)

        assertEquals(listOf(98.4, 98.9), series.points.map { it.value })
        assertEquals(98.9, series.latest)
    }

    @Test
    fun readingsSharingAnInstantAreBrokenApartByIdRatherThanByArrivalOrder() {
        // Two sources can stamp the same minute. Left to a stable sort, «the
        // last one» would be whichever row the server listed second, and the
        // same data would answer differently on the next fetch.
        val at = LocalTime(6, 0)
        val forwards =
            listOf(
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.9, at = at, id = "m-b"),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.4, at = at, id = "m-a"),
            )

        val series = trendSeries(forwards, Metric.WEIGHT, week(), ZONE)
        val reversed = trendSeries(forwards.reversed(), Metric.WEIGHT, week(), ZONE)

        assertEquals(listOf(98.4, 98.9), series.points.map { it.value })
        assertEquals(series.points.map { it.value }, reversed.points.map { it.value })
    }

    @Test
    fun bothEdgesOfTheWindowAreInsideItAndTheDaysBesideThemAreNot() {
        val readings =
            listOf(
                reading(Metric.WEIGHT, LocalDate(2026, 5, 24), 99.5),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 25), 99.1),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.4),
                reading(Metric.WEIGHT, LocalDate(2026, 6, 1), 98.2),
            )

        val series = trendSeries(readings, Metric.WEIGHT, week(), ZONE)

        assertEquals(listOf(99.1, 98.4), series.points.map { it.value })
    }

    @Test
    fun aReadingIsPlacedByThePatientsCalendarRatherThanByUtc() {
        // Both directions, because a zone bug that only over-includes at one
        // edge is invisible to a test that watches the other.
        //
        // 22:30 UTC on 24 May is 01:30 on the 25th in Moscow — inside a window
        // that opens on the 25th, and dropped by a comparison done in UTC.
        // 21:30 UTC on 31 May is 00:30 on 1 June — past a window that closes
        // today, and admitted by that same comparison.
        val earlyMorning = utcReading(LocalDate(2026, 5, 24), LocalTime(22, 30), 99.4, "m-in")
        val pastMidnight = utcReading(LocalDate(2026, 5, 31), LocalTime(21, 30), 98.2, "m-out")

        val series = trendSeries(listOf(earlyMorning, pastMidnight), Metric.WEIGHT, week(), ZONE)

        assertEquals(listOf(99.4), series.points.map { it.value })

        // And the day the series reports for it is the patient's day, not the
        // UTC one. This is the whole reason the zone travels in the result: a
        // chart places its readings on the axis the window describes, and read
        // in a second zone the reading admitted at the edge is drawn beyond it.
        assertEquals(LocalDate(2026, 5, 25), series.dayOf(series.points.single()))
    }

    @Test
    fun theDeltaIsMeasuredFromTheBaseOfTheWindowNotFromThePreviousReading() {
        // The distinction the whole step exists for. «Сегодня» asks how far
        // the last reading moved — here that is +0,4 — while a window asks how
        // far the patient has come since it opened, which is −1,2.
        val readings =
            listOf(
                reading(Metric.WEIGHT, LocalDate(2026, 5, 26), 100.0),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 29), 98.4),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.8),
            )

        val series = trendSeries(readings, Metric.WEIGHT, week(), ZONE)

        assertEquals(100.0, series.base)
        assertEquals(98.8, series.latest)
        assertEquals(-1.2, assertNotNull(series.delta), 1e-9)
    }

    @Test
    fun theBaseIsTheFirstReadingInsideTheWindowNotTheFirstOneEver() {
        val readings =
            listOf(
                reading(Metric.WEIGHT, LocalDate(2026, 4, 20), 104.0),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 26), 100.0),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.8),
            )

        val series = trendSeries(readings, Metric.WEIGHT, week(), ZONE)

        assertEquals(100.0, series.base, "April's reading is history, not this week's base")
        assertEquals(-1.2, assertNotNull(series.delta), 1e-9)
    }

    @Test
    fun oneReadingHasNothingToCompareItselfTo() {
        val series =
            trendSeries(
                listOf(reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.4)),
                Metric.WEIGHT,
                week(),
                ZONE,
            )

        // Zero would read as a plateau, and a patient with one reading has not
        // plateaued — they have started.
        assertNull(series.delta)
        assertEquals(98.4, series.base)
        assertEquals(98.4, series.average)
    }

    @Test
    fun aMetricWithNoReadingsInTheWindowIsEmptyRatherThanZero() {
        val series =
            trendSeries(
                listOf(reading(Metric.WEIGHT, LocalDate(2026, 4, 20), 104.0)),
                Metric.WEIGHT,
                week(),
                ZONE,
            )

        assertTrue(series.points.isEmpty())
        assertNull(series.base)
        assertNull(series.latest)
        assertNull(series.delta)
        assertNull(series.average)
        assertNull(series.minimum)
        assertNull(series.maximum)
    }

    @Test
    fun theAggregatesAreTakenOverTheWindowAndNotOverEverythingKnown() {
        // The four in-window values are chosen so that the mean is none of the
        // things a wrong implementation would return: it is not the median
        // (98,7), not the midpoint of min and max (98,5), not the first reading
        // and not the last. The extremes sit in the middle rather than at the
        // ends, so `minimum`/`maximum` cannot pass as `first`/`last` either.
        val readings =
            listOf(
                // Well outside the week, and both an extreme: aggregates read
                // off the whole history would show them.
                reading(Metric.WEIGHT, LocalDate(2026, 4, 20), 104.0),
                reading(Metric.WEIGHT, LocalDate(2026, 4, 27), 90.0),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 26), 99.0),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 28), 100.0),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 29), 97.0),
                reading(Metric.WEIGHT, LocalDate(2026, 5, 31), 98.4),
            )

        val series = trendSeries(readings, Metric.WEIGHT, week(), ZONE)

        assertEquals(98.6, assertNotNull(series.average), 1e-9)
        assertEquals(97.0, series.minimum)
        assertEquals(100.0, series.maximum)
    }

    @Test
    fun theFourWindowsOfTheSeedAreFourDifferentSpansAndFourDifferentSeries() {
        // What step-1 seeded six weeks for. If any two of these agreed, the
        // window switcher would have nothing to switch.
        val spans = TrendWindow.entries.associateWith { requireNotNull(it.rangeOn(MockSeed.plan, TODAY)).days }
        assertEquals(
            mapOf(
                TrendWindow.WEEK to 7,
                TrendWindow.FOUR_WEEKS to 28,
                TrendWindow.THREE_MONTHS to 84,
                TrendWindow.CYCLE to 22,
            ),
            spans,
        )

        val series =
            TrendWindow.entries.associateWith {
                trendSeries(MockSeed.measurements, Metric.HRV, requireNotNull(it.rangeOn(MockSeed.plan, TODAY)), ZONE)
            }

        // A daily metric fills its window up to the history it has: «3 месяца»
        // asks for 84 days and gets the 42 lived since the intake — so the
        // spans and the point counts are different numbers, not one number told
        // twice.
        assertEquals(
            mapOf(
                TrendWindow.WEEK to 7,
                TrendWindow.FOUR_WEEKS to 28,
                TrendWindow.THREE_MONTHS to 42,
                TrendWindow.CYCLE to 22,
            ),
            series.mapValues { (_, it) -> it.points.size },
        )

        // And they are four different *series*, not four lengths of one: each
        // opens on a different reading. Compared by day rather than by value —
        // a ramp rounded onto whole milliseconds may well repeat a number, and
        // then this would be asserting arithmetic instead of windowing.
        assertEquals(
            4,
            series.values
                .mapNotNull { it.points.firstOrNull()?.measuredAt }
                .toSet()
                .size,
            "every window opens on its own reading",
        )
        // They close on the same one, which is what leaves the base as the
        // thing that differs.
        assertEquals(
            1,
            series.values
                .mapNotNull { it.points.lastOrNull()?.measuredAt }
                .toSet()
                .size,
        )
    }

    @Test
    fun theMetricLeftUnmeasuredSaysSoInEveryWindow() {
        // `CHEST` is unmeasured on purpose, and this is the criterion that
        // keeps it that way.
        TrendWindow.entries.forEach { window ->
            val range = requireNotNull(window.rangeOn(MockSeed.plan, TODAY))
            val series = trendSeries(MockSeed.measurements, Metric.CHEST, range, ZONE)
            assertTrue(series.points.isEmpty(), "${window.name} should hold no chest readings")
            assertNull(series.average)
        }
    }
}

private fun utcReading(
    date: LocalDate,
    at: LocalTime,
    value: Double,
    id: String,
): Measurement =
    Measurement(
        id = MeasurementId(id),
        patientId = PATIENT,
        metric = Metric.WEIGHT,
        value = value,
        unit = "kg",
        measuredAt = LocalDateTime(date, at).toInstant(TimeZone.UTC),
        source = MeasurementSource.MANUAL,
        externalId = null,
        note = null,
    )
