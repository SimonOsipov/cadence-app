package app.cadence.screens.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.pressable
import app.cadence.format.formatInteger
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.Recipe
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.domain.perServing
import app.cadence.shared.domain.toMacros
import app.cadence.shared.domain.totalTimeMin

// The «Мои рецепты» / library sections and their rows — split out of
// `RecipesScreen.kt` the same reason `NutritionMealFeed.kt` was split out of
// `NutritionScreen.kt`.

private val ROW_NAME_SIZE = 20.sp
private val MINE_BADGE_SIZE = 10.sp
private val TYPE_PILL_SHAPE = RoundedCornerShape(CadenceRadius.pill)
private const val ROW_DEK_MAX_LINES = 2

/** A row's own meal-type pill, so a test can capture its fill colour without matching on copy. */
fun recipeTypeTag(recipeId: String) = "cadence-recipe-type-$recipeId"

/** «Завтрак» / «Обед» / «Ужин» / «Перекус» — `recipe/data.ts:95-100`'s `MEAL_TYPES`. */
internal fun mealTypeLabel(type: MealType): String =
    when (type) {
        MealType.BREAKFAST -> "Завтрак"
        MealType.LUNCH -> "Обед"
        MealType.DINNER -> "Ужин"
        MealType.SNACK -> "Перекус"
    }

/** «Белковые» / «Мягкие для желудка» / «Быстрые» — `recipe/data.ts:90-94`'s `TAGS`. */
internal fun tagLabel(tag: RecipeTag): String =
    when (tag) {
        RecipeTag.PROTEIN -> "Белковые"
        RecipeTag.GENTLE -> "Мягкие для желудка"
        RecipeTag.QUICK -> "Быстрые"
    }

internal data class MealTypeTone(
    val soft: Color,
    val fg: Color,
)

/**
 * `TYPE_TINT` (`RecipesScreen.tsx:13-18`), the row's own meal-type pill tone.
 * Breakfast and lunch reuse this palette's existing sand/forest tokens —
 * their hexes match exactly; dinner and snack use [CadenceColors.slateBg]/
 * [CadenceColors.clayBg] and their `Fg` pairs, added for this call site (see
 * that file's own KDoc for why those two names).
 *
 * `internal`, not `private`: `RecipeDetailScreen.kt`'s hero (step-10) is this
 * function's second call site — `RecipeDetailScreen.tsx:12` imports the same
 * `TYPE_TINT` from `RecipesScreen.tsx` rather than declaring its own, and this
 * port follows suit rather than growing a second, unlinked copy of the map.
 */
internal fun mealTypeTone(type: MealType): MealTypeTone =
    when (type) {
        MealType.BREAKFAST -> MealTypeTone(CadenceColors.sand100, CadenceColors.sand900)
        MealType.LUNCH -> MealTypeTone(CadenceColors.forest50, CadenceColors.forest700)
        MealType.DINNER -> MealTypeTone(CadenceColors.slateBg, CadenceColors.slateFg)
        MealType.SNACK -> MealTypeTone(CadenceColors.clayBg, CadenceColors.clayFg)
    }

/**
 * «Мои рецепты» — the caller only renders this when [recipes] is non-empty
 * (`RecipesScreen.tsx:311-320`'s `mine.length > 0 ?`); this composable does
 * not re-check that itself, so an empty list here would draw a heading over
 * nothing rather than being the thing that decides visibility.
 */
@Composable
internal fun RecipesSection(
    title: String,
    recipes: List<Recipe>,
    ingredients: List<Ingredient>,
    onOpen: (RecipeId) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxWidth().padding(bottom = CadenceSpacing.sm)) {
        CadenceEyebrow(title, Modifier.padding(horizontal = CadenceSpacing.xxs, vertical = CadenceSpacing.xxs))
        recipes.forEachIndexed { index, recipe ->
            RecipeRow(recipe, ingredients, onOpen = { onOpen(recipe.id) }, last = index == recipes.lastIndex)
        }
    }
}

/**
 * The clinic's library, heading «Готовые рецепты» / «Все рецепты» depending
 * on [hasMine] (`RecipesScreen.tsx:325`: `mine.length ? 'Готовые рецепты' :
 * 'Все рецепты'`), and «Под фильтр ничего не подошло.» when [recipes] is
 * empty (`:327-338`).
 */
