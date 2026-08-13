package app.cadence.screens.recipes

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
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
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonSize
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceElevation
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.pressable
import app.cadence.format.formatGrams
import app.cadence.format.formatInteger
import app.cadence.shared.domain.Ingredient
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MealType
import app.cadence.shared.domain.RecipeId
import app.cadence.shared.domain.RecipePick
import app.cadence.shared.domain.RecipeTag
import app.cadence.shared.domain.filteredByTypeAndTag
import app.cadence.shared.domain.perServing
import app.cadence.shared.domain.pick
import app.cadence.shared.domain.toMacros
import app.cadence.shared.domain.totalTimeMin
import app.cadence.shared.repository.RecipeList

// «Рецепты» — the recipe library (`CadenceRoute.Recipes`), ported from
// mobile/src/features/recipe/RecipesScreen.tsx (`RecipeLibrary`).
//
// Step-9 of docs/specs/kmp-nutrition-and-recipes.md. Row drawing and the
// meal-type tone map live in the sibling file `RecipeRow.kt`, split out the
// same way `NutritionMealFeed.kt` was split from `NutritionScreen.kt` — this
// file plus the row would have crossed detekt's `LargeClass` threshold.

/** The scrolling body, so a test can reach the screen without knowing its shape. */
const val CADENCE_RECIPES_TAG = "cadence-recipes"

/** The «Для вас сегодня» card, so a test can press it without matching on its copy. */
const val CADENCE_RECIPES_FEATURED_TAG = "cadence-recipes-featured"

private val FEATURED_SHAPE = RoundedCornerShape(CadenceRadius.xl)
private val PHOTO_SLOT_HEIGHT = 150.dp
private val FEATURED_TITLE_SIZE = 27.sp
private val BADGE_ICON_SIZE = 13.dp
private val ARROW_ICON_SIZE = 15.dp
private const val BADGE_BG_ALPHA = 0.2f
private const val REMAINING_TEXT_ALPHA = 0.74f
private const val META_TEXT_ALPHA = 0.55f

/** Every [MealType], «Все» first — `RecipesScreen.tsx:284-292`'s filter row. */
private val MEAL_TYPE_OPTIONS: List<MealType?> = listOf(null) + MealType.entries

/** Every [RecipeTag], «Любые» first — `RecipesScreen.tsx:299-307`'s filter row. */
private val RECIPE_TAG_OPTIONS: List<RecipeTag?> = listOf(null) + RecipeTag.entries

/**
 * «Рецепты» (`CadenceRoute.Recipes`).
 *
 * Takes a [RecipeList] and lambdas, never a repository — the rule
 * `NutritionScreen.kt:90-92` states for its own screen. [ingredients] is the
 * full reference table the arithmetic in `RecipeMath.kt` needs to price a
 * recipe's macros; [consumed]/[goals] are today's eaten totals and the
 * patient's targets, the two `pick`/`remaining` (`RecipeMath.kt`) score
 * against.
 *
 * **The featured card's pick is computed over [library] whole, before either
 * filter is applied** (`RecipesScreen.tsx:140`: `RECIPES.suggest(allRecipes,
 * mealTotals)` runs on `allRecipes`, and `list = RECIPES.filter(allRecipes,
 * …)` — the filtered list — is built on the very next line, never fed back
 * into `suggest`). [mealType]/[tag] state is hoisted here and only ever
 * reaches [filteredByTypeAndTag]; [pick] never sees it.
 */
@Composable
fun RecipesScreen(
    library: RecipeList,
    ingredients: List<Ingredient>,
    consumed: Macros,
    goals: Macros,
    modifier: Modifier = Modifier,
    onBack: () -> Unit = { },
    onOpen: (RecipeId) -> Unit = { },
    onCreate: () -> Unit = { },
) {
    val palette = Cadence.palette
    var mealType by remember { mutableStateOf<MealType?>(null) }
    var tag by remember { mutableStateOf<RecipeTag?>(null) }

    val allRecipes = library.recipes
    val recipePick = pick(allRecipes, ingredients, consumed, goals)
    val filtered = allRecipes.filteredByTypeAndTag(mealType, tag)
    val mine = filtered.filter { it.ownerId != null }
    val rest = filtered.filter { it.ownerId == null }

    Column(
        modifier
            .fillMaxSize()
            .background(palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars),
    ) {
        RecipesHeader(onBack = onBack, onCreate = onCreate)
        CadenceHairline()

        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .testTag(CADENCE_RECIPES_TAG),
        ) {
            if (recipePick != null) {
                RecipesFeaturedCard(
                    pick = recipePick,
                    perServing = recipePick.recipe.perServing(ingredients).toMacros(),
                    totalTimeMin = recipePick.recipe.totalTimeMin(),
                    onOpen = { onOpen(recipePick.recipe.id) },
                    modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
                )
            }

            RecipesFilters(
                mealType = mealType,
                onMealType = { mealType = it },
                tag = tag,
                onTag = { tag = it },
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            )

            if (mine.isNotEmpty()) {
                RecipesSection(
                    title = "Мои рецепты",
                    recipes = mine,
                    ingredients = ingredients,
                    onOpen = onOpen,
                    modifier = Modifier.padding(horizontal = CadenceSpacing.lg),
                )
            }

            RecipesLibrarySection(
                hasMine = mine.isNotEmpty(),
                recipes = rest,
                ingredients = ingredients,
                onOpen = onOpen,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            )
        }
    }
}

