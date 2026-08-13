package app.cadence.screens.recipes

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.hasAnyDescendant
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.Recipe
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.domain.UserId
import app.cadence.shared.repository.RecipeList
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// step-9: `RecipesScreen`. Every fixture here is hand-built, not
// `MockSeed.recipes` — that seed's own six recipes all carry `RecipeTag.PROTEIN`
// (`MockSeedRecipes.kt:111,138,162,187,211,235`), which makes a «Белковые» tag
// filter indistinguishable from no filter at all against it. This suite needs
// a tag a fixture recipe genuinely lacks, meal types that are not all the
// same, servings that are not all 1, and a «Мои рецепты» count that is not
// always exactly one.
//
// Axes deliberately left constant, and why:
//   - Every recipe carries exactly one ingredient row. Multi-row summation
//     (`Recipe.totals`) is already exercised against multi-ingredient
//     fixtures at the domain level in `RecipeMathTest.kt`; this suite's job
//     is the screen's wiring and display, and the one axis that actually
//     needed a non-1 divisor here — per-serving vs. totals — is reached by
//     `OAT_PORRIDGE`'s `servings = 2` regardless of how many ingredient rows
//     feed it.
//   - `carbsGTenths`/`fatGTenths` are `0` on all four ingredients. Neither
//     field is ever displayed anywhere on this screen (`RecipeRow` shows
//     kcal and protein only), so varying them would assert nothing visible.
//   - `dek` is a one-line template (`"$name — фикстура."`) on every fixture
//     recipe, so `RecipeRow`'s `maxLines = 2` clamp is never exercised by an
//     actual second line. Left as a known gap: `CadenceBody(recipe.dek,
//     maxLines = 2)` is a direct passthrough with no branching logic of its
//     own to mutate, so the risk this leaves uncovered is low, but a fixture
//     with a genuinely two-line description would still be the honest fix.
//   - `GOALS` is one constant across every test. Only its `proteinG` field
//     ever reaches the screen (the remainder line, `pick`'s ranking); the
//     goal-vs-remainder axis is covered by giving `CONSUMED` a non-zero
//     protein value instead (see `CONSUMED`'s own comment) rather than by
//     varying the goal.

private val PATIENT = UserId("patient-1")

private fun ingredient(
    id: String,
    kcalTenths: Int,
    proteinGTenths: Int,
) = Ingredient(IngredientId(id), nameRu = id, per100g = MacrosTenths(kcalTenths, proteinGTenths, 0, 0))

// Chicken is the highest-protein-per-serving ingredient of the four below by
// a wide margin, so `pick` (`RecipeMath.kt`) lands on `CHICKEN_SOLO`
// unambiguously — no fixture recipe comes close enough to make the soft
// kcal-ceiling penalty (`RecipeMath.kt`'s `OVER_CEILING_PENALTY_G`) relevant,
// so this suite is a pure protein-ranking case, not a penalty one — step-8's
// own `RecipeMathTest.kt` already covers the penalty boundary.
private val CHICKEN = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
private val OATS = ingredient("oats", kcalTenths = 3790, proteinGTenths = 130)
private val SALMON = ingredient("salmon", kcalTenths = 2080, proteinGTenths = 200)
private val EGG = ingredient("egg", kcalTenths = 1550, proteinGTenths = 130)

private val INGREDIENTS = listOf(CHICKEN, OATS, SALMON, EGG)

@Suppress("LongParameterList")
private fun recipe(
    id: String,
    name: String,
    mealType: MealType,
    tags: List<RecipeTag>,
    servings: Int,
    ingredientId: String,
    grams: Int,
    ownerId: UserId? = null,
    prepMin: Int = 10,
    cookMin: Int? = 10,
) = Recipe(
    id = RecipeId(id),
    ownerId = ownerId,
    name = name,
    mealType = mealType,
    tags = tags,
    servings = servings,
    prepMin = prepMin,
    cookMin = cookMin,
    dek = "$name — фикстура.",
    ingredients = listOf(RecipeIngredient(IngredientId(ingredientId), grams)),
    steps = listOf("Шаг"),
)

