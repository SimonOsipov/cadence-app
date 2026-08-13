package app.cadence.screens.today

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.pressable
import app.cadence.format.clockTime
import app.cadence.format.formatGrams
import app.cadence.format.formatInteger
import app.cadence.format.formatKcal
import app.cadence.format.formatKcalTenths
import app.cadence.format.pluralItems
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.Meal
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime

private val CARD_RADIUS = 18.dp
private val BAR_HEIGHT = 6.dp

/** How many of the day's meals `TodayMeals` shows — the prototype's own `meals.slice(-3)`. */
private const val RECENT_MEALS_LIMIT = 3

/** The four captions `MealHero` draws above the remaining-line, keyed on the day's meal count. */
private data class MealHeroHint(
    val eyebrow: String,
    val title: String,
    val meta: String,
)

/**
 * Ports `suggestNextMeal` (`mobile/src/features/meal/data.ts:244-253`).
 *
 * The prototype's function takes `meals` and an `now` string, parses an hour out of
 * `now`, and then never reads it — `void hour` on the line right after, so the branch
 * is `meals.length` alone. No dish and no clock feed the four captions; each is a
 * literal the prototype writes out, ported here verbatim.
 */
private fun suggestNextMeal(mealCount: Int): MealHeroHint =
    when (mealCount) {
        0 -> MealHeroHint("Начнём день", "Завтрак?", "Целимся в 35 г белка.")
        1 -> MealHeroHint("Следующий приём", "Скоро обед", "Подсказать, что собрать?")
        2 -> MealHeroHint("Следующий приём", "Перекус?", "Немного белка, немного фруктов.")
        else -> MealHeroHint("Последний шанс", "Лёгкий ужин?", "Запас есть — без излишеств.")
    }

/**
 * The sand card calling for the next meal.
 *
 * [mealCount] drives [suggestNextMeal]'s four captions — an eyebrow and a title,
 * both above the remaining-line, plus a meta line below it. The remaining-line
 * itself («Осталось {ккал} ккал · {белок} г белка») is unchanged by the hint: no
 * recipe is picked and no meta is folded into it, the same way the prototype's
 * own function names no dish either (see [suggestNextMeal]'s own KDoc).
 */
@Composable
fun MealHero(
    eaten: Macros,
    targets: Macros,
    mealCount: Int,
    modifier: Modifier = Modifier,
    onLogMeal: () -> Unit = { },
    onOpenRecipes: () -> Unit = { },
) {
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CARD_RADIUS)
    val kcalLeft = (targets.kcal - eaten.kcal).coerceAtLeast(0)
    val proteinLeft = (targets.proteinG - eaten.proteinG).coerceAtLeast(0)
    val hint = suggestNextMeal(mealCount)

    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .pressable(onLogMeal, interactionSource)
                .background(CadenceColors.sand100, shape)
                .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        CadenceEyebrow(hint.eyebrow, color = CadenceColors.sand900)

        BasicText(
            text = hint.title,
            style = Cadence.typography.title.copy(color = CadenceColors.ink900, fontSize = 28.sp),
        )

        BasicText(
            text = "Осталось ${formatKcal(kcalLeft)} · ${formatGrams(proteinLeft)} белка",
            style = Cadence.typography.title.copy(color = CadenceColors.ink900, fontSize = 22.sp),
        )

        BasicText(
            text = hint.meta,
            style = Cadence.typography.body.copy(color = CadenceColors.ink900.copy(alpha = 0.72f)),
        )

        Row(
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            BasicText(
                text = "Записать приём",
                modifier = Modifier.pressable(onLogMeal, remember { MutableInteractionSource() }),
                style = Cadence.typography.label.copy(color = CadenceColors.ink900, fontSize = 13.sp),
            )
            CadenceIcon(CadenceIcons.arrowRight, size = 14.dp, tint = CadenceColors.ink900)

            BasicText(
                text = "Рецепты",
                modifier =
                    Modifier
                        .pressable(onOpenRecipes, remember { MutableInteractionSource() })
                        .padding(start = CadenceSpacing.md),
                style = Cadence.typography.label.copy(color = CadenceColors.sand900, fontSize = 13.sp),
            )
        }
    }
}

/**
 * «Приёмы сегодня» — the day's energy against its target, the three macros
 * beneath it, and the last three logged meals (or the empty invitation).
 *
 * [meals] is the day's full list, not pre-trimmed: this composable slices to
 * [RECENT_MEALS_LIMIT] itself, the same shape as the prototype's own
 * `TodayMeals` taking every meal and calling `meals.slice(-3)`
 * (`TodayScreen.tsx:857`). [zone] turns each meal's `eatenAt` into the clock
 * reading shown beside it, the same reason `NutritionScreen.kt:94-98` takes
 * one rather than reading the platform zone inline.
 */
