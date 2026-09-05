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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceNumber
import app.cadence.design.CadencePill
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceSpark
import app.cadence.design.pressable
import app.cadence.format.formatDecimal
import app.cadence.format.formatDeltaSincePrevious
import app.cadence.format.pluralWeeks
import app.cadence.format.unitRu

private val CARD_RADIUS = 18.dp
private const val WEIGHT_DIGITS = 1
private val GLANCE_SPARK_HEIGHT = 70.dp

/** The glance's chart, which has no text to find it by. */
const val GLANCE_SPARK_TAG: String = "glance-spark"

/**
 * «Вес · 7 недель» — the headline biomarker, with its own history beside it. The
 * prototype captions this «7 дней» and draws seven hardcoded points, but its own
 * schedule weighs the patient **weekly**: seven readings are seven weeks, so the
 * caption follows the data (recorded in the divergence registry). The delta is
 * computed from the series, not written into it — the prototype's «↓ 0,6 кг» is a
 * literal that never moves.
 */
@Composable
fun BiomarkerGlance(
    series: List<Double>,
    latest: Double?,
    modifier: Modifier = Modifier,
    onOpen: () -> Unit = { },
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CARD_RADIUS)
    val delta = formatDeltaSincePrevious(series, unitRu("kg"))

    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .pressable(onOpen, interactionSource)
                .background(palette.paper, shape)
                .border(1.dp, palette.hairline, shape)
                .padding(CadenceSpacing.lg),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.lg),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
            CadenceEyebrow("Вес · история")

            CadenceNumber(
                value = latest?.let { formatDecimal(it, WEIGHT_DIGITS) } ?: "—",
                unit = unitRu("kg"),
                size = 34.sp,
            )

            if (delta != null) CadencePill(delta)
        }

        CadenceSpark(
            data = series.map { it.toFloat() },
            // A chart draws no text; without a handle, deleting it leaves the
            // whole suite green — which it did.
            modifier = Modifier.weight(1f).testTag(GLANCE_SPARK_TAG),
            fill = CadenceColors.forest50,
            height = GLANCE_SPARK_HEIGHT,
        )
    }
}

/** The standing prompt into the diary, whether or not a dose was logged today. */
@Composable
fun WellbeingNudge(
    modifier: Modifier = Modifier,
    onOpen: () -> Unit = { },
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CARD_RADIUS)

    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .pressable(onOpen, interactionSource)
                .background(palette.paper, shape)
                .border(1.dp, palette.hairline, shape)
                .padding(CadenceSpacing.lg),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIcon(CadenceIcons.heart, size = 22.dp, tint = CadenceColors.sand700)

        Column(Modifier.weight(1f)) {
            BasicText(
                text = "Как вы себя чувствуете?",
                style = Cadence.typography.label.copy(color = palette.ink, fontSize = 14.sp),
            )
            BasicText(
                text = "Дневник самочувствия · сегодня не отмечено",
                style = Cadence.typography.meta.copy(color = palette.subtle, fontSize = 12.sp),
            )
        }

        CadenceIcon(CadenceIcons.chevronRight, size = 18.dp, tint = palette.subtle)
    }
}

/** «Семаглутид закончится через ~4 недели», when there is no spare behind it. */
@Composable
fun ReorderCard(
    compoundName: String?,
    weeksLeft: Int,
    modifier: Modifier = Modifier,
    onOpenVials: () -> Unit = { },
) {
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CARD_RADIUS)

    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .pressable(onOpenVials, interactionSource)
                .background(CadenceColors.warningBg, shape)
                .padding(CadenceSpacing.lg),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIcon(CadenceIcons.exclamationCircle, size = 20.dp, tint = CadenceColors.warning)

        Column(Modifier.weight(1f)) {
            BasicText(
                text = "${compoundName ?: "Препарат"} закончится через ~$weeksLeft ${pluralWeeks(weeksLeft)}",
                style = Cadence.typography.label.copy(color = CadenceColors.warningFg, fontSize = 13.sp),
            )
            BasicText(
                text = "Запасного флакона нет",
                style = Cadence.typography.meta.copy(color = CadenceColors.warningFg, fontSize = 12.sp),
            )
        }

        Row(
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            BasicText(
                text = "В аптечку",
                style = Cadence.typography.label.copy(color = CadenceColors.warning, fontSize = 12.sp),
            )
            CadenceIcon(CadenceIcons.arrowRight, size = 14.dp, tint = CadenceColors.warning)
        }
    }
}
