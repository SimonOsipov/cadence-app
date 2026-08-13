package app.cadence.screens.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceButtonSize
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceElevation
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceNumber
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSegmented
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceSplitBar
import app.cadence.design.CadenceStepper
import app.cadence.design.CadenceTitle
import app.cadence.design.pressable
import app.cadence.format.formatGrams
import app.cadence.format.formatInteger
import app.cadence.format.formatKcalTenths
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.Recipe
import app.cadence.shared.domain.macrosFor
import app.cadence.shared.domain.perServing
import app.cadence.shared.domain.toMacros
import app.cadence.shared.domain.toMealDraft
import app.cadence.shared.domain.totalTimeMin
import app.cadence.shared.domain.totals
import kotlin.math.roundToInt

// «Рецепт» — the recipe detail screen (`CadenceRoute.RecipeDetail`), ported
// from mobile/src/features/recipe/RecipeDetailScreen.tsx.
//
// Step-10 of docs/specs/kmp-nutrition-and-recipes.md. Route wiring — turning
// a `RecipeRepository.recipe(id)` suspend read into a [RecipeDetailState] and
// `NutritionRepository.log` into the toast/navigation this screen only
// requests via [RecipeDetailScreen]'s `onAddToDay` — is step-13's job, the
// same split `RecipesScreen.kt`'s own KDoc draws for its own screen.

/** The scrolling body of the «найден» state, so a test can reach it without knowing its shape. */
const val CADENCE_RECIPE_DETAIL_TAG = "cadence-recipe-detail"

/** The «загружается» state's own box. */
const val CADENCE_RECIPE_DETAIL_LOADING_TAG = "cadence-recipe-detail-loading"

/** The «не найден» state's own box. */
const val CADENCE_RECIPE_DETAIL_NOT_FOUND_TAG = "cadence-recipe-detail-not-found"

/**
 * The screen's one way out, whichever state drew it: the floating button over
 * the «найден» content, or the plain header button on «не найден». Both call
 * the same [onBack], so a test that only cares "does this state have an
 * exit" never has to know which shape drew it.
 */
const val CADENCE_RECIPE_DETAIL_BACK_TAG = "cadence-recipe-detail-back"

/** «Добавить в день →», so a test can press it without matching on copy. */
const val CADENCE_RECIPE_DETAIL_ADD_TAG = "cadence-recipe-detail-add"

/** The «Макросы» card, so a test can read its numbers without knowing its layout. */
const val CADENCE_RECIPE_DETAIL_MACROS_TAG = "cadence-recipe-detail-macros"

/** One ingredient row, so a test can address the *second* row and not only the first. */
fun recipeDetailIngredientTag(index: Int): String = "cadence-recipe-detail-ingredient-$index"

/**
 * What `RecipeRepository.recipe(id)` (step-7) has told this screen so far.
 *
 * Three states, not a nullable [Recipe] alone. The spec's own step-10 section
 * names the defect this type exists against: "«загружается» не равно «не
 * найден»" — a suspend read has an in-flight moment a plain `Recipe?` cannot
 * name, and collapsing "not yet answered" into "answered with nothing" is
 * exactly that collapse. `CadenceShell.kt:250,404-425`'s own `onLoadMetric`
 * draws the same distinction one level up, at the call site, by combining two
 * nullable values (`metric != null && detail == null`) — `TrendDetailScreen`
 * itself only ever sees "answered" or "answered with nothing", because
 * `CadenceShell` renders a *different* composable (`PlaceholderScreen`) for
 * the in-flight moment. This screen has no such wrapper to hand "loading" off
 * to — it draws its own header, hero and floating exit — so the distinction
 * has to be a type this file owns, not two nullables a caller combines.
 */
sealed interface RecipeDetailState {
    /** `RecipeRepository.recipe(id)` has not answered yet. */
    data object Loading : RecipeDetailState

    /** It answered, and no recipe carries this id. */
    data object NotFound : RecipeDetailState

    /** It answered with a recipe. */
    data class Found(
        val recipe: Recipe,
    ) : RecipeDetailState
}

private const val LOADING_MESSAGE = "Загрузка…"
private const val NOT_FOUND_MESSAGE = "Рецепт не найден."
private const val BACK_DESCRIPTION = "Назад"
private const val ADD_TO_DAY_LABEL = "Добавить в день →"
private const val OWN_SUFFIX = " · моё"

