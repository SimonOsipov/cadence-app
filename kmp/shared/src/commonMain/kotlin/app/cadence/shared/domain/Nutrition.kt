package app.cadence.shared.domain

import kotlin.time.Instant

// nutrition — «meals, recipes, targets» (§03).

/** The four numbers everything in this context carries. */
data class Macros(
    val kcal: Int,
    val proteinG: Int,
    val carbsG: Int,
    val fatG: Int,
)

/** §03's `ingredients` — macros per 100 g. */
data class Ingredient(
    val id: IngredientId,
    val nameRu: String,
    val per100g: Macros,
)

/**
 * §03: «tags[] protein|gentle|quick». Closed there, so closed here — every
 * other closed set in this transcription became an enum, and a `List<String>`
 * lets a screen filter on a tag no recipe can ever carry.
 */
enum class RecipeTag(
    val code: String,
) {
    PROTEIN("protein"),
    GENTLE("gentle"),
    QUICK("quick"),
}

enum class MealType(
    val code: String,
) {
    BREAKFAST("breakfast"),
    LUNCH("lunch"),
    DINNER("dinner"),
    SNACK("snack"),
}

/** §03's `recipe_ingredients`. */
data class RecipeIngredient(
    val ingredientId: IngredientId,
    val grams: Int,
)

/** §03's `recipes`. A null `ownerId` is the clinic library. */
data class Recipe(
    val id: RecipeId,
    val ownerId: UserId?,
    val name: String,
    val mealType: MealType,
    val tags: List<RecipeTag>,
    val servings: Int,
    val prepMin: Int?,
    val cookMin: Int?,
    val dek: String?,
    val ingredients: List<RecipeIngredient>,
    val steps: List<String>,
)

/** §03: `source manual|recipe|ai_text`. */
enum class MealSource(
    val code: String,
) {
    MANUAL("manual"),
    RECIPE("recipe"),
    AI_TEXT("ai_text"),
}

/**
 * §03's `meal_items` — «kcal,p,c,f snapshot at log time».
 *
 * A snapshot, not a lookup: an ingredient's macros can be corrected later and
 * what the patient ate does not change retroactively.
 */
data class MealItem(
    val name: String,
    val grams: Int,
    val macros: Macros,
)

/** §03's `meals`. */
data class Meal(
    val id: MealId,
    val patientId: UserId,
    val eatenAt: Instant,
    val name: String,
    val source: MealSource,
    val recipeId: RecipeId?,
    val items: List<MealItem>,
) {
    /** Derived, never stored — the sum of what was actually logged. */
    val totals: Macros
        get() =
            items.fold(Macros(0, 0, 0, 0)) { acc, item ->
                Macros(
                    kcal = acc.kcal + item.macros.kcal,
                    proteinG = acc.proteinG + item.macros.proteinG,
                    carbsG = acc.carbsG + item.macros.carbsG,
                    fatG = acc.fatG + item.macros.fatG,
                )
            }
}

/**
 * §03's `nutrition_targets` — «set by the clinic per patient — was a hardcoded
 * const».
 */
data class NutritionTargets(
    val patientId: UserId,
    val macros: Macros,
    val waterMl: Int?,
)
