package app.cadence.screens.inventory

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceGauge
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.CadenceUsageRow
import app.cadence.format.dayAndMonth
import app.cadence.format.formatDose
import app.cadence.format.relativeDay
import app.cadence.shared.domain.VialDetail
import app.cadence.shared.domain.VialId
import kotlinx.datetime.LocalDate

private val HAIRLINE = 1.dp

/**
 * One vial, opened.
 *
 * Everything is already resolved — the same [VialRow] the card drew, this
 * vial's own records, and its weekly usage. The sheet computes nothing, so it
 * cannot disagree with the card the patient tapped to reach it.
 */
@Composable
fun VialDetailSheet(
    detail: VialDetail,
    today: LocalDate,
    modifier: Modifier = Modifier,
    onLogDose: (VialId) -> Unit = { },
) {
    val row = detail.row

    Column(
        modifier
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.lg),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs)) {
            CadenceTitle(row.compound?.nameRu ?: "—")
            row.dose?.let {
                val (value, unit) = formatDose(it)

                Row(
                    horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
                    verticalAlignment = Alignment.Bottom,
                ) {
                    CadenceTitle(value)
                    CadenceBody(unit, color = Cadence.palette.muted)
                }
            }
        }

        Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
            CadenceMeta("${row.remaining} / ${row.totalDoses}")
            CadenceGauge(left = row.remaining, total = row.totalDoses)
        }

        FactGrid(detail, today)
        RecentRecords(detail, today)
        UsageSection(detail)
        Actions(row.id, onLogDose)
    }
}

@Composable
private fun FactGrid(
    detail: VialDetail,
    today: LocalDate,
) {
    val row = detail.row

    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        listOf(
            // «Не вскрыт» rather than a blank: a sealed vial has no opening
            // date, and an empty field reads as data that failed to load.
            "Открыт" to (row.openedAt?.let { "${dayAndMonth(it)} · ${relativeDay(it, today)}" } ?: "Не вскрыт"),
            "Истекает" to dayAndMonth(row.expiresOn),
            "Лот" to (row.lot ?: "—"),
            "Хранится" to (row.locationRu ?: "—"),
        ).forEach { (label, value) ->
            Row(
                Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(CadenceRadius.md))
                    .background(Cadence.palette.paper)
                    .border(HAIRLINE, Cadence.palette.hairline, RoundedCornerShape(CadenceRadius.md))
                    .padding(CadenceSpacing.md),
                horizontalArrangement = Arrangement.SpaceBetween,
            ) {
                CadenceEyebrow(label)
                CadenceBody(value)
            }
        }
    }
}

@Composable
private fun RecentRecords(
    detail: VialDetail,
    today: LocalDate,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        CadenceEyebrow("Последние записи · ${detail.recent.size}")

        if (detail.recent.isEmpty()) {
            CadenceBody("Из этого флакона ещё не кололи", color = Cadence.palette.muted)
        } else {
            detail.recent.forEach { record ->
                val (value, unit) = formatDose(record.dose)

                Row(
                    Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                ) {
                    CadenceBody(
                        listOfNotNull(
                            "$value $unit",
                            record.site?.let {
                                app.cadence.design.CadenceBodyZone
                                    .of(it)
                                    ?.labelRu
                            },
                        ).joinToString(" · "),
                    )
                    CadenceMeta(relativeDay(record.date, today))
                }
            }
        }
    }
}

@Composable
private fun UsageSection(detail: VialDetail) {
    if (detail.dosesPerWeek.isEmpty()) return

    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        CadenceEyebrow("Расход по неделям")
        CadenceUsageRow(dosesPerWeek = detail.dosesPerWeek)
    }
}

@Composable
private fun Actions(
    id: VialId,
    onLogDose: (VialId) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        CadenceButton(label = "Записать дозу", onClick = { onLogDose(id) }, fillWidth = true)
        // Neither is built. A row that looks live and does nothing is worse
        // than one that says it is not ready — both are recorded as owed in
        // `docs/prototype-divergences.md`.
        CadenceButton(
            label = "Прикрепить фото",
            onClick = { },
            kind = CadenceButtonKind.SECONDARY,
            fillWidth = true,
            enabled = false,
        )
        CadenceButton(
            label = "Перенести в запас",
            onClick = { },
            kind = CadenceButtonKind.SECONDARY,
            fillWidth = true,
            enabled = false,
        )
    }
}
