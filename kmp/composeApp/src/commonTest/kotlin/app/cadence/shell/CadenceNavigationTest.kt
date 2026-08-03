package app.cadence.shell

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceTheme
import kotlin.test.Test
import kotlin.test.assertEquals
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
private fun ComposeUiTest.startShell(): NavHostController {
    lateinit var nav: NavHostController
    setContent {
        nav = rememberNavController()
        CadenceTheme { CadenceShell(navController = nav) }
    }
    return nav
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
            CadenceRoute.LogDose to "Записать дозу",
            CadenceRoute.LogMeal to "Записать приём пищи",
            CadenceRoute.AddVial to "Добавить флакон",
            CadenceRoute.RecipeBuilder to "Новый рецепт",
        )
}

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
            CadenceRouteSamples.all.forEach { (route, title) ->
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
            val nav = startShell()

            // A patient on one biomarker taps a neighbouring one. This used to
            // hit an early return and do nothing at all — the tap died, the
            // screen kept the old biomarker, and nothing anywhere said why.
            navigate { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            navigate { nav.openRoute(CadenceRoute.TrendDetail("ldl")) }

            assertEquals(listOf("Today", "TrendDetail"), nav.stack())
            onNodeWithText("Экран «Биомаркер · ldl»").assertIsDisplayed()
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
}
