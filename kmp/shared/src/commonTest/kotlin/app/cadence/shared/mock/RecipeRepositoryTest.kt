package app.cadence.shared.mock

import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.repository.RecipeDraft
import app.cadence.shared.repository.RecipeFilter
import app.cadence.shared.repository.RecipeSaveResult
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

class RecipeRepositoryTest {
    @Test
    fun searchingTvoKeepsOnlyCottageCheeseAndDropsEveryOtherIngredient() =
        runTest {
            val cadence = CadenceMocks()

            val found = cadence.recipes.ingredients("тво")

            // The set, not the count: a search that returned any single
            // ingredient would still report size 1. «Творог 5%» is the only
            // one of the 25 whose name contains «тво» — asserted here as the
            // whole set, so a match on the wrong entry cannot pass silently.
            assertEquals(listOf(IngredientId("cottage")), found.map { it.id })
            assertEquals("Творог 5%", found.single().nameRu)
        }

    @Test
    fun searchingWithNoMatchReturnsAnEmptyList() =
        runTest {
            val cadence = CadenceMocks()

            val found = cadence.recipes.ingredients("zzzzнеттакогослова")

            assertEquals(emptyList<String>(), found.map { it.id.raw })
        }

    @Test
    fun searchIsCaseInsensitive() =
        runTest {
            val cadence = CadenceMocks()

            val lower = cadence.recipes.ingredients("тво")
            val upper = cadence.recipes.ingredients("ТВО")
            val mixed = cadence.recipes.ingredients("тВо")

            assertEquals(lower.map { it.id }, upper.map { it.id })
            assertEquals(lower.map { it.id }, mixed.map { it.id })
        }

    @Test
    fun aSavedRecipeComesBackWithANonEmptyOwnerIdAndSortsBeforeTheLibrary() =
        runTest {
            val cadence = CadenceMocks()

            val draft =
                RecipeDraft(
                    name = "Мой домашний боул",
                    mealType = MealType.LUNCH,
                    ingredients = listOf(RecipeIngredient(IngredientId("chicken"), 200)),
                )
            val result = cadence.recipes.save(draft)
            assertIs<RecipeSaveResult.Saved>(result)

            val library = cadence.recipes.library(RecipeFilter())
            val saved = library.recipes.first { it.id == result.id }
            assertTrue(saved.ownerId != null, "a saved recipe must carry a non-empty ownerId")

            // The set, not just membership: every own recipe ahead of every
            // library one, not merely the new recipe somewhere before the
            // library's first entry — a mutation that inserted it second
            // among six library recipes would still satisfy "somewhere before".
            val ownCount = library.recipes.count { it.ownerId != null }
            assertEquals(1, ownCount)
            assertEquals(result.id, library.recipes.first().id, "the own recipe must be the very first entry")
            assertTrue(
                library.recipes.drop(1).all { it.ownerId == null },
                "every entry after the one own recipe must be the library's",
            )
        }

    @Test
    fun savedRecipesStayNewestFirstAmongOwnRecipes() =
        runTest {
            val cadence = CadenceMocks()

            val first = cadence.recipes.save(RecipeDraft(name = "Первый", ingredients = oneIngredient()))
            val second = cadence.recipes.save(RecipeDraft(name = "Второй", ingredients = oneIngredient()))
            assertIs<RecipeSaveResult.Saved>(first)
            assertIs<RecipeSaveResult.Saved>(second)

            val library = cadence.recipes.library(RecipeFilter())
            val ownIds = library.recipes.filter { it.ownerId != null }.map { it.id }

            assertEquals(listOf(second.id, first.id), ownIds, "the most recently saved recipe sorts first")
        }

    @Test
    fun aDraftWithNoNameIsRejectedAndWritesNothing() =
        runTest {
            val cadence = CadenceMocks()
            val before =
                cadence.recipes
                    .library(RecipeFilter())
                    .recipes.size

            val result = cadence.recipes.save(RecipeDraft(name = "  ", ingredients = oneIngredient()))
            assertIs<RecipeSaveResult.Rejected>(result)

            assertEquals(
                before,
                cadence.recipes
                    .library(RecipeFilter())
                    .recipes.size,
            )
        }

    @Test
    fun aDraftWithNoIngredientsIsRejectedAndWritesNothing() =
        runTest {
            val cadence = CadenceMocks()
            val before =
                cadence.recipes
                    .library(RecipeFilter())
                    .recipes.size

            val result = cadence.recipes.save(RecipeDraft(name = "Пустой рецепт", ingredients = emptyList()))
            assertIs<RecipeSaveResult.Rejected>(result)

            assertEquals(
                before,
                cadence.recipes
                    .library(RecipeFilter())
                    .recipes.size,
            )
        }

    @Test
    fun libraryFiltersByMealTypeToExactlyTheMatchingRecipes() =
        runTest {
            val cadence = CadenceMocks()

            val breakfasts = cadence.recipes.library(RecipeFilter(mealType = MealType.BREAKFAST)).recipes

            assertEquals(
                setOf(RecipeId("cottage-bowl"), RecipeId("spinach-omelette")),
                breakfasts.map { it.id }.toSet(),
            )
        }

    @Test
    fun libraryFiltersByTagToExactlyTheMatchingRecipes() =
        runTest {
            val cadence = CadenceMocks()

            val quick = cadence.recipes.library(RecipeFilter(tag = RecipeTag.QUICK)).recipes

            assertEquals(
                setOf(RecipeId("cottage-bowl"), RecipeId("spinach-omelette"), RecipeId("protein-smoothie")),
                quick.map { it.id }.toSet(),
            )
        }

    @Test
    fun libraryFiltersCombineRatherThanOverride() =
        runTest {
            val cadence = CadenceMocks()

            val dinnerAndGentle =
                cadence.recipes.library(RecipeFilter(mealType = MealType.DINNER, tag = RecipeTag.GENTLE)).recipes

            // Both `salmon-quinoa` and `lentil-turkey` are dinners; only
            // `lentil-turkey` also carries `gentle` — a filter that applied
            // only one of the two fields would let the other one through.
            assertEquals(listOf(RecipeId("lentil-turkey")), dinnerAndGentle.map { it.id })
        }

    @Test
    fun recipeLooksUpBothOwnAndLibraryRecipesAndNullsForAnUnknownId() =
        runTest {
            val cadence = CadenceMocks()
            val saved = cadence.recipes.save(RecipeDraft(name = "Моё", ingredients = oneIngredient()))
            assertIs<RecipeSaveResult.Saved>(saved)

            assertEquals("Моё", cadence.recipes.recipe(saved.id)?.name)
            assertEquals("Боул с курицей и тахини", cadence.recipes.recipe(RecipeId("chicken-bowl"))?.name)
            assertNull(cadence.recipes.recipe(RecipeId("no-such-recipe")))
        }

    private fun oneIngredient() = listOf(RecipeIngredient(IngredientId("chicken"), 200))
}
