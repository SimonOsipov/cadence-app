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
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
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
import app.cadence.format.formatInteger
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.MacrosTenths
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

const val CADENCE_RECIPE_BUILDER_NAME_TAG = "cadence-recipe-builder-name"

/**
 * The two stepper cards, so a test can reach *this* stepper's plus button:
 * `CadenceStepper` tags its own buttons, and this screen draws more than one.
 */
const val CADENCE_RECIPE_BUILDER_SERVINGS_TAG = "cadence-recipe-builder-servings"
const val CADENCE_RECIPE_BUILDER_TIME_TAG = "cadence-recipe-builder-time"

/** «Добавить» beside the «Ингредиенты» heading — the only way to add a second row. */
const val CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG = "cadence-recipe-builder-add-ingredient"

const val CADENCE_RECIPE_BUILDER_SAVE_TAG = "cadence-recipe-builder-save"

/** The whole save bar, so a test can measure it against [SAVE_BAR_CLEARANCE]. */
const val CADENCE_RECIPE_BUILDER_SAVE_BAR_TAG = "cadence-recipe-builder-save-bar"

/** The tag row, so a test can measure whether its three chips fit a phone. */
const val CADENCE_RECIPE_BUILDER_TAGS_TAG = "cadence-recipe-builder-tags"

fun recipeBuilderMealTypeTag(type: MealType): String = "cadence-recipe-builder-type-${type.code}"

fun recipeBuilderTagTag(tag: RecipeTag): String = "cadence-recipe-builder-tag-${tag.code}"

private const val HEADER_TITLE = "Новый рецепт"
private const val CANCEL_DESCRIPTION = "Закрыть"
private const val NAME_PLACEHOLDER = "Название рецепта"
private const val SAVE_LABEL = "Сохранить рецепт"

private const val RECIPE_BUILDER_MIN_SERVINGS = 1
private const val RECIPE_BUILDER_MAX_SERVINGS = 8
private const val RECIPE_BUILDER_DEFAULT_SERVINGS = 2
private const val RECIPE_BUILDER_MIN_TIME = 5
private const val RECIPE_BUILDER_MAX_TIME = 120
private const val RECIPE_BUILDER_TIME_STEP = 5
private const val RECIPE_BUILDER_DEFAULT_TIME = 20

private val HEADER_TITLE_SIZE = 15.sp

/** `fontSize: 28` on the prototype's own name input (`RecipeBuilderScreen.tsx:502`). */
private val NAME_FIELD_SIZE = 28.sp
private val CARD_SHAPE = RoundedCornerShape(CadenceRadius.lg)
private val HAIRLINE = 1.dp

/**
 * Clears the fixed save bar, which is overlaid on the scrolling content: a `LARGE`
 * `CadenceButton` (`CadenceControls.kt`) inside a bar padded `md` top and bottom, plus air.
 * Measured, not guessed — `theSaveBarFitsInsideItsOwnClearance` fails if the bar outgrows it.
 * The prototype's own `paddingBottom: 140` (`:487`) also covers its gradient fade, which
 * this port does not draw.
 */
internal val SAVE_BAR_CLEARANCE = 96.dp

/**
 * «Новый рецепт» (`CadenceRoute.RecipeBuilder`).
 *
 * Takes a [search] lambda, never a repository — the rule every screen in this context
 * follows (`RecipesScreen.kt:96-97`). No ingredient *table* comes in beside it: each row
 * carries the [Ingredient] the sheet handed over, so pricing a row cannot fail on an id
 * the caller's table happens not to hold.
 *
 * [onSave] receives a [RecipeDraft] whose `cookMin` is **null**, not zero: the prototype
 * writes `cookMin: 0` (`RecipeBuilderScreen.tsx:400`), which asserts «cooks in zero
 * minutes» where the form only ever asked one time question. A deliberate divergence,
 * named by the spec's own step-12.
 */
