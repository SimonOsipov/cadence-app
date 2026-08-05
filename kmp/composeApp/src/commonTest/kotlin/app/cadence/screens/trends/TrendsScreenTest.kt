package app.cadence.screens.trends

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.format.formatDecimal
import app.cadence.format.formatSignedDelta
import app.cadence.format.unitRu
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview
import app.cadence.shared.domain.meta
import app.cadence.shared.domain.notableShifts
import app.cadence.shared.domain.rangeOn
import app.cadence.shared.domain.trendSeries
import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlinx.datetime.atStartOfDayIn
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

private val TODAY = LocalDate(2026, 5, 31)
private val ZONE = TimeZone.of("Europe/Moscow")

/** What a card says instead of a number. Duplicated from the screen on purpose. */
private const val NO_DATA = "нет данных"

/** One metric's window, resolved the way the repository hands it over. */
private fun seriesOf(
    metric: Metric,
    window: TrendWindow = TrendWindow.FOUR_WEEKS,
) = trendSeries(MockSeed.measurements, metric, requireNotNull(window.rangeOn(MockSeed.plan, TODAY)), ZONE)

/** The seeded list, resolved the way the repository hands it over. */
private fun overview(
    window: TrendWindow = TrendWindow.FOUR_WEEKS,
    measurements: List<Measurement> = MockSeed.measurements,
    order: List<Metric> = Metric.entries,
): TrendsOverview {
    val range = requireNotNull(window.rangeOn(MockSeed.plan, TODAY))
    return TrendsOverview(
        window = window,
        range = range,
        metrics = order.map { MetricTrend(it.meta, trendSeries(measurements, it, range, ZONE)) },
    )
}

@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.matches(text: String): Int =
    onAllNodesWithText(text, substring = true).fetchSemanticsNodes().size

/**
 * The text is under *this* node, not merely somewhere on the screen.
 *
 * Unmerged, because a clickable card folds its children's semantics into
 * itself: in the merged tree the label is not a descendant of the card, it *is*
 * the card, and `hasAnyDescendant` finds nothing.
 */
@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.assertCardSays(
    tag: String,
    text: String,
) = onNodeWithTag(tag, useUnmergedTree = true).assert(hasAnyDescendant(hasText(text, substring = true)))

@OptIn(ExperimentalTestApi::class)
class TrendsScreenTest {
    @Test
    fun everyMetricOfTheSetHasItsOwnCard() =
        runComposeUiTest {
            // One check per card, all eight — «по одной проверке на карточку».
            // Existence only: `onNodeWithTag` throws when the tag is missing, so
            // a grid that dropped a metric fails here by name rather than by a
            // count somewhere else.
            setContent {
                CadenceTheme {
                    TrendsScreen(overview(), {}, {}, {}, {})
                }
            }

            Metric.entries.forEach { metric ->
                onNodeWithTag(cadenceTrendCardTag(metric), useUnmergedTree = true).assertExists()
            }
        }

    @Test
    fun theHeroOpensItsOwnMetric() =
        runComposeUiTest {
            // The hero has its own lambda, and it is the one card reliably
            // above the fold in the whole screen. The grid's seven are checked
            // by `TrendsMetricGridTest`, which composes them alone.
            val opened = mutableListOf<Metric>()

            setContent {
                CadenceTheme {
                    TrendsScreen(overview(), onOpenMetric = { opened += it }, {}, {}, {})
                }
            }

            onNodeWithTag(cadenceTrendCardTag(Metric.WEIGHT)).performClick()

            assertEquals(listOf(Metric.WEIGHT), opened)
        }