// Meal type varies (BREAKFAST/LUNCH/DINNER/SNACK, not all the same), servings
// vary (2 vs 1, so per-serving is not indistinguishable from totals), tags
// vary (QUICK/PROTEIN/GENTLE — not every recipe carries PROTEIN, unlike
// `MockSeed.recipes`), and «Мои рецепты» holds two, not one.
private val OAT_PORRIDGE =
    recipe(
        "oat-porridge",
        "Овсянка с бананом",
        MealType.BREAKFAST,
        listOf(RecipeTag.QUICK),
        servings = 2,
        ingredientId = "oats",
        grams = 200,
    )
private val CHICKEN_SOLO =
    recipe(
        "chicken-solo",
        "Куриная грудка соло",
        MealType.LUNCH,
        listOf(RecipeTag.PROTEIN),
        servings = 1,
        ingredientId = "chicken",
        grams = 300,
        prepMin = 10,
        cookMin = 15,
    )
private val SALMON_SOLO =
    recipe(
        "salmon-solo",
        "Лосось соло",
        MealType.DINNER,
        listOf(RecipeTag.PROTEIN),
        servings = 1,
        ingredientId = "salmon",
        grams = 200,
    )
private val OWN_OMELETTE =
    recipe(
        "own-omelette",
        "Мой омлет",
        MealType.DINNER,
        listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE),
        servings = 1,
        ingredientId = "egg",
        grams = 150,
        ownerId = PATIENT,
    )
private val OWN_SMOOTHIE =
    recipe(
        "own-smoothie",
        "Мой смузи",
        MealType.SNACK,
        listOf(RecipeTag.QUICK),
        servings = 1,
        ingredientId = "oats",
        grams = 50,
        ownerId = PATIENT,
    )

private val LIBRARY_ONLY = listOf(OAT_PORRIDGE, CHICKEN_SOLO, SALMON_SOLO)
private val FULL_LIBRARY = RecipeList(LIBRARY_ONLY + listOf(OWN_OMELETTE, OWN_SMOOTHIE))

// Large enough that no recipe's per-serving kcal trips the soft ceiling —
// `CHICKEN_SOLO` wins on raw protein alone, not on the penalty.
private val GOALS = Macros(kcal = 1000, proteinG = 200, carbsG = 0, fatG = 0)

// proteinG is 60, not 0: a zero-everywhere CONSUMED makes `remaining` read
// identical to `GOALS` on screen («Осталось 200 г белка» either way), so a
// mutation that fed the card the goal instead of the remainder would have
// shipped green. kcal stays 0 — `remaining.kcal` never reaches the screen
// (only `remaining.proteinG` does; the card's kcal readout is the *picked
// recipe's* per-serving kcal, unrelated to what's been eaten), and changing
// it would only risk moving `pick`'s soft-ceiling boundary for no visible
// assertion.
private val CONSUMED = Macros(kcal = 0, proteinG = 60, carbsG = 0, fatG = 0)

/** See [RecipesScreenTest.assertColorApprox]. */
private const val COLOR_TOLERANCE = 0.02f

/**
 * A row well inside the type pill's `CadenceSpacing.xxs` (4dp) top padding
 * band, in device pixels — see [RecipesScreenTest.topPaddingPixel]. 4dp at a
 * 3x simulator scale is ~12px; 2px stays clear of both the outer rounded
 * edge and the label's own ink.
 */
private const val TOP_PADDING_PROBE_PX = 2

@OptIn(ExperimentalTestApi::class)
class RecipesScreenTest {
    private fun ComposeUiTest.featuredHasText(text: String) =
        onNodeWithTag(CADENCE_RECIPES_FEATURED_TAG, useUnmergedTree = true)
            .assert(hasAnyDescendant(hasText(text, substring = true)))

