package app.cadence.screens.trends

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_SCRUB_CHART_TAG
import app.cadence.design.CadenceTheme
import app.cadence.format.dayAndMonth
import app.cadence.format.formatDecimal
import app.cadence.format.formatSignedDelta
import app.cadence.format.unitRu
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.doseBands
import app.cadence.shared.domain.meta
import app.cadence.shared.domain.protocolMarks
import app.cadence.shared.domain.rangeOn
import app.cadence.shared.domain.trendSeries
import app.cadence.shared.mock.MockSeed
import app.cadence.shared.repository.MetricDetail
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val TODAY = LocalDate(2026, 5, 31)
private val ZONE = TimeZone.of("Europe/Moscow")

/** Resolved the way `MockTrendsRepository` resolves it. */
private fun detail(
    metric: Metric = Metric.HRV,
    window: TrendWindow = TrendWindow.FOUR_WEEKS,
): MetricDetail {
    val range = requireNotNull(window.rangeOn(MockSeed.plan, TODAY))
    return MetricDetail(
        trend = MetricTrend(metric.meta, trendSeries(MockSeed.measurements, metric, range, ZONE)),
        bands = doseBands(MockSeed.plan, MockSeed.semaItemId, range),
        marks = protocolMarks(MockSeed.plan, MockSeed.semaItemId, range),
    )
}

@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.says(
    tag: String,
    text: String,
) = onNodeWithTag(tag, useUnmergedTree = true).assert(hasAnyDescendant(hasText(text, substring = true)))

@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.matches(text: String): Int =
    onAllNodesWithText(text, substring = true).fetchSemanticsNodes().size

