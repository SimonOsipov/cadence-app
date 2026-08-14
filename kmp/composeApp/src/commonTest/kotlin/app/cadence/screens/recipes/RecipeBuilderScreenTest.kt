package app.cadence.screens.recipes

import androidx.compose.foundation.layout.width
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import app.cadence.design.CADENCE_STEPPER_MINUS_TAG
import app.cadence.design.CADENCE_STEPPER_PLUS_TAG
import app.cadence.design.CADENCE_STEPPER_VALUE_TAG
import app.cadence.design.CadenceTheme
import app.cadence.design.splitLegendTag
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.repository.RecipeDraft
import kotlin.math.roundToInt
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

// step-12: `RecipeBuilderScreen`. What each fixture value varies, and why:
//
//   - Two ingredients with different per-100g macros *and* picked at different
//     grams, so a live total that reads only the first row, or reuses one row's
//     grams for both, cannot pass. Both are exercised in the live card and in the
//     saved draft, not only in the draft.
//   - CHICKEN's 165 kcal / 100 g is the number the per-serving assertions are
//     derived from by hand (the saved draft's own numbers come from chicken and
//     rice together): 100 g at 2 servings is 83 (825 tenths, rounded half
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
                CadenceTheme { RecipeBuilderScreen(fakeSearch(), onCancel = { cancelled++ }) }
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
                CadenceTheme { RecipeBuilderScreen(fakeSearch(), onSave = { saved = it }) }
            }
            waitForIdle()

            addIngredient(CHICKEN)

            // Both halves: the button says it is unavailable *and* pressing it writes
            // nothing. A live-looking button that silently does nothing passes the second
            // assertion alone.
            onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).assertIsNotEnabled()
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
                CadenceTheme { RecipeBuilderScreen(fakeSearch(), onSave = { saved = it }) }
            }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_NAME_TAG, useUnmergedTree = true)
                .performTextReplacement("Рис с курицей")
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).assertIsNotEnabled()
            onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).performClick()
            waitForIdle()

            assertNull(saved, "a named draft with no ingredients must not save")
        }

    /** The step's own third named test, both ends of both steppers. */
    @Test
    fun servingsStopAt1And8() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
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
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
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
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            onNodeWithTag(recipeBuilderMealTypeTag(MealType.LUNCH)).assertIsSelected()
            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).assertIsNotSelected()

            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).performClick()
            waitForIdle()

            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).assertIsSelected()
            onNodeWithTag(recipeBuilderMealTypeTag(MealType.LUNCH)).assertIsNotSelected()

            // A meal type has no «none» (unlike a tag), so re-tapping the selected one is
            // a no-op — the mutation this pins is «the type row toggles like the tag row».
            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).performClick()
            waitForIdle()
            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).assertIsSelected()
        }

    /** Tags are a set, not a choice: two stay on together, and re-tapping clears only one. */
    @Test
    fun tagsSelectTogetherAndToggleOffOneAtATime() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
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

    /** The step's own second named test: the empty state is not the only door to the sheet. */
    @Test
    fun theSheetStillOpensOnceTheEmptyStateIsGone() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_EMPTY_TAG).assertExists()
            addIngredient(CHICKEN)
            onNodeWithTag(CADENCE_RECIPE_BUILDER_EMPTY_TAG).assertDoesNotExist()

            openIngredientSheet()

            onNodeWithTag(CADENCE_INGREDIENT_PICKER_TAG).assertExists()
            addIngredient(RICE, alreadyOpen = true)

            onNodeWithTag(recipeBuilderIngredientTag(1), useUnmergedTree = true)
                .assert(hasAnyDescendant(hasText("Рис", substring = true)))
        }

    /** The empty state is itself a door, as the prototype draws it (`:652-671`). */
    @Test
    fun theEmptyStateOpensTheSheet() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            onNodeWithText("Добавьте первый ингредиент").assertExists()
            onNodeWithTag(CADENCE_RECIPE_BUILDER_EMPTY_TAG).performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_INGREDIENT_PICKER_TAG).assertExists()
        }

    @Test
    fun removingARowLeavesItsNeighbourAlone() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            addIngredient(CHICKEN)
            // Rice at 200 g, chicken at 100: a delete that drops the wrong row, or that
            // keeps the name but re-reads row 0's grams, cannot survive both assertions.
            addIngredient(RICE, gramsTaps = 10)

            onNodeWithTag(recipeBuilderIngredientRemoveTag(0)).performClick()
            waitForIdle()

            onNodeWithTag(recipeBuilderIngredientTag(0), useUnmergedTree = true)
                .assert(hasAnyDescendant(hasText("Рис", substring = true)))
            onNodeWithTag(recipeBuilderIngredientTag(1), useUnmergedTree = true).assertDoesNotExist()
            onNodeWithText("Курица").assertDoesNotExist()
            stepperValue(recipeBuilderIngredientTag(0)).assertShows("200")
        }

    /**
     * The mirror of the test above, and the only thing that tells `removeAt(index)` from
     * `removeAt(0)`: deleting row **1** must leave row 0 alone.
     */
    @Test
    fun removingTheSecondRowKeepsTheFirst() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            addIngredient(CHICKEN)
            addIngredient(RICE, gramsTaps = 10)

            onNodeWithTag(recipeBuilderIngredientRemoveTag(1)).performClick()
            waitForIdle()

            onNodeWithTag(recipeBuilderIngredientTag(0), useUnmergedTree = true)
                .assert(hasAnyDescendant(hasText("Курица", substring = true)))
            onNodeWithTag(recipeBuilderIngredientTag(1), useUnmergedTree = true).assertDoesNotExist()
            onNodeWithText("Рис").assertDoesNotExist()
            stepperValue(recipeBuilderIngredientTag(0)).assertShows("100")
        }

    /**
     * The row's own grams stepper must move *its* row. With one row on screen, `onGrams(0, …)`
     * and `onGrams(index, …)` are indistinguishable — so this one steps the second row and
     * checks the first did not follow.
     */
    @Test
    fun aRowsStepperMovesItsOwnRowAndNotItsNeighbours() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            addIngredient(CHICKEN)
            addIngredient(RICE)

            tap(recipeBuilderIngredientTag(1), CADENCE_STEPPER_PLUS_TAG, times = 10)

            stepperValue(recipeBuilderIngredientTag(1)).assertShows("200")
            stepperValue(recipeBuilderIngredientTag(0)).assertShows("100")
        }

    /**
     * The row's «{ккал} ккал · {белок} б» line, which the step names in its own acceptance
     * list and which follows the stepper. Chicken at 100 g is 165/31,0; at 200 g, 330/62,0 —
     * so a line reading `per100g` instead of `macrosFor(grams)` fails the second half.
     */
    @Test
    fun aRowsMacroLineFollowsItsGrams() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            addIngredient(CHICKEN)
            val row = { onNodeWithTag(recipeBuilderIngredientTag(0), useUnmergedTree = true) }

            row().assert(hasAnyDescendant(hasText("165 ккал · 31,0 б")))

            tap(recipeBuilderIngredientTag(0), CADENCE_STEPPER_PLUS_TAG, times = 10)

            row().assert(hasAnyDescendant(hasText("330 ккал · 62,0 б")))
        }

    /**
     * Three numbers, none of which coincide: 100 g of chicken across 2 servings is 83 kcal,
     * across 1 serving 165, and 200 g across 1 serving 330. A card that ignores servings, or
     * ignores grams, or reads the whole-recipe total instead of the per-serving one, lands
     * on the wrong one of these every time.
     */
    @Test
    fun thePerServingCardFollowsBothGramsAndServings() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            addIngredient(CHICKEN)
            perServingCard().assert(hasAnyDescendant(hasText("83")))

            tap(CADENCE_RECIPE_BUILDER_SERVINGS_TAG, CADENCE_STEPPER_MINUS_TAG)
            perServingCard().assert(hasAnyDescendant(hasText("165")))

            tap(recipeBuilderIngredientTag(0), CADENCE_STEPPER_PLUS_TAG, times = 10)
            perServingCard().assert(hasAnyDescendant(hasText("330")))

            // A second row: 330 + 123 kcal. Without it, a card folding only `rows.first()`
            // is indistinguishable from one folding all of them.
            addIngredient(RICE)
            perServingCard().assert(hasAnyDescendant(hasText("453")))

            // Bound to their own legend entries, not merely present somewhere in the card:
            // protein 62,0 + 2,7 -> «65 г», carbs 0 + 25 -> «25 г». Swapping the three
            // macro arguments leaves both strings on screen but on the wrong labels, and
            // dropping the tenths conversion turns 65 into 647.
            legend("protein").assert(hasAnyDescendant(hasText("65 г")))
            legend("carbs").assert(hasAnyDescendant(hasText("25 г")))
        }

    /** The card is not drawn at all until there is something to price (`:771`). */
    @Test
    fun thereIsNoPerServingCardBeforeTheFirstIngredient() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_PER_SERVING_TAG).assertDoesNotExist()
            addIngredient(CHICKEN)
            onNodeWithTag(CADENCE_RECIPE_BUILDER_PER_SERVING_TAG).assertExists()
        }

    @Test
    fun aRowsGramsStepperStopsAt5And600() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            addIngredient(CHICKEN)
            val row = recipeBuilderIngredientTag(0)
            stepperValue(row).assertShows("100")

            tap(row, CADENCE_STEPPER_MINUS_TAG, times = 10)
            stepperValue(row).assertShows("5")
            tap(row, CADENCE_STEPPER_MINUS_TAG)
            stepperValue(row).assertShows("5")

            tap(row, CADENCE_STEPPER_PLUS_TAG, times = 60)
            stepperValue(row).assertShows("600")
            tap(row, CADENCE_STEPPER_PLUS_TAG)
            stepperValue(row).assertShows("600")
        }

    /** The step's own fourth named test. */
    @Test
    fun theLastStepCannotBeRemoved() =
        runComposeUiTest {
            setContent { CadenceTheme { RecipeBuilderScreen(fakeSearch()) } }
            waitForIdle()

            onNodeWithTag(recipeBuilderStepTag(0), useUnmergedTree = true).assertExists()
            onNodeWithTag(recipeBuilderStepRemoveTag(0)).assertDoesNotExist()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_ADD_STEP_TAG).performClick()
            waitForIdle()

            // Distinct texts, so «removed the step asked for» is distinguishable from
            // «removed step 0»: with both blank, either outcome draws the same one field.
            onNodeWithTag(recipeBuilderStepTag(0), useUnmergedTree = true).performTextReplacement("Отварить рис")
            onNodeWithTag(recipeBuilderStepTag(1), useUnmergedTree = true).performTextReplacement("Обжарить курицу")
            waitForIdle()

            onNodeWithTag(recipeBuilderStepRemoveTag(0)).performClick()
            waitForIdle()

            onNodeWithTag(recipeBuilderStepTag(1), useUnmergedTree = true).assertDoesNotExist()
            onNodeWithTag(recipeBuilderStepTag(0), useUnmergedTree = true)
                .assert(hasText("Обжарить курицу"))
            onNodeWithTag(recipeBuilderStepRemoveTag(0)).assertDoesNotExist()
        }

    /**
     * The guard inside the model, not the one on screen: hiding the remove control and
     * refusing the removal are two separate guards, and the test above only ever exercises
     * the first. Dropping `if (steps.size > 1)` from [RecipeBuilderFormState.removeStep]
     * fails here and nowhere else.
     */
    @Test
    fun theFormRefusesToRemoveTheOnlyStep() {
        val form = RecipeBuilderFormState()

        form.removeStep(0)

        // `toList()`: `SnapshotStateList` does not answer `equals` structurally, so comparing
        // it against a plain list fails even when the contents match.
        assertEquals(listOf(""), form.steps.toList())
    }

    /**
     * The positive control for both «must not save» tests above, and the one place every
     * field is read back. Every value is moved off its default, so a draft assembled from
     * defaults — or one that forgot a field — fails here rather than passing by coincidence.
     */
    @Test
    fun theSavedDraftCarriesEveryFieldTheFormCollected() =
        runComposeUiTest {
            var saved: RecipeDraft? = null
            setContent {
                CadenceTheme { RecipeBuilderScreen(fakeSearch(), onSave = { saved = it }) }
            }
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_NAME_TAG, useUnmergedTree = true)
                .performTextReplacement("  Курица с рисом  ")
            onNodeWithTag(recipeBuilderMealTypeTag(MealType.DINNER)).performClick()
            onNodeWithTag(recipeBuilderTagTag(RecipeTag.PROTEIN)).performClick()
            onNodeWithTag(recipeBuilderTagTag(RecipeTag.QUICK)).performClick()
            tap(CADENCE_RECIPE_BUILDER_SERVINGS_TAG, CADENCE_STEPPER_PLUS_TAG)
            tap(CADENCE_RECIPE_BUILDER_TIME_TAG, CADENCE_STEPPER_PLUS_TAG)
            addIngredient(CHICKEN, gramsTaps = 10)
            addIngredient(RICE)

            onNodeWithTag(recipeBuilderStepTag(0), useUnmergedTree = true).performTextReplacement("  Отварить рис  ")
            onNodeWithTag(CADENCE_RECIPE_BUILDER_ADD_STEP_TAG).performClick()
            waitForIdle()
            onNodeWithTag(recipeBuilderStepTag(1), useUnmergedTree = true).performTextReplacement("Обжарить курицу")
            onNodeWithTag(CADENCE_RECIPE_BUILDER_ADD_STEP_TAG).performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).assertIsEnabled()
            onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_TAG).performClick()
            waitForIdle()

            // Chicken 200 g (330 kcal, 62 g protein) + rice 100 g (123 kcal, 2,7 g) over
            // three servings: 151 kcal and 21,6 -> 22 g protein per serving.
            assertEquals(
                RecipeDraft(
                    name = "Курица с рисом",
                    mealType = MealType.DINNER,
                    tags = listOf(RecipeTag.PROTEIN, RecipeTag.QUICK),
                    servings = 3,
                    prepMin = 25,
                    cookMin = null,
                    dek = "22 г белка · 151 ккал на порцию.",
                    ingredients =
                        listOf(
                            RecipeIngredient(CHICKEN.id, 200),
                            RecipeIngredient(RICE.id, 100),
                        ),
                    steps = listOf("Отварить рис", "Обжарить курицу"),
                ),
                saved,
            )
        }

    /**
     * The step-11 squeeze, in the three places this screen repeats the shape: `CadenceStepper`
     * fills the width it is given, and anything narrower than its own content
     * (52 + 20 + number + 20 + 52) flattens the plus button below its tap target. 343dp is
     * an iPhone SE's 375pt width minus its 16dp gutters (`RecipesScreenTest.kt:217`) — and
     * since the screen adds those gutters itself, what is measured here is 32dp narrower
     * than an SE, i.e. stricter than the device.
     */
    @Test
    fun everyPlusButtonStaysA52dpTapTargetAt343dp() =
        runComposeUiTest {
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    RecipeBuilderScreen(fakeSearch(), modifier = Modifier.width(343.dp))
                }
            }
            waitForIdle()
            addIngredient(CHICKEN)

            listOf(
                CADENCE_RECIPE_BUILDER_SERVINGS_TAG,
                CADENCE_RECIPE_BUILDER_TIME_TAG,
                recipeBuilderIngredientTag(0),
            ).forEach { container ->
                val bounds =
                    onNode(
                        hasTestTag(CADENCE_STEPPER_PLUS_TAG) and hasAnyAncestor(hasTestTag(container)),
                        useUnmergedTree = true,
                    ).fetchSemanticsNode().boundsInRoot
                val width = with(density) { bounds.width.toDp() }.value.roundToInt()
                val height = with(density) { bounds.height.toDp() }.value.roundToInt()

                assertEquals(52, width, "$container's plus button is not 52dp wide at 343dp")
                assertEquals(52, height, "$container's plus button is not 52dp tall at 343dp")
            }
        }

    /**
     * The tag row scrolls where the prototype wraps (`RecipeBuilderScreen.tsx:553`). That is
     * only harmless while every chip fits: a chip past the right edge is reachable by an
     * invisible swipe and by nothing else. «Мягкие для желудка» is the widest of the three,
     * so this measures the whole row's content against the screen it is drawn on.
     */
    @Test
    fun everyTagChipIsFullyVisibleAt343dp() =
        runComposeUiTest {
            setContent {
                CadenceTheme { RecipeBuilderScreen(fakeSearch(), modifier = Modifier.width(343.dp)) }
            }
            waitForIdle()

            val rowRight = onNodeWithTag(CADENCE_RECIPE_BUILDER_TAGS_TAG).fetchSemanticsNode().boundsInRoot.right

            RecipeTag.entries.forEach { tag ->
                val chip = onNodeWithTag(recipeBuilderTagTag(tag)).fetchSemanticsNode().boundsInRoot
                assertTrue(
                    chip.right <= rowRight,
                    "«${tag.code}» ends at ${chip.right}, past the row's own $rowRight — it needs a hidden swipe",
                )
            }
        }

    /**
     * The scrolling column ends in a spacer of [SAVE_BAR_CLEARANCE]; if the bar is taller
     * than that, its own last content sits under the bar and cannot be reached. Measured
     * rather than asserted in a comment, because the number was chosen by hand.
     */
    @Test
    fun theSaveBarFitsInsideItsOwnClearance() =
        runComposeUiTest {
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    RecipeBuilderScreen(fakeSearch(), modifier = Modifier.width(343.dp))
                }
            }
            waitForIdle()

            val bar = onNodeWithTag(CADENCE_RECIPE_BUILDER_SAVE_BAR_TAG).fetchSemanticsNode().boundsInRoot
            val barHeight = with(density) { bar.height.toDp() }

            assertTrue(
                barHeight <= SAVE_BAR_CLEARANCE,
                "the save bar is $barHeight tall but the content only clears $SAVE_BAR_CLEARANCE",
            )
        }
}

