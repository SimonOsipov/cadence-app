package app.cadence.screens.inventory

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceChip
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.format.pluralVials
import app.cadence.shared.domain.InventorySummary
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.VialRow

/** One count on the summary card, by the group it counts. */
fun cabinetStatTag(group: String) = "cabinet-stat-$group"

private val ADD_BUTTON = 44.dp

/**
 * «Ваша аптечка» — the cabinet.
 *
 * Takes a resolved [InventorySummary] and nothing else: no events, no plan,
 * nothing it could compute a remaining count from. A screen that could compute
 * one could disagree with the Today screen about the same vial.
 */
@Composable
fun VialsScreen(
    cabinet: InventorySummary,
    modifier: Modifier = Modifier,
    onOpenVial: (VialId) -> Unit = { },
    onAddVial: () -> Unit = { },
) {
    var filter by remember { mutableStateOf(VialFilter.ALL) }
    // Collapsed, like the prototype's: a spare the patient is not using is the
    // last thing they came here to read.
    var sealedOpen by remember { mutableStateOf(false) }

    Column(
        modifier
            .fillMaxSize()
            .background(Cadence.palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars),
    ) {
        CabinetHeader(cabinet, onAddVial)

        Column(
            Modifier
                .verticalScroll(rememberScrollState())
                .padding(horizontal = CadenceSpacing.lg),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        ) {
            if (cabinet.all.isEmpty()) {
                EmptyCabinet(onAddVial)
            } else {
                CabinetSummaryCard(cabinet)
                FilterChips(cabinet, filter, onFilter = { filter = it })

                cabinet.of(filter).filter { it.openedAt != null || filter != VialFilter.ALL }.forEach {
                    VialCard(row = it, onClick = { onOpenVial(it.id) })
                }

                if (filter == VialFilter.ALL && cabinet.sealed.isNotEmpty()) {
                    SealedSection(cabinet.sealed, sealedOpen, onToggle = { sealedOpen = !sealedOpen }, onOpenVial)
                }
            }
        }
    }
}

@Composable
private fun CabinetHeader(
    cabinet: InventorySummary,
    onAddVial: () -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(CadenceSpacing.lg),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            // The rule, not the prototype's ternary: it answers «флакона» for
            // 21, which is «флакон».
            CadenceMeta("${cabinet.all.size} ${pluralVials(cabinet.all.size)} в холодильнике")
            CadenceTitle(
                buildAnnotatedString {
                    append("Ваша ")
                    withStyle(SpanStyle(fontStyle = FontStyle.Italic, color = Cadence.palette.forestDeep)) {
                        append("аптечка")
                    }
                },
            )
        }

        Box(
            Modifier
                .size(ADD_BUTTON)
                .clip(CircleShape)
                .background(CadenceColors.forest700)
                .clickable(onClick = onAddVial)
                .clearAndSetSemantics { contentDescription = "Добавить флакон" },
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(name = "plus", tint = CadenceColors.cream)
        }
    }
}

@Composable
private fun CabinetSummaryCard(cabinet: InventorySummary) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.xl))
            .background(Cadence.palette.paper)
            .padding(CadenceSpacing.lg),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        // Tagged, because «Активные» and «На исходе» are also a chip's label
        // and a pill's, and a test asking by text alone cannot say which of the
        // three it found.
        listOf(
            Triple("Активные", cabinet.active.size, VialFilter.ACTIVE),
            Triple("Истекают", cabinet.expiring.size, VialFilter.EXPIRING),
            Triple("Запас", cabinet.sealed.size, null),
            Triple("На исходе", cabinet.low.size, VialFilter.LOW),
        ).forEach { (label, count, filter) ->
            Column(
                Modifier.testTag(cabinetStatTag(filter?.name ?: "SEALED")),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                CadenceTitle(count.toString())
                CadenceMeta(label)
            }
        }
    }
}

@Composable
private fun FilterChips(
    cabinet: InventorySummary,
    filter: VialFilter,
    onFilter: (VialFilter) -> Unit,
) {
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        VialFilter.entries.forEach { candidate ->
            CadenceChip(
                label = "${candidate.labelRu} · ${cabinet.of(candidate).size}",
                onClick = { onFilter(candidate) },
                active = candidate == filter,
            )
        }
    }
}

@Composable
private fun SealedSection(
    sealed: List<VialRow>,
    open: Boolean,
    onToggle: () -> Unit,
    onOpenVial: (VialId) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        Row(
            Modifier
                .fillMaxWidth()
                .clickable(onClick = onToggle)
                .padding(vertical = CadenceSpacing.sm),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            CadenceEyebrow("В запасе · ${sealed.size}")
            CadenceIcon(name = if (open) "chevrn-up" else "chevrn-dwn", tint = Cadence.palette.muted)
        }

        if (open) sealed.forEach { VialCard(row = it, onClick = { onOpenVial(it.id) }) }
    }
}

@Composable
private fun EmptyCabinet(onAddVial: () -> Unit) {
    Column(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.xl))
            .background(Cadence.palette.paper)
            .padding(CadenceSpacing.huge),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
    ) {
        CadenceIcon(name = "beaker", tint = CadenceColors.sand700)
        CadenceTitle("Аптечка пуста")
        CadenceBody("Добавьте флакон, чтобы видеть остаток и сроки.", color = Cadence.palette.muted)
        CadenceChip(label = "Добавить флакон", onClick = onAddVial, active = true)
    }
}