    /**
     * A same-uniform-fill sample against a rounded [app.cadence.design.CadenceRadius.pill]
     * shape came back a few 8-bit levels off pure equality on this backend
     * (`0.890…` where `0.902…` was painted) even after [waitForIdle] settled
     * any scroll animation — read as renderer dithering on a non-rectangular
     * fill, not a wrong colour, since [CadenceColors.slateBg]/[CadenceColors.clayBg]/
     * the sand/forest tones this row could have been swapped for all differ
     * from each other by tens of channel levels, far outside [COLOR_TOLERANCE].
     */
    private fun assertColorApprox(
        expected: Color,
        actual: Color,
        label: String,
    ) {
        val close =
            abs(actual.red - expected.red) < COLOR_TOLERANCE &&
                abs(actual.green - expected.green) < COLOR_TOLERANCE &&
                abs(actual.blue - expected.blue) < COLOR_TOLERANCE
        assertTrue(close, "$label is not painted $expected — sampled $actual")
    }

    /**
     * A point guaranteed to be pure background fill, never the label's own
     * glyph ink. `RecipeRow`'s type-pill `Box` has no `contentAlignment` and
     * chains `.background(tone.soft, shape).padding(...)` before the label —
     * modifier order means the background paints the *whole* outer box while
     * the text lays out only inside the padded inner box, so any pixel inside
     * the top padding band is background by construction, unlike the box's
     * horizontal or vertical centre, which a long enough label (e.g.
     * «Перекус») fills edge to edge — sampling `[width/2, height/2]` there
     * measured a glyph stroke, not the fill, and read nowhere near either
     * candidate token.
     */
    private fun ImageBitmap.topPaddingPixel(): Color = toPixelMap()[width / 2, TOP_PADDING_PROBE_PX]

    /**
     * The step's own named test. `CHICKEN_SOLO`'s per-serving protein (930
     * tenths) beats every other fixture recipe by a wide margin (`OAT_PORRIDGE`
     * 130, `SALMON_SOLO` 400, `OWN_OMELETTE` 195, `OWN_SMOOTHIE` 65), so `pick`
     * is unambiguous. Applying the «Завтрак» filter removes `CHICKEN_SOLO`
     * (LUNCH) from both list sections entirely — if the card's pick were
     * computed over the *filtered* list instead of [FULL_LIBRARY] whole
     * (`RecipesScreen.tsx:140`'s own rule), it would have nothing left to pick
     * `CHICKEN_SOLO` from and would fall back to `OAT_PORRIDGE`, changing every
     * line asserted below.
     */
    @Test
    fun theFeaturedCardNamesThePickAndIgnoresBothFilters() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipesScreen(library = FULL_LIBRARY, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            featuredHasText("Куриная грудка соло")
            featuredHasText("Осталось 140 г белка сегодня — порция закроет 93 г.")
            featuredHasText("495 ккал · 25 мин")

            // The meal-type filter chip is composed before any row, so it is
            // always the first «Завтрак» node — `CadenceSegmentedTest.kt`'s own
            // `tracks[0]`/`tracks[1]` disambiguates the same way by order.
            onAllNodesWithText("Завтрак").onFirst().performClick()
            waitForIdle()

            featuredHasText("Куриная грудка соло")
            featuredHasText("Осталось 140 г белка сегодня — порция закроет 93 г.")
            featuredHasText("495 ккал · 25 мин")
        }

    /**
     * The step's own second named test. «Быстрые» (QUICK) is the tag row's
     * own label and no row ever draws a tag pill (only the meal-type pill
     * does — `RecipeRow.kt`), so this label is unambiguous without needing
     * `onFirst()`.
     */
    @Test
    fun tappingAnActiveFilterClearsItBackToItsAllOption() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipesScreen(library = FULL_LIBRARY, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            onNodeWithText("Любые").assertIsSelected()
            onNodeWithText("Быстрые").assertIsNotSelected()

            onNodeWithText("Быстрые").performClick()
            waitForIdle()
            onNodeWithText("Быстрые").assertIsSelected()
            onNodeWithText("Любые").assertIsNotSelected()

            onNodeWithText("Быстрые").performClick()
            waitForIdle()
            onNodeWithText("Любые").assertIsSelected()
            onNodeWithText("Быстрые").assertIsNotSelected()
        }

