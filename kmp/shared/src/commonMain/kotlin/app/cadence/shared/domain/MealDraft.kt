package app.cadence.shared.domain

/**
 * What becomes a [Meal] when `NutritionRepository.log` writes it.
 *
 * Mirrors [DoseDraft] and `VialDraft`: nullable until a screen fills it in,
 * with [canLog] answering whether the save action is live. [source] has no
 * default — `LogMealScreen` (step-5) supplies [MealSource.AI_TEXT] and
 * `RecipeDetailScreen` (step-10) supplies [MealSource.RECIPE]; forcing the
 * caller to name one rather than defaulting keeps this draft from drifting
 * toward the one value this port's write path never produces.
 */
data class MealDraft(
    val name: String? = null,
    val source: MealSource? = null,
    val recipeId: RecipeId? = null,
    val items: List<MealItem> = emptyList(),
) {
    /** A name and at least one item — what a meal cannot be logged without. */
    fun canLog(): Boolean = !name.isNullOrBlank() && source != null && items.isNotEmpty()
}
