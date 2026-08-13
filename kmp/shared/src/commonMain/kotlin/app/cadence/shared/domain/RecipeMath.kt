package app.cadence.shared.domain

import kotlin.math.floor

// step-8: the recipe context's pure arithmetic — an ingredient's macros for N
// grams, a recipe's totals and per-serving figures, its total time, the
// remainder against a patient's goals, the pick score, the library filter,
// and turning a recipe into a loggable meal draft. Nothing here does I/O or
// reads a repository; every input arrives as a parameter.

private const val REFERENCE_GRAMS = 100
private const val TENTHS_PER_UNIT = 10
private const val ROUND_HALF = 0.5

/**
 * [grams] worth of this ingredient's macros, scaled from its 100 g reference
 * row (`per100g`).
 *
 * Built on [rescaleMealItem] rather than a second copy of its round-half-up
 * ratio scale: a reference row and a logged item's own grams are the same
 * "start here, scale to there" arithmetic, just starting from
 * [Ingredient.per100g] instead of a previously-logged [MealItem.macros].
 */
fun Ingredient.macrosFor(grams: Int): MacrosTenths {
    val referenceRow = MealItem(name = nameRu, grams = REFERENCE_GRAMS, macros = per100g)
    return rescaleMealItem(referenceRow, grams).macros
}

/**
 * The exact, unrounded sum of every [Recipe.ingredients] row at its own
 * grams — recipe/data.ts's `recipeTotals`, kept at tenths precision like
 * every other fold in this context (`Meal.totals`, `List<Meal>.dayTotals`).
 *
 * [ingredients] is the caller's ingredient table — `MockSeed.ingredients` in
 * this port, but never named here: the arithmetic does not know or care
 * where its rows come from.
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
 * [totals] divided evenly across [Recipe.servings], round-half-up per field —
 * recipe/data.ts's `perServing`.
 */
fun Recipe.perServing(ingredients: List<Ingredient>): MacrosTenths {
    require(servings > 0) { "a recipe with zero servings has no per-serving rate to divide by" }
    val total = totals(ingredients)
    return MacrosTenths(
        kcalTenths = divideTenths(total.kcalTenths, servings),
        proteinGTenths = divideTenths(total.proteinGTenths, servings),
        carbsGTenths = divideTenths(total.carbsGTenths, servings),
        fatGTenths = divideTenths(total.fatGTenths, servings),
    )
}

private fun divideTenths(
    tenths: Int,
    servings: Int,
): Int = floor(tenths.toDouble() / servings + ROUND_HALF).toInt()

/** Prep plus cook, treating either absent minute count as zero — recipe/data.ts's `totalTime`. */
fun Recipe.totalTimeMin(): Int = (prepMin ?: 0) + (cookMin ?: 0)

/** What is left of a patient's daily goals after [remaining]'s two fields — floored at zero, never negative. */
data class Remaining(
    val kcal: Int,
    val proteinG: Int,
)

/**
 * [goals] minus [consumed], per field, floored at zero — recipe/data.ts's
 * `remaining`, ported to take the target as a parameter instead of reading
 * the prototype's `MEAL_TARGETS` constant: a constant read from inside the
 * purest function in this context would be the one place the project's
 * "constants live in exactly one place" rule broke.
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
 * The recipe from [recipes] with the highest pick score, alongside the
 * [Remaining] it was scored against — `null` only when [recipes] is empty.
 *
 * Score is a recipe's per-serving protein, in tenths of a gram, minus a
 * [OVER_CEILING_PENALTY_G]-gram penalty when its per-serving kcal exceeds the
 * soft ceiling `remaining.kcal + `[SOFT_CEILING_KCAL_BUFFER] —
 * recipe/data.ts:224-233's `suggest`. The penalty is exactly that: a penalty,
 * not a filter — a recipe over the ceiling still competes and can still win
 * if its protein clears the gap the penalty opens.
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
 * `RecipesScreen`'s (step-9) two independent, additive filters — `null` on
 * either field is «Все» / «Любые», matching
 * [app.cadence.shared.repository.RecipeFilter]'s own `null` convention. Kept
 * in `domain` rather than next to `RecipeFilter` itself so this arithmetic
 * stays independent of the repository layer that calls it, the same
 * direction every other domain function in this context keeps.
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
 * This [Recipe] scaled to [requestedServings] portions and turned into a
 * ready-to-log [MealDraft] — recipe/data.ts:241-253's `toMeal`, folded
 * directly into the draft `RecipeDetailScreen` (step-10) writes: [MealSource.RECIPE]
 * and [Recipe.id] are set here rather than left for the screen to fill in,
 * so «add to day» cannot forget either half of [MealDraft.canLog]'s
 * `RECIPE`-needs-`recipeId` case.
 *
 * The scale factor is `requestedServings / servings`, applied to each
 * ingredient's own grams before its macros are computed — never applied to
 * [totals] after the fact, so a two-serving recipe logged at one serving
 * rescales every ingredient row exactly the way a half-sized version of the
 * same recipe would.
 */
fun Recipe.toMealDraft(
    ingredients: List<Ingredient>,
    requestedServings: Int,
): MealDraft {
    require(servings > 0) { "a recipe with zero servings has no per-serving rate to scale from" }
    require(requestedServings > 0) { "requestedServings must be positive, got $requestedServings" }

    val byId = ingredients.associateBy { it.id }
    val factor = requestedServings.toDouble() / servings
    val items =
        this.ingredients.map { row ->
            val ingredient = byId[row.ingredientId] ?: error("no ingredient with id ${row.ingredientId.raw}")
            val grams = floor(row.grams * factor + ROUND_HALF).toInt()
            MealItem(name = ingredient.nameRu, grams = grams, macros = ingredient.macrosFor(grams))
        }

    return MealDraft(
        name = name,
        source = MealSource.RECIPE,
        recipeId = id,
        items = items,
    )
}
