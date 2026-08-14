package app.cadence.screens.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceButtonSize
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSheet
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceStepper
import app.cadence.design.CadenceTextField
import app.cadence.format.formatInteger
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.RecipeIngredient
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.domain.perServingOf
import app.cadence.shared.domain.toMacros
import app.cadence.shared.repository.RecipeDraft
import kotlin.math.roundToInt

// «Новый рецепт» — the recipe builder (`CadenceRoute.RecipeBuilder`), ported
// from mobile/src/features/recipe/RecipeBuilderScreen.tsx's builder half; its
// `IngredientPicker` half shipped as `IngredientPickerSheet` in step-11.
//
// Step-12 of docs/specs/kmp-nutrition-and-recipes.md. Saving hands over a
// `RecipeDraft` and stops there: `RecipeRepository.save`, the `replaceRoute`
// onto the saved recipe and the `reloads` bump are step-13's, exactly as
// `RecipeDetailScreen`'s own «Добавить в день →» leaves its `MealDraft`.

/** The scrolling body, so a test can reach the screen without knowing its shape. */
const val CADENCE_RECIPE_BUILDER_TAG = "cadence-recipe-builder"

const val CADENCE_RECIPE_BUILDER_CLOSE_TAG = "cadence-recipe-builder-close"

/** The editable name node, so a test can type into it. */
const val CADENCE_RECIPE_BUILDER_NAME_TAG = "cadence-recipe-builder-name"

/**
 * The two stepper cards, so a test can reach *this* stepper's plus button:
 * `CadenceStepper` tags its own buttons, and this screen draws four steppers.
 */
const val CADENCE_RECIPE_BUILDER_SERVINGS_TAG = "cadence-recipe-builder-servings"
const val CADENCE_RECIPE_BUILDER_TIME_TAG = "cadence-recipe-builder-time"

/** «Добавить» beside the «Ингредиенты» heading — the only way to add a second row. */
const val CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG = "cadence-recipe-builder-add-ingredient"

const val CADENCE_RECIPE_BUILDER_SAVE_TAG = "cadence-recipe-builder-save"

fun recipeBuilderMealTypeTag(type: MealType): String = "cadence-recipe-builder-type-${type.code}"

fun recipeBuilderTagTag(tag: RecipeTag): String = "cadence-recipe-builder-tag-${tag.code}"

private const val HEADER_TITLE = "Новый рецепт"
private const val CANCEL_DESCRIPTION = "Закрыть"
private const val NAME_PLACEHOLDER = "Название рецепта"
private const val SAVE_LABEL = "Сохранить рецепт"

internal const val MIN_SERVINGS = 1
internal const val MAX_SERVINGS = 8
private const val DEFAULT_SERVINGS = 2
private const val MIN_TIME_MIN = 5
private const val MAX_TIME_MIN = 120
private const val TIME_STEP_MIN = 5
private const val DEFAULT_TIME_MIN = 20

private val HEADER_TITLE_SIZE = 15.sp
private val CARD_SHAPE = RoundedCornerShape(CadenceRadius.lg)
private val HAIRLINE = 1.dp

/** Keeps the last piece of scrolling content clear of the fixed save bar overlaid above it. */
private val SAVE_BAR_CLEARANCE = 96.dp

/**
 * «Новый рецепт» (`CadenceRoute.RecipeBuilder`).
 *
 * Takes the ingredient table and a [search] lambda, never a repository — the rule every
 * screen in this context follows (`RecipesScreen.kt:101-106`). [ingredients] prices the
 * rows the patient picks; [search] is what the embedded [IngredientPickerSheet] queries.
 *
 * [onSave] receives a [RecipeDraft] whose `cookMin` is **null**, not zero: the prototype
 * writes `cookMin: 0` (`RecipeBuilderScreen.tsx:400`), which asserts «cooks in zero
 * minutes» where the form only ever asked one time question. A deliberate divergence,
 * named by the spec's own step-12.
 */