@Composable
fun TodayMeals(
    eaten: Macros,
    targets: Macros,
    meals: List<Meal>,
    zone: TimeZone,
    modifier: Modifier = Modifier,
    onOpenNutrition: () -> Unit = { },
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CARD_RADIUS)
    val recent = meals.sortedBy { it.eatenAt }.takeLast(RECENT_MEALS_LIMIT)

    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .pressable(onOpenNutrition, interactionSource)
                .background(palette.paper, shape)
                .border(1.dp, palette.hairline, shape)
                .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        TodayMealsHeader()
        TodayMealsTotals(eaten, targets)
        RecentMealsSection(recent, zone)
    }
}

/** «Приёмы сегодня» over «Питание →» — the card's own heading, split out to keep the card short. */
@Composable
private fun TodayMealsHeader() {
    val palette = Cadence.palette

    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Bottom,
    ) {
        CadenceEyebrow("Приёмы сегодня")
        Row(
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            BasicText(
                text = "Питание",
                style = Cadence.typography.meta.copy(color = palette.subtle, fontSize = 12.sp),
            )
            CadenceIcon(CadenceIcons.arrowRight, size = 12.dp, tint = palette.subtle)
        }
    }
}

/** The kcal readout, the progress bar and the three macro legs. */
@Composable
private fun TodayMealsTotals(
    eaten: Macros,
    targets: Macros,
) {
    val palette = Cadence.palette

    Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs)) {
        BasicText(
            text = formatInteger(eaten.kcal),
            style = Cadence.typography.number.copy(color = palette.ink, fontSize = 28.sp),
        )
        BasicText(
            text = "/ ${formatKcal(targets.kcal)}",
            style = Cadence.typography.meta.copy(color = palette.subtle, fontSize = 13.sp),
        )
    }

    KcalBar(eaten.kcal, targets.kcal)

    Row(horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.lg)) {
        MacroLeg("Б", eaten.proteinG, targets.proteinG)
        MacroLeg("Ж", eaten.fatG, targets.fatG)
        MacroLeg("У", eaten.carbsG, targets.carbsG)
    }
}

/** The last three meals, each separated by a hairline, or the empty invitation. */
@Composable
private fun RecentMealsSection(
    recent: List<Meal>,
    zone: TimeZone,
) {
    if (recent.isEmpty()) {
        BasicText(
            text = "Сегодня пока ничего — первый приём приземлится здесь.",
            style = Cadence.typography.body.copy(color = Cadence.palette.muted),
        )
    } else {
        Column {
            recent.forEachIndexed { index, meal ->
                RecentMealRow(meal, zone)
                if (index < recent.lastIndex) CadenceHairline()
            }
        }
    }
}

/** One row of [TodayMeals]' recent list: a letter avatar, the name, time · items, and kcal. */
@Composable
private fun RecentMealRow(
    meal: Meal,
    zone: TimeZone,
) {
    val palette = Cadence.palette

    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MealAvatar(meal.name)

        Column(Modifier.weight(1f)) {
            BasicText(
                text = meal.name,
                maxLines = 1,
                style = Cadence.typography.body.copy(color = palette.ink),
            )
            BasicText(
                text =
                    "${clockTime(meal.eatenAt.toLocalDateTime(zone))} · " +
                        "${meal.items.size} ${pluralItems(meal.items.size)}",
                style = Cadence.typography.meta.copy(color = palette.subtle),
            )
        }

        Column(horizontalAlignment = Alignment.End) {
            BasicText(
                text = formatKcalTenths(meal.totals.kcalTenths),
                style = Cadence.typography.number.copy(color = palette.ink, fontSize = 14.sp),
            )
        }
    }
}

private val AVATAR_SIZE = 32.dp
private val AVATAR_SHAPE = RoundedCornerShape(CadenceRadius.md)

/** The letter-avatar box, the same idiom `NutritionMealFeed.kt`'s `MealAvatar` uses. */
@Composable
private fun MealAvatar(name: String) {
    val letter = (name.firstOrNull() ?: 'П').uppercaseChar().toString()

    Box(
        Modifier.size(AVATAR_SIZE).clip(AVATAR_SHAPE).background(CadenceColors.sand100),
        contentAlignment = Alignment.Center,
    ) {
        BasicText(
            text = letter,
            style = Cadence.typography.titleEmphasis.copy(fontSize = 16.sp, color = CadenceColors.sand700),
        )
    }
}

@Composable
private fun KcalBar(
    eaten: Int,
    target: Int,
) {
    val palette = Cadence.palette
    val fraction = if (target <= 0) 0f else (eaten.toFloat() / target).coerceIn(0f, 1f)

    Box(
        modifier =
            Modifier
                .fillMaxWidth()
                .height(BAR_HEIGHT)
                .background(palette.sunk, RoundedCornerShape(CadenceRadius.pill)),
    ) {
        Box(
            Modifier
                .fillMaxWidth(fraction)
                .height(BAR_HEIGHT)
                .background(CadenceColors.forest700, RoundedCornerShape(CadenceRadius.pill)),
        )
    }
}

@Composable
private fun MacroLeg(
    label: String,
    eaten: Int,
    target: Int,
) {
    BasicText(
        text = "$label ${formatInteger(eaten)}/${formatInteger(target)}",
        style = Cadence.typography.meta.copy(color = Cadence.palette.subtle, fontSize = 12.sp),
    )
}
