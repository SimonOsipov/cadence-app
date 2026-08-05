package app.cadence.shared.mock

import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.ProtocolMarkKind
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.notableShifts
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

private fun mocks() = CadenceMocks(clock = FixedCadenceClock.at("2026-05-31T09:00:00Z"), zone = ZONE)

/** What the two trend screens read, resolved against the seed. */
class MockTrendsTest {
    @Test
    fun theOverviewCarriesEveryMetricOfTheDataModel() =
        runTest {
            val overview = mocks().trends.overview(TrendWindow.FOUR_WEEKS)

            // The *list*, not the set: `TrendsOverview.hero` is
            // `metrics.firstOrNull()`, and the only production place that fixes
            // the order is this repository. Sorted by code instead, the hero
            // becomes «Грудь» — the unmeasured metric, in the largest card on
            // the screen — and a set assertion would not notice.
            assertEquals(Metric.entries.toList(), overview.metrics.map { it.meta.metric })
            assertEquals(Metric.WEIGHT, overview.hero?.meta?.metric)
            assertEquals(TrendWindow.FOUR_WEEKS, overview.window)
            assertEquals(28, overview.range.days)
        }

    @Test
    fun theWindowAskedForIsTheWindowMeasured() =
        runTest {
            // Four windows, four different spans — the switcher has something
            // to switch, and the repository is not answering with one of them
            // whatever it was asked.
            val spans =
                TrendWindow.entries.associateWith {
                    mocks()
                        .trends
                        .overview(it)
                        .range.days
                }

            assertEquals(
                mapOf(
                    TrendWindow.WEEK to 7,
                    TrendWindow.FOUR_WEEKS to 28,
                    TrendWindow.THREE_MONTHS to 84,
                    TrendWindow.CYCLE to 22,
                ),
                spans,
            )

            // And the readings were clipped to it, not merely reported against
            // it: a repository measuring one window while answering with
            // another's dates passes every assertion above.
            val weights =
                TrendWindow.entries.associateWith { window ->
                    mocks()
                        .trends
                        .overview(window)
                        .metrics
                        .single { it.meta.metric == Metric.WEIGHT }
                        .series.points
                        .size
                }
            assertEquals(
                mapOf(
                    TrendWindow.WEEK to 1,
                    TrendWindow.FOUR_WEEKS to 4,
                    TrendWindow.THREE_MONTHS to 8,
                    TrendWindow.CYCLE to 4,
                ),
                weights,
            )
        }

    @Test
    fun theShiftsComeOutOfTheSeedRatherThanOutOfAConstant() =
        runTest {
            val overview = mocks().trends.overview(TrendWindow.CYCLE)
            val shifts = overview.notableShifts()

            assertTrue(shifts.isNotEmpty())
            // Every one of them moved, and the unmeasured metric is not among
            // them — the prototype's three cards are hand-written prose, two of
            // which name things §03 has no metric for.
            assertTrue(shifts.all { it.movement != null })
            assertTrue(shifts.none { it.meta.metric == Metric.CHEST })
        }

    @Test
    fun oneMetricComesWithTheProtocolDrawnBesideIt() =
        runTest {
            val detail = mocks().trends.metric(Metric.HRV, TrendWindow.CYCLE)

            assertEquals(Metric.HRV, detail.trend.meta.metric)
            // The whole of «весь цикл» so far is inside the first phase, so
            // there is one band and the only mark is the start.
            assertEquals(listOf(0.25), detail.bands.map { it.dose.value })
            assertEquals(listOf(ProtocolMarkKind.START), detail.marks.map { it.kind })
            assertEquals(LocalDate(2026, 5, 10), detail.marks.single().date)
        }

    @Test
    fun aWiderWindowBringsTheTitrationsIntoView() =
        runTest {
            // The seed's course runs past today, so «весь цикл» stops at the
            // 31st and shows one band. Nothing else in the repository would
            // reveal that the bands are clipped by the window rather than
            // fetched whole.
            val cycle = mocks().trends.metric(Metric.WEIGHT, TrendWindow.CYCLE)
            val week = mocks().trends.metric(Metric.WEIGHT, TrendWindow.WEEK)

            assertTrue(
                cycle.bands
                    .single()
                    .range.days >
                    week.bands
                        .single()
                        .range.days,
            )
            assertEquals(
                7,
                week.bands
                    .single()
                    .range.days,
                "the band is clipped to the window it was asked for",
            )
        }

    @Test
    fun aCourseThatHasNotBegunAnswersWithADayAndNothingInIt() =
        runTest {
            // `rangeOn` gives null before the start date, and a repository has
            // to answer something. Recorded as a divergence; measured here,
            // because a fallback nothing exercises is a fallback that can rot
            // into an inverted range and throw out of `TrendRange`.
            val early = CadenceMocks(clock = FixedCadenceClock.at("2026-05-01T09:00:00Z"), zone = ZONE)
            val overview = early.trends.overview(TrendWindow.CYCLE)

            assertEquals(1, overview.range.days)
            assertEquals(LocalDate(2026, 5, 1), overview.range.from)
            // That one day's readings, and no others — the daily metrics were
            // already being taken before the course began.
            assertTrue(
                overview.metrics.all { trend ->
                    trend.series.points.all { trend.series.dayOf(it) == overview.range.from }
                },
            )
            // And nothing can have «moved» inside a single day, so the section
            // stays away rather than ranking eight metrics by nothing.
            assertEquals(emptyList(), overview.notableShifts())
        }

    @Test
    fun anUnmeasuredMetricStillHasARowAndAnEmptySeries() =
        runTest {
            val detail = mocks().trends.metric(Metric.CHEST, TrendWindow.CYCLE)

            assertEquals(Metric.CHEST, detail.trend.meta.metric)
            assertTrue(
                detail.trend.series.points
                    .isEmpty(),
            )
            // The prescription is still drawn: a metric nobody measured was
            // still measured against a protocol.
            assertTrue(detail.bands.isNotEmpty())
        }
}
