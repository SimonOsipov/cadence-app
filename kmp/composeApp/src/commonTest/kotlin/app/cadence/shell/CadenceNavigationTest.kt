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
 * The back stack as screen names, root first.
 *
 * Names rather than a count: «two entries deep» is true of a great many wrong
 * stacks, and every assertion below is really about *which* screens are behind
 * the user, which is the thing a back button walks.
 *
 * A type-safe route reads `app.cadence.shell.CadenceRoute.TrendDetail/{id}`;
 * the argument pattern comes off first, then the package.
 */
private fun NavHostController.stack(): List<String> =
    currentBackStack.value
        .filter { it.destination !is NavGraph }
        .mapNotNull { it.destination.route }
        .map { it.substringBefore('/').substringBefore('?').substringAfterLast('.') }

/**
 * The shell under test, with its controller handed back.
 *
 * The controller is the point: «did that tap navigate» is not answerable from
 * what is on screen when two routes render the same word.
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
 * The shell with the window held by the test, the way `CadenceApp` holds it.
 *
 * The default-argument version cannot see the sharing: both screens would read
 * the same default and agree without anything passing between them.
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
 * One instance of every route.
 *
 * Parameterised routes cannot be enumerated from the sealed hierarchy, so they
 * are listed by hand — and this list being incomplete is exactly what
 * [CadenceNavigationTest.everyRouteInTheGraphRenders] cannot catch, so it is
 * kept beside the hierarchy it mirrors.
 */
