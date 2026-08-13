package app.cadence.screens.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceStepper
import app.cadence.design.CadenceTextField
import app.cadence.design.CadenceTitle
import app.cadence.design.pressable
import app.cadence.format.formatDecimal
import app.cadence.format.formatKcalTenths
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.IngredientId
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.macrosFor
import kotlin.math.roundToInt

// «Ингредиент» — the ingredient picker sheet, ported from
// mobile/src/features/recipe/RecipeBuilderScreen.tsx's `IngredientPicker`
// (`:127` the search, `:130-132` the grams reset).
//
// Step-11 of docs/specs/kmp-nutrition-and-recipes.md. A surface of its own,
// ahead of `RecipeBuilderScreen` (step-12): the builder cannot be assembled
// without a way to pick an ingredient, so this ships first with its own
// state and its own tests. [search] arrives as a `suspend` lambda from the
// shell — `LogMealScreen.kt`'s own `parse` param takes the same shape, for
// the same reason: this screen reaches into no repository of its own.

const val CADENCE_INGREDIENT_PICKER_TAG = "cadence-ingredient-picker"
const val CADENCE_INGREDIENT_PICKER_SEARCH_TAG = "cadence-ingredient-picker-search"
const val CADENCE_INGREDIENT_PICKER_CLOSE_TAG = "cadence-ingredient-picker-close"
const val CADENCE_INGREDIENT_PICKER_FOOTER_TAG = "cadence-ingredient-picker-footer"
const val CADENCE_INGREDIENT_PICKER_ADD_TAG = "cadence-ingredient-picker-add"
const val CADENCE_INGREDIENT_PICKER_EMPTY_TAG = "cadence-ingredient-picker-empty"

/** One row, so a test can address a specific ingredient rather than position in the list. */
fun ingredientPickerRowTag(id: IngredientId): String = "cadence-ingredient-picker-row-${id.raw}"

private const val HEADER_TITLE = "Ингредиент"
private const val CLOSE_DESCRIPTION = "Закрыть"
private const val SEARCH_PLACEHOLDER = "Найти продукт"
private const val NOT_FOUND_MESSAGE = "Ничего не найдено."
private const val ADD_LABEL = "Добавить"

/** `RecipeBuilderScreen.tsx:130-132`'s own default — the grams a new selection lands on. */
private const val DEFAULT_GRAMS = 100
private const val GRAMS_MIN = 5.0
private const val GRAMS_MAX = 600.0
private const val GRAMS_STEP = 10.0
private const val TENTHS_PER_UNIT = 10.0

private val HEADER_TITLE_SIZE = 24.sp
private val GRAMS_STEPPER_WIDTH = 180.dp

/**
 * «Ингредиент» — the ingredient picker (`docs/specs/kmp-nutrition-and-recipes.md`'s
 * step-11). Takes a [search] lambda and reports the choice via [onAdd], never a
 * repository: the rule every screen in this context follows
 * (`RecipesScreen.kt:101-106`).
 *
 * Grams resets to [DEFAULT_GRAMS] whenever a **different** product is picked, but
 * survives re-tapping the one already selected — [pick] below is the one place that
 * decides, matching `RecipeBuilderScreen.tsx:130-132`'s own `setGrams(100)` on select.
 */
@Composable
fun IngredientPickerSheet(
    search: suspend (String) -> List<Ingredient>,
    modifier: Modifier = Modifier,
    onClose: () -> Unit = { },
    onAdd: (RecipeIngredient) -> Unit = { },
) {
    var query by remember { mutableStateOf("") }
    var results by remember { mutableStateOf<List<Ingredient>>(emptyList()) }
    var selected by remember { mutableStateOf<Ingredient?>(null) }
    var grams by remember { mutableStateOf(DEFAULT_GRAMS) }

    // Re-runs on every keystroke, not only on submit: `RecipeRepository.ingredients`
    // (step-7) already trims and matches case-insensitively, so nothing here
    // re-filters its answer.
    LaunchedEffect(query) { results = search(query) }

    fun pick(ingredient: Ingredient) {
        if (selected?.id != ingredient.id) grams = DEFAULT_GRAMS
        selected = ingredient
    }

    Column(
        modifier
            .fillMaxSize()
            .background(Cadence.palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars)
            .testTag(CADENCE_INGREDIENT_PICKER_TAG),
    ) {
        IngredientPickerHeader(onClose)
        IngredientPickerSearchField(query = query, onQueryChange = { query = it })

        Column(
            Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = CadenceSpacing.lg),
        ) {
            if (results.isEmpty()) {
                CadenceBody(
                    NOT_FOUND_MESSAGE,
                    color = Cadence.palette.subtle,
                    modifier =
                        Modifier
                            .padding(vertical = CadenceSpacing.md)
                            .testTag(CADENCE_INGREDIENT_PICKER_EMPTY_TAG),
                )
            } else {
                results.forEach { ingredient ->
                    IngredientPickerRow(
                        ingredient = ingredient,
                        isSelected = ingredient.id == selected?.id,
                        onClick = { pick(ingredient) },
                    )
                }
            }
        }

        selected?.let { ingredient ->
            IngredientPickerFooter(
                ingredient = ingredient,
                grams = grams,
                onGramsChange = { grams = it },
                onAdd = {
                    onAdd(RecipeIngredient(ingredient.id, grams))
                    onClose()
                },
            )
        }
    }
}

