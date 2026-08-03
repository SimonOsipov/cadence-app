package app.cadence.screens.today

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceElevation
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.cadenceEmphasisedTitle
import app.cadence.design.pressable
import app.cadence.format.formatDose
import app.cadence.shared.domain.Dose

private val HERO_RADIUS = 24.dp
private val HERO_PADDING = 22.dp
private val HERO_TITLE = 36.sp

/**
 * The forest card at the top of «Сегодня»: what is due, and the one button
 * that does something about it.
 *
 * The compound and its dose are two runs of one line — «Семаглутид» over
 * «0,25 мг» in the drawn italic — because the prototype nests them in one
 * `Text` and they wrap as one paragraph. The dose is the *week's*: it comes
 * from the phase, so the card titrates where the prototype's literal does not.
 */
@Composable
fun TodayHero(
    compoundName: String?,
    dose: Dose?,
    logged: Boolean,
    modifier: Modifier = Modifier,
    onLogDose: () -> Unit = { },
    onOpenQuickFeel: () -> Unit = { },
) {
    val palette = Cadence.palette
    val shape = RoundedCornerShape(HERO_RADIUS)

    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .shadow(CadenceElevation.md, shape, clip = false)
                .background(palette.forestBg, shape)
                .padding(HERO_PADDING),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
    ) {
        HeroBadge(logged)

        val (value, unit) = dose?.let(::formatDose) ?: ("" to "")
        CadenceTitle(
            text =
                cadenceEmphasisedTitle(
                    prefix = if (compoundName == null) "" else "$compoundName\n",
                    emphasis = if (dose == null) "" else "$value $unit",
                    emphasisColor = CadenceColors.sand300,
                ),
            size = HERO_TITLE,
            color = CadenceColors.cream,
        )

        BasicText(
            text =
                if (logged) {
                    "Записано · ротация."
                } else {
                    "Недельная инъекция, запланирована на сегодня."
                },
            style = Cadence.typography.body.copy(color = palette.onForest.copy(alpha = 0.75f)),
        )

        HeroAction(logged, onLogDose)

        if (logged) {
            // The prototype only offers the check-in once there is a dose to
            // check in about, which is also the only time the question means
            // anything.
            QuickFeelPrompt(onOpenQuickFeel)
        }
    }
}

@Composable
private fun HeroBadge(logged: Boolean) {
    Row(
        modifier =
            Modifier
                .background(
                    CadenceColors.cream.copy(alpha = 0.12f),
                    RoundedCornerShape(CadenceRadius.pill),
                ).padding(horizontal = 10.dp, vertical = CadenceSpacing.xxs),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        BasicText(
            text = if (logged) "Записано сегодня" else "Сегодня утром",
            style = Cadence.typography.label.copy(color = CadenceColors.sand300, fontSize = 11.sp),
        )
    }
}

@Composable
private fun HeroAction(
    logged: Boolean,
    onLogDose: () -> Unit,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CadenceRadius.pill)

    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .pressable(onLogDose, interactionSource)
                .background(
                    if (logged) CadenceColors.cream.copy(alpha = 0.12f) else CadenceColors.sand500,
                    shape,
                ).padding(vertical = 14.dp, horizontal = CadenceSpacing.xl),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (logged) {
            CadenceIcon(CadenceIcons.check, size = 16.dp, tint = CadenceColors.cream, strokeWidth = 2f)
        }
        BasicText(
            text = if (logged) "Открыть детали" else "Записать →",
            style =
                Cadence.typography.label.copy(
                    color = if (logged) CadenceColors.cream else CadenceColors.ink900,
                    fontSize = 14.sp,
                ),
        )
    }
}

@Composable
private fun QuickFeelPrompt(onOpen: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CadenceRadius.pill)

    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .pressable(onOpen, interactionSource)
                .border(1.dp, CadenceColors.cream.copy(alpha = 0.2f), shape)
                .padding(vertical = 11.dp, horizontal = CadenceSpacing.lg),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIcon(CadenceIcons.heart, size = 15.dp, tint = CadenceColors.sand300)
        BasicText(
            text = "Как перенесли дозу? Отметить",
            style = Cadence.typography.label.copy(color = CadenceColors.sand300, fontSize = 13.sp),
        )
    }
}