/**
 * Opens the picker sheet, picks [ingredient] at `100 + 10 × gramsTaps` grams, adds it — and
 * asserts the row actually landed. Without that last check every «must not save» test in
 * this file would pass just as happily against a sheet that added nothing at all.
 */
@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.addIngredient(
    ingredient: Ingredient,
    gramsTaps: Int = 0,
    alreadyOpen: Boolean = false,
) {
    val rowsBefore = ingredientRowCount()
    if (!alreadyOpen) openIngredientSheet()
    onNodeWithTag(ingredientPickerRowTag(ingredient.id), useUnmergedTree = true).performClick()
    waitForIdle()
    // Scoped to the sheet's own footer: the builder underneath keeps its steppers
    // composed while the sheet is open, so the bare tag matches several nodes.
    tap(CADENCE_INGREDIENT_PICKER_FOOTER_TAG, CADENCE_STEPPER_PLUS_TAG, times = gramsTaps)
    onNodeWithTag(CADENCE_INGREDIENT_PICKER_ADD_TAG).performClick()
    waitForIdle()

    assertEquals(
        rowsBefore + 1,
        ingredientRowCount(),
        "«Добавить» did not put ${ingredient.nameRu} into the builder",
    )
}

/** Rows are indexed from zero and drawn in order, so the first missing index is the count. */
@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.ingredientRowCount(): Int {
    var count = 0
    while (onAllNodesWithTag(recipeBuilderIngredientTag(count), useUnmergedTree = true)
            .fetchSemanticsNodes()
            .isNotEmpty()
    ) {
        count++
    }
    return count
}

@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.perServingCard() =
    onNodeWithTag(CADENCE_RECIPE_BUILDER_PER_SERVING_TAG, useUnmergedTree = true)

/** One legend entry of the live card's split bar — its label and its grams, together. */
@OptIn(ExperimentalTestApi::class)
internal fun ComposeUiTest.legend(macro: String) = onNodeWithTag(splitLegendTag(macro), useUnmergedTree = true)

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