@Composable
private fun IngredientPickerHeader(onClose: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceTitle(HEADER_TITLE, size = HEADER_TITLE_SIZE)
        CadenceIconButton(
            icon = CadenceIcons.xMark,
            contentDescription = CLOSE_DESCRIPTION,
            onClick = onClose,
            background = Cadence.palette.sunk,
            modifier = Modifier.testTag(CADENCE_INGREDIENT_PICKER_CLOSE_TAG),
        )
    }
}

/**
 * The magnifying glass beside the field is the exact call site
 * [app.cadence.design.CadenceTextField]'s own KDoc names for its two-modifier
 * shape (`RecipeBuilderScreen.tsx:187-199`) — [Modifier.weight] on the box via
 * the outer `modifier`, the search's own tag on the editable node via `fieldModifier`.
 */
@Composable
private fun IngredientPickerSearchField(
    query: String,
    onQueryChange: (String) -> Unit,
) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIcon(paths = CadenceIcons.magnifyingGlass, tint = Cadence.palette.subtle)
        CadenceTextField(
            value = query,
            onValueChange = onQueryChange,
            placeholder = SEARCH_PLACEHOLDER,
            singleLine = true,
            modifier = Modifier.weight(1f),
            fieldModifier = Modifier.testTag(CADENCE_INGREDIENT_PICKER_SEARCH_TAG),
        )
    }
}

/** One result row: name, «{ккал} ккал · {белок} г белка / 100 г», highlighted when selected. */
@Composable
private fun IngredientPickerRow(
    ingredient: Ingredient,
    isSelected: Boolean,
    onClick: () -> Unit,
) {
    val palette = Cadence.palette

    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.md))
            .background(if (isSelected) CadenceColors.forest50 else Color.Transparent)
            .pressable(onClick, remember { MutableInteractionSource() })
            .testTag(ingredientPickerRowTag(ingredient.id))
            .semantics { selected = isSelected }
            .padding(CadenceSpacing.md),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column {
            CadenceBody(ingredient.nameRu, color = if (isSelected) CadenceColors.forest800 else palette.ink)
            CadenceMeta(referenceLabel(ingredient), color = palette.subtle)
        }
        if (isSelected) CadenceIcon(paths = CadenceIcons.checkCircle, tint = CadenceColors.forest700)
    }
}

/** «{ккал} ккал · {белок} г белка / 100 г» — the reference row's own text, per the spec's wording. */
private fun referenceLabel(ingredient: Ingredient): String =
    "${formatKcalTenths(ingredient.per100g.kcalTenths)} · ${proteinLabel(ingredient.per100g.proteinGTenths)} / 100 г"

/**
 * A [app.cadence.shared.domain.MacrosTenths] protein field, rounded for display without
 * a second `toMacros()` — the same reason [formatKcalTenths] gives for kcal, mirroring
 * the file-private `tenthsLabel` idiom `NutritionMealFeed.kt`/`LogMealItemsList.kt`
 * already use for this exact field. `toMacros()` stays the single conversion, at the
 * `TodaySummary` boundary (`Nutrition.kt:62-77`).
 */
private fun proteinLabel(proteinGTenths: Int): String =
    "${formatDecimal(proteinGTenths / TENTHS_PER_UNIT, digits = 0)} г белка"

/**
 * Appears only once a product is selected: name, the live recalculation at [grams] via
 * [macrosFor] (never a stored total), the grams stepper and «Добавить».
 */
@Composable
private fun IngredientPickerFooter(
    ingredient: Ingredient,
    grams: Int,
    onGramsChange: (Int) -> Unit,
    onAdd: () -> Unit,
) {
    val palette = Cadence.palette
    val macros = ingredient.macrosFor(grams)

    Column(
        Modifier
            .fillMaxWidth()
            .background(palette.bg)
            // Unverifiable by test: WindowInsets.navigationBars is zero in every test
            // configuration. Kept for parity with every sibling bottom bar
            // (`RecipeDetailScreen.kt`, `RecipesScreen.kt` and `TodayScreen.kt` all
            // pad by this same inset).
            .windowInsetsPadding(WindowInsets.navigationBars)
            .testTag(CADENCE_INGREDIENT_PICKER_FOOTER_TAG),
    ) {
        CadenceHairline()
        Column(Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md)) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                CadenceMeta(ingredient.nameRu, color = palette.muted)
                CadenceMeta(
                    "${formatKcalTenths(macros.kcalTenths)} · ${proteinLabel(macros.proteinGTenths)}",
                    color = CadenceColors.forest700,
                )
            }
            Row(
                Modifier.fillMaxWidth().padding(top = CadenceSpacing.md),
                horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                CadenceStepper(
                    value = grams.toDouble(),
                    onChange = { onGramsChange(it.roundToInt()) },
                    min = GRAMS_MIN,
                    max = GRAMS_MAX,
                    step = GRAMS_STEP,
                    decimals = 0,
                    unit = "г",
                    modifier = Modifier.width(GRAMS_STEPPER_WIDTH),
                )
                CadenceButton(
                    label = ADD_LABEL,
                    onClick = onAdd,
                    kind = CadenceButtonKind.PRIMARY,
                    fillWidth = true,
                    modifier = Modifier.weight(1f).testTag(CADENCE_INGREDIENT_PICKER_ADD_TAG),
                )
            }
        }
    }
}