/** Back, «Рецепты», «+ Создать» (`RecipesScreen.tsx:148-180`). */
@Composable
private fun RecipesHeader(
    onBack: () -> Unit,
    onCreate: () -> Unit,
) {
    val palette = Cadence.palette

    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIconButton(
            icon = CadenceIcons.chevronLeft,
            contentDescription = "Назад",
            onClick = onBack,
            background = palette.sunk,
        )
        BasicText(
            text = "Рецепты",
            style =
                Cadence.typography.label.copy(
                    color = palette.ink,
                    fontSize = 15.sp,
                    fontWeight = FontWeight.SemiBold,
                ),
        )
        Spacer(Modifier.weight(1f))
        CadenceButton(
            label = "Создать",
            onClick = onCreate,
            icon = CadenceIcons.plus,
            size = CadenceButtonSize.SMALL,
        )
    }
}

/**
 * «Для вас сегодня»: the 150dp camera-on-linen photo slot, the picked
 * recipe's name, the remaining-protein line and the «Открыть рецепт →» /
 * kcal·time footer (`RecipesScreen.tsx:184-276`).
 *
 * [perServing] and [totalTimeMin] are passed in rather than recomputed here:
 * the caller already has [RecipePick.recipe] and the ingredient table, and
 * this card has no arithmetic of its own to add.
 */
@Composable
private fun RecipesFeaturedCard(
    pick: RecipePick,
    perServing: Macros,
    totalTimeMin: Int,
    onOpen: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier
            .fillMaxWidth()
            .shadow(CadenceElevation.lg, FEATURED_SHAPE, clip = false)
            .clip(FEATURED_SHAPE)
            .background(CadenceColors.forest800, FEATURED_SHAPE)
            .pressable(onOpen, remember { MutableInteractionSource() })
            .testTag(CADENCE_RECIPES_FEATURED_TAG),
    ) {
        Box(
            Modifier.fillMaxWidth().height(PHOTO_SLOT_HEIGHT).background(CadenceColors.linen),
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(paths = CadenceIcons.camera, tint = CadenceColors.ink400)
        }

        Column(Modifier.padding(CadenceSpacing.xl)) {
            FeaturedBadge()

            CadenceTitle(
                text = pick.recipe.name,
                size = FEATURED_TITLE_SIZE,
                color = CadenceColors.cream,
                modifier = Modifier.padding(top = CadenceSpacing.sm),
            )

            BasicText(
                text =
                    "Осталось ${formatGrams(pick.remaining.proteinG)} белка сегодня — " +
                        "порция закроет ${formatGrams(perServing.proteinG)}.",
                style = Cadence.typography.body.copy(color = CadenceColors.cream.copy(alpha = REMAINING_TEXT_ALPHA)),
                modifier = Modifier.padding(top = CadenceSpacing.sm),
            )

            Row(
                Modifier.fillMaxWidth().padding(top = CadenceSpacing.md),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                BasicText(
                    text = "Открыть рецепт",
                    style = Cadence.typography.label.copy(color = CadenceColors.sand300, fontSize = 13.sp),
                )
                Spacer(Modifier.width(CadenceSpacing.xs))
                CadenceIcon(paths = CadenceIcons.arrowRight, size = ARROW_ICON_SIZE, tint = CadenceColors.sand300)
                Spacer(Modifier.weight(1f))
                BasicText(
                    text = "${formatInteger(perServing.kcal)} ккал · $totalTimeMin мин",
                    style =
                        Cadence.typography.number.copy(
                            color = CadenceColors.cream.copy(alpha = META_TEXT_ALPHA),
                            fontSize = 11.sp,
                        ),
                )
            }
        }
    }
}

/** «Для вас сегодня», sparkles on a sand pill (`RecipesScreen.tsx:209-226`). */
@Composable
private fun FeaturedBadge() {
    Row(
        Modifier
            .background(CadenceColors.sand500.copy(alpha = BADGE_BG_ALPHA), RoundedCornerShape(CadenceRadius.pill))
            .padding(horizontal = CadenceSpacing.md, vertical = CadenceSpacing.xxs),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIcon(paths = CadenceIcons.sparkles, size = BADGE_ICON_SIZE, tint = CadenceColors.sand300)
        BasicText(
            text = "Для вас сегодня",
            style = Cadence.typography.label.copy(color = CadenceColors.sand300, fontSize = 11.sp),
        )
    }
}

