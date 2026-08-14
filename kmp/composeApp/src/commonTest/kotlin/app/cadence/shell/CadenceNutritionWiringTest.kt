package app.cadence.shell

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import app.cadence.design.CadenceTheme
import app.cadence.screens.inventory.ADD_VIAL_SAVE_TAG
import app.cadence.screens.inventory.addVialFieldTag
import app.cadence.screens.nutrition.CADENCE_NUTRITION_RECIPES_LINK_TAG
import app.cadence.screens.nutrition.CADENCE_NUTRITION_TAG
import app.cadence.screens.nutrition.LOG_MEAL_CHAT_FIELD_TAG
import app.cadence.screens.nutrition.LOG_MEAL_SAVE_TAG
import app.cadence.screens.recipes.CADENCE_INGREDIENT_PICKER_ADD_TAG
import app.cadence.screens.recipes.CADENCE_RECIPES_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_BUILDER_NAME_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_BUILDER_SAVE_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_DETAIL_ADD_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_DETAIL_BACK_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_DETAIL_TAG
import app.cadence.screens.trends.CADENCE_TRENDS_HERO_TAG
import app.cadence.screens.trends.CADENCE_TREND_DETAIL_STATS_TAG
import app.cadence.screens.trends.cadenceTrendCardTag
import app.cadence.screens.trends.cadenceTrendWindowTag
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.ProtocolCadence
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.ProtocolRow
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.mock.MockSeed
import app.cadence.shared.parsing.MockMealParser
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

// The nutrition port's wiring, at the level `CadenceApp` composes it — split out of
// `CadenceShellDataTest` when that class outgrew detekt's ceiling. Everything here walks the
// same path a patient does: a tap, a screen, a repository, and what the next screen shows.

private fun wiringMocks() = CadenceMocks(FixedCadenceClock.at("2026-05-31T09:00:00Z"), TimeZone.of("Europe/Moscow"))

@OptIn(ExperimentalTestApi::class)
class CadenceNutritionWiringTest {
    /**
     * The tab, not the word: «Питание» is also the eyebrow of Today's own meals card, so a
     * text query matches two nodes and clicks neither.
     */
    private fun ComposeUiTest.nutritionTab() = onNodeWithContentDescription("Питание")

    /**
     * The write's own answer decides whether the wizard closes. Unreachable through the mock,
     * whose parse always names a meal — so the case is driven by handing the shell an action
     * that rejects, which is what M9's server will do when a draft doesn't survive the round
     * trip. Without this, «close on any answer» passes every other test in the suite.
     */
    @Test
    fun aRejectedMealLeavesTheWizardOpenWithWhatWasTyped() =
        runComposeUiTest {
            val mocks = wiringMocks()
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                var summary by remember { mutableStateOf<TodaySummary?>(null) }
                LaunchedEffect(Unit) { summary = mocks.today.today() }
                CadenceTheme {
                    CadenceShell(
                        navController = nav,
                        data = CadenceShellData(summary = summary, now = mocks.nowLocal()),
                        actions =
                            CadenceShellActions(
                                parseMeal = { text -> MockMealParser().parse(text) },
                                onMealLogged = { MealLogResult.Rejected },
                            ),
                    )
                }
            }
            waitForIdle()

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            waitForIdle()
            onNodeWithTag(LOG_MEAL_CHAT_FIELD_TAG, useUnmergedTree = true).performTextReplacement("курица с рисом")
            waitForIdle()
            onNodeWithText("Разобрать →").performClick()
            waitForIdle()
            onNodeWithTag(LOG_MEAL_SAVE_TAG).performScrollTo().performClick()
            waitForIdle()