@Composable
fun RecipeBuilderScreen(
    search: suspend (String) -> List<Ingredient>,
    modifier: Modifier = Modifier,
    onCancel: () -> Unit = { },
    onSave: (RecipeDraft) -> Unit = { },
) {
    val form = remember { RecipeBuilderFormState() }
    var pickerOpen by remember { mutableStateOf(false) }

    Box(modifier.fillMaxSize().background(Cadence.palette.bg)) {
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
                RecipeBuilderTopFields(form)
                RecipeBuilderContents(form, onAddIngredient = { pickerOpen = true })
                Spacer(
                    Modifier.height(
                        SAVE_BAR_CLEARANCE + WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding(),
                    ),
                )
            }
        }

        RecipeBuilderSaveBar(
            enabled = form.canSave,
            // Built here rather than on every recomposition: the fold and the generated
            // description are only ever needed by this one tap.
            onSave = { onSave(form.draft()) },
            modifier = Modifier.align(Alignment.BottomCenter).windowInsetsPadding(WindowInsets.navigationBars),
        )

        CadenceSheet(open = pickerOpen, onDismiss = { pickerOpen = false }) {
            IngredientPickerSheet(
                search = search,
                onClose = { pickerOpen = false },
                onAdd = form::addRow,
                modifier = Modifier.fillMaxHeight(SHEET_HEIGHT_FRACTION),
            )
        }
    }
}

/** `contentStyle={{ height: '78%' }}` on the prototype's own sheet (`RecipeBuilderScreen.tsx:933`). */
private const val SHEET_HEIGHT_FRACTION = 0.78f

/** One picked row: the product itself, so nothing downstream has to resolve an id. */
internal data class BuilderRow(
    val ingredient: Ingredient,
    val grams: Int,
)

/**
 * Everything the form holds. A class rather than seven `remember`s in the screen: the two
 * section composables and [draft] all read the same fields, and threading them one by one
 * pushed the screen past detekt's length limit.
 *
 * The lists are exposed read-only and mutate only through the methods below, so the one
 * invariant this type carries — a recipe always has at least one step row — cannot be
 * stepped around by a caller in the same file.
 */
@Stable
internal class RecipeBuilderFormState {
    var name by mutableStateOf("")
    var mealType by mutableStateOf(MealType.LUNCH)
    var servings by mutableStateOf(RECIPE_BUILDER_DEFAULT_SERVINGS)
    var timeMin by mutableStateOf(RECIPE_BUILDER_DEFAULT_TIME)

    private val mutableTags = mutableStateListOf<RecipeTag>()
    private val mutableRows = mutableStateListOf<BuilderRow>()

    /** One empty step from the start, as the prototype opens (`RecipeBuilderScreen.tsx:373`). */
    private val mutableSteps = mutableStateListOf("")

    val tags: List<RecipeTag> get() = mutableTags
    val rows: List<BuilderRow> get() = mutableRows
    val steps: List<String> get() = mutableSteps

    /** The same two conditions [RecipeDraft.canSave] states, asked before a draft exists. */
    val canSave: Boolean get() = name.isNotBlank() && mutableRows.isNotEmpty()

    fun toggleTag(tag: RecipeTag) {
        if (!mutableTags.remove(tag)) mutableTags.add(tag)
    }

    fun addRow(
        ingredient: Ingredient,
        grams: Int,
    ) {
        mutableRows.add(BuilderRow(ingredient, grams))
    }

    fun setGrams(
        index: Int,
        grams: Int,
    ) {
        mutableRows[index] = mutableRows[index].copy(grams = grams)
    }

    fun removeRow(index: Int) {
        mutableRows.removeAt(index)
    }

    fun addStep() {
        mutableSteps.add("")
    }

    fun setStep(
        index: Int,
        text: String,
    ) {
        mutableSteps[index] = text
    }

    fun removeStep(index: Int) {
        if (mutableSteps.size > 1) mutableSteps.removeAt(index)
    }

    /** One source for both the live card and the generated description — never two folds. */
    fun perServing(): MacrosTenths =
        mutableRows
            .map { RecipeIngredient(it.ingredient.id, it.grams) }
            .perServingOf(mutableRows.map { it.ingredient }, servings)

    /**
     * [RecipeDraft.prepMin] takes the one time the form asks for; `cookMin` stays null. The
     * description is generated, never typed — the prototype's own `dek` (`:403`).
     */
    fun draft(): RecipeDraft {
        val perServing = perServing().toMacros()
        return RecipeDraft(
            name = name.trim().ifBlank { null },
            mealType = mealType,
            tags = tags.toList(),
            servings = servings,
            prepMin = timeMin,
            cookMin = null,
            dek = "${formatInteger(perServing.proteinG)} г белка · ${formatInteger(perServing.kcal)} ккал на порцию.",
            ingredients = mutableRows.map { RecipeIngredient(it.ingredient.id, it.grams) },
            // A blank step is a row the patient added and never filled, not an instruction.
            steps = mutableSteps.mapNotNull { it.trim().ifBlank { null } },
        )
    }
}

