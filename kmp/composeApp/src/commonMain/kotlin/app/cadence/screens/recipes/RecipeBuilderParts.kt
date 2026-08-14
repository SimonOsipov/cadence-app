package app.cadence.screens.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceNumber
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceSplitBar
import app.cadence.design.CadenceStepper
import app.cadence.design.CadenceTextField
import app.cadence.design.pressable
import app.cadence.format.formatDecimal
import app.cadence.format.formatInteger
import app.cadence.format.formatKcalTenths
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.macrosFor
import app.cadence.shared.domain.toMacros
import kotlin.math.roundToInt

// The builder's three lower sections, split out of `RecipeBuilderScreen.kt` the same reason
// `RecipeRow.kt` was split out of `RecipesScreen.kt` — one file holding all of it is simply
// too long to hold in view.

/** The «Добавьте первый ингредиент» box, which is itself a door to the sheet. */
const val CADENCE_RECIPE_BUILDER_EMPTY_TAG = "cadence-recipe-builder-empty"

/** The live «На порцию» card, drawn only once there is something to price. */
const val CADENCE_RECIPE_BUILDER_PER_SERVING_TAG = "cadence-recipe-builder-per-serving"

const val CADENCE_RECIPE_BUILDER_ADD_STEP_TAG = "cadence-recipe-builder-add-step"

/** One ingredient row, so a test can reach the *second* row's own stepper. */
fun recipeBuilderIngredientTag(index: Int): String = "cadence-recipe-builder-ingredient-$index"

fun recipeBuilderIngredientRemoveTag(index: Int): String = "cadence-recipe-builder-ingredient-remove-$index"

fun recipeBuilderStepTag(index: Int): String = "cadence-recipe-builder-step-$index"

fun recipeBuilderStepRemoveTag(index: Int): String = "cadence-recipe-builder-step-remove-$index"

private const val EMPTY_MESSAGE = "Добавьте первый ингредиент"
private const val STEP_PLACEHOLDER = "Опишите шаг"
private const val REMOVE_DESCRIPTION = "Удалить"

/**
 * Same floor, ceiling and rate as the picker sheet's own grams stepper (step-11). The
 * ceiling is a deliberate divergence: the prototype's row clamps only `Math.max(5, g)`
 * (`RecipeBuilderScreen.tsx:383`) and has no upper bound. Registered in the spec's step-15.
 */
private const val ROW_GRAMS_MIN = 5.0
private const val ROW_GRAMS_MAX = 600.0
private const val ROW_GRAMS_STEP = 10.0
private const val TENTHS_PER_UNIT = 10.0

private val CARD_SHAPE = RoundedCornerShape(CadenceRadius.lg)
private val HAIRLINE = 1.dp
private val EMPTY_BORDER = 1.5.dp
private val EMPTY_DASH = 6.dp
private val STEP_BADGE_SIZE = 30.dp
private val PER_SERVING_KCAL_SIZE = 22.sp

/**
 * The ingredient list: the empty state, or a card of rows each with its name, its own
 * kcal/protein line, a grams stepper and a delete (`RecipeBuilderScreen.tsx:649-768`).
 *
 * The grams stepper sits under the name rather than beside it: at 343dp a row of
 * name + stepper + delete leaves the stepper well under the ~150dp its two 52dp buttons
 * and padded number need. Pinned by a bounds test.
 */
@Composable
internal fun RecipeBuilderIngredients(
    rows: List<BuilderRow>,
    onGrams: (Int, Int) -> Unit,
    onRemove: (Int) -> Unit,
    onAdd: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Column(modifier.fillMaxWidth()) {
        if (rows.isEmpty()) {
            RecipeBuilderEmptyIngredients(onAdd)
            return@Column
        }

        Column(
            Modifier
                .fillMaxWidth()
                .clip(CARD_SHAPE)
                .background(palette.paper, CARD_SHAPE)
                .border(HAIRLINE, palette.hairline, CARD_SHAPE),
        ) {
            rows.forEachIndexed { index, row ->
                RecipeBuilderIngredientRow(
                    index = index,
                    ingredient = row.ingredient,
                    grams = row.grams,
                    onGrams = { onGrams(index, it) },
                    onRemove = { onRemove(index) },
                )
                if (index != rows.lastIndex) CadenceHairline()
            }
        }
    }
}

@Composable
private fun RecipeBuilderEmptyIngredients(onAdd: () -> Unit) {
    val palette = Cadence.palette

    Row(
        Modifier
            .fillMaxWidth()
            .clip(CARD_SHAPE)
            .pressable(onAdd, remember { MutableInteractionSource() })
            .dashedBorder(palette.border)
            .testTag(CADENCE_RECIPE_BUILDER_EMPTY_TAG)
            .padding(CadenceSpacing.xl),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIcon(paths = CadenceIcons.plus, tint = palette.subtle)
        CadenceBody(EMPTY_MESSAGE, color = palette.subtle)
    }
}

/** `borderStyle: 'dashed'` (`RecipeBuilderScreen.tsx:659`) — Compose has no dashed border modifier. */
private fun Modifier.dashedBorder(color: Color) =
    drawBehind {
        val stroke = EMPTY_BORDER.toPx()
        val dash = EMPTY_DASH.toPx()
        // Inset by half the stroke: a stroke is centred on its path, so an un-inset
        // rect loses its outer half to clipping.
        drawRoundRect(
            color = color,
            topLeft = Offset(stroke / 2, stroke / 2),
            size = Size(size.width - stroke, size.height - stroke),
            cornerRadius = CornerRadius(CadenceRadius.lg.toPx()),
            style = Stroke(width = stroke, pathEffect = PathEffect.dashPathEffect(floatArrayOf(dash, dash))),
        )
    }

