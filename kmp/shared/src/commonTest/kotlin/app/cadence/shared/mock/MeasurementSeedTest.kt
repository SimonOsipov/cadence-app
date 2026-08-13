package app.cadence.shared.mock

import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.MeasurementSource
import app.cadence.shared.domain.Metric
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** Read off a watch every morning. */
private val IMPORTED = listOf(Metric.HRV, Metric.RHR, Metric.SLEEP)

/** Written down by the patient on the weigh-in. */
private val BY_HAND = listOf(Metric.BODY_FAT, Metric.WAIST, Metric.HIP)

private fun mocks() =
    CadenceMocks(
        clock = FixedCadenceClock.at("2026-05-31T09:00:00Z"),
        zone = ZONE,
    )

/** Every point of a metric, not the `DEFAULT_POINTS` tail. */
private suspend fun all(metric: Metric): List<Measurement> =
    mocks()
        .measurements
        .series(metric, points = 1_000)
        .points

private fun Measurement.date(): LocalDate = measuredAt.toLocalDateTime(ZONE).date

/**
 * A chart with two points has nothing to say about a window, and four windows over the same
 * seven readings would leave the switcher inert — these properties are asserted once here
 * rather than re-derived on every screen.
 */
class MeasurementSeedTest {
    @Test
    fun everyMetricButChestHasBeenMeasured() =
        runTest {
            // Chest stays unmeasured on purpose: «a metric with no readings says so» needs a
            // metric that actually has none.
            val unmeasured = Metric.entries.filter { all(it).isEmpty() }

            assertEquals(listOf(Metric.CHEST), unmeasured)
        }

    @Test
    fun theHistoryStartsOnTheDayThePatientJoinedTheClinic() =
        runTest {
            // Weight is the exception, and not a tidy one: its eight readings begin 8 days
            // before the intake, predating the clinic — but the literals carry two mutation
            // traps (an unsorted seed, more points than DEFAULT_POINTS) regenerating them
            // would destroy, so the inconsistency is recorded rather than fixed here.
            val joined = assertNotNull(MockSeed.profile.joinedAt)

            for (metric in Metric.entries - Metric.CHEST - Metric.WEIGHT) {
                assertEquals(joined, all(metric).first().date(), "${metric.code} starts elsewhere")
            }
            assertTrue(
                all(Metric.WEIGHT).first().date() < joined,
                "the weight seed no longer predates the intake — if that was deliberate, delete this line " +
                    "rather than restoring it: it records a known inconsistency, not a requirement",
            )
        }

    @Test
    fun theImportedMetricsAreDailyAndTheManualOnesAreWeekly() =
        runTest {
            for (metric in IMPORTED) {
                assertEquals(42, all(metric).size, "${metric.code}: one reading a day, intake to today")
            }

            for (metric in BY_HAND) {
                val days = all(metric).map { it.date() }

                assertEquals(7, days.size, "${metric.code}: the intake plus six weigh-ins")
                assertEquals(MockSeed.profile.joinedAt, days.first())
                // The count alone would survive the whole series sliding to Saturday.
                assertTrue(
                    days.drop(1).all { it.dayOfWeek == DayOfWeek.SUNDAY },
                    "${metric.code} is not measured on the weigh-in: $days",
                )
            }
        }

    @Test
    fun aWeekOfHistoryIsNotTheFirstWeekOfIt() =
        runTest {
            // The window switcher has to switch to the *recent* end: `take` where `takeLast`
            // was meant would draw the patient's first week forever, and both are seven points long.
            val week = mocks().measurements.series(Metric.HRV, points = 7).points

            assertEquals(7, week.size)
            assertTrue(
                week.map { it.value } != all(Metric.HRV).map { it.value }.take(7),
                "the last seven readings are the first seven",
            )
            assertEquals(LocalDate(2026, 5, 25), week.first().date())
        }

