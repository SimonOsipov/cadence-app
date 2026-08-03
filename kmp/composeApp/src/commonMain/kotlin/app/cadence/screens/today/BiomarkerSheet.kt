package app.cadence.screens.today

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceNumber
import app.cadence.design.CadencePill
import app.cadence.design.CadenceSheet
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceSpark
import app.cadence.design.CadenceTitle
import app.cadence.format.formatDecimal
import app.cadence.format.formatDelta
import app.cadence.format.unitRu

private val SHEET_SPARK_HEIGHT = 70.dp
private const val READING_DIGITS = 1

/**
 * The sheet behind the headline glance, ported from
 * mobile/src/features/today/BiomarkerSheet.tsx.
 *
 * Takes plain values rather than a repository or a `MetricSeries`: the glance
 * above it already holds the same numbers, and two components rendering one
 * series from two shapes is where they start to disagree.
 */
@Composable
fun BiomarkerSheet(
    open: Boolean,
    title: String,
    points: List<Double>,
    latest: Double?,
    unit: String,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
    onOpenTrend: () -> Unit = { },
) {
    CadenceSheet(open = open, onDismiss = onDismiss, modifier = modifier) {
        CadenceEyebrow("Биомаркер", Modifier.padding(bottom = CadenceSpacing.xs))
        CadenceTitle(title, size = 32.sp)

        if (latest == null) {
            // Not an empty chart: a blank box beside a caption reads as «failed
            // to load», and a patient waits instead of measuring.
            BasicText(
                text = "Пока нет измерений",
                modifier = Modifier.padding(vertical = CadenceSpacing.lg),
                style = Cadence.typography.body.copy(color = Cadence.palette.subtle),
            )
        } else {
            Reading(points, latest, unit)
        }

        CadenceButton(
            label = "Открыть детали тренда",
            onClick = onOpenTrend,
            kind = CadenceButtonKind.SECONDARY,
            fillWidth = true,
            modifier = Modifier.padding(top = CadenceSpacing.lg),
        )
    }
}

@Composable
private fun Reading(
    points: List<Double>,
    value: Double,
    unit: String,
) {
    val delta = formatDelta(points, unitRu(unit))

    Column(
        modifier = Modifier.fillMaxWidth().padding(top = CadenceSpacing.md),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        CadenceNumber(value = formatDecimal(value, READING_DIGITS), unit = unitRu(unit), size = 34.sp)

        if (delta != null) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                CadencePill(delta)
                BasicText(
                    text = "к прошлой неделе",
                    style = Cadence.typography.meta.copy(color = Cadence.palette.subtle, fontSize = 12.sp),
                )
            }
        }

        CadenceSpark(
            data = points.map { it.toFloat() },
            modifier = Modifier.fillMaxWidth(),
            fill = CadenceColors.forest50,
            height = SHEET_SPARK_HEIGHT,
        )
    }
}
