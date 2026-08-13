package app.cadence.screens.recipes

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
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_STEPPER_MINUS_TAG
import app.cadence.design.CADENCE_STEPPER_PLUS_TAG
import app.cadence.design.CADENCE_STEPPER_VALUE_TAG
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.RecipeIngredient
import kotlin.test.Test
import kotlin.test.assertEquals

// step-11: `IngredientPickerSheet`. Every fixture value's own axis, and why it varies
// (or does not):
//
//   - Three ingredients, not one, with three different kcal/protein pairs
//     (165/31, 121/17, 123/2.7) — a fixture where every row carried the same
//     macros would let [eachRowShowsItsOwnKcalAndProteinPer100g] pass even if
//     every row secretly read `fixtures[0]`.
//   - Two of the three are selectable across tests specifically to prove the
//     grams reset is conditioned on *which* product was picked, not merely
//     "a pick happened" — a fixture with only one selectable product cannot
//     tell "resets on every pick" from "resets only on a different pick"
//     apart, which is exactly the mutation the spec names.
//   - Grams moves to 200 in the reset/recalculation tests, never left at the
//     default 100: 100 is both `DEFAULT_GRAMS` and a stepper value the sheet
//     could reach by coincidence, so a test that only ever observes 100
//     cannot tell "actually reset" from "never moved". 200 is also two whole
//     multiples of `CHICKEN`'s per-100g macros (330 kcal, 62 g protein) with
//     no rounding ambiguity, so the live-recalculation assertion checks exact
//     numbers rather than "some number changed".
//   - The no-match search query is nonsense text against a *non-empty*
//     three-ingredient reference set, not a search against an empty
//     repository — the latter would make "Ничего не найдено." indistinguishable
//     from a picker wired to no data at all.

private val CHICKEN = Ingredient(IngredientId("chicken"), nameRu = "Курица", per100g = MacrosTenths(1650, 310, 0, 36))
private val COTTAGE =
    Ingredient(IngredientId("cottage"), nameRu = "Творог 5%", per100g = MacrosTenths(1210, 170, 30, 50))
private val RICE = Ingredient(IngredientId("rice"), nameRu = "Рис", per100g = MacrosTenths(1230, 27, 250, 10))
private val FIXTURE_INGREDIENTS = listOf(CHICKEN, COTTAGE, RICE)

/**
 * Mirrors `RecipeRepository.ingredients` (step-7) closely enough to exercise this
 * screen's wiring — trim, case-insensitive `contains` — without the screen itself
 * ever re-implementing that match. The screen's own job is only to call [search]
 * with whatever the field holds.
 */
private fun fakeSearch(ingredients: List<Ingredient> = FIXTURE_INGREDIENTS): suspend (String) -> List<Ingredient> =
    { query ->
        val trimmed = query.trim()
        ingredients.filter { it.nameRu.contains(trimmed, ignoreCase = true) }
    }

@OptIn(ExperimentalTestApi::class)
class IngredientPickerSheetTest {
    @Test
    fun rendersHeaderPlaceholderAndCallsOnCloseFromTheXButton() =
        runComposeUiTest {
            var closed = 0
            setContent {
                CadenceTheme {
                    IngredientPickerSheet(search = fakeSearch(), onClose = { closed++ })
                }
            }

            onNodeWithText("Ингредиент").assertExists()
            onNodeWithText("Найти продукт").assertExists()

            onNodeWithTag(CADENCE_INGREDIENT_PICKER_CLOSE_TAG).performClick()
            waitForIdle()

            assertEquals(1, closed, "the header's close button did not report a tap")
        }

