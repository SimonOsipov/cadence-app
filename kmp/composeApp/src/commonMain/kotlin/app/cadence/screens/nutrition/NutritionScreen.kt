package app.cadence.screens.nutrition

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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceElevation
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMacroBar
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceNumber
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceRings
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTabBar
import app.cadence.design.CadenceTitle
import app.cadence.design.CadenceWeekBars
import app.cadence.design.cadenceEmphasisedTitle
import app.cadence.design.pressable
import app.cadence.format.formatInteger
import app.cadence.format.pluralMeals
import app.cadence.format.weekdayShort
import app.cadence.shared.domain.Macros
import app.cadence.shared.repository.NutritionDay
import app.cadence.shared.repository.NutritionWeek
import kotlinx.datetime.TimeZone
import kotlin.math.roundToInt

// «Питание» — the nutrition tab (`CadenceRoute.Nutrition`), ported from
// mobile/src/features/meal/NutritionScreen.tsx.
//
// Step-6 of docs/specs/kmp-nutrition-and-recipes.md was split across two
// agents. This file draws the header, the hero, the rings-plus-macros card,
// the week section, the «Рецепты» transition card and the tab bar; its
// sibling [NutritionMealFeed.kt] draws the meal feed — split out the same way
// `LogMealItemsList.kt` was, once this file plus the feed would have crossed
// detekt's `LargeClass` threshold.

/** The scrolling body, so a test can reach the screen without knowing its shape. */
const val CADENCE_NUTRITION_TAG = "cadence-nutrition"

/** The back button's own 40dp box, balanced on the header's other side. */
private val HEADER_BALANCE_SIZE = 40.dp

/** `fontSize: 32` on the prototype's hero title (`NutritionScreen.tsx:446`). */
private val HERO_TITLE_SIZE = 32.sp

/** `fontSize: 32` on the prototype's eaten-kcal readout (`NutritionScreen.tsx:503`). */
private val RING_TOTAL_SIZE = 32.sp

private val CARD_SHAPE = RoundedCornerShape(CadenceRadius.lg)
private val HAIRLINE = 1.dp

/**
 * «Питание» (`CadenceRoute.Nutrition`).
 *
 * Takes a [NutritionDay] and lambdas, never a repository — the same rule
 * `TodayScreen.kt:35-38` states for its own screen: this file is handed a
 * day and reports taps, so the mock repository can become the Ktor client
 * without this file changing.
 *
 * [zone] is a parameter rather than read off the platform inside this file:
 * a meal's own `eatenAt` is an `Instant` (`Nutrition.kt:171`), and the local
 * clock reading it renders as is a rendering choice a test needs to be able
 * to fix, the same reason `CadenceMocks.kt:99` takes a zone rather than
 * calling `TimeZone.currentSystemDefault()` inline where it is used.
 */
@Composable
fun NutritionScreen(
    day: NutritionDay,
    week: NutritionWeek,
    modifier: Modifier = Modifier,
    zone: TimeZone = TimeZone.currentSystemDefault(),
    onBack: () -> Unit = { },
    onLogMeal: () -> Unit = { },
    onOpenRecipes: () -> Unit = { },
    onSelectTab: (CadenceDestination) -> Unit = { },
    onOpenActions: () -> Unit = { },
) {
    val palette = Cadence.palette

    Column(
        modifier
            .fillMaxSize()
            .background(palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars),
    ) {
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .testTag(CADENCE_NUTRITION_TAG),
        ) {
            NutritionHeader(onBack)
            NutritionHero(mealCount = day.meals.size)
            NutritionRingsCard(
                eaten = day.totals,
                targets = day.targets.macros,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            )
            NutritionMealFeed(
                meals = day.meals,
                zone = zone,
                onLogMeal = onLogMeal,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            )
            NutritionWeekSection(
                week = week,
                kcalGoal = day.targets.macros.kcal,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            )
            NutritionRecipesLink(
                onOpenRecipes = onOpenRecipes,
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            )
        }

        CadenceTabBar(
            active = CadenceDestination.NUTRITION,
            onSelect = onSelectTab,
            onLog = onOpenActions,
        )
    }
}

/**
 * Back, «Питание». The prototype's «⋯» button (`NutritionScreen.tsx:435`) is
 * not ported — it has no handler in the source, so there is nothing for this
 * port to wire. The trailing [Spacer] keeps the title centred the way that
 * inert button did, honestly, without drawing a control that does nothing.
 */
@Composable
private fun NutritionHeader(onBack: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIconButton(
            icon = CadenceIcons.chevronLeft,
            contentDescription = "Назад",
            onClick = onBack,
            background = Cadence.palette.sunk,
        )
        BasicText(
            text = "Питание",
            style = Cadence.typography.body.copy(color = Cadence.palette.muted, fontSize = 13.sp),
        )
        Spacer(Modifier.size(HEADER_BALANCE_SIZE))
    }
}