/** The «На порцию» / «Всё» toggle — `RecipeDetailScreen.tsx:236-272`'s `mode` state. */
private enum class MacroMode(
    val labelRu: String,
) {
    PER_SERVING("На порцию"),
    WHOLE("Всё"),
}

private const val RECIPE_DETAIL_MIN_SERVINGS = 1
private const val RECIPE_DETAIL_MAX_SERVINGS = 6
private const val RECIPE_DETAIL_DEFAULT_SERVINGS = 1
private const val TENTHS_PER_UNIT = 10.0

private val CARD_SHAPE = RoundedCornerShape(CadenceRadius.lg)
private val HAIRLINE = 1.dp
private val HERO_TITLE_SIZE = 34.sp
private val HERO_META_ICON_SIZE = 15.dp

/** `insets.top + 52` on the prototype's hero (`RecipeDetailScreen.tsx:132`) — 32dp + 20dp clears the floating back. */
private val HERO_TOP_PADDING = CadenceSpacing.huge + CadenceSpacing.xl
private const val HERO_TAG_ALPHA = 0.7f
private val MACRO_KCAL_SIZE = 34.sp
private val STEP_BADGE_SIZE = 30.dp
private val FLOATING_BACK_SIZE = 40.dp
private val SERVINGS_STEPPER_WIDTH = 150.dp

/** Keeps the last piece of scrolling content clear of the fixed bottom bar overlaid above it. */
private val BOTTOM_BAR_CLEARANCE = 96.dp

/**
 * «Рецепт» (`CadenceRoute.RecipeDetail`).
 *
 * Takes a [RecipeDetailState] and [ingredients], never a repository — the
 * rule every screen in this context follows (`RecipesScreen.kt:101-106`).
 * [onAddToDay] receives a ready [MealDraft] built by
 * [app.cadence.shared.domain.toMealDraft] — `source = RECIPE` and `recipeId`
 * are already set there (step-8's own KDoc), so this screen cannot forget
 * either half of `MealDraft.canLog`'s RECIPE case. Writing the draft,
 * raising the confirm toast and returning to «Питание» are step-13's job,
 * the same way `LogMealScreen.kt`'s `onSave` only hands over a [MealDraft].
 */
@Composable
fun RecipeDetailScreen(
    state: RecipeDetailState,
    ingredients: List<Ingredient>,
    modifier: Modifier = Modifier,
    onBack: () -> Unit = { },
    onAddToDay: (MealDraft) -> Unit = { },
) {
    when (state) {
        RecipeDetailState.Loading -> {
            RecipeDetailLoading(modifier)
        }

        RecipeDetailState.NotFound -> {
            RecipeDetailNotFound(modifier, onBack)
        }

        is RecipeDetailState.Found -> {
            RecipeDetailFound(state.recipe, ingredients, modifier, onBack, onAddToDay)
        }
    }
}

/** «загружается» — no back, no content, nothing else on screen to mistake for «не найден». */
@Composable
private fun RecipeDetailLoading(modifier: Modifier = Modifier) {
    val palette = Cadence.palette

    Box(
        modifier
            .fillMaxSize()
            .background(palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars)
            .testTag(CADENCE_RECIPE_DETAIL_LOADING_TAG),
        contentAlignment = Alignment.Center,
    ) {
        CadenceBody(LOADING_MESSAGE, color = palette.subtle)
    }
}

/** «не найден» — the message plus the one way out (`RecipeDetailScreen.tsx:87-118`). */
@Composable
private fun RecipeDetailNotFound(
    modifier: Modifier = Modifier,
    onBack: () -> Unit = { },
) {
    val palette = Cadence.palette

    Column(
        modifier
            .fillMaxSize()
            .background(palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars)
            .testTag(CADENCE_RECIPE_DETAIL_NOT_FOUND_TAG),
    ) {
        Row(Modifier.fillMaxWidth().padding(CadenceSpacing.md)) {
            BackButton(onBack, background = palette.sunk)
        }
        CadenceBody(
            NOT_FOUND_MESSAGE,
            color = palette.subtle,
            modifier = Modifier.padding(horizontal = CadenceSpacing.xl, vertical = CadenceSpacing.md),
        )
    }
}

/** A plain, non-floating back circle — the «не найден» state's own exit. */
@Composable
private fun BackButton(
    onBack: () -> Unit,
    background: Color,
) {
    Box(
        Modifier
            .size(FLOATING_BACK_SIZE)
            .clip(CircleShape)
            .background(background)
            .pressable(onBack, remember { MutableInteractionSource() })
            .testTag(CADENCE_RECIPE_DETAIL_BACK_TAG)
            .semantics { contentDescription = BACK_DESCRIPTION },
        contentAlignment = Alignment.Center,
    ) {
        CadenceIcon(paths = CadenceIcons.chevronLeft, tint = Cadence.palette.ink2)
    }
}

