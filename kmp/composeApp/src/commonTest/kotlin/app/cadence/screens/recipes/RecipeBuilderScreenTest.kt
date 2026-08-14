package app.cadence.screens.recipes

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_STEPPER_MINUS_TAG
import app.cadence.design.CADENCE_STEPPER_PLUS_TAG
import app.cadence.design.CADENCE_STEPPER_VALUE_TAG
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.repository.RecipeDraft
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

// step-12: `RecipeBuilderScreen`. What each fixture value varies, and why:
//
//   - Two ingredients with different per-100g macros *and* picked at different
//     grams, so a live total that reads only the first row, or reuses one row's
//     grams for both, cannot pass.
//   - CHICKEN's 165 kcal / 100 g is the one number every macro assertion is
//     derived from by hand: 100 g at 2 servings is 83 (825 tenths, rounded half
//     up), at 1 serving 165, at 200 g and 1 serving 330. Three numbers no two of
//     which coincide, so neither grams nor servings can drop out of the
//     arithmetic unnoticed.
//   - Every form value the save test asserts is moved off its default —
//     type DINNER (default LUNCH), servings 3 (default 2), time 25 (default 20) —
//     because a draft assembled from defaults would match a screen that never
//     read the form at all.

private val CHICKEN = Ingredient(IngredientId("chicken"), nameRu = "Курица", per100g = MacrosTenths(1650, 310, 0, 36))
private val RICE = Ingredient(IngredientId("rice"), nameRu = "Рис", per100g = MacrosTenths(1230, 27, 250, 10))
private val TABLE = listOf(CHICKEN, RICE)

private fun fakeSearch(ingredients: List<Ingredient> = TABLE): suspend (String) -> List<Ingredient> =
    { query -> ingredients.filter { it.nameRu.contains(query.trim(), ignoreCase = true) } }