@Composable
private fun RecipeBuilderIngredientRow(
    index: Int,
    ingredient: Ingredient,
    grams: Int,
    onGrams: (Int) -> Unit,
    onRemove: () -> Unit,
) {
    val palette = Cadence.palette
    val macros = ingredient.macrosFor(grams)

    Column(
        Modifier
            .fillMaxWidth()
            .testTag(recipeBuilderIngredientTag(index))
            .padding(horizontal = CadenceSpacing.md, vertical = CadenceSpacing.sm),
    ) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                CadenceBody(ingredient.nameRu, maxLines = 1)
                CadenceMeta(rowMacrosLabel(macros), color = palette.subtle)
            }
            CadenceIconButton(
                icon = CadenceIcons.xMark,
                contentDescription = REMOVE_DESCRIPTION,
                onClick = onRemove,
                tint = palette.placeholder,
                modifier = Modifier.testTag(recipeBuilderIngredientRemoveTag(index)),
            )
        }
        CadenceStepper(
            value = grams.toDouble(),
            onChange = { onGrams(it.roundToInt()) },
            min = ROW_GRAMS_MIN,
            max = ROW_GRAMS_MAX,
            step = ROW_GRAMS_STEP,
            decimals = 0,
            unit = "г",
        )
    }
}

/** «{ккал} ккал · {белок} б» — the prototype's own abbreviation (`RecipeBuilderScreen.tsx:713`). */
private fun rowMacrosLabel(macros: MacrosTenths): String =
    "${formatKcalTenths(macros.kcalTenths)} · ${formatDecimal(macros.proteinGTenths / TENTHS_PER_UNIT, digits = 1)} б"

/**
 * The live «На порцию» card (`RecipeBuilderScreen.tsx:770-819`): dark, and recomputed from
 * [perServing] on every grams and servings change rather than from a stored total.
 */
@Composable
internal fun RecipeBuilderPerServingCard(
    perServing: MacrosTenths,
    modifier: Modifier = Modifier,
) {
    val macros = perServing.toMacros()

    Column(
        modifier
            .fillMaxWidth()
            .background(CadenceColors.forest800, CARD_SHAPE)
            .testTag(CADENCE_RECIPE_BUILDER_PER_SERVING_TAG)
            .padding(CadenceSpacing.lg),
    ) {
        Row(
            Modifier.fillMaxWidth().padding(bottom = CadenceSpacing.sm),
            verticalAlignment = Alignment.Bottom,
        ) {
            CadenceEyebrow("На порцию", color = CadenceColors.sand300)
            Spacer(Modifier.weight(1f))
            CadenceNumber(
                value = formatInteger(macros.kcal),
                unit = "ккал",
                size = PER_SERVING_KCAL_SIZE,
                color = CadenceColors.cream,
                unitColor = CadenceColors.sand300,
            )
        }
        CadenceSplitBar(
            proteinG = perServing.proteinGTenths / TENTHS_PER_UNIT,
            carbsG = perServing.carbsGTenths / TENTHS_PER_UNIT,
            fatG = perServing.fatGTenths / TENTHS_PER_UNIT,
            trackColor = CadenceColors.cream.copy(alpha = DARK_TRACK_ALPHA),
            labelColor = CadenceColors.cream.copy(alpha = DARK_LABEL_ALPHA),
            valueColor = CadenceColors.cream,
        )
    }
}

/** `rgba(246,241,234,.14)` and `.6` — cream at the prototype's own two alphas (`:39,56`). */
private const val DARK_TRACK_ALPHA = 0.14f
private const val DARK_LABEL_ALPHA = 0.6f

/** «Приготовление»: a numbered field per step, the last of which cannot be removed (`:822-893`). */
@Composable
internal fun RecipeBuilderSteps(
    steps: List<String>,
    onStep: (Int, String) -> Unit,
    onRemove: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        steps.forEachIndexed { index, step ->
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
                verticalAlignment = Alignment.Top,
            ) {
                StepBadge(index)
                CadenceTextField(
                    value = step,
                    onValueChange = { onStep(index, it) },
                    placeholder = STEP_PLACEHOLDER,
                    minLines = STEP_FIELD_MIN_LINES,
                    modifier = Modifier.weight(1f),
                    fieldModifier = Modifier.testTag(recipeBuilderStepTag(index)),
                )
                if (steps.size > 1) {
                    CadenceIconButton(
                        icon = CadenceIcons.xMark,
                        contentDescription = REMOVE_DESCRIPTION,
                        onClick = { onRemove(index) },
                        tint = Cadence.palette.placeholder,
                        modifier = Modifier.testTag(recipeBuilderStepRemoveTag(index)),
                    )
                }
            }
        }
    }
}

private const val STEP_FIELD_MIN_LINES = 2

@Composable
private fun StepBadge(index: Int) {
    Box(
        Modifier
            .size(STEP_BADGE_SIZE)
            .clip(CircleShape)
            .background(CadenceColors.forest50),
        contentAlignment = Alignment.Center,
    ) {
        BasicText(
            text = "${index + 1}",
            style = Cadence.typography.number.copy(color = CadenceColors.forest700, fontSize = 14.sp),
        )
    }
}