/**
 * «Тарелка сегодня» — empty reads «Пока ничего — начнём, когда *будете
 * готовы*.», otherwise «{N} приём/приёма/приёмов.» (`NutritionScreen.tsx:438-466`).
 * [pluralMeals] is the declension this project already carries for exactly
 * this noun — no second one is written here.
 */
@Composable
private fun NutritionHero(mealCount: Int) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.xl)
            .padding(top = CadenceSpacing.xs, bottom = CadenceSpacing.lg),
    ) {
        CadenceEyebrow("Тарелка сегодня", Modifier.padding(bottom = CadenceSpacing.xs))
        if (mealCount == 0) {
            CadenceTitle(
                text =
                    cadenceEmphasisedTitle(
                        prefix = "Пока ничего — начнём, когда ",
                        emphasis = "будете готовы",
                        suffix = ".",
                    ),
                size = HERO_TITLE_SIZE,
            )
        } else {
            CadenceTitle("$mealCount ${pluralMeals(mealCount)}.", size = HERO_TITLE_SIZE)
        }
    }
}

/**
 * `CadenceRings` with «ККАЛ» and the calorie percentage in the centre, plus
 * an overline, the eaten/target readout and three [CadenceMacroBar]s — the
 * ring-and-macros block (`NutritionScreen.tsx:468-522`).
 *
 * This is [CadenceMacroBar]'s first real call site with three siblings on
 * one screen — [id] keys each bar's own track/fill tags
 * (`macroTrackTag`/`macroFillTag` in `CadenceMacroBar.kt`) so a test can
 * address any one of the three without touching the other two.
 *
 * Carbs draw in `sand600` — the token this branch added for exactly this bar
 * (`CadenceColors.kt:41-44`); the prototype's own literal `#a5773d` had no
 * name before it.
 */
@Composable
private fun NutritionRingsCard(
    eaten: Macros,
    targets: Macros,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Row(
        modifier
            .fillMaxWidth()
            .shadow(CadenceElevation.sm, CARD_SHAPE, clip = false)
            .background(palette.paper, CARD_SHAPE)
            .border(HAIRLINE, palette.hairline, CARD_SHAPE)
            .padding(CadenceSpacing.lg),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.lg),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceRings(
            kcal = eaten.kcal.toDouble(),
            kcalGoal = targets.kcal.toDouble(),
            proteinG = eaten.proteinG.toDouble(),
            proteinGoal = targets.proteinG.toDouble(),
        )

        NutritionRingsMacros(eaten, targets, Modifier.weight(1f))
    }
}

/**
 * The eaten/target readout and the three macro bars beside [CadenceRings] —
 * split out of [NutritionRingsCard] itself, which tripped detekt's
 * `LongMethod` threshold with this inlined.
 */
@Composable
private fun NutritionRingsMacros(
    eaten: Macros,
    targets: Macros,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Column(modifier) {
        CadenceEyebrow("ккал сегодня", Modifier.padding(bottom = CadenceSpacing.xxs))
        Row(
            verticalAlignment = Alignment.Bottom,
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
            modifier = Modifier.padding(bottom = CadenceSpacing.md),
        ) {
            BasicText(
                text = formatInteger(eaten.kcal),
                style = Cadence.typography.number.copy(fontSize = RING_TOTAL_SIZE, color = palette.ink),
            )
            BasicText(
                text = "/ ${formatInteger(targets.kcal)}",
                style = Cadence.typography.meta.copy(color = palette.subtle),
            )
        }

        CadenceMacroBar(
            id = "protein",
            label = "белок",
            value = eaten.proteinG.toDouble(),
            goal = targets.proteinG.toDouble(),
            unit = "г",
            color = CadenceColors.forest700,
            modifier = Modifier.padding(bottom = CadenceSpacing.xs),
        )
        CadenceMacroBar(
            id = "carbs",
            label = "углеводы",
            value = eaten.carbsG.toDouble(),
            goal = targets.carbsG.toDouble(),
            unit = "г",
            color = CadenceColors.sand600,
            modifier = Modifier.padding(bottom = CadenceSpacing.xs),
        )
        CadenceMacroBar(
            id = "fat",
            label = "жиры",
            value = eaten.fatG.toDouble(),
            goal = targets.fatG.toDouble(),
            unit = "г",
            color = CadenceColors.sand700,
        )
    }
}

/** The icon-box beside «Рецепты и конструктор», the prototype's `width: 44, height: 44`. */
private val RECIPES_ICON_BOX_SIZE = 44.dp

/** The «Рецепты» card's own icon, the prototype's `size={22}`. */
private val RECIPES_ICON_SIZE = 22.dp

/** The trailing chevron, the prototype's `size={18}`. */
private val CHEVRON_SIZE = 18.dp

/** «{N} г / день», `fontSize: 18` on the prototype's protein-average readout (`NutritionScreen.tsx:625`). */
private val PROTEIN_AVERAGE_SIZE = 18.sp

