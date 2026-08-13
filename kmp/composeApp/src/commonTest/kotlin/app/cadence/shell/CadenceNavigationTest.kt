package app.cadence.shell

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.getBoundsInRoot
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceTheme
import app.cadence.format.formatSignedDelta
import app.cadence.format.unitRu
import app.cadence.screens.trends.CADENCE_TRENDS_HERO_TAG
import app.cadence.screens.trends.CADENCE_TREND_DETAIL_UNKNOWN_TAG
import app.cadence.screens.trends.cadenceTrendCardTag
import app.cadence.screens.trends.cadenceTrendWindowTag
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview
import app.cadence.shared.domain.doseBands
import app.cadence.shared.domain.meta
import app.cadence.shared.domain.protocolMarks
import app.cadence.shared.domain.rangeOn
import app.cadence.shared.domain.trendSeries
import app.cadence.shared.mock.MockSeed
import app.cadence.shared.repository.MetricDetail
import kotlinx.coroutines.awaitCancellation
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The back stack as screen names, root first. Names rather than a count: «two entries deep»
 * is true of many wrong stacks. A type-safe route reads `…CadenceRoute.TrendDetail/{id}`;
 * the argument pattern comes off first, then the package.
 */
private fun NavHostController.stack(): List<String> =
    currentBackStack.value
        .filter { it.destination !is NavGraph }
        .mapNotNull { it.destination.route }
        .map { it.substringBefore('/').substringBefore('?').substringAfterLast('.') }

/**
 * The controller is the point: «did that tap navigate» isn't answerable from what's on
 * screen when two routes render the same word.
 */
@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.startShell(
    trends: TrendsOverview? = null,
    onLoadMetric: suspend (Metric, TrendWindow) -> MetricDetail? = { _, _ -> null },
): NavHostController {
    lateinit var nav: NavHostController
    setContent {
        nav = rememberNavController()
        CadenceTheme { CadenceShell(navController = nav, trends = trends, onLoadMetric = onLoadMetric) }
    }
    return nav
}

/**
 * The window held by the test, like `CadenceApp` holds it — the default-argument version
 * can't see the sharing since both screens would read the same default.
 */
@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.startShellSharingItsWindow(): NavHostController {
    lateinit var nav: NavHostController
    setContent {
        nav = rememberNavController()
        var window by remember { mutableStateOf(TrendWindow.THREE_MONTHS) }
        CadenceTheme {
            CadenceShell(
                navController = nav,
                trends = navOverview(window),
                trendWindow = window,
                onTrendWindow = { window = it },
                onLoadMetric = { metric, chosen -> navDetail(metric, chosen) },
            )
        }
    }
    return nav
}

private val NAV_TODAY = LocalDate(2026, 5, 31)
private val NAV_ZONE = TimeZone.of("Europe/Moscow")

/** The seeded window, resolved the way `MockTrendsRepository` resolves it. */
private fun navRange(window: TrendWindow) = requireNotNull(window.rangeOn(MockSeed.plan, NAV_TODAY))

private fun navOverview(window: TrendWindow = TrendWindow.THREE_MONTHS): TrendsOverview {
    val range = navRange(window)
    return TrendsOverview(
        window = window,
        range = range,
        metrics = Metric.entries.map { MetricTrend(it.meta, trendSeries(MockSeed.measurements, it, range, NAV_ZONE)) },
    )
}

private fun navDetail(
    metric: Metric,
    window: TrendWindow,
): MetricDetail {
    val range = navRange(window)
    return MetricDetail(
        trend = MetricTrend(metric.meta, trendSeries(MockSeed.measurements, metric, range, NAV_ZONE)),
        bands = doseBands(MockSeed.plan, MockSeed.semaItemId, range),
        marks = protocolMarks(MockSeed.plan, MockSeed.semaItemId, range),
    )
}

/** Runs a navigation call on the UI thread and lets the graph settle. */
@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.navigate(block: () -> Unit) {
    runOnUiThread(block)
    waitForIdle()
}

/**
 * One instance of every route. Parameterised routes can't be enumerated from the sealed
 * hierarchy, so they're listed by hand — kept beside the hierarchy it mirrors since an
 * incomplete list is exactly what [CadenceNavigationTest.everyRouteInTheGraphRenders] can't catch.
 */
