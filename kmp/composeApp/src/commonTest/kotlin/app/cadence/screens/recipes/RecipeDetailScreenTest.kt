package app.cadence.screens.recipes

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_STEPPER_MINUS_TAG
import app.cadence.design.CADENCE_STEPPER_PLUS_TAG
import app.cadence.design.CADENCE_STEPPER_VALUE_TAG
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.MealSource
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.Recipe
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.domain.UserId
import app.cadence.shared.domain.toMealDraft
import kotlin.test.Test
import kotlin.test.assertEquals

// step-10: `RecipeDetailScreen`. Every fixture value's own axis, and why it
// varies (or does not):
//
//   - `BOWL.servings = 2`, not 1. A one-serving fixture recipe makes «На
//     порцию»/«Всё» and the servings stepper indistinguishable from no-ops —
//     both would read the same numbers either way. Two servings also puts
//     the default stepper value (1) a serving away from `BOWL.servings`
//     (2), so `toMealDraft`'s `requestedServings / servings` divisor is
//     never silently 1/1 at the screen's own default state.
//   - Two ingredients (`CHICKEN`, `RICE`), not one, with different grams
//     (300 vs 200) and different per-100g macros. A single-ingredient
//     fixture cannot catch a row that always reads `recipe.ingredients[0]`
//     regardless of `index` — [ingredientRowsShowEachRowsOwnGramsAndKcal]
//     asserts both rows by their own tag.
//   - `CHICKEN` and `RICE`'s macros are chosen so per-serving and whole
//     totals round to visibly different numbers (371 vs 741 kcal) — an
//     equal-looking pair would let «На порцию»/«Всё» pass while actually
//     reading the same `MacrosTenths` for both modes.
//   - `RICE` carries a non-zero `carbsGTenths`/`fatGTenths` unlike `CHICKEN`
//     (whose `carbsGTenths` is 0) — between them every one of `Macros`'s four
//     fields is non-zero somewhere, so `CadenceSplitBar`'s three-way split
//     is never fed an all-zero share.
//   - `BOWL.tags` carries two of the three [RecipeTag] values and `dek` is a
//     real sentence, not a placeholder, so the hero's own tag row and
//     description are exercised with real content rather than an empty list.
//   - `OWNED_BOWL` is `BOWL.copy(ownerId = PATIENT)` — same numbers, only
//     `ownerId` differs — so the «· моё» suffix test cannot pass on some
//     other difference between the two fixtures.
//
// Left deliberately constant: `prepMin`/`cookMin` are always non-null (this
// screen never reaches `totalTimeMin`'s `?: 0` branch, which is already
// covered at the domain level, `RecipeMathTest.kt`'s own
// `totalTimeMinTreatsANullCookMinAsZero`) and `dek` is always present — a null
// `dek` is a direct `?.let` skip with no branching logic worth a second fixture.

private val PATIENT = UserId("patient-1")

private val CHICKEN =
    Ingredient(IngredientId("chicken"), nameRu = "Курица", per100g = MacrosTenths(1650, 310, 0, 36))
private val RICE =
    Ingredient(IngredientId("rice"), nameRu = "Рис", per100g = MacrosTenths(1230, 27, 280, 3))
private val INGREDIENTS = listOf(CHICKEN, RICE)

private val BOWL =
    Recipe(
        id = RecipeId("chicken-bowl"),
        ownerId = null,
        name = "Куриный боул",
        mealType = MealType.LUNCH,
        tags = listOf(RecipeTag.PROTEIN, RecipeTag.QUICK),
        servings = 2,
        prepMin = 10,
        cookMin = 15,
        dek = "Курица, рис и овощи в одной миске.",
        ingredients = listOf(RecipeIngredient(CHICKEN.id, 300), RecipeIngredient(RICE.id, 200)),
        steps = listOf("Нарежьте курицу.", "Отварите рис.", "Соберите боул."),
    )

private val OWNED_BOWL = BOWL.copy(id = RecipeId("own-bowl"), ownerId = PATIENT)

@OptIn(ExperimentalTestApi::class)
class RecipeDetailScreenTest {
    private fun ComposeUiTest.macrosCardHasText(text: String) =
        onNodeWithTag(CADENCE_RECIPE_DETAIL_MACROS_TAG, useUnmergedTree = true)
            .assert(hasAnyDescendant(hasText(text, substring = true)))