@Composable
fun RecipeBuilderScreen(
    ingredients: List<Ingredient>,
    search: suspend (String) -> List<Ingredient>,
    modifier: Modifier = Modifier,
    onCancel: () -> Unit = { },
    onSave: (RecipeDraft) -> Unit = { },
) {
    val palette = Cadence.palette
    var name by remember { mutableStateOf("") }
    var mealType by remember { mutableStateOf(MealType.LUNCH) }
    val tags = remember { mutableStateListOf<RecipeTag>() }
    var servings by remember { mutableStateOf(DEFAULT_SERVINGS) }
    var timeMin by remember { mutableStateOf(DEFAULT_TIME_MIN) }
    val rows = remember { mutableStateListOf<RecipeIngredient>() }
    var pickerOpen by remember { mutableStateOf(false) }
    val navigationBarInset = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()

    val perServing = rows.toList().perServingOf(ingredients, servings).toMacros()
    val draft =
        RecipeDraft(
            name = name.trim().ifBlank { null },
            mealType = mealType,
            tags = tags.toList(),
            servings = servings,
            prepMin = timeMin,
            cookMin = null,
            dek = "${formatInteger(perServing.proteinG)} г белка · ${formatInteger(perServing.kcal)} ккал на порцию.",
            ingredients = rows.toList(),
            steps = emptyList(),
        )

    Box(modifier.fillMaxSize().background(palette.bg)) {
        Column(Modifier.fillMaxSize().windowInsetsPadding(WindowInsets.statusBars)) {
            RecipeBuilderHeader(onCancel)
            CadenceHairline()

            Column(
                Modifier
                    .fillMaxWidth()
                    .weight(1f)
                    .verticalScroll(rememberScrollState())
                    .testTag(CADENCE_RECIPE_BUILDER_TAG),
            ) {
                CadenceTextField(
                    value = name,
                    onValueChange = { name = it },
                    placeholder = NAME_PLACEHOLDER,
                    singleLine = true,
                    modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
                    fieldModifier = Modifier.testTag(CADENCE_RECIPE_BUILDER_NAME_TAG),
                )

                RecipeBuilderTypeAndTags(
                    mealType = mealType,
                    onMealType = { mealType = it },
                    tags = tags.toList(),
                    onToggleTag = { tag -> if (tag in tags) tags.remove(tag) else tags.add(tag) },
                    modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
                )

                RecipeBuilderCounts(
                    servings = servings,
                    onServings = { servings = it },
                    timeMin = timeMin,
                    onTimeMin = { timeMin = it },
                    modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
                )

                RecipeBuilderSectionHeading(
                    title = "Ингредиенты",
                    action = "Добавить",
                    actionTag = CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG,
                    onAction = { pickerOpen = true },
                    modifier = Modifier.padding(horizontal = CadenceSpacing.lg).padding(top = CadenceSpacing.md),
                )

                Spacer(Modifier.height(SAVE_BAR_CLEARANCE + navigationBarInset))
            }
        }

        RecipeBuilderSaveBar(
            enabled = draft.canSave(),
            onSave = { onSave(draft) },
            modifier = Modifier.align(Alignment.BottomCenter).windowInsetsPadding(WindowInsets.navigationBars),
        )

        CadenceSheet(open = pickerOpen, onDismiss = { pickerOpen = false }) {
            IngredientPickerSheet(
                search = search,
                onClose = { pickerOpen = false },
                onAdd = { row -> rows.add(row) },
                modifier = Modifier.fillMaxHeight(SHEET_HEIGHT_FRACTION),
            )
        }
    }
}

/** `contentStyle={{ height: '78%' }}` on the prototype's own sheet (`RecipeBuilderScreen.tsx:933`). */
private const val SHEET_HEIGHT_FRACTION = 0.78f

@Composable
private fun RecipeBuilderHeader(onCancel: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIconButton(
            icon = CadenceIcons.xMark,
            contentDescription = CANCEL_DESCRIPTION,
            onClick = onCancel,
            background = Cadence.palette.sunk,
            modifier = Modifier.testTag(CADENCE_RECIPE_BUILDER_CLOSE_TAG),
        )
        BasicText(
            text = HEADER_TITLE,
            style =
                Cadence.typography.label.copy(
                    color = Cadence.palette.ink,
                    fontSize = HEADER_TITLE_SIZE,
                    fontWeight = FontWeight.SemiBold,
                ),
        )
    }
}

/**
 * «Тип приёма» as single choice and the tag row as a set — the prototype's own two
 * handlers (`RecipeBuilderScreen.tsx:529,559`): the type has no "none", so tapping the
 * selected one changes nothing, while a tag toggles off.
 */