/**
 * «найден»: the hero, the macros card, the ingredients, the steps, a floating
 * back button and the bottom bar — everything below is `Box`-stacked over one
 * scrolling column, matching the prototype's own `position: 'absolute'` back
 * button and gradient-backed footer (`RecipeDetailScreen.tsx:409-514`).
 *
 * [mode] and [servings] are hoisted here, not further down: [mode] only ever
 * changes what the macros card *shows* — [onAddToDay] below never reads it —
 * and [servings] only ever changes what the bottom bar *writes*, never the
 * macros card. Two independent pieces of state for two independent
 * questions, the same separation the spec's own step-10 section pins as two
 * of its named tests.
 */
@Composable
private fun RecipeDetailFound(
    recipe: Recipe,
    ingredients: List<Ingredient>,
    modifier: Modifier = Modifier,
    onBack: () -> Unit = { },
    onAddToDay: (MealDraft) -> Unit = { },
) {
    val palette = Cadence.palette
    var mode by remember { mutableStateOf(MacroMode.PER_SERVING) }
    var servings by remember { mutableStateOf(RECIPE_DETAIL_DEFAULT_SERVINGS) }

    Box(modifier.fillMaxSize().background(palette.bg)) {
        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .testTag(CADENCE_RECIPE_DETAIL_TAG),
        ) {
            RecipeDetailHero(recipe)
            RecipeDetailMacrosCard(
                recipe = recipe,
                ingredients = ingredients,
                mode = mode,
                onModeChange = { mode = it },
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            )
            RecipeDetailIngredientsCard(
                recipe = recipe,
                ingredients = ingredients,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.xs),
            )
            RecipeDetailSteps(
                recipe = recipe,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
            )
            Spacer(Modifier.height(BOTTOM_BAR_CLEARANCE))
        }

        Box(
            Modifier
                .align(Alignment.TopStart)
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(CadenceSpacing.md),
        ) {
            FloatingBackButton(onBack)
        }

        RecipeDetailBottomBar(
            servings = servings,
            onServingsChange = { servings = it },
            // requestedServings is the stepper's own value, read fresh at the
            // moment of the tap — never the macros card's `mode`, which this
            // draft does not take as a parameter at all.
            onAddToDay = { onAddToDay(recipe.toMealDraft(ingredients, requestedServings = servings)) },
            modifier = Modifier.align(Alignment.BottomCenter).windowInsetsPadding(WindowInsets.navigationBars),
        )
    }
}

/** The floating circle over the scrolled content — the prototype's `position: 'absolute'` back button. */
@Composable
private fun FloatingBackButton(onBack: () -> Unit) {
    val palette = Cadence.palette

    Box(
        Modifier
            .size(FLOATING_BACK_SIZE)
            .shadow(CadenceElevation.md, CircleShape, clip = false)
            .clip(CircleShape)
            .background(palette.bg)
            .pressable(onBack, remember { MutableInteractionSource() })
            .testTag(CADENCE_RECIPE_DETAIL_BACK_TAG)
            .semantics { contentDescription = BACK_DESCRIPTION },
        contentAlignment = Alignment.Center,
    ) {
        CadenceIcon(paths = CadenceIcons.chevronLeft, tint = palette.ink2)
    }
}

/**
 * The tone-tinted hero: type (with «моё» for an owned recipe), name,
 * description, time, servings and tags (`RecipeDetailScreen.tsx:128-209`).
 *
 * [mealTypeTone] is `RecipeRow.kt`'s own tone map, promoted to `internal` for
 * this, its second call site — the same map the prototype's
 * `RecipeDetailScreen.tsx:12` imports from `RecipesScreen.tsx` rather than
 * redeclaring.
 */
