package app.cadence.shell

import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceTheme
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
import app.cadence.screens.recipes.CADENCE_RECIPE_DETAIL_LOADING_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_DETAIL_NOT_FOUND_TAG
import app.cadence.screens.recipes.CADENCE_RECIPE_DETAIL_TAG
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealId
import app.cadence.shared.domain.NutritionTargets
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.mock.MockSeed
import app.cadence.shared.mock.ingredients
import app.cadence.shared.mock.recipes
import app.cadence.shared.parsing.MockMealParser
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.NutritionDay
import app.cadence.shared.repository.RecipeList
import app.cadence.shared.repository.RecipeSaveResult
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.CompletableDeferred
import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// The nutrition port's wiring, at the level `CadenceApp` composes it — split out of
// `CadenceShellDataTest` when that class outgrew detekt's ceiling. Everything here walks the
// same path a patient does: a tap, a screen, a repository, and what the next screen shows.

private fun wiringMocks() = CadenceMocks(FixedCadenceClock.at("2026-05-31T09:00:00Z"), TimeZone.of("Europe/Moscow"))

/** One product, so a half-read `CadenceShellData` still has something the builder can add. */
private val WIRING_INGREDIENT =
    Ingredient(IngredientId("chicken"), nameRu = "Куриная грудка", per100g = MacrosTenths(1650, 310, 0, 36))

