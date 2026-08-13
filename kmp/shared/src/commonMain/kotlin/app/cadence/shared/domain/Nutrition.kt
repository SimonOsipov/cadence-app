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
 *
 * `kcalTenths` deviates from §4's literal wording, which scopes «0,1 г» to the
 * gram fields — see the step-2 deviation note. kcal is not exempt from the
 * "sum precisely, round once" boundary the other three fields get: scaling an
 * ingredient's per-100g kcal by an item's actual grams fractures the same way
 * the gram fields do (rice at 240 g is 123 × 2,4 = 295,2 kcal, not a whole
 * number), so a whole-kcal-per-item type would smuggle a second rounding in
 * upstream of the one boundary this type exists to guarantee.
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
    /**
     * §03's value, kept for the enum's completeness. `MockSeed.meals` is the
     * only writer of it in this port — `LogMealScreen` writes [AI_TEXT] and
     * `RecipeDetailScreen` writes [RECIPE]; `MealDraft.canLog` requires a
     * caller to name one of the two explicitly rather than default here.
     */
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
 * A day's macros, rounded once — every meal's exact [Meal.totals] folded
 * together and only then handed to [toMacros].
 *
 * The repository layer's one call site for [toMacros]: `TodaySummary.mealMacros`
 * and `NutritionDay.totals` both fold their day's meals through this function
 * rather than each calling [toMacros] on its own sum, so «today» on the Today
 * screen and «today» on the Nutrition screen cannot end up rounding the same
 * numbers two different ways.
 */
fun List<Meal>.dayTotals(): Macros = map { it.totals }.sumTenths().toMacros()

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
                kcalTenths = scaleRounded(original.macros.kcalTenths, grams, original.grams),
                proteinGTenths = scaleRounded(original.macros.proteinGTenths, grams, original.grams),
                carbsGTenths = scaleRounded(original.macros.carbsGTenths, grams, original.grams),
                fatGTenths = scaleRounded(original.macros.fatGTenths, grams, original.grams),
            ),
    )
}

/**
 * Round-half-up scale of [value] by `numerator / denominator` — the tie
 * always rounds up, never to even, matching [MacrosTenths]'s KDoc on why a
 * patient reading a whole number expects a half to go up.
 *
 * `internal`, not `private`: `RecipeMath.kt`'s per-serving divide and its
 * requested-servings grams scale are the same "scale this integer by a
 * ratio, round half up" arithmetic as [rescaleMealItem]'s grams scale here —
 * a division by servings is `scaleRounded(total, 1, servings)`, and a grams
 * scale by `requestedServings / servings` is `scaleRounded(grams,
 * requestedServings, servings)`. One implementation shared across both
 * files rather than a second copy of the rule living beside this one.
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