    private fun ComposeUiTest.stepperShows(value: String) =
        onNodeWithTag(CADENCE_STEPPER_VALUE_TAG, useUnmergedTree = true)
            .assert(hasAnyDescendant(hasText(value, substring = true)))

    /**
     * The step's own first named test. `BOWL`'s per-serving kcal (371) and
     * whole-recipe kcal (741) are provably different numbers, so a
     * mode-toggle-does-nothing mutant reads 371 in both places. The write
     * half is the point the mutation this test guards against would miss
     * without it: a caller that fed `mode` into `toMealDraft` would still
     * pass a test that only checked the displayed numbers.
     */
    @Test
    fun theModeToggleChangesShownNumbersButNotTheRecordedAmount() =
        runComposeUiTest {
            var added: MealDraft? = null
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(
                        state = RecipeDetailState.Found(BOWL),
                        ingredients = INGREDIENTS,
                        onAddToDay = { added = it },
                    )
                }
            }

            macrosCardHasText("371")
            onNodeWithText("Всё").performClick()
            waitForIdle()
            macrosCardHasText("741")

            onNodeWithTag(CADENCE_RECIPE_DETAIL_ADD_TAG).performClick()
            waitForIdle()

            // requestedServings = 1 — the bottom bar's own default, untouched
            // by the macros toggle above, which never reaches this draft.
            assertEquals(
                BOWL.toMealDraft(INGREDIENTS, requestedServings = 1),
                added,
                "the «Всё» toggle changed what was written, not only what was shown",
            )
        }

    /**
     * The step's own second named test — the deliberate non-improvement.
     * `BOWL.perServing` (371 kcal) and `BOWL.totals` (741 kcal) are the same
     * two numbers [theModeToggleChangesShownNumbersButNotTheRecordedAmount]
     * uses, so a mutant that makes the stepper drive the card would read 741
     * here, not 371.
     */
    @Test
    fun theServingsStepperDoesNotMoveTheMacrosCard() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(state = RecipeDetailState.Found(BOWL), ingredients = INGREDIENTS)
                }
            }

            macrosCardHasText("371")

            onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            stepperShows("2")

            macrosCardHasText("371")
        }

    /**
     * The step's own third named test. Every field below is asserted twice:
     * once against [Recipe.toMealDraft] as the oracle (already proven
     * correct at the domain level, `RecipeMathTest.kt`) — which shows the
     * screen wires the stepper's own value into `requestedServings` and
     * nothing else — and once as a direct doubling relationship on the raw
     * numbers, which is what the spec's own wording asks for and what a bug
     * in *this* test's oracle call could not hide.
     */
    @Test
    fun twoServingsWriteAMealExactlyTwiceOneServingsWithRecipeSourceAndId() =
        runComposeUiTest {
            val added = mutableListOf<MealDraft>()
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(
                        state = RecipeDetailState.Found(BOWL),
                        ingredients = INGREDIENTS,
                        onAddToDay = { added += it },
                    )
                }
            }

            onNodeWithTag(CADENCE_RECIPE_DETAIL_ADD_TAG).performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_ADD_TAG).performClick()
            waitForIdle()

            assertEquals(2, added.size)
            val (oneServing, twoServings) = added

            assertEquals(BOWL.toMealDraft(INGREDIENTS, requestedServings = 1), oneServing)
            assertEquals(BOWL.toMealDraft(INGREDIENTS, requestedServings = 2), twoServings)

            assertEquals(MealSource.RECIPE, oneServing.source)
            assertEquals(BOWL.id, oneServing.recipeId)
            assertEquals(MealSource.RECIPE, twoServings.source)
            assertEquals(BOWL.id, twoServings.recipeId)

            oneServing.items.zip(twoServings.items).forEach { (one, two) ->
                assertEquals(one.grams * 2, two.grams, "${two.name}'s grams did not exactly double")
                assertEquals(
                    one.macros.kcalTenths * 2,
                    two.macros.kcalTenths,
                    "${two.name}'s kcal did not exactly double",
                )
                assertEquals(
                    one.macros.proteinGTenths * 2,
                    two.macros.proteinGTenths,
                    "${two.name}'s protein did not exactly double",
                )
            }
        }

    /**
     * The step's own fourth named test, both ends: a floor-only or
     * ceiling-only stepper would pass half of this.
     */
    @Test
    fun theServingsStepperBoundsHoldAtBothEnds() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(state = RecipeDetailState.Found(BOWL), ingredients = INGREDIENTS)
                }
            }

            stepperShows("1")
            onNodeWithTag(CADENCE_STEPPER_MINUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            stepperShows("1")

            repeat(5) {
                onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
                waitForIdle()
            }
            stepperShows("6")

            onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            stepperShows("6")
        }

    /**
     * The step's own fifth named test. One composition, one state variable
     * swapped in place — the same recomposition-driven-prop-change shape
     * `RecipesScreenTest.kt`'s `theMineSectionAppearsAfterARecipeIsSaved`
     * uses — so a mutant that renders «загружается» and «не найден» as the
     * same content cannot pass both halves: the loading box and its message
     * must vanish exactly when the not-found box and its message appear.
     */
    @Test
    fun loadingIsNotEqualToNotFound() =
        runComposeUiTest {
            var state by mutableStateOf<RecipeDetailState>(RecipeDetailState.Loading)
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(state = state, ingredients = INGREDIENTS)
                }
            }

            onNodeWithTag(CADENCE_RECIPE_DETAIL_LOADING_TAG).assertExists()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_NOT_FOUND_TAG).assertDoesNotExist()
            onNodeWithText("Рецепт не найден.").assertDoesNotExist()
            // Loading draws no exit at all — see the next test for the
            // state that must.
            onNodeWithTag(CADENCE_RECIPE_DETAIL_BACK_TAG).assertDoesNotExist()

            state = RecipeDetailState.NotFound
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_DETAIL_NOT_FOUND_TAG).assertExists()
            onNodeWithText("Рецепт не найден.").assertExists()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_LOADING_TAG).assertDoesNotExist()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_BACK_TAG).assertExists()
        }

    /** The step's own sixth named test. */
    @Test
    fun theFoundStateHasAnExit() =
        runComposeUiTest {
            var backCount = 0
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(
                        state = RecipeDetailState.Found(BOWL),
                        ingredients = INGREDIENTS,
                        onBack = { backCount++ },
                    )
                }
            }

            onNodeWithTag(CADENCE_RECIPE_DETAIL_BACK_TAG).assertExists()
            onNodeWithTag(CADENCE_RECIPE_DETAIL_BACK_TAG).performClick()

            assertEquals(1, backCount, "the floating back button did not report a tap")
        }

    /**
     * Not one of the step's six named tests, but the fixture-trap the file
     * header names: a row that always reads `recipe.ingredients[0]` would
     * still draw *some* grams and kcal on row 1 — an existence check alone
     * cannot see the bug. Both rows are asserted by their own text.
     */
    @Test
    fun ingredientRowsShowEachRowsOwnGramsAndKcal() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(state = RecipeDetailState.Found(BOWL), ingredients = INGREDIENTS)
                }
            }

            val chickenRow = onNodeWithTag(recipeDetailIngredientTag(0), useUnmergedTree = true).performScrollTo()
            chickenRow.assert(hasAnyDescendant(hasText("300 г")))
            chickenRow.assert(hasAnyDescendant(hasText("495 ккал")))

            val riceRow = onNodeWithTag(recipeDetailIngredientTag(1), useUnmergedTree = true).performScrollTo()
            riceRow.assert(hasAnyDescendant(hasText("200 г")))
            riceRow.assert(hasAnyDescendant(hasText("246 ккал")))
        }

    /** The hero's «· моё» suffix, present only for a recipe carrying an `ownerId`. */
    @Test
    fun theHeroAddsTheOwnSuffixOnlyForAnOwnedRecipe() =
        runComposeUiTest {
            var state by mutableStateOf<RecipeDetailState>(RecipeDetailState.Found(BOWL))
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(state = state, ingredients = INGREDIENTS)
                }
            }

            onNodeWithText("МОЁ", substring = true).assertDoesNotExist()

            state = RecipeDetailState.Found(OWNED_BOWL)
            waitForIdle()

            onNodeWithText("МОЁ", substring = true).assertExists()
        }

    /** The macros toggle's own selection state, both directions. */
    @Test
    fun theMacrosToggleReportsWhichModeIsSelected() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipeDetailScreen(state = RecipeDetailState.Found(BOWL), ingredients = INGREDIENTS)
                }
            }

            onNodeWithText("На порцию").assertIsSelected()
            onNodeWithText("Всё").assertIsNotSelected()

            onNodeWithText("Всё").performClick()
            waitForIdle()

            onNodeWithText("Всё").assertIsSelected()
            onNodeWithText("На порцию").assertIsNotSelected()
        }
}
