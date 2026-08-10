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
 * What the trends screens need out of the seed before they can be built.
 *
 * A chart with two points has nothing to say about a window, and four windows
 * over the same seven readings would leave the switcher inert. These are the
 * properties the rest of the feature reads off, asserted here once rather than
 * re-derived on every screen.
 */
class MeasurementSeedTest {
    @Test
    fun everyMetricButChestHasBeenMeasured() =
        runTest {
            // Chest stays unmeasured on purpose: «a metric with no readings
            // says so» is an acceptance criterion, and it needs a metric in the
            // mock that actually has none.
            val unmeasured = Metric.entries.filter { all(it).isEmpty() }

            assertEquals(listOf(Metric.CHEST), unmeasured)
        }

    @Test
    fun theHistoryStartsOnTheDayThePatientJoinedTheClinic() =
        runTest {
            // Deeper than the intake would describe a patient the clinic had
            // not met yet. Every measured metric starts there — the tape ones
            // because that is when the tape came out.
            //
            // Weight is the exception, and not a tidy one: its eight readings
            // begin on 12 April, eight days before the patient joined. That
            // predates the clinic, but the literals carry two mutation traps
            // (an unsorted seed, more points than `DEFAULT_POINTS`) that
            // regenerating them would destroy, so the inconsistency is recorded
            // rather than fixed here.
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
            // Six weeks. A watch reports every morning; the manual metrics are
            // taken on the protocol's weigh-in Sunday.
            for (metric in IMPORTED) {
                assertEquals(42, all(metric).size, "${metric.code}: one reading a day, intake to today")
            }

            // The intake, then six Sundays.
            for (metric in BY_HAND) {
                val days = all(metric).map { it.date() }

                assertEquals(7, days.size, "${metric.code}: the intake plus six weigh-ins")
                assertEquals(MockSeed.profile.joinedAt, days.first())
                // The count alone would survive the whole series sliding to
                // Saturday — and the reason these days were chosen is that they
                // are the days the protocol already weighs in on.
                assertTrue(
                    days.drop(1).all { it.dayOfWeek == DayOfWeek.SUNDAY },
                    "${metric.code} is not measured on the weigh-in: $days",
                )
            }
        }

    @Test
    fun aWeekOfHistoryIsNotTheFirstWeekOfIt() =
        runTest {
            // The point of seeding this deep: the window switcher has to have
            // something to switch, and it has to switch to the *recent* end.
            // `take` where `takeLast` was meant would draw the patient's first
            // week forever, and both are seven points long.
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
            // The unit is the whole contract with the formatter — numbers are
            // data, formatting is presentation — and the source is what §11
            // allows for that metric. A tape reading imported from a watch, or
            // an HRV in beats per minute, would be a plausible-looking lie.
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
            // Ids are generated from the metric and the day. Anything that
            // indexes by id — a chart's keys, an edit, a delete — would hit the
            // wrong reading the moment two collide.
            val ids = MockSeed.measurements.map { it.id }

            assertEquals(ids.size, ids.toSet().size, "the seed holds duplicate measurement ids")
        }

    @Test
    fun aWatchedMetricHasBadDaysOnTheWayUp() =
        runTest {
            // A chart of a perfect line reads as a mock, and a slope with no
            // noise cannot show a window doing anything a shorter window would
            // not. What the ripple actually buys is that the series turns back
            // on itself — a bad night inside a good six weeks.
            //
            // «The steps are not all equal» would not have said this: rounding
            // a straight ramp onto whole numbers already produces steps of 0
            // and 1. A reversal is the thing only the ripple can produce, and
            // switching it off gives exactly zero of them.
            //
            // Only the daily metrics carry this. On the weekly ones the ripple
            // is deliberately smaller than a week's progress, so they descend
            // cleanly — a tape that read backwards every other week would be a
            // measuring error, not a mood.
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
            // The prototype's waist runs 37,5 → 35,0 under a «см» label beside
            // a thigh of 64,5 см: that series is in inches. Ported as written
            // it would draw a 188 cm patient with a 37 cm waist.
            for (metric in listOf(Metric.WAIST, Metric.HIP)) {
                val points = all(metric)

                assertTrue(points.isNotEmpty(), "${metric.code} has nothing to be in centimetres")
                assertTrue(points.all { it.unit == "cm" }, "${metric.code} is not in centimetres")
                assertTrue(
                    points.all { it.value > 60.0 },
                    "${metric.code} carries the prototype's inches: ${points.map { it.value }}",
                )
                // A tape is read to the millimetre, and the prototype prints
                // one decimal. This guards the rounding grid, not the ripple —
                // the interpolation alone lands off the whole number, so
                // `aWatchedMetricHasBadDaysOnTheWayUp` is what holds the ripple.
                assertTrue(
                    points.any { it.value % 1.0 != 0.0 },
                    "${metric.code} is measured to a whole centimetre: ${points.map { it.value }}",
                )
            }
        }

    @Test
    fun eachMetricMovesTheWayProgressMovesForIt() =
        runTest {
            // Six weeks on the protocol have to be visible, and visible in the
            // right direction — otherwise every trend reads «no progress» and
            // the screens are drawing a flat line.
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
            // The ripple is zeroed on the first and last day on purpose, so a
            // metric starts and ends where the seed says rather than wherever
            // the wave happened to land. Every «latest reading» on every screen
            // is that last point.
            assertEquals(50.0, all(Metric.HRV).first().value)
            assertEquals(58.0, all(Metric.HRV).last().value)
            assertEquals(104.0, all(Metric.WAIST).first().value)
            assertEquals(99.0, all(Metric.WAIST).last().value)
        }

    @Test
    fun theNewestReadingIsStillNotAWeight() =
        runTest {
            // The trap `theLatestWeightIsTheLatestWeightAndNotTheLatestReading`
            // is built on: a lookup that takes the last row instead of the last
            // row *of that metric* puts an HRV on the Today screen. Six more
            // metrics could quietly hand the last row back to weight.
            val newest = MockSeed.measurements.maxBy { it.measuredAt }

            assertEquals(Metric.HRV, newest.metric)
        }
}