@Composable
private fun RecipeBuilderTopFields(form: RecipeBuilderFormState) {
    RecipeBuilderNameField(
        name = form.name,
        onName = { form.name = it },
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
    )

    RecipeBuilderTypeAndTags(
        mealType = form.mealType,
        onMealType = { form.mealType = it },
        tags = form.tags,
        onToggleTag = form::toggleTag,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
    )

    RecipeBuilderCounts(
        servings = form.servings,
        onServings = { form.servings = it },
        timeMin = form.timeMin,
        onTimeMin = { form.timeMin = it },
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
    )
}

@Composable
private fun RecipeBuilderContents(
    form: RecipeBuilderFormState,
    onAddIngredient: () -> Unit,
) {
    RecipeBuilderSectionHeading(
        title = "Ингредиенты",
        action = "Добавить",
        actionTag = CADENCE_RECIPE_BUILDER_ADD_INGREDIENT_TAG,
        onAction = onAddIngredient,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg).padding(top = CadenceSpacing.md),
    )

    RecipeBuilderIngredients(
        rows = form.rows,
        onGrams = form::setGrams,
        onRemove = form::removeRow,
        onAdd = onAddIngredient,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
    )

    if (form.rows.isNotEmpty()) {
        RecipeBuilderPerServingCard(
            perServing = form.perServing(),
            modifier = Modifier.padding(horizontal = CadenceSpacing.lg).padding(bottom = CadenceSpacing.sm),
        )
    }

    RecipeBuilderSectionHeading(
        title = "Приготовление",
        action = "Шаг",
        actionTag = CADENCE_RECIPE_BUILDER_ADD_STEP_TAG,
        onAction = form::addStep,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg).padding(top = CadenceSpacing.md),
    )

    RecipeBuilderSteps(
        steps = form.steps,
        onStep = form::setStep,
        onRemove = form::removeStep,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
    )
}

/**
 * The recipe's name, drawn as a title over a single rule rather than in a bordered box
 * (`RecipeBuilderScreen.tsx:492-508`): the prototype's one display-font input, and the
 * only field on this screen whose value is a heading rather than a form value. Not
 * [app.cadence.design.CadenceTextField], which is the bordered-box shape its own KDoc
 * describes — a different role, not a different skin.
 */
@Composable
private fun RecipeBuilderNameField(
    name: String,
    onName: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val style = Cadence.typography.title.copy(color = palette.ink, fontSize = NAME_FIELD_SIZE)

    Column(modifier.fillMaxWidth()) {
        Box(Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.xxs, vertical = CadenceSpacing.sm)) {
            if (name.isEmpty()) BasicText(text = NAME_PLACEHOLDER, style = style.copy(color = palette.placeholder))
            BasicTextField(
                value = name,
                onValueChange = onName,
                textStyle = style,
                singleLine = true,
                modifier = Modifier.fillMaxWidth().testTag(CADENCE_RECIPE_BUILDER_NAME_TAG),
            )
        }
        CadenceHairline()
    }
}

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
 *
 * The tag row scrolls where the prototype wraps (`:553`, `flexWrap: 'wrap'`). Measured
 * rather than assumed: `everyTagChipIsFullyVisibleAt343dp` holds all three chips inside a
 * 343dp screen, so nothing is reachable only by an invisible swipe.
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
            Modifier.horizontalScroll(rememberScrollState()).testTag(CADENCE_RECIPE_BUILDER_TAGS_TAG),
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
            min = RECIPE_BUILDER_MIN_SERVINGS,
            max = RECIPE_BUILDER_MAX_SERVINGS,
            step = 1,
            tag = CADENCE_RECIPE_BUILDER_SERVINGS_TAG,
        )
        RecipeBuilderCountCard(
            label = "Время, мин",
            value = timeMin,
            onChange = onTimeMin,
            min = RECIPE_BUILDER_MIN_TIME,
            max = RECIPE_BUILDER_MAX_TIME,
            step = RECIPE_BUILDER_TIME_STEP,
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
            .testTag(CADENCE_RECIPE_BUILDER_SAVE_BAR_TAG)
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
