package app.cadence.screens.nutrition

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.pressable
import app.cadence.format.clockTime
import app.cadence.format.formatDecimal
import app.cadence.format.formatInteger
import app.cadence.format.pluralItems
import app.cadence.shared.domain.Meal
import app.cadence.shared.domain.MealItem
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime

// «Приёмы сегодня» — the meal feed for `NutritionScreen.kt` (`NutritionScreen.tsx:524-583`).
// Split out the same reason `LogMealItemsList.kt` was split from `LogMealScreen.kt`: the
// shell plus this component would have crossed detekt's `LargeClass` threshold.

private val CARD_SHAPE = RoundedCornerShape(CadenceRadius.lg)
private val HAIRLINE = 1.dp
private val AVATAR_SIZE = 36.dp
private val AVATAR_SHAPE = RoundedCornerShape(CadenceRadius.md)
private const val TENTHS_PER_UNIT = 10.0

/**
 * A tenths field as a whole unit, for display only — never `MacrosTenths.toMacros()`.
 * That conversion stays the single path to a `Macros` value, at the
 * `TodaySummary`/`NutritionDay.totals` boundary (`Nutrition.kt:62-77,197`); a meal
 * card or position row rounds straight from its own tenths through this helper
 * instead of minting a second, unread `Macros` value. Mirrors `LogMealItemsList.kt`'s
 * own `tenthsLabel`, kept file-private in both for the same reason.
 */
private fun tenthsLabel(tenths: Int): String = formatDecimal(tenths / TENTHS_PER_UNIT, digits = 0)

/**
 * «Приёмы сегодня», the meal counter, and either the empty invitation or the list
 * of today's meals. The counter drops the prototype's denominator: «N из 4» carries
 * a constant with no source (`NutritionScreen.tsx:544`) and is not ported — only the
 * count remains.
 */
@Composable
internal fun NutritionMealFeed(
    meals: List<Meal>,
    zone: TimeZone,
    onLogMeal: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

    Column(modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().padding(bottom = CadenceSpacing.sm),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Bottom,
        ) {
            CadenceEyebrow("Приёмы сегодня")
            BasicText(
                text = formatInteger(meals.size),
                style = Cadence.typography.number.copy(fontSize = 12.sp, color = palette.subtle),
            )
        }

        if (meals.isEmpty()) {
            EmptyFeedCard(onLogMeal)
        } else {
            Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
                meals.forEach { meal -> MealFeedCard(meal, zone) }
            }
        }
    }
}

/**
 * «Сегодня пока ничего. **Запишите первый приём**, чтобы начать ритм.»
 * (`NutritionScreen.tsx:548-575`) — the emphasised run is its own pressable node
 * rather than a link span inside one `AnnotatedString`: this codebase's house rule
 * is that a tappable run has to be a real, addressable node (the same reason
 * `MealHero.kt:78-92`'s «Записать приём» / «Рецепты» are their own `BasicText`
 * with their own `pressable` modifier, not clickable spans).
 */
@Composable
private fun EmptyFeedCard(onLogMeal: () -> Unit) {
    val palette = Cadence.palette

    Column(
        Modifier
            .fillMaxWidth()
            .clip(CARD_SHAPE)
            .background(palette.paper, CARD_SHAPE)
            .border(HAIRLINE, palette.hairline, CARD_SHAPE)
            .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
    ) {
        CadenceBody("Сегодня пока ничего.", color = palette.muted)
        Row {
            BasicText(
                text = "Запишите первый приём",
                modifier = Modifier.pressable(onLogMeal, remember { MutableInteractionSource() }),
                style = Cadence.typography.body.copy(color = CadenceColors.forest700, fontWeight = FontWeight.Medium),
            )
            CadenceBody(", чтобы начать ритм.", color = palette.muted)
        }
    }
}