    @Test
    fun theHeroIsTheFirstMetricOfTheSetAndIsNotRepeatedInTheGrid() =
        runComposeUiTest {
            // «What I looked at last time» is a second kind of memory, and there
            // is nowhere to keep it until the profile block. The prototype
            // features whichever biomarker was opened last.
            setContent {
                CadenceTheme {
                    TrendsScreen(overview(), {}, {}, {}, {})
                }
            }

            // The hero *is* the weight card, rather than merely existing while
            // weight happens to be drawn somewhere.
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true)
                .assert(hasAnyDescendant(hasTestTag(cadenceTrendCardTag(Metric.WEIGHT))))
            // And it is not repeated in the grid below. Exact text, not a
            // substring: «Вес» is inside «Весь цикл» on the chip row.
            assertEquals(1, onAllNodesWithText("Вес").fetchSemanticsNodes().size)
        }

    @Test
    fun tappingAWindowChipAsksForThatWindowAndNotTheOneAlreadyChosen() =
        runComposeUiTest {
            val chosen = mutableListOf<TrendWindow>()

            setContent {
                CadenceTheme {
                    TrendsScreen(overview(TrendWindow.FOUR_WEEKS), {}, onWindowChange = { chosen += it }, {}, {})
                }
            }

            TrendWindow.entries.forEach { window ->
                onNodeWithTag(cadenceTrendWindowTag(window)).performScrollTo().performClick()
            }

            // Including the one already selected: the screen reports the tap and
            // the shell decides, rather than swallowing it here.
            assertEquals(TrendWindow.entries.toList(), chosen)
        }

    @Test
    fun theChosenChipSaysItIsChosen() =
        runComposeUiTest {
            // `CadenceChip` expresses `active` in colour alone, and a colour
            // lands in no semantics: without this a screen reader hears four
            // identical chips and no test can tell which window is on.
            setContent {
                CadenceTheme {
                    TrendsScreen(overview(TrendWindow.THREE_MONTHS), {}, {}, {}, {})
                }
            }

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.THREE_MONTHS), useUnmergedTree = true).assertIsSelected()
            (TrendWindow.entries - TrendWindow.THREE_MONTHS).forEach {
                onNodeWithTag(cadenceTrendWindowTag(it), useUnmergedTree = true).assertIsNotSelected()
            }
        }

    @Test
    fun theWindowChipsCarryThePrototypesOwnWords() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    TrendsScreen(overview(), {}, {}, {}, {})
                }
            }

            listOf("7 дней", "4 недели", "3 месяца", "Весь цикл").forEach {
                onNodeWithText(it).assertExists()
            }
        }

    @Test
    fun theHeroTakesItsNumberFromTheWindowAndNotFromTheWholeCycle() =
        runComposeUiTest {
            // The half of the prototype's bug its own delta gets right, and the
            // half nothing else here would catch. Its hero reads the value from
            // `series.cycle` whatever window is chosen; on the seed the last
            // weight is 31 May in every window, so the discriminating fixture
            // is a week with no weight in it at all.
            val noRecentWeight =
                MockSeed.measurements.filterNot {
                    it.metric == Metric.WEIGHT && it.measuredAt >= LocalDate(2026, 5, 25).atStartOfDayIn(ZONE)
                }
            val week = overview(TrendWindow.WEEK, noRecentWeight)
            val cycle = overview(TrendWindow.CYCLE, noRecentWeight)
            val cycleValue = requireNotNull(cycle.hero?.series?.latest)
            assertEquals(emptyList(), week.hero?.series?.points, "the fixture has to empty the week")

            setContent {
                CadenceTheme {
                    TrendsScreen(week, {}, {}, {}, {})
                }
            }

            // «нет данных», not the cycle's last weight.
            assertEquals(0, matches(formatDecimal(cycleValue, 1)), "a cycle reading has no place in a week window")
            assertCardSays(cadenceTrendCardTag(Metric.WEIGHT), NO_DATA)
        }

    @Test
    fun everyCardShowsItsOwnLatestReading() =
        runComposeUiTest {
            // The number itself, which no other assertion here reads. Swapping
            // `series.latest` for `series.base` — or for the average — leaves
            // every other test in this file green.
            val all = overview()

            setContent {
                CadenceTheme {
                    TrendsScreen(all, {}, {}, {}, {})
                }
            }

            all.metrics.forEach { trend ->
                val latest = trend.series.latest ?: return@forEach
                onNodeWithTag(cadenceTrendCardTag(trend.meta.metric))
                assertCardSays(cadenceTrendCardTag(trend.meta.metric), formatDecimal(latest, trend.meta.decimals))
            }
        }

    @Test
    fun aCardWithOneReadingShowsTheNumberAndNoDelta() =
        runComposeUiTest {
            // «7 дней» leaves weight with a single reading. A delta defaulted to
            // zero would draw «→ 0,0 кг» — the plateau `TrendSeries.delta`'s own
            // KDoc refuses to claim — and every other test would stay green.
            val week = overview(TrendWindow.WEEK)
            val hero = requireNotNull(week.hero)
            assertEquals(1, hero.series.points.size, "the fixture has to leave weight with one reading")
            assertNull(hero.series.delta)

            setContent {
                CadenceTheme {
                    TrendsScreen(week, {}, {}, {}, {})
                }
            }

            assertEquals(0, matches("→"), "nothing plateaued; nothing says it did")
        }

    @Test
    fun theHeroReadsItsDeltaFromTheWindowThatWasChosen() =
        runComposeUiTest {
            // The prototype's bug, not ported: its hero takes the *value* from
            // the cycle series and the *delta* from the selected one, so
            // switching to «7 дней» changes the pill under a number that never
            // moves.
            //
            // «4 недели» and «3 месяца» are the pair that separates them: the
            // weight's window opens on 10 May in one and 12 April in the other,
            // so the base — and therefore the delta — differs, while the latest
            // reading is 31 May in both.
            val month = overview(TrendWindow.FOUR_WEEKS)
            val quarter = overview(TrendWindow.THREE_MONTHS)
            val hero = requireNotNull(month.hero)
            val monthDelta = requireNotNull(month.hero?.series?.delta)
            val quarterDelta = requireNotNull(quarter.hero?.series?.delta)
            assertEquals(quarter.hero?.series?.latest, hero.series.latest, "the same last reading in both")
            assertTrue(monthDelta != quarterDelta, "the fixture has to distinguish the two deltas")

            setContent {
                CadenceTheme {
                    TrendsScreen(month, {}, {}, {}, {})
                }
            }

            val shown = formatSignedDelta(monthDelta, unitRu(hero.meta.unit), hero.meta.decimals)
            val other = formatSignedDelta(quarterDelta, unitRu(hero.meta.unit), hero.meta.decimals)
            assertTrue(matches(shown) > 0, "the hero shows the chosen window's delta")
            assertEquals(0, matches(other), "and not the one from a window nobody asked for")
        }

    @Test
    fun aMetricWithNoReadingsSaysSoRatherThanShowingAnEmptyCard() =
        runComposeUiTest {
            // Chest is unmeasured in the seed on purpose, and this is the
            // criterion that keeps it that way.
            setContent {
                CadenceTheme {
                    TrendsScreen(overview(), {}, {}, {}, {})
                }
            }

            // Under the chest card, not merely somewhere on the screen: chest
            // is the only unmeasured metric today, so an unanchored assertion
            // would pass while some other card went blank.
            onNodeWithTag(cadenceTrendCardTag(Metric.CHEST))
            assertCardSays(cadenceTrendCardTag(Metric.CHEST), NO_DATA)
        }

    @Test
    fun theNotableShiftsNameTheMetricsThatActuallyMovedMost() =
        runComposeUiTest {
            val all = overview()
            val shifts = all.notableShifts()

            setContent {
                CadenceTheme {
                    TrendsScreen(all, {}, {}, {}, {})
                }
            }

            onNodeWithTag(CADENCE_TRENDS_SHIFTS_TAG).assertExists()
            // Derived, not the prototype's three hand-written cards — two of
            // which name things §03 has no metric for at all.
            // A label appears both in its own card and in the shifts row, so
            // this counts rather than asserting a single node.
            shifts.forEach { shift ->
                assertTrue(matches(shift.meta.label) >= 2, "${shift.meta.label} is named in the shifts as well")
            }
        }

    @Test
    fun aWindowWhereNothingMovedDrawsNoShiftsSectionAtAll() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    TrendsScreen(overview(measurements = emptyList()), {}, {}, {}, {})
                }
            }

            // An empty «Заметные сдвиги» heading reads as a section that failed
            // to load rather than one with nothing to say.
            onNodeWithTag(CADENCE_TRENDS_SHIFTS_TAG).assertDoesNotExist()
        }

    @Test
    fun theTwoLinkRowsOpenTheirOwnScreens() =
        runComposeUiTest {
            var journal = 0
            var body = 0

            setContent {
                CadenceTheme {
                    TrendsScreen(overview(), {}, {}, onOpenJournal = { journal++ }, onOpenBody = { body++ })
                }
            }

            onNodeWithTag(CADENCE_TRENDS_JOURNAL_TAG).performScrollTo().performClick()
            onNodeWithTag(CADENCE_TRENDS_BODY_TAG).performScrollTo().performClick()

            assertEquals(1 to 1, journal to body)
        }
}

