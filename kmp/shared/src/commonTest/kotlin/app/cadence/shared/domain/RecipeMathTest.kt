package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

// step-8: pure recipe arithmetic, no repository and no MockSeed — every
// fixture below is built by hand so a test's expectation is never derived
// from the same table the code under test also reads.

private fun ingredient(
    id: String,
    kcalTenths: Int,
    proteinGTenths: Int,
    carbsGTenths: Int = 0,
    fatGTenths: Int = 0,
) = Ingredient(
    id = IngredientId(id),
    nameRu = id,
    per100g = MacrosTenths(kcalTenths, proteinGTenths, carbsGTenths, fatGTenths),
)

private fun recipe(
    id: String,
    mealType: MealType = MealType.LUNCH,
    tags: List<RecipeTag> = emptyList(),
    servings: Int = 1,
    ingredients: List<RecipeIngredient> = emptyList(),
) = Recipe(
    id = RecipeId(id),
    ownerId = null,
    name = id,
    mealType = mealType,
    tags = tags,
    servings = servings,
    prepMin = 10,
    cookMin = 10,
    dek = null,
    ingredients = ingredients,
    steps = emptyList(),
)

class RecipeMathTest {
    // ── macrosFor ──────────────────────────────────────────────────────

    @Test
    fun macrosForScalesFromThePer100gRowByExactRatio() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310, fatGTenths = 36)

        val forThreeHundredGrams = chicken.macrosFor(300)

        // 300 g is an exact ×3 of the 100 g row — every field multiplies
        // cleanly, so a correct scale and a broken one cannot agree here.
        assertEquals(MacrosTenths(4950, 930, 0, 108), forThreeHundredGrams)
    }

    @Test
    fun macrosForAtTheReferenceGramsReturnsThePer100gRowUnchanged() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310, fatGTenths = 36)

        assertEquals(chicken.per100g, chicken.macrosFor(100))
    }

    // ── totals / perServing ───────────────────────────────────────────

    @Test
    fun totalsSumsEveryIngredientAtItsOwnGrams() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val rice = ingredient("rice", kcalTenths = 1230, proteinGTenths = 27)
        val bowl =
            recipe(
                "bowl",
                servings = 2,
                ingredients =
                    listOf(
                        RecipeIngredient(chicken.id, 300),
                        RecipeIngredient(rice.id, 200),
                    ),
            )

        val totals = bowl.totals(listOf(chicken, rice))

        // chicken 300g: kcal 4950, protein 930. rice 200g: kcal 2460, protein 54.
        assertEquals(MacrosTenths(7410, 984, 0, 0), totals)
    }

    @Test
    fun perServingDividesTotalsByServingCount() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val bowl =
            recipe(
                "bowl",
                servings = 2,
                ingredients = listOf(RecipeIngredient(chicken.id, 400)),
            )

        val perServing = bowl.perServing(listOf(chicken))

        // Totals at 400g: kcal 6600, protein 1240. Halved: kcal 3300, protein 620.
        assertEquals(MacrosTenths(3300, 620, 0, 0), perServing)
    }

    // ── totalTimeMin ───────────────────────────────────────────────────

    @Test
    fun totalTimeMinAddsPrepAndCook() {
        val bowl = recipe("bowl").copy(prepMin = 15, cookMin = 20)

        assertEquals(35, bowl.totalTimeMin())
    }

    @Test
    fun totalTimeMinTreatsANullCookMinAsZero() {
        val quickBowl = recipe("quick").copy(prepMin = 5, cookMin = null)

        assertEquals(5, quickBowl.totalTimeMin())
    }

    // ── remaining ──────────────────────────────────────────────────────

    @Test
    fun remainingIsTheGapBetweenGoalsAndWhatWasConsumed() {
        val consumed = Macros(kcal = 800, proteinG = 40, carbsG = 0, fatG = 0)
        val goals = Macros(kcal = 2000, proteinG = 150, carbsG = 0, fatG = 0)

        val rem = remaining(consumed, goals)

        assertEquals(Remaining(kcal = 1200, proteinG = 110), rem)
    }

    @Test
    fun remainingIsFlooredAtZeroOnBothFieldsWhenConsumedExceedsGoals() {
        val consumed = Macros(kcal = 3000, proteinG = 200, carbsG = 0, fatG = 0)
        val goals = Macros(kcal = 2000, proteinG = 150, carbsG = 0, fatG = 0)

        val rem = remaining(consumed, goals)

        assertEquals(Remaining(kcal = 0, proteinG = 0), rem)
    }

    // ── pick ───────────────────────────────────────────────────────────
    //
    // Both fixtures share the same goals/consumed, giving a soft kcal
    // ceiling of remaining.kcal (1000) + 250 = 1250 kcal per serving. Every
    // recipe here is one ingredient at exactly 100 g, so its per-serving
    // macros equal the ingredient's per-100g row untouched by rounding —
    // the score difference in each case comes only from the arithmetic
    // under test, never from a rescale artifact.

    private val pickGoals = Macros(kcal = 2000, proteinG = 999, carbsG = 0, fatG = 0)
    private val pickConsumed = Macros(kcal = 1000, proteinG = 0, carbsG = 0, fatG = 0)

    private fun onePortionRecipe(
        id: String,
        kcal: Int,
        proteinG: Int,
    ) = recipe(
        id,
        servings = 1,
        ingredients = listOf(RecipeIngredient(IngredientId(id), 100)),
    ) to ingredient(id, kcalTenths = kcal * 10, proteinGTenths = proteinG * 10)

    @Test
    fun anOverCeilingRecipeLosesWhenTheProteinDifferenceIsUnderThePenalty() {
        // Under ceiling (1000 kcal <= 1250), 100 g protein -> score 1000 (tenths).
        val (under, underIngredient) = onePortionRecipe("under", kcal = 1000, proteinG = 100)
        // Over ceiling (1300 kcal > 1250), 130 g protein -> score 1300 - 400 = 900.
        // Protein difference is 30 g, under the 40 g penalty.
        val (over, overIngredient) = onePortionRecipe("over", kcal = 1300, proteinG = 130)

        val result = pick(listOf(under, over), listOf(underIngredient, overIngredient), pickConsumed, pickGoals)

        assertEquals(RecipeId("under"), result?.recipe?.id, "the under-ceiling recipe must win")
    }

    @Test
    fun anOverCeilingRecipeWinsWhenTheProteinDifferenceExceedsThePenalty() {
        // Same under-ceiling recipe as above: score 1000.
        val (under, underIngredient) = onePortionRecipe("under", kcal = 1000, proteinG = 100)
        // Over ceiling (1300 kcal > 1250), 150 g protein -> score 1500 - 400 = 1100.
        // Protein difference is 50 g, over the 40 g penalty.
        val (over, overIngredient) = onePortionRecipe("over", kcal = 1300, proteinG = 150)

        val result = pick(listOf(under, over), listOf(underIngredient, overIngredient), pickConsumed, pickGoals)

        assertEquals(RecipeId("over"), result?.recipe?.id, "the over-ceiling recipe must win")
    }

    @Test
    fun pickCarriesTheSameRemainingItWasScoredAgainst() {
        val (under, underIngredient) = onePortionRecipe("under", kcal = 1000, proteinG = 100)

        val result = pick(listOf(under), listOf(underIngredient), pickConsumed, pickGoals)

        assertEquals(Remaining(kcal = 1000, proteinG = 999), result?.remaining)
    }

    @Test
    fun pickOfAnEmptyListIsNull() {
        assertNull(pick(emptyList(), emptyList(), pickConsumed, pickGoals))
    }

    // ── filteredByTypeAndTag ─────────────────────────────────────────
    //
    // A hand-built fixture, not MockSeed: every one of MockSeed's six seed
    // recipes carries RecipeTag.PROTEIN, so filtering that seed by PROTEIN
    // is indistinguishable from no filter at all. Here each of the four
    // meal types and three tags separates a different, known subset.

    private val breakfast = recipe("breakfast", mealType = MealType.BREAKFAST, tags = listOf(RecipeTag.PROTEIN))
    private val lunch = recipe("lunch", mealType = MealType.LUNCH, tags = listOf(RecipeTag.GENTLE))
    private val dinner = recipe("dinner", mealType = MealType.DINNER, tags = listOf(RecipeTag.QUICK))
    private val snack =
        recipe("snack", mealType = MealType.SNACK, tags = listOf(RecipeTag.PROTEIN, RecipeTag.GENTLE, RecipeTag.QUICK))
    private val fourRecipes = listOf(breakfast, lunch, dinner, snack)

    @Test
    fun filteringByEveryMealTypeReturnsExactlyThatMealTypesRecipes() {
        val expected =
            mapOf(
                MealType.BREAKFAST to setOf(breakfast.id),
                MealType.LUNCH to setOf(lunch.id),
                MealType.DINNER to setOf(dinner.id),
                MealType.SNACK to setOf(snack.id),
            )
        assertEquals(MealType.entries.toSet(), expected.keys, "the fixture must cover every meal type")

        for ((mealType, ids) in expected) {
            val found = fourRecipes.filteredByTypeAndTag(mealType, tag = null).map { it.id }.toSet()
            assertEquals(ids, found, "filtering by $mealType")
        }
    }

    @Test
    fun filteringByEveryTagReturnsExactlyThatTagsRecipes() {
        val expected =
            mapOf(
                RecipeTag.PROTEIN to setOf(breakfast.id, snack.id),
                RecipeTag.GENTLE to setOf(lunch.id, snack.id),
                RecipeTag.QUICK to setOf(dinner.id, snack.id),
            )
        assertEquals(RecipeTag.entries.toSet(), expected.keys, "the fixture must cover every tag")

        for ((tag, ids) in expected) {
            val found = fourRecipes.filteredByTypeAndTag(mealType = null, tag).map { it.id }.toSet()
            assertEquals(ids, found, "filtering by $tag")
        }
    }

    @Test
    fun noFilterReturnsEveryRecipe() {
        val found = fourRecipes.filteredByTypeAndTag(mealType = null, tag = null).map { it.id }.toSet()

        assertEquals(fourRecipes.map { it.id }.toSet(), found)
    }

    @Test
    fun combiningMealTypeAndTagRequiresBothToMatch() {
        // `lunch` carries GENTLE, not QUICK — an OR-filter would let it
        // through on the meal type alone.
        val found = fourRecipes.filteredByTypeAndTag(MealType.LUNCH, RecipeTag.QUICK)

        assertTrue(found.isEmpty(), "lunch does not carry QUICK, so the AND-filter must drop it")
    }

    // ── toMealDraft ────────────────────────────────────────────────────

    @Test
    fun toMealDraftOnATwoServingRecipeScalesGramsByPortionsOverServings() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val twoServingBowl =
            recipe(
                "chicken-bowl",
                servings = 2,
                ingredients = listOf(RecipeIngredient(chicken.id, 300)),
            )

        // 2 servings requested on a 2-serving recipe -> factor 1 -> grams unchanged.
        val oneRecipeWorth = twoServingBowl.toMealDraft(listOf(chicken), requestedServings = 2)
        // 4 servings requested -> factor 2 -> grams doubled. This is the case a
        // "servings divisor lost" mutant (factor = requestedServings, ignoring
        // recipe.servings) cannot distinguish from ×4: it would also double at
        // requestedServings = 2 already, so this second point is required, not
        // merely confirmatory.
        val doublePortions = twoServingBowl.toMealDraft(listOf(chicken), requestedServings = 4)
        // 1 serving requested -> factor 0.5 -> grams halved.
        val halfPortions = twoServingBowl.toMealDraft(listOf(chicken), requestedServings = 1)

        assertEquals(300, oneRecipeWorth.items.single().grams)
        assertEquals(600, doublePortions.items.single().grams)
        assertEquals(150, halfPortions.items.single().grams)
    }

    @Test
    fun toMealDraftCarriesTheRecipeSourceAndId() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val bowl =
            recipe(
                "chicken-bowl",
                servings = 2,
                ingredients = listOf(RecipeIngredient(chicken.id, 300)),
            )

        val draft = bowl.toMealDraft(listOf(chicken), requestedServings = 2)

        assertEquals(MealSource.RECIPE, draft.source)
        assertEquals(bowl.id, draft.recipeId)
        assertEquals(bowl.name, draft.name)
        assertTrue(draft.canLog())
    }
}