    /** The step's own second named test. */
    @Test
    fun noFooterBeforeASelection() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            // Not vacuous: the initial (empty) query already matches all three
            // fixture ingredients, so rows exist and nothing is selected yet.
            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).assertExists()
            onNodeWithTag(CADENCE_INGREDIENT_PICKER_FOOTER_TAG).assertDoesNotExist()
        }

    /** The step's own first named test. */
    @Test
    fun searchWithNoMatchDrawsNothingFound() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            onNodeWithTag(CADENCE_INGREDIENT_PICKER_SEARCH_TAG, useUnmergedTree = true)
                .performTextReplacement("тыквенныйзефиркоторогонет")
            waitForIdle()

            onNodeWithText("Ничего не найдено.").assertExists()
            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).assertDoesNotExist()
            onNodeWithTag(ingredientPickerRowTag(COTTAGE.id), useUnmergedTree = true).assertDoesNotExist()
            onNodeWithTag(ingredientPickerRowTag(RICE.id), useUnmergedTree = true).assertDoesNotExist()
        }

    /** Guards the fixture trap this file's header names: a row always reading `results[0]`. */
    @Test
    fun eachRowShowsItsOwnKcalAndProteinPer100g() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            val chickenRow = onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true)
            chickenRow.assert(hasAnyDescendant(hasText("165 ккал", substring = true)))
            chickenRow.assert(hasAnyDescendant(hasText("31 г белка", substring = true)))

            val cottageRow = onNodeWithTag(ingredientPickerRowTag(COTTAGE.id), useUnmergedTree = true)
            cottageRow.assert(hasAnyDescendant(hasText("121 ккал", substring = true)))
            cottageRow.assert(hasAnyDescendant(hasText("17 г белка", substring = true)))
        }

    @Test
    fun theSelectedRowIsHighlightedAndOthersAreNot() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).assertIsNotSelected()
            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).performClick()
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).assertIsSelected()
            onNodeWithTag(ingredientPickerRowTag(COTTAGE.id), useUnmergedTree = true).assertIsNotSelected()
        }

    /** The step's own third named test, against the mutation «grams carry over from the previous pick». */
    @Test
    fun pickingASecondProductReturnsGramsTo100() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).performClick()
            waitForIdle()
            stepperShows("100")

            repeat(3) {
                onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
                waitForIdle()
            }
            stepperShows("130")

            onNodeWithTag(ingredientPickerRowTag(COTTAGE.id), useUnmergedTree = true).performClick()
            waitForIdle()

            stepperShows("100")
        }

    /**
     * The deliberate non-improvement: re-tapping the *same* already-selected product
     * must not reset grams — otherwise [pickingASecondProductReturnsGramsTo100] alone
     * cannot tell "resets on a different pick" from "resets on every click".
     */
    @Test
    fun reselectingTheSameProductDoesNotResetGrams() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).performClick()
            waitForIdle()
            repeat(3) {
                onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
                waitForIdle()
            }
            stepperShows("130")

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).performClick()
            waitForIdle()

            stepperShows("130")
        }

    @Test
    fun footerRecalculatesLiveAsGramsChange() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).performClick()
            waitForIdle()

            footerHasText("165 ккал")
            footerHasText("31 г белка")

            // 100 g -> 200 g, ten +10 clicks — an exact double with no rounding.
            repeat(10) {
                onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
                waitForIdle()
            }
            stepperShows("200")

            footerHasText("330 ккал")
            footerHasText("62 г белка")
        }

    /** The step's own fourth named test, both ends. */
    @Test
    fun gramsStepperBoundsHoldAt5And600() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).performClick()
            waitForIdle()

            repeat(10) {
                onNodeWithTag(CADENCE_STEPPER_MINUS_TAG, useUnmergedTree = true).performClick()
                waitForIdle()
            }
            stepperShows("5")
            onNodeWithTag(CADENCE_STEPPER_MINUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            stepperShows("5")

            repeat(60) {
                onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
                waitForIdle()
            }
            stepperShows("600")
            onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            stepperShows("600")
        }

    @Test
    fun addingWritesTheChosenIngredientAtItsGramsAndClosesTheSheet() =
        runComposeUiTest {
            var added: RecipeIngredient? = null
            var closed = 0
            setContent {
                CadenceTheme {
                    IngredientPickerSheet(
                        search = fakeSearch(),
                        onAdd = { added = it },
                        onClose = { closed++ },
                    )
                }
            }
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(CHICKEN.id), useUnmergedTree = true).performClick()
            waitForIdle()
            repeat(10) {
                onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
                waitForIdle()
            }
            stepperShows("200")

            onNodeWithTag(CADENCE_INGREDIENT_PICKER_ADD_TAG).performClick()
            waitForIdle()

            assertEquals(RecipeIngredient(CHICKEN.id, 200), added)
            assertEquals(1, closed, "«Добавить» must close the sheet, not merely add the item")
        }

    /** The footer names whichever product is selected, not always the first row's. */
    @Test
    fun theFooterNamesTheProductThatWasActuallySelected() =
        runComposeUiTest {
            setContent {
                CadenceTheme { IngredientPickerSheet(search = fakeSearch()) }
            }
            waitForIdle()

            onNodeWithTag(ingredientPickerRowTag(RICE.id), useUnmergedTree = true).performClick()
            waitForIdle()

            footerHasText("Рис")
        }

    private fun ComposeUiTest.stepperShows(value: String) =
        onNodeWithTag(CADENCE_STEPPER_VALUE_TAG, useUnmergedTree = true)
            .assert(hasAnyDescendant(hasText(value, substring = true)))

    private fun ComposeUiTest.footerHasText(text: String) =
        onNodeWithTag(CADENCE_INGREDIENT_PICKER_FOOTER_TAG, useUnmergedTree = true)
            .assert(hasAnyDescendant(hasText(text, substring = true)))
}
