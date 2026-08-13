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
        // The window travels with the result: the axis and the dose bands under it are both
        // drawn from these two dates.
        assertEquals(week(), chest.range)
    }

    @Test
    fun theSeriesIsOrderedOldestFirstEvenWhenTheReadingsAreNot() {
        // Middle reading is the lowest of the three, so ordering by date and by value
        // disagree — a sort on the wrong key would still pass against a series that only
        // fell. Ids run backwards against dates for the same reason: a tie-break promoted to
        // the primary key would sort correctly by accident on every other fixture.
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
        // §03: «latest reading wins, whatever the source».
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
        // Left to a stable sort, «the last one» would be whichever row the server listed
        // second, and the same data would answer differently on the next fetch.
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
        // Both directions: a zone bug over-including at one edge is invisible to a test that
        // only watches the other. 22:30 UTC on 24 May is 01:30 on the 25th in Moscow (inside
        // a window opening the 25th, dropped in UTC); 21:30 UTC on 31 May is 00:30 on 1 June
        // (past a window closing today, admitted in UTC).
        val earlyMorning = utcReading(LocalDate(2026, 5, 24), LocalTime(22, 30), 99.4, "m-in")
        val pastMidnight = utcReading(LocalDate(2026, 5, 31), LocalTime(21, 30), 98.2, "m-out")

        val series = trendSeries(listOf(earlyMorning, pastMidnight), Metric.WEIGHT, week(), ZONE)

        assertEquals(listOf(99.4), series.points.map { it.value })

        // The whole reason the zone travels in the result: read in a second zone, the
        // reading admitted at the edge would be drawn beyond it.
        assertEquals(LocalDate(2026, 5, 25), series.dayOf(series.points.single()))
    }

    @Test
    fun theDeltaIsMeasuredFromTheBaseOfTheWindowNotFromThePreviousReading() {
        // «Сегодня» asks how far the last reading moved (+0,4); a window asks how far the
        // patient has come since it opened (−1,2).
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

        // Zero would read as a plateau; a patient with one reading has started, not plateaued.
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
        // The four in-window values are chosen so the mean is none of the things a wrong
        // implementation would return: not the median (98,7), not the min/max midpoint
        // (98,5), not first or last. Extremes sit in the middle, so min/max can't pass as first/last.
        val readings =
            listOf(
                // Well outside the week and both extremes: aggregates over the whole history
                // would show them.
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
        // If any two of these agreed, the window switcher would have nothing to switch.
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

        // A daily metric fills its window up to the history it has: «3 месяца» asks for 84
        // days and gets the 42 lived since the intake.
        assertEquals(
            mapOf(
                TrendWindow.WEEK to 7,
                TrendWindow.FOUR_WEEKS to 28,
                TrendWindow.THREE_MONTHS to 42,
                TrendWindow.CYCLE to 22,
            ),
            series.mapValues { (_, it) -> it.points.size },
        )

        // Four different *series*, not four lengths of one: each opens on a different
        // reading. Compared by day, not value — a rounded ramp may repeat a number.
        assertEquals(
            4,
            series.values
                .mapNotNull { it.points.firstOrNull()?.measuredAt }
                .toSet()
                .size,
            "every window opens on its own reading",
        )
        // They close on the same one, leaving the base as the thing that differs.
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
