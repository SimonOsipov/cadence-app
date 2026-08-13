package app.cadence.screens.today

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
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceHairline
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceSpacing
import app.cadence.design.pressable
import app.cadence.format.formatDose
import app.cadence.shared.domain.OccurrenceStatus
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.ProtocolRow

private val STRIP_RADIUS = 18.dp
private val TILE = 40.dp
private val TILE_RADIUS = 12.dp
private val ROW_INSET = 68.dp

/**
 * «Протокол этой недели». The rows come from [ProtocolRow], a projection of the
 * same occurrences the Schedule screen renders — §03's seventh correction. In the
 * prototype this is a literal array of three whose doses never titrate and whose
 * state is a hand-wired boolean.
 */
@Composable
fun ProtocolStrip(
    rows: List<ProtocolRow>,
    modifier: Modifier = Modifier,
    onOpenSchedule: () -> Unit = { },
) {
    if (rows.isEmpty()) return
    val palette = Cadence.palette
    val shape = RoundedCornerShape(STRIP_RADIUS)
    val interactionSource = remember { MutableInteractionSource() }

    Column(
        modifier = modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.xxs),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Bottom,
        ) {
            CadenceEyebrow("Протокол этой недели")
            BasicText(
                text = "Весь график",
                modifier = Modifier.pressable(onOpenSchedule, interactionSource),
                style = Cadence.typography.label.copy(color = CadenceColors.forest700, fontSize = 12.sp),
            )
        }

        Column(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .background(palette.paper, shape)
                    .border(1.dp, palette.hairline, shape),
        ) {
            rows.forEachIndexed { index, row ->
                ProtocolStripRow(row)
                if (index < rows.lastIndex) {
                    CadenceHairline(Modifier.padding(start = ROW_INSET))
                }
            }
        }
    }
}

@Composable
private fun ProtocolStripRow(row: ProtocolRow) {
    val palette = Cadence.palette
    val supplement = row.kind == ProtocolItemKind.SUPPLEMENT

    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = 14.dp, vertical = CadenceSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier =
                Modifier
                    .size(TILE)
                    .background(
                        if (supplement) palette.sunk else CadenceColors.forest50,
                        RoundedCornerShape(TILE_RADIUS),
                    ),
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(
                paths = row.compound?.icon?.let { CadenceIcons.byName[it] } ?: CadenceIcons.beaker,
                size = 20.dp,
                tint = if (supplement) palette.ink2 else CadenceColors.forest700,
            )
        }

        Column(Modifier.weight(1f)) {
            BasicText(
                text = row.compound?.nameRu ?: "—",
                style = Cadence.typography.label.copy(color = palette.ink, fontSize = 14.sp),
            )
            BasicText(
                text = row.dose?.let { formatDose(it).let { (v, u) -> "$v $u" } } ?: cadenceLabel(row),
                style = Cadence.typography.meta.copy(color = palette.subtle, fontSize = 12.sp),
            )
        }

        row.todayStatus?.let { status ->
            Column(horizontalAlignment = Alignment.End) {
                BasicText(
                    text = "Сегодня",
                    style = Cadence.typography.number.copy(color = palette.ink2, fontSize = 13.sp),
                )
                BasicText(
                    text = if (status == OccurrenceStatus.DONE) "записано" else "ждёт",
                    style = Cadence.typography.meta.copy(color = palette.subtle, fontSize = 11.sp),
                )
            }
        }
    }
}

/** What a row says when it has no dose to say — the supplement's case. */
private fun cadenceLabel(row: ProtocolRow): String =
    if (row.kind == ProtocolItemKind.SUPPLEMENT) "На ночь" else "Подкожно"