private object CadenceRouteSamples {
    /**
     * Titles are the point: asserting only that navigation didn't throw proved nothing a
     * `composable<Journal> { }` with an empty body wouldn't also pass — thirteen of the
     * nineteen routes had no other test touching their content.
     */
    val all: List<Pair<CadenceRoute, String>> =
        listOf(
            CadenceRoute.Today to "Сегодня",
            CadenceRoute.Trends to "Тренды",
            // Title is dead weight (nothing draws it), kept only so the count check still
            // crosses the samples against the graph.
            CadenceRoute.TrendDetail("hrv") to "Биомаркер · hrv",
            CadenceRoute.Nutrition to "Питание",
            CadenceRoute.Vials to "Аптечка",
            CadenceRoute.Schedule to "Расписание",
            CadenceRoute.Learn to "Обучение",
            CadenceRoute.Article("a-1") to "Статья · a-1",
            CadenceRoute.Journal to "Дневник",
            CadenceRoute.Body to "Тело",
            CadenceRoute.Recipes to "Рецепты",
            CadenceRoute.RecipeDetail("r-1") to "Рецепт · r-1",
            CadenceRoute.Profile to "Профиль",
            CadenceRoute.ChatList to "Чаты",
            CadenceRoute.ChatThread("ksenia") to "Переписка · ksenia",
            CadenceRoute.LogDose() to "Записать дозу",
            CadenceRoute.LogMeal to "Записать приём пищи",
            CadenceRoute.AddVial to "Добавить флакон",
            CadenceRoute.RecipeBuilder to "Новый рецепт",
        )
}

/**
 * Routes no longer drawing «Экран «X»»; stay in the sample list so the count check still
 * crosses it against the graph, but the placeholder-title walk skips them.
 */
private val PORTED_ROUTES =
    setOf<CadenceRoute>(
        CadenceRoute.AddVial,
        // Draws the metric, «такой метрики нет», or a «Метрика» placeholder, never «Биомаркер ·
        // hrv». `Trends` isn't here — it falls back to its own placeholder, which still matches.
        CadenceRoute.TrendDetail("hrv"),
    )

@OptIn(ExperimentalTestApi::class)
class CadenceNavigationTest {
    @Test
    fun theShellStartsOnToday() =
        runComposeUiTest {
            val nav = startShell()

            onNodeWithText("Экран «Сегодня»").assertIsDisplayed()
            assertEquals(listOf("Today"), nav.stack())
        }

    @Test
    fun aTapOnADestinationPushesIt() =
        runComposeUiTest {
            val nav = startShell()

            onNodeWithText("Тренды").performClick()
            waitForIdle()

            assertEquals(listOf("Today", "Trends"), nav.stack())
        }

    @Test
    fun openingARouteAlreadyInTheStackReturnsToItInsteadOfStackingASecond() =
        runComposeUiTest {
            val nav = startShell()

            // The prototype's «добавить в день» hands back to Nutrition with `navigate`; a
            // plain push would leave two Nutrition entries and a back button revisiting the recipe.
            navigate { nav.selectDestination(CadenceDestination.NUTRITION) }
            navigate { nav.openRoute(CadenceRoute.Recipes) }
            navigate { nav.openRoute(CadenceRoute.RecipeDetail("r-1")) }
            assertEquals(listOf("Today", "Nutrition", "Recipes", "RecipeDetail"), nav.stack())

            navigate { nav.openRoute(CadenceRoute.Nutrition) }

            assertEquals(listOf("Today", "Nutrition"), nav.stack())
        }

    @Test
    fun pushingARouteAlreadyInTheStackAddsASecondCopy() =
        runComposeUiTest {
            val nav = startShell()

            // The one place the prototype asks for push by name: reusing the instance would
            // leave the reader on the article they just left.
            navigate { nav.openRoute(CadenceRoute.Article("a-1")) }
            navigate { nav.pushRoute(CadenceRoute.Article("a-2")) }

            assertEquals(listOf("Today", "Article", "Article"), nav.stack())
            onNodeWithText("Экран «Статья · a-2»").assertIsDisplayed()
        }

