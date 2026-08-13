package app.cadence.shared.domain

/**
 * Mirrors [DoseDraft] and `VialDraft`: nullable until a screen fills it in, [canLog]
 * answers whether save is live. [source] has no default — forcing the caller to name it
 * keeps this draft from drifting toward the one value this port's write path never produces.
 */
data class MealDraft(
    val name: String? = null,
    val source: MealSource? = null,
    val recipeId: RecipeId? = null,
    val items: List<MealItem> = emptyList(),
) {
    /** A `RECIPE` draft missing [recipeId] is half of `RecipeDetailScreen`'s write, not a valid one. */
    fun canLog(): Boolean =
        !name.isNullOrBlank() &&
            source != null &&
            items.isNotEmpty() &&
            (source != MealSource.RECIPE || recipeId != null)
}