@Composable
internal fun RecipesLibrarySection(
    hasMine: Boolean,
    recipes: List<Recipe>,
    ingredients: List<Ingredient>,
    onOpen: (RecipeId) -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Column(modifier.fillMaxWidth()) {
        CadenceEyebrow(
            if (hasMine) "Готовые рецепты" else "Все рецепты",
            Modifier.padding(horizontal = CadenceSpacing.xxs, vertical = CadenceSpacing.xxs),
        )
        if (recipes.isEmpty()) {
            CadenceBody(
                "Под фильтр ничего не подошло.",
                color = palette.subtle,
                modifier = Modifier.padding(vertical = CadenceSpacing.md, horizontal = CadenceSpacing.xxs),
            )
        } else {
            recipes.forEachIndexed { index, recipe ->
                RecipeRow(recipe, ingredients, onOpen = { onOpen(recipe.id) }, last = index == recipes.lastIndex)
            }
        }
    }
}

/**
 * One typographic row: name, a «Моё» pill for an owned recipe, a two-line
 * description, the meal-type tone pill, calories and protein per serving,
 * and total time (`RecipeRow`, `RecipesScreen.tsx:42-124`).
 *
 * Calories and protein share one text run rather than the prototype's two
 * separately coloured runs (`:107-117`) — the same simplification
 * `MealHero.kt`'s remaining-line and `NutritionMealFeed.kt`'s meal-card
 * meta line already make for a compound numeric sentence, for the same
 * reason: nothing here reads the colour, only the numbers.
 */
@Composable
private fun RecipeRow(
    recipe: Recipe,
    ingredients: List<Ingredient>,
    onOpen: () -> Unit,
    last: Boolean,
) {
    val palette = Cadence.palette
    val perServing = recipe.perServing(ingredients).toMacros()
    val tone = mealTypeTone(recipe.mealType)

    Column(
        Modifier
            .fillMaxWidth()
            .pressable(onOpen, remember { MutableInteractionSource() })
            .padding(vertical = CadenceSpacing.md, horizontal = CadenceSpacing.xxs),
    ) {
        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
            // CadenceTitle, not a raw BasicText: every text primitive in
            // `CadenceText.kt` pairs `maxLines` with `TextOverflow.Ellipsis`
            // (`:51,75,138,154`) — a raw `BasicText(maxLines = 1)` here clips a
            // long name mid-glyph instead of ellipsizing it.
            CadenceTitle(
                text = recipe.name,
                size = ROW_NAME_SIZE,
                color = palette.ink,
                maxLines = 1,
                modifier = Modifier.weight(1f, fill = false),
            )
            if (recipe.ownerId != null) MineBadge()
        }

        CadenceBody(
            recipe.dek.orEmpty(),
            color = palette.subtle,
            maxLines = ROW_DEK_MAX_LINES,
            modifier = Modifier.padding(top = CadenceSpacing.xxs),
        )

        Row(
            Modifier.fillMaxWidth().padding(top = CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        ) {
            Box(
                Modifier
                    .testTag(recipeTypeTag(recipe.id.raw))
                    .background(
                        tone.soft,
                        TYPE_PILL_SHAPE,
                    ).padding(horizontal = CadenceSpacing.sm, vertical = CadenceSpacing.xxs),
            ) {
                BasicText(
                    text = mealTypeLabel(recipe.mealType),
                    style = Cadence.typography.label.copy(color = tone.fg, fontSize = 11.sp),
                )
            }
            BasicText(
                text = "${formatInteger(perServing.kcal)} ккал · ${formatInteger(perServing.proteinG)} г белка",
                style = Cadence.typography.number.copy(color = palette.ink2, fontSize = 11.5.sp),
            )
            Spacer(Modifier.weight(1f))
            CadenceMeta("${recipe.totalTimeMin()} мин", color = palette.subtle)
        }
    }

    if (!last) CadenceHairline()
}

/** «Моё» — an owned recipe's own badge (`RecipesScreen.tsx:67-80`). */
@Composable
private fun MineBadge() {
    Box(
        Modifier
            .background(CadenceColors.forest50, RoundedCornerShape(CadenceRadius.pill))
            .padding(horizontal = CadenceSpacing.sm, vertical = CadenceSpacing.xxs),
    ) {
        BasicText(
            text = "Моё",
            style =
                Cadence.typography.label.copy(
                    color = CadenceColors.forest700,
                    fontSize = MINE_BADGE_SIZE,
                    fontWeight = FontWeight.SemiBold,
                ),
        )
    }
}