    @Test
    fun theTodayTabReturnsToTheRootFromAnyDepth() =
        runComposeUiTest {
            val nav = startShell()

            navigate { nav.openRoute(CadenceRoute.Trends) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            assertEquals(listOf("Today", "Trends", "TrendDetail"), nav.stack())

            navigate { nav.selectDestination(CadenceDestination.TODAY) }

            assertEquals(listOf("Today"), nav.stack())
        }

    @Test
    fun aTabFromDeepInsideAnotherTabDoesNotStackOnTopOfIt() =
        runComposeUiTest {
            val nav = startShell()

            // Pops to root before navigating: without it, four bar taps leave four entries
            // and a back button that walks tab history.
            navigate { nav.openRoute(CadenceRoute.Trends) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            navigate { nav.selectDestination(CadenceDestination.INVENTORY) }

            assertEquals(listOf("Today", "Vials"), nav.stack())
        }

    @Test
    fun replacingSwapsTheTopEntryRatherThanCoveringIt() =
        runComposeUiTest {
            val nav = startShell()

            // Matches the prototype: going back must not return to the screen that just finished.
            navigate { nav.openRoute(CadenceRoute.ChatThread("ksenia")) }
            navigate { nav.replaceRoute(CadenceRoute.ChatList) }

            assertEquals(listOf("Today", "ChatList"), nav.stack())
        }

    @Test
    fun everyRouteInTheGraphRenders() =
        runComposeUiTest {
            val nav = startShell()

            // A route missing from the NavHost throws only when navigated to, which is never
            // until the screens land — this walks all of them so a missing `composable<…>`
            // fails now. The title assertion makes it a rendering test, not a registration
            // test: `hasRoute` after a navigate that didn't throw is nearly tautological.
            CadenceRouteSamples.all.filterNot { it.first in PORTED_ROUTES }.forEach { (route, title) ->
                navigate { nav.pushRoute(route) }
                assertTrue(
                    nav.currentBackStackEntry?.destination?.hasRoute(route::class) == true,
                    "no destination registered for $route",
                )
                onNodeWithText("Экран «$title»").assertIsDisplayed()
            }
        }

    @Test
    fun onlyTheFourBarDestinationsCarryTheBar() =
        runComposeUiTest {
            val nav = startShell()

            // Schedule and Journal have no bar; hoisting it into the shell would put one
            // everywhere with nothing on screen to say so.
            onNodeWithText("Аптечка").assertIsDisplayed()

            navigate { nav.openRoute(CadenceRoute.Schedule) }

            assertTrue(onAllNodesWithText("Аптечка").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun everyRouteIsAccountedForInTheSamples() =
        runComposeUiTest {
            val nav = startShell()

            // No `sealedSubclasses` on Native, so the hand-written sample list could drift
            // from the graph unnoticed; this crosses them so at least that direction can't.
            val registered = nav.graph.count { it !is NavGraph }

            assertEquals(registered, CadenceRouteSamples.all.size, "a route in the graph has no sample")
        }

    @Test
    fun openingTheScreenYouAreOnWithDifferentArgumentsShowsTheNewOnes() =
        runComposeUiTest {
            val nav = startShell(onLoadMetric = { metric, window -> navDetail(metric, window) })

            // A patient tapping a neighbouring biomarker used to hit an early return and do
            // nothing — screen name assertion below catches the dead tap that a dumb-string one wouldn't.
            navigate { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("weight")) }

            assertEquals(listOf("Today", "TrendDetail"), nav.stack())
            onNodeWithText("Вес").assertIsDisplayed()
            assertEquals(0, onAllNodesWithText("HRV").fetchSemanticsNodes().size)
        }

    @Test
    fun theTrendsTabDrawsTheListOnceItsDataHasLanded() =
        runComposeUiTest {
            val nav = startShell(trends = navOverview())

            navigate { nav.openRoute(CadenceRoute.Trends) }

            // The hero card is reliably above the fold, so "is on screen" applies.
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertIsDisplayed()
            assertEquals(0, onAllNodesWithText("Экран «Тренды»").fetchSemanticsNodes().size)
        }

    @Test
    fun aWindowChosenOnTheListIsTheWindowTheMetricOpensWith() =
        runComposeUiTest {
            // Reading the default on both screens proves nothing — two screens each
            // remembering their own would agree too — so the window is changed on the list first.
            val nav = startShellSharingItsWindow()

            navigate { nav.openRoute(CadenceRoute.Trends) }
            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK)).performClick()
            waitForIdle()
            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK), useUnmergedTree = true).assertIsSelected()