/**
 * One logged meal: a letter avatar, the name, «{время} · N позиций · {белок} г
 * белка», calories on the right, and — expanded — its items with their macros,
 * grams and calories (`MealCard`, `NutritionScreen.tsx:170-312`). [Meal.totals] is
 * the exact tenths fold (`Nutrition.kt:183-184`); rounds it through [tenthsLabel]
 * rather than calling `toMacros()` a second time — see that function's own KDoc.
 */
@Composable
private fun MealFeedCard(
    meal: Meal,
    zone: TimeZone,
) {
    var expanded by remember { mutableStateOf(false) }
    val palette = Cadence.palette
    val totals = meal.totals

    Column(
        Modifier
            .fillMaxWidth()
            .clip(CARD_SHAPE)
            .background(palette.paper, CARD_SHAPE)
            .border(HAIRLINE, palette.hairline, CARD_SHAPE),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .pressable({ expanded = !expanded }, remember { MutableInteractionSource() })
                .padding(CadenceSpacing.md),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MealAvatar(meal.name)

            Column(Modifier.weight(1f)) {
                CadenceBody(meal.name, maxLines = 1)
                val posWord = pluralItems(meal.items.size)
                CadenceMeta(
                    "${clockTime(meal.eatenAt.toLocalDateTime(zone))} · " +
                        "${meal.items.size} $posWord · ${tenthsLabel(totals.proteinGTenths)} г белка",
                    color = palette.subtle,
                )
            }

            Column(horizontalAlignment = Alignment.End) {
                BasicText(
                    text = tenthsLabel(totals.kcalTenths),
                    style = Cadence.typography.number.copy(fontSize = 16.sp, color = palette.ink),
                )
                CadenceMeta("ккал", color = palette.subtle)
            }

            CadenceIcon(
                paths = if (expanded) CadenceIcons.chevronUp else CadenceIcons.chevronDown,
                size = 16.dp,
                tint = palette.subtle,
            )
        }

        if (expanded) {
            CadenceHairline()
            Column(
                Modifier.padding(CadenceSpacing.md),
                verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            ) {
                meal.items.forEach { item -> MealFeedItemRow(item) }
            }
        }
    }
}

/**
 * The letter-avatar box — `.charAt(0).toUpperCase()` (`NutritionScreen.tsx:204`). Falls
 * back to «П», not the prototype's own `'M'`: RU is the product language, so a Latin
 * fallback has no place here even though it is unreachable in practice — every seeded
 * and logged meal carries a non-empty name.
 */
@Composable
private fun MealAvatar(name: String) {
    val letter = (name.firstOrNull() ?: 'П').uppercaseChar().toString()

    Box(
        Modifier.size(AVATAR_SIZE).clip(AVATAR_SHAPE).background(CadenceColors.sand100),
        contentAlignment = Alignment.Center,
    ) {
        BasicText(
            text = letter,
            style = Cadence.typography.titleEmphasis.copy(fontSize = 18.sp, color = CadenceColors.sand700),
        )
    }
}

/**
 * One expanded position: name, «Белок Xг · Углеводы Yг · Жиры Zг», grams and its
 * own calories (`NutritionScreen.tsx:250-306`) — every number rounded through
 * [tenthsLabel], never a second `toMacros()`.
 */
@Composable
private fun MealFeedItemRow(item: MealItem) {
    val palette = Cadence.palette

    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            CadenceBody(item.name, maxLines = 1, color = palette.ink2)
            CadenceMeta(
                "Белок ${tenthsLabel(item.macros.proteinGTenths)}г · " +
                    "Углеводы ${tenthsLabel(item.macros.carbsGTenths)}г · " +
                    "Жиры ${tenthsLabel(item.macros.fatGTenths)}г",
                color = palette.subtle,
            )
        }
        CadenceMeta("${item.grams} г", color = palette.subtle)
        BasicText(
            text = tenthsLabel(item.macros.kcalTenths),
            style = Cadence.typography.meta.copy(color = palette.ink2),
        )
    }
}
