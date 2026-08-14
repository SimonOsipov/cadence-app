package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

// Pure arithmetic, no repository and no MockSeed — every fixture is hand-built so a test's
// expectation is never derived from the same table the code under test reads.

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

        // chicken 300g (kcal 4950, protein 930) + rice 200g (kcal 2460, protein 54).
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

        // Totals at 400g (kcal 6600, protein 1240), halved.
        assertEquals(MacrosTenths(3300, 620, 0, 0), perServing)
    }

    @Test
    fun perServingRoundsHalfUpAtATenthsTie() {
        // Every field here is odd, so dividing by 2 lands on an exact .5 — the shape a plain
        // integer divide (1651/2 == 825) resolves differently from round-half-up (-> 826).
        // Every other perServing fixture divides exactly, so none of them catch this.
        val tie = ingredient("tie", kcalTenths = 1651, proteinGTenths = 311, carbsGTenths = 201, fatGTenths = 91)
        val bowl = recipe("tie-bowl", servings = 2, ingredients = listOf(RecipeIngredient(tie.id, 100)))

        val perServing = bowl.perServing(listOf(tie))

        assertEquals(MacrosTenths(826, 156, 101, 46), perServing)
    }

    // ── totalsOf / perServingOf (rows without a Recipe) ────────────────

    /**
     * Two rows with different grams *and* different per-100g macros: a single row, or two
     * rows sharing either number, would let a fold that reads `rows[0]` twice pass.
     */
    @Test
    fun totalsOfSumsLooseRowsTheSameWayARecipeDoes() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val rice = ingredient("rice", kcalTenths = 1230, proteinGTenths = 27)
        val rows = listOf(RecipeIngredient(chicken.id, 300), RecipeIngredient(rice.id, 200))

        assertEquals(MacrosTenths(7410, 984, 0, 0), rows.totalsOf(listOf(chicken, rice)))
    }

    /**
     * 300 g over **4** servings, not 3: at 3 the answer is numerically the per-100g row
     * itself, which a function returning `per100g` and dividing by nothing would also give.
     */
    @Test
    fun perServingOfDividesLooseRowsByServings() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val rows = listOf(RecipeIngredient(chicken.id, 300))

        // 300 g is 4950/930 tenths; a quarter of that, rounded half up, is 1238/233.
        assertEquals(MacrosTenths(1238, 233, 0, 0), rows.perServingOf(listOf(chicken), servings = 4))
    }

    @Test
    fun aRecipesOwnPerServingAgreesWithItsRows() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val rice = ingredient("rice", kcalTenths = 1230, proteinGTenths = 27)
        val table = listOf(chicken, rice)
        val bowl =
            recipe(
                "bowl",
                servings = 4,
                ingredients = listOf(RecipeIngredient(chicken.id, 250), RecipeIngredient(rice.id, 180)),
            )

        // A hand-computed expectation as well as the equality: after the delegation both
        // sides are the same function, so equality alone would hold for any arithmetic.
        // Chicken 250 g (4125/775) + rice 180 g (2214/48,6 -> 2214/49) over four servings.
        assertEquals(MacrosTenths(1585, 206, 0, 0), bowl.perServing(table))
        assertEquals(bowl.perServing(table), bowl.ingredients.perServingOf(table, bowl.servings))
        assertEquals(bowl.totals(table), bowl.ingredients.totalsOf(table))
    }

    @Test
    fun perServingOfRefusesZeroServings() {
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)

        assertFailsWith<IllegalArgumentException> {
            listOf(RecipeIngredient(chicken.id, 100)).perServingOf(listOf(chicken), servings = 0)
        }
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
    // Both fixtures share the same goals/consumed: soft kcal ceiling of remaining.kcal (1000)
    // + 250 = 1250 per serving. Every recipe is one ingredient at exactly 100g, so per-serving
    // macros equal the per-100g row untouched by rounding.

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

    @Test
    fun pickScoresPerServingRatherThanTheWholeRecipesTotals() {
        // A: 1 serving, 100g -> 500 kcal / 100g protein, per-serving and total agree.
        val ingredientA = ingredient("a", kcalTenths = 5000, proteinGTenths = 1000)
        val recipeA = recipe("a", servings = 1, ingredients = listOf(RecipeIngredient(IngredientId("a"), 100)))

        // B: 2 servings, 200g at 300 kcal/80g protein per 100g -> per-serving 80g loses to
        // A's 100g, but scored on whole totals B's 160g would beat A instead — the mutation
        // this test catches (`perServing` swapped for `totals` at the scoring call site).
        val ingredientB = ingredient("b", kcalTenths = 3000, proteinGTenths = 800)
        val recipeB = recipe("b", servings = 2, ingredients = listOf(RecipeIngredient(IngredientId("b"), 200)))

        val result = pick(listOf(recipeA, recipeB), listOf(ingredientA, ingredientB), pickConsumed, pickGoals)

        assertEquals(
            RecipeId("a"),
            result?.recipe?.id,
            "per-serving protein must decide, not the two-serving recipe's total",
        )
    }

    @Test
    fun aRecipeStrictlyUnderTheSoftCeilingBufferStaysUnpenalized() {
        // Pins the 250 buffer: 1200 kcal sits under the 1250 ceiling, but shrinking the
        // buffer to 0 (ceiling 1000) would push it over and flip the winner.
        val (fits, fitsIngredient) = onePortionRecipe("fits", kcal = 1200, proteinG = 100)
        val (never, neverIngredient) = onePortionRecipe("never", kcal = 500, proteinG = 90)

        val result = pick(listOf(fits, never), listOf(fitsIngredient, neverIngredient), pickConsumed, pickGoals)

        assertEquals(
            RecipeId("fits"),
            result?.recipe?.id,
            "1200 kcal is under the 1250 kcal ceiling and must stay unpenalized",
        )
    }

    @Test
    fun aRecipeExactlyAtTheSoftCeilingStaysUnpenalized() {
        // The prototype's `ps.kcal <= rem.kcal + 250` is inclusive; pins `>` against a `>=`
        // mutant, which would penalize this recipe and flip the winner.
        val (atCeiling, atCeilingIngredient) = onePortionRecipe("at-ceiling", kcal = 1250, proteinG = 100)
        val (never, neverIngredient) = onePortionRecipe("never", kcal = 500, proteinG = 90)

        val result =
            pick(listOf(atCeiling, never), listOf(atCeilingIngredient, neverIngredient), pickConsumed, pickGoals)

        assertEquals(RecipeId("at-ceiling"), result?.recipe?.id, "exactly at the ceiling must still count as fitting")
    }

    // ── filteredByTypeAndTag ─────────────────────────────────────────
    //
    // Hand-built, not MockSeed: every MockSeed recipe carries RecipeTag.PROTEIN, so filtering
    // by it there is indistinguishable from no filter.

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
        // `lunch` carries GENTLE, not QUICK — an OR-filter would let it through on meal type alone.
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

        // 2 requested on a 2-serving recipe -> factor 1 -> unchanged.
        val oneRecipeWorth = twoServingBowl.toMealDraft(listOf(chicken), requestedServings = 2)
        // 4 requested -> factor 2 -> doubled. A "servings divisor lost" mutant (factor =
        // requestedServings, ignoring recipe.servings) would also double at 2, so this
        // second point is required, not confirmatory.
        val doublePortions = twoServingBowl.toMealDraft(listOf(chicken), requestedServings = 4)
        // 1 serving requested -> factor 0.5 -> grams halved.
        val halfPortions = twoServingBowl.toMealDraft(listOf(chicken), requestedServings = 1)

        assertEquals(300, oneRecipeWorth.items.single().grams)
        assertEquals(600, doublePortions.items.single().grams)
        assertEquals(150, halfPortions.items.single().grams)
    }

    @Test
    fun toMealDraftRoundsTheGramsScaleHalfUpAtATenthsTie() {
        // 150g / 4 == 37.5, an exact tie: plain integer divide gives 37, round-half-up
        // gives 38. The sibling test's factors (1, 2, 1/2) all divide exactly, so only this
        // one can tell `scaleRounded` apart from plain division.
        val chicken = ingredient("chicken", kcalTenths = 1650, proteinGTenths = 310)
        val fourServingBowl =
            recipe(
                "chicken-bowl",
                servings = 4,
                ingredients = listOf(RecipeIngredient(chicken.id, 150)),
            )

        val draft = fourServingBowl.toMealDraft(listOf(chicken), requestedServings = 1)

        assertEquals(38, draft.items.single().grams)
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