            assertTrue(
                nav.currentBackStack.value.any { it.destination.hasRoute<CadenceRoute.LogMeal>() },
                "a rejected draft dismissed the wizard as if it had been recorded",
            )
            onNodeWithText("Куриная грудка", substring = true).assertExists()
        }

    @Test
    fun theNutritionTabDrawsItsOwnScreenAndKeepsTheTabBar() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = wiringMocks()) } }

            nutritionTab().performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_NUTRITION_TAG).assertExists()
            // The tab bar is the reason this is a tab and not a pushed route: the placeholder
            // drew one too, so the screen alone would not tell the two apart.
            onNodeWithContentDescription("Аптечка").assertExists()
            // From the repository, not a constant: the seeded day is 840 kcal against 1800.
            onNodeWithText("840", substring = true).assertExists()
        }

    @Test
    fun theRecipesRouteOpensFromNutritionAndListsTheLibrary() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = wiringMocks()) }
            }

            nutritionTab().performClick()
            waitForIdle()
            onNodeWithTag(CADENCE_NUTRITION_RECIPES_LINK_TAG).performScrollTo().performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPES_TAG).assertExists()
            assertTrue(
                nav.currentBackStack.value.any { it.destination.hasRoute<CadenceRoute.Recipes>() },
                "the card did not push the recipe library",
            )
            // A seeded recipe by name, so «the screen rendered» cannot pass on an empty library.
            onNodeWithText("Лосось с киноа и брокколи").performScrollTo().assertExists()
        }

    @Test
    fun openingARecipeShowsThatRecipesOwnCard() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = wiringMocks()) } }

            nutritionTab().performClick()
            waitForIdle()
            onNodeWithTag(CADENCE_NUTRITION_RECIPES_LINK_TAG).performScrollTo().performClick()
            waitForIdle()
            // Not the first row and not the featured card: those two would also be reached by a
            // detail route that ignores the id it was given.
            onNodeWithText("Тёплая чечевица с индейкой").performScrollTo().performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_TAG).assertExists()
            onNodeWithText("Тёплая чечевица с индейкой").assertExists()
        }

    /**
     * Step-12's own named test, deferred to this step because until now the builder's route
     * was a placeholder: the saved recipe opens straight away, and **back leaves for the
     * library, not for the form that made it**.
     */
    @Test
    fun savingARecipeOpensItsCardAndBackReturnsToTheLibrary() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = wiringMocks()) }
            }

            openRecipes()
            onNodeWithText("Создать").performClick()
            waitForIdle()
            buildARecipe("Курица на пару")

            onNodeWithTag(CADENCE_RECIPE_DETAIL_TAG).assertExists()
            onNodeWithText("Курица на пару").assertExists()
            assertTrue(
                nav.currentBackStack.value.none { it.destination.hasRoute<CadenceRoute.RecipeBuilder>() },
                "the builder is still on the stack behind its own result",
            )

            onNodeWithTag(CADENCE_RECIPE_DETAIL_BACK_TAG).performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPES_TAG).assertExists()
        }

    /** The `reloads` bump: without it the library the screen holds is the one read before the save. */
    @Test
    fun aSavedRecipeJoinsMyRecipes() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = wiringMocks()) }
            }

            openRecipes()
            // `ignoreCase`: `CadenceEyebrow` upper-cases what it is given, so a plain query
            // matches nothing — including the "absent" assertion, which would have passed
            // against a section that was there all along.
            myRecipesHeading().assertDoesNotExist()

            onNodeWithText("Создать").performClick()
            waitForIdle()
            buildARecipe("Курица на пару")
            onNodeWithTag(CADENCE_RECIPE_DETAIL_BACK_TAG).performClick()
            waitForIdle()

            myRecipesHeading().performScrollTo().assertExists()
            onNodeWithText("Курица на пару").performScrollTo().assertExists()
        }

    /**
     * The other write this step wires: a recipe added to the day is a logged meal, confirmed
     * by the same toast and landing on «Питание» — where the patient can see it.
     */
    @Test
    fun addingARecipeToTheDayLogsItAndLandsOnNutrition() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = wiringMocks()) } }

            openRecipes()
            onNodeWithText("Тёплая чечевица с индейкой").performScrollTo().performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_ADD_TAG).performClick()
            waitForIdle()

            onNodeWithText("Тёплая чечевица с индейкой · записано").assertExists()
            onNodeWithTag(CADENCE_NUTRITION_TAG).assertExists()
            // Three meals now, where the seed has two: the write reached the same store «Питание» reads.
            onNodeWithText("3 приёма", substring = true).assertExists()
        }

    private fun ComposeUiTest.myRecipesHeading() = onNodeWithText("Мои рецепты", ignoreCase = true)

    private fun ComposeUiTest.openRecipes() {
        nutritionTab().performClick()
        waitForIdle()
        onNodeWithTag(CADENCE_NUTRITION_RECIPES_LINK_TAG).performScrollTo().performClick()
        waitForIdle()
    }

    /** Names the recipe, picks the one ingredient the save gate requires, and saves. */
    private fun ComposeUiTest.buildARecipe(name: String) {
        onNodeWithTag(CADENCE_RECIPE_BUILDER_NAME_TAG, useUnmergedTree = true).performTextReplacement(name)
        waitForIdle()
        onNodeWithTag(CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG).performScrollTo().performClick()
        waitForIdle()
        onNodeWithText("Куриная грудка").performClick()
        waitForIdle()
        onNodeWithTag(CADENCE_INGREDIENT_PICKER_ADD_TAG).performClick()
        waitForIdle()
        onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).performClick()
        waitForIdle()
    }
}