    /**
     * The step's own third named test. Unfiltered, `FULL_LIBRARY` has two own
     * recipes, so the heading reads «Готовые рецепты». Filtering to «Обед»
     * (LUNCH) leaves `CHICKEN_SOLO` as the only match and both own recipes
     * (DINNER, SNACK) out of the filtered `mine` set, switching the heading to
     * «Все рецепты» — `RecipesScreen.tsx:325`'s own `mine.length ? … : …`
     * reads the *filtered* `mine`, not whether the patient has any own recipe
     * at all.
     */
    @Test
    fun theLibraryHeadingSwitchesWhenTheFilteredMineSetIsEmpty() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipesScreen(library = FULL_LIBRARY, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            onNodeWithText("Готовые рецепты".uppercase()).assertExists()
            onNodeWithText("Все рецепты".uppercase()).assertDoesNotExist()

            onAllNodesWithText("Обед").onFirst().performClick()
            waitForIdle()

            onNodeWithText("Все рецепты".uppercase()).assertExists()
            onNodeWithText("Готовые рецепты".uppercase()).assertDoesNotExist()
        }

    /**
     * The step's own fourth named test. [library] is hoisted into a
     * `mutableStateOf` outside the composable, the same way a real screen
     * would receive a fresh [RecipeList] snapshot from the shell after
     * `RecipeRepository.save` (step-13's wiring) — a recomposition-driven
     * prop change, not a second `setContent`.
     */
    @Test
    fun theMineSectionAppearsAfterARecipeIsSaved() =
        runComposeUiTest {
            var library by mutableStateOf(RecipeList(LIBRARY_ONLY))
            setContent {
                CadenceTheme {
                    RecipesScreen(library = library, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            onNodeWithText("Мои рецепты".uppercase()).assertDoesNotExist()

            library = RecipeList(LIBRARY_ONLY + OWN_OMELETTE)
            waitForIdle()

            onNodeWithText("Мои рецепты".uppercase()).assertExists()
            onNodeWithText("Мой омлет").assertExists()
        }

    /**
     * The step's own fifth named test. «Перекус» (SNACK) matches only
     * `OWN_SMOOTHIE`, an owned recipe — no *library* recipe is SNACK — so the
     * library section's own `rest` list empties out while `mine` stays
     * populated, proving the empty message is a property of `rest` alone, not
     * of the whole filtered set.
     */
    @Test
    fun anEmptyLibraryResultDrawsItsOwnMessage() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipesScreen(library = FULL_LIBRARY, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            onNodeWithText("Под фильтр ничего не подошло.").assertDoesNotExist()

            onAllNodesWithText("Перекус").onFirst().performClick()
            waitForIdle()

            onNodeWithText("Под фильтр ничего не подошло.").assertExists()
            // The mine section survives — the message is `rest`'s own, not global.
            onNodeWithText("Мой смузи").assertExists()
        }

    /**
     * Not one of the step's five named tests, but the requirement right next
     * to them: "фильтры независимы и складываются" — additive, not either/or.
     * «Ужин» (DINNER) alone would keep both `SALMON_SOLO` and `OWN_OMELETTE`;
     * «Мягкие для желудка» (GENTLE) alone would keep only `OWN_OMELETTE`
     * anyway (`SALMON_SOLO` never carries GENTLE). Combined, `SALMON_SOLO`
     * must drop out — proving the two axes AND together rather than a mutation
     * that ORs them (which would keep `SALMON_SOLO` on the DINNER match alone)
     * or that drops the meal-type axis silently (same visible symptom).
     */
    @Test
    fun theTwoFiltersCombineRatherThanEitherAloneDeciding() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipesScreen(library = FULL_LIBRARY, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            onAllNodesWithText("Ужин").onFirst().performClick()
            onNodeWithText("Мягкие для желудка").performClick()
            waitForIdle()

            onNodeWithText("Мой омлет").assertExists()
            onNodeWithText("Лосось соло").assertDoesNotExist()
            onNodeWithText("Под фильтр ничего не подошло.").assertExists()
        }

    /**
     * Not a named test, but the row's own «на порцию» promise: mutating
     * `RecipeRow`'s `recipe.perServing(ingredients)` to `recipe.totals(ingredients)`
     * survived the whole suite until this test existed, because nothing
     * asserted a specific row's numbers. `OAT_PORRIDGE` is the one fixture
     * recipe with `servings = 2`, so its totals (758 kcal, 26 g protein) and
     * its per-serving figures (379 kcal, 13 g protein) are provably different
     * numbers — a totals-instead-of-per-serving bug reads 758/26 here, not
     * 379/13.
     */
    @Test
    fun theRowShowsPerServingMacrosNotRecipeTotals() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipesScreen(library = FULL_LIBRARY, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            onNodeWithText("379 ккал · 13 г белка").assertExists()
            onNodeWithText("758 ккал · 26 г белка").assertDoesNotExist()
        }

    /**
     * The four tokens `CadenceColors.kt` added for this step are used at
     * `mealTypeTone` but were otherwise unwitnessed — the same precedent
     * `NutritionScreenTest.kt:303` sets for `sand600`, a newly added token
     * gets a pixel assertion, not just a compiling reference. `SALMON_SOLO`
     * (DINNER) and `OWN_SMOOTHIE` (SNACK) are the fixture's own representatives
     * of the two tones that are not already covered by an existing sand/forest
     * token — swapping the two in `mealTypeTone` would still compile and
     * still paint *a* colour, just the wrong one.
     */
    @Test
    fun theDinnerAndSnackTypePillsPaintTheirOwnTokens() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    RecipesScreen(library = FULL_LIBRARY, ingredients = INGREDIENTS, consumed = CONSUMED, goals = GOALS)
                }
            }

            onNodeWithTag(recipeTypeTag("salmon-solo"), useUnmergedTree = true).performScrollTo()
            waitForIdle()
            val dinnerPill = onNodeWithTag(recipeTypeTag("salmon-solo"), useUnmergedTree = true).captureToImage()
            assertColorApprox(CadenceColors.slateBg, dinnerPill.topPaddingPixel(), "the DINNER pill")

            onNodeWithTag(recipeTypeTag("own-smoothie"), useUnmergedTree = true).performScrollTo()
            waitForIdle()
            val snackPill = onNodeWithTag(recipeTypeTag("own-smoothie"), useUnmergedTree = true).captureToImage()
            assertColorApprox(CadenceColors.clayBg, snackPill.topPaddingPixel(), "the SNACK pill")
        }