    @Test
    fun aMetricIsReadTheOneWayItCanBe() =
        runTest {
            // Numbers are data, formatting is presentation: a tape reading imported from a
            // watch, or an HRV in bpm, would be a plausible-looking lie.
            val units =
                mapOf(
                    Metric.HRV to "ms",
                    Metric.RHR to "bpm",
                    Metric.SLEEP to "/100",
                    Metric.BODY_FAT to "%",
                    Metric.WAIST to "cm",
                    Metric.HIP to "cm",
                )

            for ((metric, unit) in units) {
                val points = all(metric)

                assertTrue(points.isNotEmpty(), "${metric.code} has no readings to check")
                assertEquals(setOf(unit), points.map { it.unit }.toSet(), "${metric.code} is in the wrong unit")
            }
            for (metric in IMPORTED) {
                assertTrue(
                    all(metric).all { it.source == MeasurementSource.HEALTH_KIT },
                    "${metric.code} is not imported",
                )
            }
            for (metric in BY_HAND) {
                assertTrue(
                    all(metric).all { it.source == MeasurementSource.MANUAL },
                    "${metric.code} claims to be imported: §11 has no importer for it",
                )
            }
        }

    @Test
    fun noTwoReadingsShareAnId() =
        runTest {
            // Anything indexing by id (a chart's keys, an edit, a delete) hits the wrong
            // reading the moment two collide.
            val ids = MockSeed.measurements.map { it.id }

            assertEquals(ids.size, ids.toSet().size, "the seed holds duplicate measurement ids")
        }

    @Test
    fun aWatchedMetricHasBadDaysOnTheWayUp() =
        runTest {
            // The ripple's job is a reversal — a bad night inside a good six weeks — which
            // "steps aren't all equal" wouldn't test (rounding a straight ramp already
            // produces steps of 0 and 1). Only daily metrics carry this: on the weekly ones
            // the ripple is deliberately smaller than a week's progress, so they descend cleanly.
            for (metric in IMPORTED) {
                val rising = all(metric).last().value > all(metric).first().value
                val reversals =
                    all(metric)
                        .zipWithNext { a, b -> b.value - a.value }
                        .count { if (rising) it < 0 else it > 0 }

                assertTrue(reversals > 0, "${metric.code} runs straight: the ripple is doing nothing")
            }
        }

    @Test
    fun theTapeIsInCentimetresAndOnARealBody() =
        runTest {
            // The prototype's waist series is in inches; ported as written it would draw a
            // 188cm patient with a 37cm waist.
            for (metric in listOf(Metric.WAIST, Metric.HIP)) {
                val points = all(metric)

                assertTrue(points.isNotEmpty(), "${metric.code} has nothing to be in centimetres")
                assertTrue(points.all { it.unit == "cm" }, "${metric.code} is not in centimetres")
                assertTrue(
                    points.all { it.value > 60.0 },
                    "${metric.code} carries the prototype's inches: ${points.map { it.value }}",
                )
                // Guards the rounding grid, not the ripple — `aWatchedMetricHasBadDaysOnTheWayUp` holds that.
                assertTrue(
                    points.any { it.value % 1.0 != 0.0 },
                    "${metric.code} is measured to a whole centimetre: ${points.map { it.value }}",
                )
            }
        }

    @Test
    fun eachMetricMovesTheWayProgressMovesForIt() =
        runTest {
            val rising = listOf(Metric.HRV, Metric.SLEEP)
            val falling = listOf(Metric.RHR, Metric.BODY_FAT, Metric.WAIST, Metric.HIP)

            for (metric in rising) {
                val points = all(metric)
                assertTrue(points.last().value > points.first().value, "${metric.code} did not rise")
            }
            for (metric in falling) {
                val points = all(metric)
                assertTrue(points.last().value < points.first().value, "${metric.code} did not fall")
            }
        }

    @Test
    fun theEndpointsAreTheNumbersTheSeedNames() =
        runTest {
            // Ripple is zeroed on the first and last day, so a metric starts and ends where
            // the seed says rather than wherever the wave happened to land.
            assertEquals(50.0, all(Metric.HRV).first().value)
            assertEquals(58.0, all(Metric.HRV).last().value)
            assertEquals(104.0, all(Metric.WAIST).first().value)
            assertEquals(99.0, all(Metric.WAIST).last().value)
        }

    @Test
    fun theNewestReadingIsStillNotAWeight() =
        runTest {
            // The trap `theLatestWeightIsTheLatestWeightAndNotTheLatestReading` is built on:
            // a lookup taking the last row instead of the last row *of that metric* puts an
            // HRV on the Today screen.
            val newest = MockSeed.measurements.maxBy { it.measuredAt }

            assertEquals(Metric.HRV, newest.metric)
        }
}