@OptIn(ExperimentalTestApi::class)
class TrendDetailScreenTest {
    @Test
    fun eachAggregateBoxHoldsItsOwnNumber() =
        runComposeUiTest {
            // One check per box. All four come off the same window as the chart,
            // and they are four different numbers on this fixture — a strip
            // wired to one of them four times passes any single assertion.
            val one = detail()
            val series = one.trend.series
            val digits = one.trend.meta.decimals

            setContent {
                CadenceTheme {
                    TrendDetailScreen(one, TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            says(cadenceTrendStatTag("avg"), formatDecimal(requireNotNull(series.average), digits))
            says(cadenceTrendStatTag("min"), formatDecimal(requireNotNull(series.minimum), digits))
            says(cadenceTrendStatTag("max"), formatDecimal(requireNotNull(series.maximum), digits))
            says(
                cadenceTrendStatTag("delta"),
                formatSignedDelta(requireNotNull(series.delta), unitRu(one.trend.meta.unit), digits),
            )
            // Compared as *rendered*, not as raw doubles: two of the four can
            // round to one string on a zero-decimal metric, and then a
            // box-swap mutant survives.
            assertEquals(
                4,
                setOf(
                    formatDecimal(requireNotNull(series.average), digits),
                    formatDecimal(requireNotNull(series.minimum), digits),
                    formatDecimal(requireNotNull(series.maximum), digits),
                    formatSignedDelta(requireNotNull(series.delta), unitRu(one.trend.meta.unit), digits),
                ).size,
                "the fixture has to distinguish the four boxes",
            )
        }

    @Test
    fun theFourBoxesShareTheStripEvenly() =
        runComposeUiTest {
            // Measured, not described. A `Row` hands width out in child order,
            // and «Δ» — the only box carrying an arrow and a unit — is last and
            // widest, so without an even share it is the one a narrow screen
            // clips. Nothing else in this file reads geometry, so a missing
            // weight would otherwise be invisible.
            setContent {
                CadenceTheme {
                    TrendDetailScreen(detail(), TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            val widths =
                listOf("avg", "min", "max", "delta").map {
                    onNodeWithTag(cadenceTrendStatTag(it), useUnmergedTree = true)
                        .fetchSemanticsNode()
                        .boundsInRoot.width
                }

            assertTrue(widths.max() - widths.min() < 2f, "four even boxes, not $widths")
        }

    @Test
    fun theAggregatesFollowTheWindowRatherThanTheWholeHistory() =
        runComposeUiTest {
            // «Сред.» over four weeks is not «сред.» over three months, and the
            // strip has to say which one it was asked about.
            val month = detail(window = TrendWindow.FOUR_WEEKS)
            val quarter = detail(window = TrendWindow.THREE_MONTHS)
            val digits = month.trend.meta.decimals
            val shown = formatDecimal(requireNotNull(month.trend.series.average), digits)
            val other = formatDecimal(requireNotNull(quarter.trend.series.average), digits)
            assertTrue(shown != other, "the fixture has to distinguish the two windows")

            setContent {
                CadenceTheme {
                    TrendDetailScreen(month, TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            says(cadenceTrendStatTag("avg"), shown)
            assertEquals(0, matches(other), "no number from a window nobody asked for")
        }

    @Test
    fun theRecentRowsAreTheTailOfTheWindowNewestFirst() =
        runComposeUiTest {
            // Newest first, because that is the order a log is read in — and
            // the opposite of the chart above, which runs left to right.
            val one = detail()
            val newest =
                one.trend.series.points
                    .last()
            val digits = one.trend.meta.decimals

            setContent {
                CadenceTheme {
                    TrendDetailScreen(one, TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            // The day, not the value: HRV rounds to whole milliseconds and the
            // last two readings both render «58», so a bare number is satisfied
            // by the neighbouring row.
            val newestDay = dayAndMonth(one.trend.series.dayOf(newest))
            val olderDay =
                dayAndMonth(one.trend.series.dayOf(one.trend.series.points[one.trend.series.points.size - 3]))
            says(CADENCE_TREND_DETAIL_RECENT_TAG, newestDay)

            // The reading before the tail is not in the list: seven rows, not
            // twenty-eight.
            val tooOld =
                one.trend.series.points
                    .dropLast(7)
                    .last()
            assertEquals(0, matches(dayAndMonth(one.trend.series.dayOf(tooOld))), "only the tail is listed")

            // And the newest sits above the older one. Presence alone says
            // nothing about order: dropping `.reversed()` leaves the same seven
            // rows, read the wrong way round.
            val newestTop = onNodeWithText(newestDay, substring = true).fetchSemanticsNode().boundsInRoot.top
            val olderTop = onNodeWithText(olderDay, substring = true).fetchSemanticsNode().boundsInRoot.top
            assertTrue(newestTop < olderTop, "the newest reading is at the top: $newestTop vs $olderTop")
        }

    @Test
    fun theCaptionSaysWhatTheDashedLinesAre() =
        runComposeUiTest {
            // The chart draws them; nothing else on the screen explains them.
            setContent {
                CadenceTheme {
                    TrendDetailScreen(detail(), TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            assertTrue(matches("Пунктир — изменения протокола") > 0)
        }

    @Test
    fun aMetricWithNoReadingsSaysSoAndDrawsNoAggregates() =
        runComposeUiTest {
            // Chest is unmeasured in the seed on purpose. A strip of «—» under
            // four headings is worse than saying there is nothing.
            setContent {
                CadenceTheme {
                    TrendDetailScreen(detail(Metric.CHEST), TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            assertTrue(matches("нет данных") > 0)
            onNodeWithTag(CADENCE_TREND_DETAIL_STATS_TAG).assertDoesNotExist()
            onNodeWithTag(CADENCE_TREND_DETAIL_RECENT_TAG).assertDoesNotExist()
        }

    @Test
    fun anIdentifierThatNamesNoMetricSaysSoRatherThanCrashing() =
        runComposeUiTest {
            // The prototype has a `thigh` and a `bmi` that §03 does not, and a
            // deep link can carry anything at all.
            setContent {
                CadenceTheme {
                    TrendDetailScreen(null, TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            onNodeWithTag(CADENCE_TREND_DETAIL_UNKNOWN_TAG).assertExists()
            onNodeWithTag(CADENCE_TREND_DETAIL_STATS_TAG).assertDoesNotExist()
            // And the way back is still there, or the screen is a dead end.
            onNodeWithTag(CADENCE_TREND_DETAIL_BACK_TAG).assertExists()
        }

    @Test
    fun theChipThatIsChosenIsTheWindowTheScreenWasGiven() =
        runComposeUiTest {
            // `detail` and `window` are two parameters that have to agree, and
            // nothing in `MetricDetail` enforces it — so a screen that
            // hardcoded a window, or step-8 wiring a stale one, would show a
            // chart of one window under a chip saying another.
            setContent {
                CadenceTheme {
                    TrendDetailScreen(detail(window = TrendWindow.WEEK), TrendWindow.WEEK, {}, {})
                }
            }

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK), useUnmergedTree = true).assertIsSelected()
            (TrendWindow.entries - TrendWindow.WEEK).forEach {
                onNodeWithTag(cadenceTrendWindowTag(it), useUnmergedTree = true).assertIsNotSelected()
            }
        }

    @Test
    fun theChartIsNotDrawnForAMetricWithNoReadings() =
        runComposeUiTest {
            // «говорит об этом, а не рисует пустую ось» — the axis, the dose
            // bands and the invitation to drag are all suppressed, the way
            // `BiomarkerSheet` suppresses its chart.
            setContent {
                CadenceTheme {
                    TrendDetailScreen(detail(Metric.CHEST), TrendWindow.FOUR_WEEKS, {}, {})
                }
            }

            onNodeWithTag(CADENCE_SCRUB_CHART_TAG).assertDoesNotExist()
            assertEquals(0, matches("Тяните по графику"), "nothing to drag, nothing inviting it")
        }

    @Test
    fun theBackControlGoesBack() =
        runComposeUiTest {
            var back = 0

            setContent {
                CadenceTheme {
                    TrendDetailScreen(detail(), TrendWindow.FOUR_WEEKS, {}, onBack = { back++ })
                }
            }

            onNodeWithTag(CADENCE_TREND_DETAIL_BACK_TAG).performClick()

            assertEquals(1, back)
        }

    @Test
    fun theWindowSwitcherReportsTheWindowItWasTapped() =
        runComposeUiTest {
            // The same component the list screen uses, and the same hoisted
            // choice: the prototype keeps the timeframe in app state so the two
            // screens agree.
            val chosen = mutableListOf<TrendWindow>()

            setContent {
                CadenceTheme {
                    TrendDetailScreen(detail(), TrendWindow.FOUR_WEEKS, onWindowChange = { chosen += it }, {})
                }
            }

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK)).performClick()

            assertEquals(listOf(TrendWindow.WEEK), chosen)
        }
}