@OptIn(ExperimentalTestApi::class)
class TrendsMetricGridTest {
    @Test
    fun eachCardOpensItsOwnMetricAndNotTheOneBesideIt() =
        runComposeUiTest {
            // One exact check per card, and the list is deliberately not in
            // enum order: a grid reporting the card's *position* names the
            // wrong metric for both. That is not hypothetical — it is what this
            // grid actually did until this test was written.
            val opened = mutableListOf<Metric>()
            val order = listOf(Metric.CHEST, Metric.SLEEP)

            setContent {
                CadenceTheme {
                    TrendsMetricGrid(order.map { MetricTrend(it.meta, seriesOf(it)) }, onOpen = { opened += it })
                }
            }

            order.forEach { onNodeWithTag(cadenceTrendCardTag(it)).performClick() }

            assertEquals(order, opened)
        }

    @Test
    fun anOddCardStillGetsARowOfItsOwn() =
        runComposeUiTest {
            // Three cards fill one row and start a second, so the odd-one-out
            // branch is exercised. Tags only, not clicks: on this target the
            // first card of a multi-row grid is clicked at the row below it —
            // deterministically, and unaffected by `waitForIdle`. Asserting
            // through that would pin the anomaly rather than the screen.
            val order = listOf(Metric.CHEST, Metric.SLEEP, Metric.HRV)

            setContent {
                CadenceTheme {
                    TrendsMetricGrid(order.map { MetricTrend(it.meta, seriesOf(it)) }, onOpen = {})
                }
            }

            order.forEach { onNodeWithTag(cadenceTrendCardTag(it), useUnmergedTree = true).assertExists() }
        }

    @Test
    fun aCardWithNoReadingsSaysSoAndOneWithThemShowsItsNumber() =
        runComposeUiTest {
            // Both branches of `MetricReading`, side by side: chest is
            // unmeasured in the seed on purpose, weight is not.
            val trends = listOf(Metric.CHEST, Metric.WEIGHT).map { MetricTrend(it.meta, seriesOf(it)) }
            val weight = trends.last()

            setContent {
                CadenceTheme {
                    TrendsMetricGrid(trends, onOpen = {})
                }
            }

            assertCardSays(cadenceTrendCardTag(Metric.CHEST), NO_DATA)
            assertCardSays(
                cadenceTrendCardTag(Metric.WEIGHT),
                formatDecimal(requireNotNull(weight.series.latest), weight.meta.decimals),
            )
        }
}