private object CadenceRouteSamples {
    /**
     * Every route with the title its screen must draw.
     *
     * The titles are the point. Asserting only that navigation did not throw
     * proved nothing a `composable<Journal> { }` with an empty body would not
     * also pass — and thirteen of the nineteen routes had no other test
     * touching their content. The four carrying an argument spell it out, so a
     * screen that drops its parameter fails here too.
     */
    val all: List<Pair<CadenceRoute, String>> =
        listOf(
            CadenceRoute.Today to "Сегодня",
            CadenceRoute.Trends to "Тренды",
            // The title is dead weight now — nothing draws it — but the entry
            // stays so the count check still crosses the samples against the
            // graph.
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
 * Routes that no longer draw «Экран «X»». They stay in the sample list, so the
 * count check still crosses it against the graph; only the placeholder-title
 * walk skips them.
 */
private val PORTED_ROUTES =
    setOf<CadenceRoute>(
        CadenceRoute.AddVial,
        // The walk's title no longer matches in any state: the route draws the
        // metric, «такой метрики нет», or — on the first read — a placeholder
        // titled «Метрика» rather than «Биомаркер · hrv». `Trends` is *not*
        // here: it falls back to the destination's own placeholder, which the
        // walk does still match.
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

            // Today → Nutrition → Recipes → RecipeDetail, then the prototype's
            // «добавить в день» hands back to Nutrition with `navigate`. React
            // Navigation walks back to the existing Nutrition; a plain push
            // would leave two of them and a back button that visits the recipe
            // again.
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

            // An article linking to an article is the one place the prototype
            // asks for `push` by name. Reusing the instance would leave the
            // reader on the article they just left.
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

            // `changeTab` pops to the root *before* it navigates, so the bar
            // never builds a stack of tabs. Without the pop, four taps on the
            // bar would leave four entries and a back button that walks the
            // user's tab history.
            navigate { nav.openRoute(CadenceRoute.Trends) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            navigate { nav.selectDestination(CadenceDestination.INVENTORY) }

            assertEquals(listOf("Today", "Vials"), nav.stack())
        }

    @Test
    fun replacingSwapsTheTopEntryRatherThanCoveringIt() =
        runComposeUiTest {
            val nav = startShell()

            // The prototype's chat thread replaces itself with the list, and
            // the builder replaces itself with the saved recipe: in both, going
            // back must not return to the screen that just finished.
            navigate { nav.openRoute(CadenceRoute.ChatThread("ksenia")) }
            navigate { nav.replaceRoute(CadenceRoute.ChatList) }

            assertEquals(listOf("Today", "ChatList"), nav.stack())
        }

    @Test
    fun everyRouteInTheGraphRenders() =
        runComposeUiTest {
            val nav = startShell()

            // A route declared in CadenceRoute but never added to the NavHost
            // throws only when something navigates to it — which, until the
            // screens land, is never. This walks all of them so a missing
            // `composable<…>` fails now rather than in step 9 of the block.
            //
            // The title assertion is what makes it a rendering test rather than
            // a registration test: `hasRoute` after a navigate that did not
            // throw is very nearly tautological.
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

            // In the prototype the bar lives inside the four screens that have
            // one, not in the navigator: Schedule and Journal have none. A port
            // that hoists the bar into the shell puts one on every screen, and
            // nothing on screen would say so.
            onNodeWithText("Аптечка").assertIsDisplayed()

            navigate { nav.openRoute(CadenceRoute.Schedule) }

            assertTrue(onAllNodesWithText("Аптечка").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun everyRouteIsAccountedForInTheSamples() =
        runComposeUiTest {
            val nav = startShell()

            // The sample list is hand-written — no `sealedSubclasses` on Native
            // — so a route added to the graph without a sample would go
            // unwalked and its own comment admits it. This crosses the list
            // against the graph so at least that direction cannot drift.
            val registered = nav.graph.count { it !is NavGraph }

            assertEquals(registered, CadenceRouteSamples.all.size, "a route in the graph has no sample")
        }

    @Test
    fun openingTheScreenYouAreOnWithDifferentArgumentsShowsTheNewOnes() =
        runComposeUiTest {
            val nav = startShell(onLoadMetric = { metric, window -> navDetail(metric, window) })

            // A patient on one biomarker taps a neighbouring one. This used to
            // hit an early return and do nothing at all — the tap died, the
            // screen kept the old biomarker, and nothing anywhere said why.
            //
            // Two *real* metrics now, because the screen draws their names: a
            // dead tap leaves «HRV» on screen where «Вес» is expected.
            navigate { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("weight")) }

            assertEquals(listOf("Today", "TrendDetail"), nav.stack())
            onNodeWithText("Вес").assertIsDisplayed()
            assertEquals(0, onAllNodesWithText("HRV").fetchSemanticsNodes().size)
        }

    @Test
    fun theTrendsTabDrawsTheListOnceItsDataHasLanded() =
        runComposeUiTest {
            // Gated like the cabinet: the placeholder stands in while the first
            // read is in flight, and a list composed against nothing would say
            // «нет данных» on all eight cards — a sentence about the patient
            // rather than about the fetch.
            val nav = startShell(trends = navOverview())

            navigate { nav.openRoute(CadenceRoute.Trends) }

            // The hero: the one card reliably above the fold, so the stronger
            // «is on screen» rather than «is in the tree» applies.
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertIsDisplayed()
            assertEquals(0, onAllNodesWithText("Экран «Тренды»").fetchSemanticsNodes().size)
        }

    @Test
    fun aWindowChosenOnTheListIsTheWindowTheMetricOpensWith() =
        runComposeUiTest {
            // The choice has to *travel*. Reading the default on both screens
            // proves nothing — two screens each remembering their own would
            // agree too. So the window is changed on the list first.
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
            // And the *data* follows, not only the chip: «7 дней» leaves weight
            // with one reading, «3 месяца» with eight, so the hero's delta is
            // present in one and absent in the other.
            val nav = startShellSharingItsWindow()
            val hero = requireNotNull(navOverview(TrendWindow.THREE_MONTHS).hero)
            val quarterDelta =
                formatSignedDelta(
                    requireNotNull(hero.series.delta),
                    unitRu(hero.meta.unit),
                    hero.meta.decimals,
                )
            // Derived, not guessed: «7 дней» leaves weight with one reading and
            // therefore no delta at all, so the string can only come from the
            // wider window.
            assertNull(navOverview(TrendWindow.WEEK).hero?.series?.delta, "one reading in a week")

            navigate { nav.openRoute(CadenceRoute.Trends) }
            assertTrue(
                onAllNodesWithText(quarterDelta, substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the three-month delta is on screen: $quarterDelta",
            )

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK)).performClick()
            waitForIdle()

            assertEquals(0, onAllNodesWithText(quarterDelta, substring = true).fetchSemanticsNodes().size)
            // Positive as well as negative: «the list re-read» and «the list is
            // no longer composed» both satisfy an absence, and the click
            // mutates state the graph builder closed over.
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertExists()
        }

    @Test
    fun tappingAMetricOnTheListOpensThatMetric() =
        runComposeUiTest {
            // The screen's main tap, and nothing else exercises it: an
            // `onOpenMetric = { }` leaves the whole suite green otherwise. The
            // hero is the card reliably above the fold.
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
            // One value on the screen, two different facts. A read that has not
            // landed must not answer «Такой метрики нет» — the graph tells them
            // apart, the way the schedule route does.
            val nav = startShell(onLoadMetric = { _, _ -> awaitCancellation() })

            navigate { nav.openRoute(CadenceRoute.TrendDetail("weight")) }

            onNodeWithTag(CADENCE_TREND_DETAIL_UNKNOWN_TAG).assertDoesNotExist()
            onNodeWithText("Экран «Метрика»").assertIsDisplayed()
        }

    @Test
    fun aRouteCarryingACodeNoMetricAnswersToSaysSoRatherThanCrashing() =
        runComposeUiTest {
            // A deep link can carry anything, and the prototype has a `thigh`
            // and a `bmi` §03 does not. The route resolves the code; the screen
            // renders the answer.
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

            // The way out of fifteen of the nineteen routes, and nothing
            // clicked it until a mutation showed `back = { }` passing the whole
            // suite.
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

            // Today is the start destination; a back chevron there would be an
            // affordance for something that cannot happen.
            assertTrue(onAllNodesWithContentDescription("Назад").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun eachTabScreenHighlightsItsOwnDestination() =
        runComposeUiTest {
            val nav = startShell()

            // TabBarTest proves the bar highlights what it is told to. This
            // proves the shell tells it the right thing — passing TODAY for
            // every screen left the whole suite green.
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

            // The other half of the dead tap. Popping back to an existing entry
            // lands on the arguments it was *created* with, so asking for
            // «weight» put the patient on «hrv» — measured, and worse than the
            // version that merely pushed a duplicate.
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

            // The common case, and the one the argument fix must not break: a
            // bottom-bar destination carries nothing to update, so it is
            // returned to rather than rebuilt, and its entry survives.
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

            // A native fullScreenModal leaves the screen under it where it was.
            // Compose reads a screen's exit transition from the screen being
            // left, so overrides placed on the modal could not affect this —
            // they fired on a different event entirely, and the KDoc claimed
            // otherwise for a whole review round.
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

            // Every screen here is a PlaceholderScreen with the same padding,
            // so a settled title's left edge is the reference: a screen sliding
            // up is already at it, one sliding in from the right is not.
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

            // The negative of the test above: without it, «the underlay does
            // not move» would also pass against a NavHost that animates nothing
            // at all.
            runOnUiThread { nav.openRoute(CadenceRoute.Schedule) }
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS / 2L)
            waitForIdle()

            assertNotEquals(atRest, onNodeWithText("Экран «Сегодня»").getBoundsInRoot(), "a push did not parallax")
        }
}