    /**
     * `CADENCE_RECIPES_FEATURED_TAG`'s own KDoc says it exists "so a test can
     * press it" — nothing had, until this test. Both the featured card and a
     * row report through the same `onOpen`, and `RecipeRow`'s own `onOpen =
     * { onOpen(recipe.id) }` closure (one per row, built inside a
     * `forEachIndexed`) is exactly the shape that reports the wrong row's id
     * when a loop variable is captured incorrectly — this test presses two
     * different rows/cards and checks two different ids land, not just that
     * *an* id lands.
     */
    @Test
    fun tappingTheFeaturedCardAndARowReportsTheirOwnRecipeId() =
        runComposeUiTest {
            var opened: RecipeId? = null
            setContent {
                CadenceTheme {
                    RecipesScreen(
                        library = FULL_LIBRARY,
                        ingredients = INGREDIENTS,
                        consumed = CONSUMED,
                        goals = GOALS,
                        onOpen = { opened = it },
                    )
                }
            }

            onNodeWithTag(CADENCE_RECIPES_FEATURED_TAG).performScrollTo().performClick()
            waitForIdle()
            assertEquals(RecipeId("chicken-solo"), opened, "the featured card did not report the picked recipe's id")

            opened = null
            onNodeWithText("Мой омлет").performScrollTo().performClick()
            waitForIdle()
            assertEquals(RecipeId("own-omelette"), opened, "the row did not report its own recipe's id")

            // The «Моё» pill exists on this same row — scoped to it rather
            // than a bare `onNodeWithText("Моё")`, which both own recipes'
            // rows satisfy and so is never a single match on this fixture.
            onNodeWithText("Мой омлет").assert(hasText("Моё"))
        }
}