@Composable
private fun RecipeDetailHero(recipe: Recipe) {
    val palette = Cadence.palette
    val tone = mealTypeTone(recipe.mealType)
    val typeLine = mealTypeLabel(recipe.mealType) + if (recipe.ownerId != null) OWN_SUFFIX else ""

    Column(
        Modifier
            .fillMaxWidth()
            .background(tone.soft)
            .padding(horizontal = CadenceSpacing.xl)
            .padding(top = HERO_TOP_PADDING, bottom = CadenceSpacing.xxl),
    ) {
        CadenceEyebrow(typeLine, color = tone.fg, modifier = Modifier.padding(bottom = CadenceSpacing.md))
        CadenceTitle(text = recipe.name, size = HERO_TITLE_SIZE, color = palette.ink)
        recipe.dek?.takeIf { it.isNotBlank() }?.let {
            CadenceBody(it, color = palette.muted, modifier = Modifier.padding(top = CadenceSpacing.sm))
        }
        Row(
            Modifier.fillMaxWidth().padding(top = CadenceSpacing.md),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.lg),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            HeroIconMeta(CadenceIcons.clock, "${recipe.totalTimeMin()} мин", tone.fg)
            HeroIconMeta(CadenceIcons.cake, "${recipe.servings} порц.", tone.fg)
            recipe.tags.forEach { tag -> HeroTagPill(tagLabel(tag), tone.fg) }
        }
    }
}

@Composable
private fun HeroIconMeta(
    icon: List<String>,
    text: String,
    tint: Color,
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
    ) {
        CadenceIcon(paths = icon, size = HERO_META_ICON_SIZE, tint = tint)
        BasicText(text = text, style = Cadence.typography.body.copy(color = tint, fontSize = 12.5.sp))
    }
}

/**
 * One hero tag pill. The prototype paints `rgba(255,255,255,.5)` over the
 * tone-tinted hero (`RecipeDetailScreen.tsx:196-202`) — a literal this port
 * does not carry over, per the "tokens only" rule. [Cadence.palette.paper]
 * at [HERO_TAG_ALPHA] reads the same translucent-light-chip role from a
 * token this palette already has, rather than a raw white.
 */
@Composable
private fun HeroTagPill(
    label: String,
    tint: Color,
) {
    Box(
        Modifier
            .background(Cadence.palette.paper.copy(alpha = HERO_TAG_ALPHA), RoundedCornerShape(CadenceRadius.pill))
            .padding(horizontal = CadenceSpacing.md, vertical = CadenceSpacing.xxs),
    ) {
        BasicText(text = label, style = Cadence.typography.label.copy(color = tint, fontSize = 11.5.sp))
    }
}

/**
 * «Макросы»: the «На порцию» / «Всё» toggle, the large kcal readout, the
 * portions caption and [CadenceSplitBar] (`RecipeDetailScreen.tsx:211-299`).
 *
 * [mode] changes only what this card *shows* — [recipe.perServing]/[totals]
 * decide which [app.cadence.shared.domain.MacrosTenths] to read, and neither
 * this card nor its caller ever feeds [mode] into [MealDraft] — that draft
 * comes from [recipe.toMealDraft] with the bottom bar's own `servings`, a
 * value this composable never sees. `CadenceSegmented` is used as it already
 * is — full-width, equal segments — rather than the prototype's hug-width
 * inline toggle: `CadenceSegmented.kt`'s own KDoc names this exact call site
 * as one of the component's two intended uses.
 */
@Composable
private fun RecipeDetailMacrosCard(
    recipe: Recipe,
    ingredients: List<Ingredient>,
    mode: MacroMode,
    onModeChange: (MacroMode) -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val macrosTenths = if (mode == MacroMode.PER_SERVING) recipe.perServing(ingredients) else recipe.totals(ingredients)
    val macros = macrosTenths.toMacros()
    val portionsLabel = if (mode == MacroMode.PER_SERVING) "1 порция" else "${recipe.servings} порц."

    Column(
        modifier
            .fillMaxWidth()
            .shadow(CadenceElevation.sm, CARD_SHAPE, clip = false)
            .background(palette.paper, CARD_SHAPE)
            .border(HAIRLINE, palette.hairline, CARD_SHAPE)
            .testTag(CADENCE_RECIPE_DETAIL_MACROS_TAG)
            .padding(CadenceSpacing.lg),
    ) {
        CadenceEyebrow("Макросы", modifier = Modifier.padding(bottom = CadenceSpacing.md))
        CadenceSegmented(
            options = MacroMode.entries,
            selected = mode,
            onSelect = onModeChange,
            label = { it.labelRu },
            modifier = Modifier.padding(bottom = CadenceSpacing.md),
        )
        Row(
            Modifier.fillMaxWidth().padding(bottom = CadenceSpacing.md),
            verticalAlignment = Alignment.Bottom,
        ) {
            CadenceNumber(value = formatInteger(macros.kcal), unit = "ккал", size = MACRO_KCAL_SIZE)
            Spacer(Modifier.weight(1f))
            CadenceMeta(portionsLabel, color = palette.subtle)
        }
        CadenceSplitBar(
            proteinG = macrosTenths.proteinGTenths / TENTHS_PER_UNIT,
            carbsG = macrosTenths.carbsGTenths / TENTHS_PER_UNIT,
            fatG = macrosTenths.fatGTenths / TENTHS_PER_UNIT,
        )
    }
}

