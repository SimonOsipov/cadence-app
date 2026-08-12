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
    /**
     * A non-blank name, a source, at least one item, and — when [source] is
     * [MealSource.RECIPE] — a [recipeId]: what a meal cannot be logged
     * without. The spec's step-10 criterion pairs `RECIPE` with `recipeId`
     * explicitly; a `RECIPE` draft missing one is half of `RecipeDetailScreen`'s
     * write, not a valid one.
     */
    fun canLog(): Boolean =
        !name.isNullOrBlank() &&
            source != null &&
            items.isNotEmpty() &&
            (source != MealSource.RECIPE || recipeId != null)
}
