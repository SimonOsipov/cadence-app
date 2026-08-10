package app.cadence.screens.inventory

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceGauge
import app.cadence.design.CadenceMeta
import app.cadence.design.CadencePill
import app.cadence.design.CadencePillTone
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.format.dayAndMonth
import app.cadence.format.formatDose
import app.cadence.shared.domain.VialRow
import app.cadence.shared.domain.VialStatus

/** What each state is called on a card, and in what tone. */
private fun VialStatus.pill(): Pair<String, CadencePillTone>? =
    when (this) {
        VialStatus.ACTIVE -> "Активный" to CadencePillTone.FOREST

        VialStatus.LOW -> "На исходе" to CadencePillTone.DANGER

        VialStatus.EXPIRING -> "Истекает" to CadencePillTone.WARNING

        VialStatus.SEALED -> "Запас" to CadencePillTone.NEUTRAL

        // A disposed vial never reaches a card: `inventorySummary` leaves it
        // out of every group, because it is history rather than stock.
        VialStatus.DISPOSED -> null
    }

private val HAIRLINE = 1.dp

/**
 * One vial, as the cabinet lists it.
 *
 * Everything on it is already resolved — the compound's name, the dose in
 * force, what is left. The card computes nothing, which is what stops it
 * disagreeing with the Today screen about the same number.
 */
@Composable
fun VialCard(
    row: VialRow,
    modifier: Modifier = Modifier,
    onClick: () -> Unit = { },
) {
    Column(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.lg))
            .background(Cadence.palette.paper)
            .border(HAIRLINE, Cadence.palette.hairline, RoundedCornerShape(CadenceRadius.lg))
            .clickable(onClick = onClick)
            .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            Column(Modifier.weight(1f)) {
                CadenceBody(row.compound?.nameRu ?: "—")
                // The lot is on the card whether or not a dose is in force:
                // it is how a patient tells two vials of one compound apart,
                // and a vial of something no longer prescribed still has one.
                CadenceMeta(
                    listOfNotNull(
                        row.dose?.let { formatDose(it).let { (value, unit) -> "$value $unit" } },
                        row.lot,
                    ).joinToString(" · ").ifEmpty { "—" },
                )
            }
            row.status.pill()?.let { (label, tone) -> CadencePill(label, tone = tone) }
        }

        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Bottom,
        ) {
            CadenceMeta("Доз осталось")
            CadenceMeta("${row.remaining} / ${row.totalDoses}")
        }

        CadenceGauge(left = row.remaining, total = row.totalDoses)

        CadenceMeta("Истекает ${dayAndMonth(row.expiresOn)}")
    }
}
