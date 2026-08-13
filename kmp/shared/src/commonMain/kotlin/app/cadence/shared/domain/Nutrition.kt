package app.cadence.shared.domain

import kotlin.math.floor
import kotlin.time.Instant

// nutrition — «meals, recipes, targets» (§03).

/** The four numbers everything in this context carries. */
data class Macros(
    val kcal: Int,
    val proteinG: Int,
    val carbsG: Int,
    val fatG: Int,
)

private const val TENTHS_PER_UNIT = 10
private const val ROUND_HALF = 0.5

/**
 * Macros at 0,1g precision, as a whole number of tenths — the fixed-point integer [Dose]'s
 * KDoc calls the escape hatch for "if a total is ever needed": `+` here is exact integer
 * addition, with no rounding hidden inside it the way float addition would carry one in.
 * `kcalTenths` deviates from §4's literal wording (which scopes «0,1 г» to the gram fields —
 * see the step-2 deviation note): kcal fractures the same way scaling grams does, so a
 * whole-kcal-per-item type would smuggle a second rounding upstream of this type's one boundary.
 */
data class MacrosTenths(
    val kcalTenths: Int,
    val proteinGTenths: Int,
    val carbsGTenths: Int,
    val fatGTenths: Int,
) {
    operator fun plus(other: MacrosTenths) =
        MacrosTenths(
            kcalTenths = kcalTenths + other.kcalTenths,
            proteinGTenths = proteinGTenths + other.proteinGTenths,
            carbsGTenths = carbsGTenths + other.carbsGTenths,
            fatGTenths = fatGTenths + other.fatGTenths,
        )

    companion object {
        val ZERO = MacrosTenths(0, 0, 0, 0)
    }
}

/**
 * The one conversion from tenths to whole-gram [Macros], called once per read on a folded
 * [MacrosTenths] — never per item or per meal. Round half up, not `kotlin.math.round`: Kotlin's
 * `round` breaks ties to even, and a patient reading a whole gram expects the half to go up.
 */
fun MacrosTenths.toMacros(): Macros =
    Macros(
        kcal = roundTenths(kcalTenths),
        proteinG = roundTenths(proteinGTenths),
        carbsG = roundTenths(carbsGTenths),
        fatG = roundTenths(fatGTenths),
    )

private fun roundTenths(tenths: Int): Int = floor(tenths / TENTHS_PER_UNIT.toDouble() + ROUND_HALF).toInt()

/**
 * Exact fold: tenths add without rounding, so a day's total is the same whether computed in
 * one fold or assembled from each meal's own [Meal.totals]. Only [toMacros] ever rounds.
 */
fun Iterable<MacrosTenths>.sumTenths(): MacrosTenths = fold(MacrosTenths.ZERO) { acc, next -> acc + next }

/** §03's `ingredients` — macros per 100 g. */
data class Ingredient(
    val id: IngredientId,
    val nameRu: String,
    val per100g: MacrosTenths,
)

/**
 * §03: «tags[] protein|gentle|quick». An enum, not `List<String>`, so a screen can't filter
 * on a tag no recipe can ever carry.
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
    /** §03's value, kept for enum completeness; `MockSeed.meals` is the only writer of it in this port. */
    MANUAL("manual"),
    RECIPE("recipe"),
    AI_TEXT("ai_text"),
}

/**
 * §03's `meal_items`. A snapshot, not a lookup: an ingredient's macros can be corrected
 * later and what the patient ate doesn't change retroactively.
 */
data class MealItem(
    val name: String,
    val grams: Int,
    val macros: MacrosTenths,
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
    /**
     * Derived, never stored. Stays a [MacrosTenths]: rounding it here would be a second
     * rounding on top of [MacrosTenths.toMacros]'s.
     */
    val totals: MacrosTenths
        get() = items.map { it.macros }.sumTenths()
}

/**
 * Rounded once — every meal's exact [Meal.totals] folded together and only then handed to
 * [toMacros]. `TodaySummary.mealMacros` and `NutritionDay.totals` both fold through this
 * function rather than each calling [toMacros] on its own sum, so Today and Nutrition can't
 * round the same day's numbers two different ways.
 */
fun List<Meal>.dayTotals(): Macros = map { it.totals }.sumTenths().toMacros()

/**
 * Base is always [original], never the item mid-edit — the prototype's `rescaleItem`
 * recomputes from whatever it currently holds, so «−10г, then +10г» doesn't return the
 * starting number: each step rounds from the previous step's rounded output. Threading the
 * same [original] through every edit removes the drift instead of shrinking it.
 */
fun rescaleMealItem(
    original: MealItem,
    newGrams: Int,
): MealItem {
    require(original.grams > 0) { "an item with zero grams has no rate to scale from" }
    val grams = newGrams.coerceAtLeast(0)

    return original.copy(
        grams = grams,
        macros =
            MacrosTenths(
                kcalTenths = scaleRounded(original.macros.kcalTenths, grams, original.grams),
                proteinGTenths = scaleRounded(original.macros.proteinGTenths, grams, original.grams),
                carbsGTenths = scaleRounded(original.macros.carbsGTenths, grams, original.grams),
                fatGTenths = scaleRounded(original.macros.fatGTenths, grams, original.grams),
            ),
    )
}

/**
 * Round-half-up scale of [value] by `numerator / denominator`, matching [MacrosTenths]'s KDoc
 * on why a patient reading a whole number expects a half to go up. `internal`, not `private`:
 * `RecipeMath.kt`'s per-serving divide is the same arithmetic, shared rather than duplicated.
 */
internal fun scaleRounded(
    value: Int,
    numerator: Int,
    denominator: Int,
): Int = floor(value.toDouble() * numerator / denominator + ROUND_HALF).toInt()

/**
 * §03's `nutrition_targets` — «set by the clinic per patient — was a hardcoded
 * const».
 */
data class NutritionTargets(
    val patientId: UserId,
    val macros: Macros,
    val waterMl: Int?,
)