@Composable
private fun RecipeBuilderTypeAndTags(
    mealType: MealType,
    onMealType: (MealType) -> Unit,
    tags: List<RecipeTag>,
    onToggleTag: (RecipeTag) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        CadenceEyebrow("Тип приёма")
        Row(
            Modifier.horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        ) {
            MealType.entries.forEach { type ->
                RecipeChip(
                    label = mealTypeLabel(type),
                    active = type == mealType,
                    onClick = { onMealType(type) },
                    modifier = Modifier.testTag(recipeBuilderMealTypeTag(type)),
                )
            }
        }
        Row(
            Modifier.horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        ) {
            RecipeTag.entries.forEach { tag ->
                RecipeChip(
                    label = tagLabel(tag),
                    active = tag in tags,
                    onClick = { onToggleTag(tag) },
                    modifier = Modifier.testTag(recipeBuilderTagTag(tag)),
                    activeBackground = CadenceColors.sand500,
                    activeForeground = CadenceColors.ink900,
                )
            }
        }
    }
}

/**
 * «Порций» and «Время, мин».
 *
 * Stacked, not the prototype's two side-by-side cards (`RecipeBuilderScreen.tsx:585-647`):
 * `CadenceStepper` is two 52dp buttons around an `xl`-padded number, and half of a 343dp
 * phone leaves it under 140dp — the squeeze that flattened the ingredient sheet's plus
 * button in step-11, in two more places. Pinned by a bounds test, not by eye.
 */
@Composable
private fun RecipeBuilderCounts(
    servings: Int,
    onServings: (Int) -> Unit,
    timeMin: Int,
    onTimeMin: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        RecipeBuilderCountCard(
            label = "Порций",
            value = servings,
            onChange = onServings,
            min = MIN_SERVINGS,
            max = MAX_SERVINGS,
            step = 1,
            tag = CADENCE_RECIPE_BUILDER_SERVINGS_TAG,
        )
        RecipeBuilderCountCard(
            label = "Время, мин",
            value = timeMin,
            onChange = onTimeMin,
            min = MIN_TIME_MIN,
            max = MAX_TIME_MIN,
            step = TIME_STEP_MIN,
            tag = CADENCE_RECIPE_BUILDER_TIME_TAG,
        )
    }
}

@Composable
private fun RecipeBuilderCountCard(
    label: String,
    value: Int,
    onChange: (Int) -> Unit,
    min: Int,
    max: Int,
    step: Int,
    tag: String,
) {
    val palette = Cadence.palette

    Column(
        Modifier
            .fillMaxWidth()
            .background(palette.paper, CARD_SHAPE)
            .border(HAIRLINE, palette.hairline, CARD_SHAPE)
            .testTag(tag)
            .padding(horizontal = CadenceSpacing.md, vertical = CadenceSpacing.md),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        CadenceMeta(label, color = palette.subtle)
        CadenceStepper(
            value = value.toDouble(),
            onChange = { onChange(it.roundToInt()) },
            min = min.toDouble(),
            max = max.toDouble(),
            step = step.toDouble(),
            decimals = 0,
        )
    }
}

/** A section heading with its own action — «Ингредиенты» + «Добавить» (`RecipeBuilderScreen.tsx:409-430`). */
@Composable
internal fun RecipeBuilderSectionHeading(
    title: String,
    action: String,
    actionTag: String,
    onAction: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceEyebrow(title)
        CadenceButton(
            label = action,
            onClick = onAction,
            icon = CadenceIcons.plus,
            kind = CadenceButtonKind.GHOST,
            size = CadenceButtonSize.SMALL,
            modifier = Modifier.testTag(actionTag),
        )
    }
}

@Composable
private fun RecipeBuilderSaveBar(
    enabled: Boolean,
    onSave: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier
            .fillMaxWidth()
            .background(Cadence.palette.bg)
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
    ) {
        CadenceButton(
            label = SAVE_LABEL,
            onClick = onSave,
            kind = CadenceButtonKind.PRIMARY,
            size = CadenceButtonSize.LARGE,
            icon = CadenceIcons.check,
            fillWidth = true,
            enabled = enabled,
            modifier = Modifier.weight(1f).testTag(CADENCE_RECIPE_BUILDER_SAVE_TAG),
        )
    }
}
