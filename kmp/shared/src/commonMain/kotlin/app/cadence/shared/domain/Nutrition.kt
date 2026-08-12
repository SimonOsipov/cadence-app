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
 * Macros at 0,1 g precision, carried as a whole number of tenths.
 *
 * [Dose] stays `Double` because a dose is never summed, and its KDoc names the
 * escape hatch this type is: «if a total is ever needed, the type changes
 * here, once». Macros are that total — `Meal.totals` folds a meal's items and
 * a day folds its meals — and a fixed-point integer rather than `Double` is
 * the change, because a many-term sum has to add exactly and only add. Tenths
 * as `Int` do that by construction: `+` on this type is integer addition, with
 * no rounding hiding inside it the way float addition would carry one in.
 *
 * The reference table (`Ingredient.per100g`) and a logged position
 * (`MealItem.macros`) both carry this type — decision spec's §4. `Macros`
 * itself stays whole grams; [toMacros] is the only place tenths become it.
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
 * The one conversion from tenths to whole-gram [Macros] — the spec puts it «at
 * the boundary of `TodaySummary` and display», never per item and never per
 * meal, so this is called once per read: on a folded [MacrosTenths], not on
 * each [MealItem] or [Meal] that fed it.
 *
 * Round half up, not `kotlin.math.round`: the reason `formatDecimal` already
 * gives — Kotlin's `round` breaks ties to even, and a patient reading a whole
 * gram expects the half to go up, same as a dose or a weight does.
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
 * Exact fold across many [MacrosTenths] — tenths add without rounding, so a
 * day's total from several meals is the same number whether it is computed in
 * one fold or assembled from each meal's own [Meal.totals]. Only [toMacros]
 * ever rounds; a caller that rounds each term first and then adds is the
 * mutation §4's day-total test exists to catch.
 */
fun Iterable<MacrosTenths>.sumTenths(): MacrosTenths = fold(MacrosTenths.ZERO) { acc, next -> acc + next }

/** §03's `ingredients` — macros per 100 g. */
data class Ingredient(
    val id: IngredientId,
    val nameRu: String,
    val per100g: MacrosTenths,
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
     * Derived, never stored — the exact sum of what was actually logged, at
     * tenths precision. Stays a [MacrosTenths]: rounding it here would be a
     * second rounding on top of [MacrosTenths.toMacros]'s, and the spec's
     * boundary is one.
     */
    val totals: MacrosTenths
        get() = items.map { it.macros }.sumTenths()
}

/**
 * Rescales [original]'s macros to [newGrams], grams and all four fields
 * together.
 *
 * The base is always [original], never the item mid-edit — that is the whole
 * point of taking two arguments instead of one. The prototype's `rescaleItem`
 * takes a single item and recomputes from whatever it currently holds
 * (`meal/data.ts:140-151`), so «−10 г, then +10 г» does not return the number
 * it started from: each step rounds, and the second step scales from the
 * first step's rounded output rather than the true original. Threading the
 * same [original] through every edit removes the drift instead of shrinking
 * it — at `newGrams == original.grams` the scale factor is exactly one and
 * every field returns unchanged, not merely close.
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
                kcalTenths = scaleTenths(original.macros.kcalTenths, grams, original.grams),
                proteinGTenths = scaleTenths(original.macros.proteinGTenths, grams, original.grams),
                carbsGTenths = scaleTenths(original.macros.carbsGTenths, grams, original.grams),
                fatGTenths = scaleTenths(original.macros.fatGTenths, grams, original.grams),
            ),
    )
}

/** Round-half-up scale of a tenths value by `newGrams / originalGrams`. */
private fun scaleTenths(
    tenths: Int,
    newGrams: Int,
    originalGrams: Int,
): Int = floor(tenths.toDouble() * newGrams / originalGrams + ROUND_HALF).toInt()

/**
 * §03's `nutrition_targets` — «set by the clinic per patient — was a hardcoded
 * const».
 */
data class NutritionTargets(
    val patientId: UserId,
    val macros: Macros,
    val waterMl: Int?,
)