            navigate { nav.openRoute(CadenceRoute.TrendDetail("weight")) }

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK), useUnmergedTree = true).assertIsSelected()
            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.THREE_MONTHS), useUnmergedTree = true)
                .assertIsNotSelected()
        }

    @Test
    fun theListReadsTheWindowItWasSwitchedTo() =
        runComposeUiTest {
            // «7 дней» leaves weight with one reading (no delta), «3 месяца» with eight —
            // so the hero's delta being present or absent proves the data followed, not just the chip.
            val nav = startShellSharingItsWindow()
            val hero = requireNotNull(navOverview(TrendWindow.THREE_MONTHS).hero)
            val quarterDelta =
                formatSignedDelta(
                    requireNotNull(hero.series.delta),
                    unitRu(hero.meta.unit),
                    hero.meta.decimals,
                )
            assertNull(navOverview(TrendWindow.WEEK).hero?.series?.delta, "one reading in a week")

            navigate { nav.openRoute(CadenceRoute.Trends) }
            assertTrue(
                onAllNodesWithText(quarterDelta, substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the three-month delta is on screen: $quarterDelta",
            )

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK)).performClick()
            waitForIdle()

            assertEquals(0, onAllNodesWithText(quarterDelta, substring = true).fetchSemanticsNodes().size)
            // Positive check too: an absence alone wouldn't distinguish "re-read" from
            // "no longer composed".
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertExists()
        }

    @Test
    fun tappingAMetricOnTheListOpensThatMetric() =
        runComposeUiTest {
            // Nothing else exercises this tap: an `onOpenMetric = { }` leaves the whole suite green otherwise.
            val nav = startShellSharingItsWindow()

            navigate { nav.openRoute(CadenceRoute.Trends) }
            onNodeWithTag(cadenceTrendCardTag(Metric.WEIGHT)).performClick()
            waitForIdle()

            assertEquals(listOf("Today", "Trends", "TrendDetail"), nav.stack())
            onNodeWithText("Вес").assertIsDisplayed()
        }

    @Test
    fun aMetricStillLoadingIsNotAMetricThatDoesNotExist() =
        runComposeUiTest {
            // A read still in flight must not answer «Такой метрики нет» — same distinction the schedule route makes.
            val nav = startShell(onLoadMetric = { _, _ -> awaitCancellation() })

            navigate { nav.openRoute(CadenceRoute.TrendDetail("weight")) }

            onNodeWithTag(CADENCE_TREND_DETAIL_UNKNOWN_TAG).assertDoesNotExist()
            onNodeWithText("Экран «Метрика»").assertIsDisplayed()
        }

    @Test
    fun aRouteCarryingACodeNoMetricAnswersToSaysSoRatherThanCrashing() =
        runComposeUiTest {
            // The prototype has a `thigh` and a `bmi` §03 doesn't; a deep link can carry anything.
            val nav = startShell(onLoadMetric = { m, w -> navDetail(m, w) })

            navigate { nav.openRoute(CadenceRoute.TrendDetail("thigh")) }

            assertEquals(listOf("Today", "TrendDetail"), nav.stack())
            onNodeWithTag(CADENCE_TREND_DETAIL_UNKNOWN_TAG).assertIsDisplayed()
        }

    @Test
    fun openingTheScreenYouAreOnDoesNotStackASecondCopy() =
        runComposeUiTest {
            val nav = startShell()

            navigate { nav.openRoute(CadenceRoute.Trends) }
            navigate { nav.openRoute(CadenceRoute.Trends) }

            assertEquals(listOf("Today", "Trends"), nav.stack())
        }

    @Test
    fun backReturnsToTheScreenBehind() =
        runComposeUiTest {
            val nav = startShell()

            // Nothing clicked this until a mutation showed `back = { }` passing the whole suite.
            navigate { nav.openRoute(CadenceRoute.Schedule) }
            assertEquals(listOf("Today", "Schedule"), nav.stack())

            onNodeWithContentDescription("Назад").performClick()
            waitForIdle()

            assertEquals(listOf("Today"), nav.stack())
        }

    @Test
    fun theRootOffersNoWayBack() =
        runComposeUiTest {
            startShell()

            assertTrue(onAllNodesWithContentDescription("Назад").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun eachTabScreenHighlightsItsOwnDestination() =
        runComposeUiTest {
            val nav = startShell()

            // TabBarTest proves the bar highlights what it's told; this proves the shell
            // tells it the right thing — passing TODAY for every screen left the suite green.
            navigate { nav.selectDestination(CadenceDestination.INVENTORY) }
            onNodeWithText("Аптечка").assertIsSelected()

            navigate { nav.selectDestination(CadenceDestination.TRENDS) }
            onNodeWithText("Тренды").assertIsSelected()

            navigate { nav.selectDestination(CadenceDestination.NUTRITION) }
            onNodeWithText("Питание").assertIsSelected()
        }

    @Test
    fun returningToAScreenBelowBringsTheArgumentsYouAskedFor() =
        runComposeUiTest {
            val nav = startShell(onLoadMetric = { metric, window -> navDetail(metric, window) })

            // Measured: popping back to an existing entry lands on the arguments it was
            // *created* with, so asking for «weight» put the patient on «hrv».
            navigate { nav.openRoute(CadenceRoute.Trends) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            navigate { nav.openRoute(CadenceRoute.Journal) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("weight")) }

            assertEquals(listOf("Today", "Trends", "TrendDetail"), nav.stack())
            onNodeWithText("Вес").assertIsDisplayed()
            assertEquals(0, onAllNodesWithText("HRV").fetchSemanticsNodes().size)
        }

    @Test
    fun returningToAnArgumentLessScreenKeepsItsEntry() =
        runComposeUiTest {
            val nav = startShell()

            // The common case the argument fix must not break: no arguments to update, so the
            // entry is returned to rather than rebuilt.
            navigate { nav.selectDestination(CadenceDestination.NUTRITION) }
            val entryId = nav.currentBackStackEntry?.id
            navigate { nav.openRoute(CadenceRoute.Recipes) }
            navigate { nav.openRoute(CadenceRoute.Nutrition) }

            assertEquals(listOf("Today", "Nutrition"), nav.stack())
            assertEquals(entryId, nav.currentBackStackEntry?.id, "Nutrition was rebuilt rather than returned to")
        }

    @Test
    fun theScreenBeneathAModalDoesNotMove() =
        runComposeUiTest {
            mainClock.autoAdvance = false
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceShell(navController = nav) }
            }

            val atRest = onNodeWithText("Экран «Сегодня»").getBoundsInRoot()

            // Compose reads a screen's exit transition from the screen being left, so overrides
            // on the modal itself can't affect this — the KDoc claimed otherwise for a whole review round.
            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS / 2L)
            waitForIdle()

            assertEquals(atRest, onNodeWithText("Экран «Сегодня»").getBoundsInRoot(), "the underlay drifted")
        }

    @Test
    fun aModalArrivesFromBelowAndAPushFromTheSide() =
        runComposeUiTest {
            mainClock.autoAdvance = false
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceShell(navController = nav) }
            }

            // Every screen here is a PlaceholderScreen with the same padding, so a settled
            // title's left edge is the reference: sliding up is already at it, sliding in isn't.
            val settledLeft = onNodeWithText("Экран «Сегодня»").getBoundsInRoot().left

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS / 2L)
            waitForIdle()
            assertEquals(
                settledLeft,
                onNodeWithText("Экран «Записать приём пищи»").getBoundsInRoot().left,
                "a modal slid in sideways instead of rising",
            )

            runOnUiThread { nav.popBackStack() }
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS * 2L)
            waitForIdle()

            runOnUiThread { nav.openRoute(CadenceRoute.Schedule) }
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS / 2L)
            waitForIdle()
            assertNotEquals(
                settledLeft,
                onNodeWithText("Экран «Расписание»").getBoundsInRoot().left,
                "an ordinary push did not come in from the side",
            )
        }

    @Test
    fun anOrdinaryPushDoesMoveTheScreenBeneath() =
        runComposeUiTest {
            mainClock.autoAdvance = false
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceShell(navController = nav) }
            }

            val atRest = onNodeWithText("Экран «Сегодня»").getBoundsInRoot()

            // Negative of the test above: without it, "the underlay doesn't move" would also
            // pass against a NavHost that animates nothing at all.
            runOnUiThread { nav.openRoute(CadenceRoute.Schedule) }
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS / 2L)
            waitForIdle()

            assertNotEquals(atRest, onNodeWithText("Экран «Сегодня»").getBoundsInRoot(), "a push did not parallax")
        }
}
