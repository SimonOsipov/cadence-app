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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.pressable
import app.cadence.format.formatInteger
import app.cadence.shared.domain.Macros

private val CARD_RADIUS = 18.dp
private val BAR_HEIGHT = 6.dp

/**
 * The sand card calling for the next meal.
 *
 * What the prototype puts here is a suggested dish from the recipe library,
 * picked by `suggestNextMeal`. That library and its rule arrive with the
 * nutrition section (step 8 of the block); what is portable now is the half
 * that is arithmetic — how much of the day's target is still open — and the two
 * ways out. Recorded as a partial in the divergence registry.
 */
@Composable
fun MealHero(
    eaten: Macros,
    targets: Macros,
    modifier: Modifier = Modifier,
    onLogMeal: () -> Unit = { },
    onOpenRecipes: () -> Unit = { },
) {
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CARD_RADIUS)
    val kcalLeft = (targets.kcal - eaten.kcal).coerceAtLeast(0)
    val proteinLeft = (targets.proteinG - eaten.proteinG).coerceAtLeast(0)

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
        CadenceEyebrow("Следующий приём", color = CadenceColors.sand900)

        BasicText(
            text = "Осталось ${formatInteger(kcalLeft)} ккал · ${formatInteger(proteinLeft)} г белка",
            style = Cadence.typography.title.copy(color = CadenceColors.ink900, fontSize = 22.sp),
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
 * «Приёмы сегодня» — the day's energy against its target, with the three
 * macros beneath it.
 */
@Composable
fun TodayMeals(
    eaten: Macros,
    targets: Macros,
    modifier: Modifier = Modifier,
    onOpenNutrition: () -> Unit = { },
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CARD_RADIUS)

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

        Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs)) {
            BasicText(
                text = formatInteger(eaten.kcal),
                style = Cadence.typography.number.copy(color = palette.ink, fontSize = 28.sp),
            )
            BasicText(
                text = "/ ${formatInteger(targets.kcal)} ккал",
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
