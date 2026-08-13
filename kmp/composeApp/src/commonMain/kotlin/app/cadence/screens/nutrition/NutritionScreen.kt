package app.cadence.screens.nutrition

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceElevation
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMacroBar
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceRings
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.cadenceEmphasisedTitle
import app.cadence.format.formatInteger
import app.cadence.format.pluralMeals
import app.cadence.shared.domain.Macros
import app.cadence.shared.repository.NutritionDay
import kotlinx.datetime.TimeZone

// «Питание» — the nutrition tab (`CadenceRoute.Nutrition`), ported from
// mobile/src/features/meal/NutritionScreen.tsx.
//
// Step-6 of docs/specs/kmp-nutrition-and-recipes.md is split across two
// agents. This file and its sibling [NutritionMealFeed.kt] (split out the
// same way `LogMealItemsList.kt` was, once the shell plus the feed would
// have crossed detekt's `LargeClass` threshold) draw the header, the hero,
// the rings-plus-macros card and the meal feed. The week section, the
// «Рецепты» transition card and the tab bar are the second half's own scope
// — see [NutritionWeekSectionSeam], [NutritionRecipesLinkSeam] and
// [NutritionTabBarSeam] below for the shape each one still needs.

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
    modifier: Modifier = Modifier,
    zone: TimeZone = TimeZone.currentSystemDefault(),
    onBack: () -> Unit = { },
    onLogMeal: () -> Unit = { },
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
            NutritionWeekSectionSeam()
            NutritionRecipesLinkSeam()
        }

        NutritionTabBarSeam()
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

/**
 * «Прошлая неделя» — [CadenceWeekBars][app.cadence.design.CadenceWeekBars],
 * weekday labels read from real dates (not literals — the step's own test
 * cranks the clock off `DEMO_NOW`'s Sunday to prove it), the dashed «цель
 * {N}» line and the «Белок · средн.» footer (`NutritionScreen.tsx:585-645`).
 *
 * Left as a stub: this half of step-6 stops at the meal feed above it. The
 * real composable this position expects, per the spec's step-6 section and
 * `NutritionRepository.week()`:
 *
 * ```
 * NutritionWeekSection(week: NutritionWeek, kcalGoal: Int, modifier: Modifier = Modifier)
 * ```
 *
 * The prototype's «↑ 6 г к прошлой» pill (`NutritionScreen.tsx:642`) is a
 * static string with no source and is **not** ported.
 */
@Composable
private fun NutritionWeekSectionSeam() = Unit

/**
 * «Рецепты» — the transition card into the recipe library
 * (`NutritionScreen.tsx:647-698`).
 *
 * Left as a stub for the same reason as [NutritionWeekSectionSeam]. Real
 * shape:
 *
 * ```
 * NutritionRecipesLink(onOpenRecipes: () -> Unit, modifier: Modifier = Modifier)
 * ```
 */
@Composable
private fun NutritionRecipesLinkSeam() = Unit

/**
 * The bottom tab bar (`NutritionScreen.tsx:702`).
 *
 * Left as a stub for the same reason as [NutritionWeekSectionSeam]. Real
 * call, the same shape `TrendsScreen.kt:144-148` makes for its own
 * destination:
 *
 * ```
 * CadenceTabBar(active = CadenceDestination.NUTRITION, onSelect = onSelectTab, onLog = onOpenActions)
 * ```
 *
 * which means [NutritionScreen] itself gains `onSelectTab: (CadenceDestination) -> Unit = { }`
 * and `onOpenActions: () -> Unit = { }` parameters alongside this call.
 */
@Composable
private fun NutritionTabBarSeam() = Unit