/** The «Рецепты» card, so a test can press it without matching on its copy. */
const val CADENCE_NUTRITION_RECIPES_LINK_TAG = "cadence-nutrition-recipes-link"

/**
 * Day labels for [CadenceWeekBars], read from each column's own `date`
 * rather than a fixed `listOf("Пн", "Вт", ...)` — the prototype's own literal
 * (`NutritionScreen.tsx:317`). The step's test winds the week off a Wednesday precisely
 * because `DEMO_NOW` is a Sunday: on that day the derived order coincides with the
 * prototype's literal Monday-first list, so a mutation that hardcodes the labels back to
 * that literal would pass silently on the demo fixture alone.
 */
private fun weekDayLabels(week: NutritionWeek): List<String> =
    week.days.mapIndexed { index, day ->
        if (index == week.days.lastIndex) "Сег" else weekdayShort(day.date.dayOfWeek)
    }

/**
 * «Белок · средн.» — the week's average daily protein, rounded half up like the
 * prototype's own `Math.round(sum / length)` (`NutritionScreen.tsx:410-411`). [NutritionWeek]
 * always carries seven days (`NutritionRepository.week`'s own contract), so an empty list
 * is not a shape this function's caller can produce — the guard exists only so a malformed
 * fixture divides by a size it does not have rather than by zero.
 */
private fun weekProteinAverage(week: NutritionWeek): Int {
    val days = week.days
    if (days.isEmpty()) return 0
    return (days.sumOf { it.proteinG }.toDouble() / days.size).roundToInt()
}

/**
 * «Прошлая неделя» — [CadenceWeekBars], weekday labels read from real dates, the dashed
 * «цель {N}» line and the «Белок · средн.» footer (`NutritionScreen.tsx:585-645`).
 *
 * The prototype's «↑ 6 г к прошлой» pill (`NutritionScreen.tsx:642`) is a static string
 * with no source and is **not** ported — see the KDoc on [weekDayLabels] for the other
 * deliberate omission this section carries, and the file-level note above for why this one
 * needed a wound clock to prove.
 */
@Composable
private fun NutritionWeekSection(
    week: NutritionWeek,
    kcalGoal: Int,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Column(
        modifier
            .fillMaxWidth()
            .shadow(CadenceElevation.sm, CARD_SHAPE, clip = false)
            .background(palette.paper, CARD_SHAPE)
            .border(HAIRLINE, palette.hairline, CARD_SHAPE)
            .padding(CadenceSpacing.lg),
    ) {
        Row(
            Modifier.fillMaxWidth().padding(bottom = CadenceSpacing.md),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Bottom,
        ) {
            CadenceEyebrow("Прошлая неделя")
            CadenceMeta("ккал · по дням")
        }

        CadenceWeekBars(
            values = week.days.map { it.kcal.toDouble() },
            labels = weekDayLabels(week),
            goal = kcalGoal.toDouble(),
            goalLabel = "цель ${formatInteger(kcalGoal)}",
            todayIndex = week.days.lastIndex,
        )

        CadenceHairline(Modifier.padding(top = CadenceSpacing.md, bottom = CadenceSpacing.sm))

        Column {
            CadenceEyebrow("Белок · средн.", Modifier.padding(bottom = CadenceSpacing.xxs))
            CadenceNumber(
                value = formatInteger(weekProteinAverage(week)),
                unit = "г / день",
                size = PROTEIN_AVERAGE_SIZE,
            )
        }
    }
}

/**
 * «Рецепты» — the transition card into the recipe library (`NutritionScreen.tsx:647-698`).
 */
@Composable
private fun NutritionRecipesLink(
    onOpenRecipes: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Column(modifier.fillMaxWidth()) {
        CadenceEyebrow("Рецепты", Modifier.padding(bottom = CadenceSpacing.sm))
        Row(
            Modifier
                .fillMaxWidth()
                .shadow(CadenceElevation.sm, CARD_SHAPE, clip = false)
                .background(palette.paper, CARD_SHAPE)
                .border(HAIRLINE, palette.hairline, CARD_SHAPE)
                .pressable(onOpenRecipes, remember { MutableInteractionSource() })
                .testTag(CADENCE_NUTRITION_RECIPES_LINK_TAG)
                .padding(CadenceSpacing.md),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier
                    .size(RECIPES_ICON_BOX_SIZE)
                    .clip(RoundedCornerShape(CadenceRadius.md))
                    .background(CadenceColors.forest50),
                contentAlignment = Alignment.Center,
            ) {
                CadenceIcon(paths = CadenceIcons.bookOpen, size = RECIPES_ICON_SIZE, tint = CadenceColors.forest700)
            }
            Column(Modifier.weight(1f)) {
                CadenceBody("Рецепты и конструктор", maxLines = 1)
                CadenceMeta("Белковые блюда · соберите своё", maxLines = 1)
            }
            CadenceIcon(paths = CadenceIcons.chevronRight, size = CHEVRON_SIZE, tint = palette.subtle)
        }
    }
}