/**
 * «Ингредиенты · N порц.», grams and kcal per row (`RecipeDetailScreen.tsx:301-360`).
 *
 * Always [recipe.servings], never the bottom bar's `servings` — this heading
 * and every row's grams describe the recipe's own card, which does not move
 * when the stepper does, the same rule [RecipeDetailMacrosCard] follows.
 */
@Composable
private fun RecipeDetailIngredientsCard(
    recipe: Recipe,
    ingredients: List<Ingredient>,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val byId = ingredients.associateBy { it.id }

    Column(modifier.fillMaxWidth()) {
        CadenceEyebrow(
            "Ингредиенты · ${recipe.servings} порц.",
            modifier = Modifier.padding(bottom = CadenceSpacing.sm),
        )
        Column(
            Modifier
                .fillMaxWidth()
                .clip(CARD_SHAPE)
                .background(palette.paper, CARD_SHAPE)
                .border(HAIRLINE, palette.hairline, CARD_SHAPE),
        ) {
            recipe.ingredients.forEachIndexed { index, row ->
                val ingredient = byId[row.ingredientId] ?: error("no ingredient with id ${row.ingredientId.raw}")
                val rowMacros = ingredient.macrosFor(row.grams)

                Row(
                    Modifier
                        .fillMaxWidth()
                        .testTag(recipeDetailIngredientTag(index))
                        .padding(vertical = CadenceSpacing.md, horizontal = CadenceSpacing.lg),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
                ) {
                    CadenceBody(ingredient.nameRu, modifier = Modifier.weight(1f))
                    CadenceMeta(formatGrams(row.grams), color = palette.ink2)
                    CadenceMeta(formatKcalTenths(rowMacros.kcalTenths), color = palette.subtle)
                }
                if (index != recipe.ingredients.lastIndex) CadenceHairline()
            }
        }
    }
}

/** «Приготовление», a numbered list (`RecipeDetailScreen.tsx:362-406`). */
@Composable
private fun RecipeDetailSteps(
    recipe: Recipe,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Column(modifier.fillMaxWidth()) {
        CadenceEyebrow("Приготовление", modifier = Modifier.padding(bottom = CadenceSpacing.sm))
        Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md)) {
            recipe.steps.forEachIndexed { index, step ->
                Row(horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md)) {
                    Box(
                        Modifier.size(STEP_BADGE_SIZE).clip(CircleShape).background(CadenceColors.forest50),
                        contentAlignment = Alignment.Center,
                    ) {
                        BasicText(
                            text = "${index + 1}",
                            style = Cadence.typography.number.copy(color = CadenceColors.forest700, fontSize = 14.sp),
                        )
                    }
                    CadenceBody(step, color = palette.ink2, modifier = Modifier.weight(1f))
                }
            }
        }
    }
}

/**
 * The servings stepper (1…6, default 1) and «Добавить в день →»
 * (`RecipeDetailScreen.tsx:431-514`). [servings] never reaches the macros
 * card above — only [onAddToDay]'s caller reads it, via
 * [recipe.toMealDraft]'s `requestedServings`.
 */
@Composable
private fun RecipeDetailBottomBar(
    servings: Int,
    onServingsChange: (Int) -> Unit,
    onAddToDay: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Row(
        modifier
            .fillMaxWidth()
            .background(palette.bg)
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceStepper(
            value = servings.toDouble(),
            onChange = { onServingsChange(it.roundToInt()) },
            min = RECIPE_DETAIL_MIN_SERVINGS.toDouble(),
            max = RECIPE_DETAIL_MAX_SERVINGS.toDouble(),
            step = 1.0,
            decimals = 0,
            modifier = Modifier.width(SERVINGS_STEPPER_WIDTH),
        )
        CadenceButton(
            label = ADD_TO_DAY_LABEL,
            onClick = onAddToDay,
            kind = CadenceButtonKind.PRIMARY,
            size = CadenceButtonSize.LARGE,
            fillWidth = true,
            modifier = Modifier.weight(1f).testTag(CADENCE_RECIPE_DETAIL_ADD_TAG),
        )
    }
}