/** A day with nothing in it — the point is that it landed, not what it holds. */
private fun wiringDay() =
    NutritionDay(
        date = LocalDate.parse("2026-05-31"),
        meals = emptyList(),
        totals = Macros(0, 0, 0, 0),
        targets = NutritionTargets(MockSeed.patientId, Macros(1800, 140, 200, 60), waterMl = null),
    )

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
            // From the repository, not a constant, and both halves: the seeded day is 840 kcal
            // against a 1800 target, and passing the eaten total for both would render «840 / 840».
            onNodeWithText("840").assertExists()
            onNodeWithText("/ 1\u00A0800").assertExists()
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
            // And it is *this* recipe in the feed, at the clock's own time — the count alone
            // would pass for any meal, and the name alone for a feed listing the wrong one.
            onNodeWithText("Тёплая чечевица с индейкой").performScrollTo().assertExists()
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

    /**
     * The header chip is the only thing the wizard does with the clock reading this step
     * made public, and nothing observed it: `now` replaced by any literal, or read in `UTC`
     * instead of the patient's zone, left the whole suite green while stamping the meal three
     * hours off. 09:00Z in Europe/Moscow is 12:00.
     */
    @Test
    fun theWizardStampsTheClockReadingItWasGivenInThePatientsZone() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = wiringMocks()) }
            }
            waitForIdle()

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            waitForIdle()

            onNodeWithText("12:00 · Вс 31 мая").assertExists()
        }

    /** The wizard's own write, seen where the patient sees it — the toast is not the feed. */
    @Test
    fun aMealLoggedInTheWizardJoinsTheDaysFeed() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = wiringMocks()) }
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
            // The confirmation overlay swallows every touch while it is up
            // (`ConfirmToastTest.theToastSwallowsEveryTouchWhileItIsUp`), so the tab tap has to
            // wait it out rather than land on the scrim.
            mainClock.advanceTimeBy(CADENCE_CONFIRM_TOAST_MS + 100L)
            waitForIdle()

            nutritionTab().performClick()
            waitForIdle()

            // Named and counted: the count alone would pass for any meal at all, and the name
            // alone would pass for a feed that lists what was typed rather than what was written.
            onNodeWithText("3 приёма", substring = true).assertExists()
            onNodeWithText("12:00", substring = true).performScrollTo().assertExists()
        }

    /** A rejected save keeps the form, exactly as a rejected meal keeps the wizard. */
    @Test
    fun aRejectedRecipeLeavesTheBuilderOpen() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme {
                    CadenceShell(
                        navController = nav,
                        actions =
                            CadenceShellActions(
                                searchIngredients = { listOf(WIRING_INGREDIENT) },
                                onRecipeSaved = { RecipeSaveResult.Rejected },
                            ),
                    )
                }
            }
            waitForIdle()

            runOnUiThread { nav.openRoute(CadenceRoute.RecipeBuilder) }
            waitForIdle()
            buildARecipe("Курица на пару")

            assertTrue(
                nav.currentBackStack.value.any { it.destination.hasRoute<CadenceRoute.RecipeBuilder>() },
                "a rejected save dismissed the builder as if the recipe had been written",
            )
        }

    /**
     * The card's own three states, at the level that chooses between them. «Ещё не ответили»
     * is a suspended read, not an answer of null — a route that collapses the two tells a
     * patient «Рецепт не найден.» while the read is still in flight.
     */
    @Test
    fun theCardSaysLoadingUntilTheReadAnswers() =
        runComposeUiTest {
            val gate = CompletableDeferred<Unit>()
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme {
                    CadenceShell(
                        navController = nav,
                        // The table is supplied, so what this test measures is the *recipe*
                        // read alone: the card waits on both, and an unread table would keep
                        // it «загружается» whatever `loadRecipe` answered.
                        data = CadenceShellData(ingredients = MockSeed.ingredients),
                        actions =
                            CadenceShellActions(
                                loadRecipe = {
                                    gate.await()
                                    null
                                },
                            ),
                    )
                }
            }
            waitForIdle()

            runOnUiThread { nav.openRoute(CadenceRoute.RecipeDetail("whatever")) }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_LOADING_TAG).assertExists()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_NOT_FOUND_TAG).assertDoesNotExist()

            gate.complete(Unit)
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_NOT_FOUND_TAG).assertExists()
        }

    /** The second clause of each gate: one read landing does not make a screen composable. */
    @Test
    fun eachRouteWaitsForEveryReadItDrawsFrom() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            val day = wiringDay()
            setContent {
                nav = rememberNavController()
                CadenceTheme {
                    CadenceShell(
                        navController = nav,
                        // Deliberately half-read: the day without its week, and the library
                        // without the table that prices it.
                        data = CadenceShellData(nutritionDay = day, recipes = RecipeList(emptyList())),
                    )
                }
            }
            waitForIdle()

            runOnUiThread { nav.selectDestination(CadenceDestination.NUTRITION) }
            waitForIdle()
            onNodeWithTag(CADENCE_NUTRITION_TAG).assertDoesNotExist()

            runOnUiThread { nav.openRoute(CadenceRoute.Recipes) }
            waitForIdle()
            onNodeWithTag(CADENCE_RECIPES_TAG).assertDoesNotExist()
        }

    /**
     * The wizard's own second clause, isolated: the day *has* landed here, so what the
     * assertion turns on is the clock reading alone. Folded into the test above it would
     * have passed on `summary == null` and proved nothing about `now`.
     */
    @Test
    fun theWizardWaitsForTheClockReadingAndNotOnlyTheDay() =
        runComposeUiTest {
            val mocks = wiringMocks()
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                var summary by remember { mutableStateOf<TodaySummary?>(null) }
                LaunchedEffect(Unit) { summary = mocks.today.today() }
                CadenceTheme {
                    CadenceShell(navController = nav, data = CadenceShellData(summary = summary))
                }
            }
            waitForIdle()

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            waitForIdle()

            onNodeWithTag(LOG_MEAL_CHAT_FIELD_TAG, useUnmergedTree = true).assertDoesNotExist()
        }

    /**
     * The write is held open, so the second tap lands while the first is still in flight —
     * which is what the guard is for, and what «two taps in one frame» was the wrong way to
     * ask for. Without it both taps queue their own coroutine and the day gets the recipe
     * twice, each with its own toast.
     */
    @Test
    fun asecondTapWhileTheWriteIsInFlightDoesNotWriteAgain() =
        runComposeUiTest {
            val gate = CompletableDeferred<Unit>()
            var writes = 0
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme {
                    CadenceShell(
                        navController = nav,
                        data = CadenceShellData(ingredients = MockSeed.ingredients),
                        actions =
                            CadenceShellActions(
                                loadRecipe = { MockSeed.recipes.first() },
                                onMealLogged = {
                                    writes++
                                    gate.await()
                                    MealLogResult.Written(MealId("written"), Macros(0, 0, 0, 0))
                                },
                            ),
                    )
                }
            }
            waitForIdle()

            runOnUiThread {
                nav.openRoute(
                    CadenceRoute.RecipeDetail(
                        MockSeed.recipes
                            .first()
                            .id.raw,
                    ),
                )
            }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_ADD_TAG).performClick()
            waitForIdle()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_ADD_TAG).performClick()
            waitForIdle()

            assertEquals(1, writes, "the second tap queued another write while the first was in flight")

            gate.complete(Unit)
            waitForIdle()
        }

    /**
     * The card's *other* read. A recipe with no price table is not a card with blank
     * numbers: `RecipeMath.totalsOf` throws on the first row it cannot find, so the answer
     * has to stay «загружается» until the table lands — the same gate «Рецепты» keeps, on
     * the screen that actually prices every row.
     */
    @Test
    fun theCardWaitsForThePriceTableEvenOnceItsRecipeHasAnswered() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme {
                    CadenceShell(
                        navController = nav,
                        // The recipe answers immediately; the table has not been read at all.
                        actions = CadenceShellActions(loadRecipe = { MockSeed.recipes.first() }),
                    )
                }
            }
            waitForIdle()

            runOnUiThread {
                nav.openRoute(
                    CadenceRoute.RecipeDetail(
                        MockSeed.recipes
                            .first()
                            .id.raw,
                    ),
                )
            }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_LOADING_TAG).assertExists()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_TAG).assertDoesNotExist()
        }
}
