package app.cadence.shared.domain

// The recipe context's pure arithmetic. Nothing here does I/O or reads a repository; every
// input arrives as a parameter.

private const val REFERENCE_GRAMS = 100
private const val TENTHS_PER_UNIT = 10

/**
 * Built on [rescaleMealItem], not a second copy of its round-half-up scale: same
 * "start here, scale to there" arithmetic, starting from [Ingredient.per100g] instead of a
 * logged [MealItem.macros].
 */
fun Ingredient.macrosFor(grams: Int): MacrosTenths {
    val referenceRow = MealItem(name = nameRu, grams = REFERENCE_GRAMS, macros = per100g)
    return rescaleMealItem(referenceRow, grams).macros
}

/**
 * Exact, unrounded sum of every [Recipe.ingredients] row, kept at tenths precision like
 * every other fold in this context. [ingredients] is the caller's table; the arithmetic
 * doesn't know where rows come from.
 */
fun Recipe.totals(ingredients: List<Ingredient>): MacrosTenths {
    val byId = ingredients.associateBy { it.id }
    return this.ingredients
        .map { row ->
            val ingredient = byId[row.ingredientId] ?: error("no ingredient with id ${row.ingredientId.raw}")
            ingredient.macrosFor(row.grams)
        }.sumTenths()
}

/**
 * [totals] divided evenly across [Recipe.servings] via [scaleRounded] at `numerator = 1` —
 * the same round-half-up rule [rescaleMealItem] uses, not a second copy of it.
 */
fun Recipe.perServing(ingredients: List<Ingredient>): MacrosTenths {
    require(servings > 0) { "a recipe with zero servings has no per-serving rate to divide by" }
    val total = totals(ingredients)
    return MacrosTenths(
        kcalTenths = scaleRounded(total.kcalTenths, numerator = 1, denominator = servings),
        proteinGTenths = scaleRounded(total.proteinGTenths, numerator = 1, denominator = servings),
        carbsGTenths = scaleRounded(total.carbsGTenths, numerator = 1, denominator = servings),
        fatGTenths = scaleRounded(total.fatGTenths, numerator = 1, denominator = servings),
    )
}

/** Prep plus cook, treating either absent minute count as zero — recipe/data.ts's `totalTime`. */
fun Recipe.totalTimeMin(): Int = (prepMin ?: 0) + (cookMin ?: 0)

/** [remaining]'s two fields, floored at zero, never negative. */
data class Remaining(
    val kcal: Int,
    val proteinG: Int,
)

/**
 * Ported to take the target as a parameter, not read the prototype's `MEAL_TARGETS`
 * constant: that would be the one place "constants live in exactly one place" broke.
 */
fun remaining(
    consumed: Macros,
    goals: Macros,
): Remaining =
    Remaining(
        kcal = (goals.kcal - consumed.kcal).coerceAtLeast(0),
        proteinG = (goals.proteinG - consumed.proteinG).coerceAtLeast(0),
    )

/** What [pick] chose, and the [Remaining] it was scored against — a screen needs both. */
data class RecipePick(
    val recipe: Recipe,
    val remaining: Remaining,
)

private const val SOFT_CEILING_KCAL_BUFFER = 250
private const val OVER_CEILING_PENALTY_G = 40

/**
 * `null` only when [recipes] is empty. Score is per-serving protein minus a
 * [OVER_CEILING_PENALTY_G]-gram penalty over the soft kcal ceiling — a penalty, not a
 * filter: a recipe over the ceiling still competes and can win if its protein clears the gap.
 */
fun pick(
    recipes: List<Recipe>,
    ingredients: List<Ingredient>,
    consumed: Macros,
    goals: Macros,
): RecipePick? {
    val rem = remaining(consumed, goals)
    val ceilingKcalTenths = (rem.kcal + SOFT_CEILING_KCAL_BUFFER) * TENTHS_PER_UNIT
    val best =
        recipes.maxByOrNull { candidate ->
            val perServingMacros = candidate.perServing(ingredients)
            val overCeiling = perServingMacros.kcalTenths > ceilingKcalTenths
            val penalty = if (overCeiling) OVER_CEILING_PENALTY_G * TENTHS_PER_UNIT else 0
            perServingMacros.proteinGTenths - penalty
        } ?: return null
    return RecipePick(recipe = best, remaining = rem)
}

/**
 * Two independent, additive filters — `null` on either field is «Все»/«Любые». Kept in
 * `domain`, not next to `RecipeFilter`, so this stays independent of the calling repository layer.
 */
fun List<Recipe>.filteredByTypeAndTag(
    mealType: MealType?,
    tag: RecipeTag?,
): List<Recipe> =
    filter { recipe ->
        (mealType == null || recipe.mealType == mealType) &&
            (tag == null || tag in recipe.tags)
    }

/**
 * [MealSource.RECIPE] and [Recipe.id] are set here, not left for the screen, so «add to day»
 * can't forget either half of [MealDraft.canLog]'s `RECIPE`-needs-`recipeId` case. Scale
 * factor `requestedServings / servings` is applied to each ingredient's own grams before
 * macros are computed, never to [totals] after the fact — so a two-serving recipe logged at
 * one serving rescales exactly like a half-sized version of the same recipe would.
 */
fun Recipe.toMealDraft(
    ingredients: List<Ingredient>,
    requestedServings: Int,
): MealDraft {
    require(servings > 0) { "a recipe with zero servings has no per-serving rate to scale from" }
    require(requestedServings > 0) { "requestedServings must be positive, got $requestedServings" }

    val byId = ingredients.associateBy { it.id }
    val items =
        this.ingredients.map { row ->
            val ingredient = byId[row.ingredientId] ?: error("no ingredient with id ${row.ingredientId.raw}")
            val grams = scaleRounded(row.grams, numerator = requestedServings, denominator = servings)
            MealItem(name = ingredient.nameRu, grams = grams, macros = ingredient.macrosFor(grams))
        }

    return MealDraft(
        name = name,
        source = MealSource.RECIPE,
        recipeId = id,
        items = items,
    )
}