@OptIn(ExperimentalTestApi::class)
class RecipeBuilderScreenTest {
    @Test
    fun theCloseButtonReportsACancel() =
        runComposeUiTest {
            var cancelled = 0
            setContent {
                CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch(), onCancel = { cancelled++ }) }
            }

            onNodeWithText("Новый рецепт").assertExists()
            onNodeWithTag(CADENCE_RECIPE_BUILDER_CLOSE_TAG).performClick()
            waitForIdle()

            assertEquals(1, cancelled, "the close button did not report a cancel")
        }

    /** The step's own first named test, one half. */
    @Test
    fun savingIsRefusedWithoutAName() =
        runComposeUiTest {
            var saved: RecipeDraft? = null
            setContent {
                CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch(), onSave = { saved = it }) }
            }
            waitForIdle()

            addIngredient(CHICKEN)

            onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).performClick()
            waitForIdle()

            assertNull(saved, "a draft with an ingredient but no name must not save")
        }

    /** The step's own first named test, the other half — the two cases are checked apart. */
    @Test
    fun savingIsRefusedWithoutAnIngredient() =
        runComposeUiTest {
            var saved: RecipeDraft? = null
            setContent {
                CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch(), onSave = { saved = it }) }
            }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_NAME_TAG, useUnmergedTree = true)
                .performTextReplacement("Рис с курицей")
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).performClick()
            waitForIdle()

            assertNull(saved, "a named draft with no ingredients must not save")
        }

    /** The step's own third named test, both ends of both steppers. */
    @Test
    fun servingsStopAt1And8() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch()) } }
            waitForIdle()

            stepperValue(CADENCE_RECIPE_BUILDER_SERVINGS_TAG).assertShows("2")

            tap(CADENCE_RECIPE_BUILDER_SERVINGS_TAG, CADENCE_STEPPER_MINUS_TAG, times = 2)
            stepperValue(CADENCE_RECIPE_BUILDER_SERVINGS_TAG).assertShows("1")
            tap(CADENCE_RECIPE_BUILDER_SERVINGS_TAG, CADENCE_STEPPER_MINUS_TAG)
            stepperValue(CADENCE_RECIPE_BUILDER_SERVINGS_TAG).assertShows("1")

            tap(CADENCE_RECIPE_BUILDER_SERVINGS_TAG, CADENCE_STEPPER_PLUS_TAG, times = 7)
            stepperValue(CADENCE_RECIPE_BUILDER_SERVINGS_TAG).assertShows("8")
            tap(CADENCE_RECIPE_BUILDER_SERVINGS_TAG, CADENCE_STEPPER_PLUS_TAG)
            stepperValue(CADENCE_RECIPE_BUILDER_SERVINGS_TAG).assertShows("8")
        }

    @Test
    fun timeStepsByFiveAndStopsAt5And120() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch()) } }
            waitForIdle()

            stepperValue(CADENCE_RECIPE_BUILDER_TIME_TAG).assertShows("20")

            // One tap, not the full walk: proves the rate is 5, which a walk to the
            // floor would reach from any rate that divides the distance.
            tap(CADENCE_RECIPE_BUILDER_TIME_TAG, CADENCE_STEPPER_PLUS_TAG)
            stepperValue(CADENCE_RECIPE_BUILDER_TIME_TAG).assertShows("25")

            tap(CADENCE_RECIPE_BUILDER_TIME_TAG, CADENCE_STEPPER_MINUS_TAG, times = 4)
            stepperValue(CADENCE_RECIPE_BUILDER_TIME_TAG).assertShows("5")
            tap(CADENCE_RECIPE_BUILDER_TIME_TAG, CADENCE_STEPPER_MINUS_TAG)
            stepperValue(CADENCE_RECIPE_BUILDER_TIME_TAG).assertShows("5")

            tap(CADENCE_RECIPE_BUILDER_TIME_TAG, CADENCE_STEPPER_PLUS_TAG, times = 23)
            stepperValue(CADENCE_RECIPE_BUILDER_TIME_TAG).assertShows("120")
            tap(CADENCE_RECIPE_BUILDER_TIME_TAG, CADENCE_STEPPER_PLUS_TAG)
            stepperValue(CADENCE_RECIPE_BUILDER_TIME_TAG).assertShows("120")
        }

    @Test
    fun mealTypeDefaultsToLunchAndPicksOneAtATime() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch()) } }
            waitForIdle()

            onNodeWithTag(recipeBuilderMealTypeTag(MealType.LUNCH)).assertIsSelected()
            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).assertIsNotSelected()

            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).performClick()
            waitForIdle()

            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).assertIsSelected()
            onNodeWithTag(recipeBuilderMealTypeTag(MealType.LUNCH)).assertIsNotSelected()
        }

    /** Tags are a set, not a choice: two stay on together, and re-tapping clears only one. */
    @Test
    fun tagsSelectTogetherAndToggleOffOneAtATime() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(TABLE, fakeSearch()) } }
            waitForIdle()

            RecipeTag.entries.forEach { onNodeWithTag(recipeBuilderTagTag(it)).assertIsNotSelected() }

            onNodeWithTag(recipeBuilderTagTag(RecipeTag.PROTEIN)).performClick()
            onNodeWithTag(recipeBuilderTagTag(RecipeTag.QUICK)).performClick()
            waitForIdle()

            onNodeWithTag(recipeBuilderTagTag(RecipeTag.PROTEIN)).assertIsSelected()
            onNodeWithTag(recipeBuilderTagTag(RecipeTag.QUICK)).assertIsSelected()

            onNodeWithTag(recipeBuilderTagTag(RecipeTag.PROTEIN)).performClick()
            waitForIdle()

            onNodeWithTag(recipeBuilderTagTag(RecipeTag.PROTEIN)).assertIsNotSelected()
            onNodeWithTag(recipeBuilderTagTag(RecipeTag.QUICK)).assertIsSelected()
        }
}

/** Opens the picker sheet from whichever control is currently on screen, picks [ingredient], adds it. */
@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.addIngredient(
    ingredient: Ingredient,
    gramsTaps: Int = 0,
) {
    openIngredientSheet()
    onNodeWithTag(ingredientPickerRowTag(ingredient.id), useUnmergedTree = true).performClick()
    waitForIdle()
    // Scoped to the sheet's own footer: the builder underneath keeps its steppers
    // composed while the sheet is open, so the bare tag matches several nodes.
    tap(CADENCE_INGREDIENT_PICKER_FOOTER_TAG, CADENCE_STEPPER_PLUS_TAG, times = gramsTaps)
    onNodeWithTag(CADENCE_INGREDIENT_PICKER_ADD_TAG).performClick()
    waitForIdle()
}

@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.openIngredientSheet() {
    onNodeWithTag(CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG).performClick()
    waitForIdle()
}

@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.tap(
    container: String,
    buttonTag: String,
    times: Int = 1,
) = repeat(times) {
    onNode(hasTestTag(buttonTag) and hasAnyAncestor(hasTestTag(container)), useUnmergedTree = true).performClick()
    waitForIdle()
}

@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.stepperValue(container: String) =
    onNode(
        hasTestTag(CADENCE_STEPPER_VALUE_TAG) and hasAnyAncestor(hasTestTag(container)),
        useUnmergedTree = true,
    )

/**
 * Exact text, never `substring`: «5» is a substring of «25», «15» and «125», so a
 * substring assertion on a stepper passes at four values it should fail at.
 */
internal fun SemanticsNodeInteraction.assertShows(value: String) = assert(hasAnyDescendant(hasText(value)))