/**
 * The two filter rows: meal type (`RecipesScreen.tsx:284-292`) and tag
 * (`:298-307`), independent and additive — both flow straight into
 * [app.cadence.shared.domain.filteredByTypeAndTag] by the caller.
 *
 * Ported as the prototype draws it: a horizontally scrolling strip of
 * hug-width pills ([RecipesFilterChip]/[RecipesFilterRow]), not
 * [app.cadence.design.CadenceSegmented]. An earlier version of this screen
 * used [app.cadence.design.CadenceSegmented] — every segment forced to equal
 * width by [Modifier.weight] — because it already existed and named a
 * recipe card's per-serving toggle as a call site. Measured against this
 * screen's own nine values rather than assumed: at a 358dp row (390pt phone
 * minus the 16dp gutters) «Мягкие для желудка» is allotted 72dp against a
 * natural width of 132dp, and at 343dp (375pt, iPhone SE) «Завтрак» and
 * «Перекус» clip too — three of nine values unreadable on ordinary phones,
 * which is a broken filter, not a visual divergence a smaller device could
 * shrug off. Hug-width chips in a scrolling row never truncate, because
 * nothing forces a chip narrower than its own label.
 *
 * [RecipesFilterRow] does not know "tap the active option to clear it" on
 * its own: it forwards whatever option was tapped unconditionally, which is
 * exactly right for every *other* option (tapping «Все» while «Завтрак» is
 * active must still land on «Все», not toggle). The wrapper below adds
 * exactly the one extra rule the prototype's own handler has
 * (`setMealType(mealType === t.id ? null : t.id)`, `RecipesScreen.tsx:290`):
 * tapping the option that is already selected reports `null` instead of
 * reporting itself. Because «Все»/«Любые» are themselves modelled as the
 * `null` option in [MEAL_TYPE_OPTIONS]/[RECIPE_TAG_OPTIONS] — the prototype
 * draws «Все»/«Любые» as a real chip too, not a "no chip selected" state
 * (`RecipesScreen.tsx:284,299`) — the same one line of logic covers both
 * "clear a real filter" and "tapping «Все» while it is already selected is
 * a no-op" without a special case for either.
 */
@Composable
private fun RecipesFilters(
    mealType: MealType?,
    onMealType: (MealType?) -> Unit,
    tag: RecipeTag?,
    onTag: (RecipeTag?) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        RecipesFilterRow(
            options = MEAL_TYPE_OPTIONS,
            selected = mealType,
            onSelect = { tapped -> onMealType(if (tapped == mealType) null else tapped) },
            label = { option -> option?.let(::mealTypeLabel) ?: "Все" },
        )
        RecipesFilterRow(
            options = RECIPE_TAG_OPTIONS,
            selected = tag,
            onSelect = { tapped -> onTag(if (tapped == tag) null else tapped) },
            label = { option -> option?.let(::tagLabel) ?: "Любые" },
        )
    }
}

/**
 * One filter axis: every [options] value as its own hug-width, horizontally
 * scrolling pill — `RecipesScreen.tsx:279-293`/`:294-308`'s two `ScrollView`s.
 * Generic over `T` the same reason `CadenceSegmented` is: a screen selects
 * with its own type ([MealType]`?`/[RecipeTag]`?` here), and [label] is the
 * one place that turns a value into Russian copy.
 */
@Composable
private fun <T> RecipesFilterRow(
    options: List<T>,
    selected: T,
    onSelect: (T) -> Unit,
    label: (T) -> String,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier.horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        options.forEach { option ->
            RecipesFilterChip(
                label = label(option),
                active = option == selected,
                onClick = { onSelect(option) },
            )
        }
    }
}

/**
 * `FilterChip` (`RecipesScreen.tsx:20-39`): a hug-width pill, solid forest
 * when active, an outline in [app.cadence.design.CadencePalette.border]
 * otherwise. Not [app.cadence.design.CadenceChip] — that primitive's active
 * state is `palette.ink`, matching a different prototype control, while this
 * screen's own filter chip is `forest700`/`cream`, read directly off
 * `RecipesScreen.tsx:29-34` rather than off a component built for another
 * screen's colours.
 */
@Composable
private fun RecipesFilterChip(
    label: String,
    active: Boolean,
    onClick: () -> Unit,
) {
    val palette = Cadence.palette
    val shape = RoundedCornerShape(CadenceRadius.pill)
    val background = if (active) CadenceColors.forest700 else Color.Transparent
    val outline = if (active) CadenceColors.forest700 else palette.border
    val foreground = if (active) CadenceColors.cream else palette.ink2

    Box(
        Modifier
            .pressable(onClick, remember { MutableInteractionSource() })
            .background(background, shape)
            .border(1.dp, outline, shape)
            // Merged `selected`, the same idiom `CadenceSegmented` uses for its
            // own option boxes — lets a test assert both that this chip is
            // selected and that a sibling is not.
            .semantics(mergeDescendants = true) { this.selected = active }
            .padding(horizontal = CadenceSpacing.md, vertical = CadenceSpacing.xs),
    ) {
        BasicText(
            text = label,
            style = Cadence.typography.label.copy(color = foreground, fontSize = 13.sp),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}
