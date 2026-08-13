package app.cadence.screens.inventory

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTextField
import app.cadence.design.CadenceTitle
import app.cadence.shared.domain.Compound
import app.cadence.shared.domain.CompoundId
import app.cadence.shared.domain.VialDraft
import kotlinx.datetime.LocalDate

/** A field of «Добавить флакон», by what it asks for. */
fun addVialFieldTag(field: String) = "add-vial-$field"

/** The save button, whose label is also a word on other screens. */
const val ADD_VIAL_SAVE_TAG = "add-vial-save"

private val HAIRLINE = 1.dp

/**
 * «Добавить флакон».
 *
 * **No dose field.** The prototype asks «Дозировка · 0,25 мг» and stores it on the
 * vial — the same number the protocol's phase already decides, a derived value
 * stored that goes out of step the first time a doctor titrates. What the glass
 * carries is a concentration: §03's `concentration_label`, a fact about the vial
 * rather than the prescription.
 */
@Composable
fun AddVialScreen(
    compounds: List<Compound>,
    today: LocalDate,
    modifier: Modifier = Modifier,
    onSave: (VialDraft) -> Unit = { },
    onCancel: () -> Unit = { },
) {
    var draft by remember { mutableStateOf(VialDraft()) }
    var expiryText by remember { mutableStateOf("") }

    // Typed but unusable: the field holds what was typed, and the draft holds
    // only a date it could parse. Otherwise a half-typed date reads as no date
    // and the reason for a dead button is invisible.
    val expired = draft.expiresOn?.let { it < today } == true

    Column(
        modifier
            .fillMaxSize()
            .background(Cadence.palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars)
            .verticalScroll(rememberScrollState())
            .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.lg),
    ) {
        CadenceTitle("Новый флакон")

        CompoundPicker(compounds, draft.compoundId, onPick = { draft = draft.copy(compoundId = it) })

        VialFields(draft, expiryText, expired, onDraft = { draft = it }, onExpiryText = { expiryText = it })

        CadenceButton(
            label = "Сохранить",
            onClick = { onSave(draft) },
            modifier = Modifier.testTag(ADD_VIAL_SAVE_TAG),
            fillWidth = true,
            enabled = draft.canSave(today),
        )
        CadenceButton(
            label = "Отмена",
            onClick = onCancel,
            fillWidth = true,
            kind = app.cadence.design.CadenceButtonKind.GHOST,
        )
    }
}

@Composable
private fun Field(
    label: String,
    tag: String,
    placeholder: String,
    value: String,
    onChange: (String) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs)) {
        CadenceEyebrow(label)
        CadenceTextField(
            value = value,
            onValueChange = onChange,
            placeholder = placeholder,
            fieldModifier = Modifier.testTag(addVialFieldTag(tag)),
        )
    }
}

@Composable
private fun CompoundPicker(
    compounds: List<Compound>,
    chosenId: CompoundId?,
    onPick: (CompoundId) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        CadenceEyebrow("Препарат")
        compounds.forEach { compound ->
            val chosen = compound.id == chosenId

            Box(
                Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(CadenceRadius.md))
                    .background(if (chosen) Cadence.palette.paper else Cadence.palette.bg)
                    .border(
                        HAIRLINE,
                        if (chosen) CadenceColors.forest700 else Cadence.palette.border,
                        RoundedCornerShape(CadenceRadius.md),
                    ).clickable { onPick(compound.id) }
                    .padding(CadenceSpacing.md),
            ) {
                CadenceBody(compound.nameRu)
            }
        }
    }
}

@Composable
private fun VialFields(
    draft: VialDraft,
    expiryText: String,
    expired: Boolean,
    onDraft: (VialDraft) -> Unit,
    onExpiryText: (String) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.lg)) {
        Field(
            label = "Концентрация",
            tag = "concentration",
            placeholder = "1 мг/мл",
            value = draft.concentrationLabel.orEmpty(),
            onChange = { onDraft(draft.copy(concentrationLabel = it.ifBlank { null })) },
        )

        Field(
            label = "Сколько доз",
            tag = "total",
            placeholder = "12",
            value = if (draft.totalDoses == 0) "" else draft.totalDoses.toString(),
            onChange = { onDraft(draft.copy(totalDoses = it.toIntOrNull() ?: 0)) },
        )

        Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs)) {
            Field(
                label = "До какого",
                tag = "expires",
                placeholder = "2026-12-01",
                value = expiryText,
                onChange = {
                    onExpiryText(it)
                    onDraft(draft.copy(expiresOn = runCatching { LocalDate.parse(it) }.getOrNull()))
                },
            )
            if (expired) CadenceMeta("Срок уже истёк", color = CadenceColors.danger)
        }

        Field(
            label = "Лот",
            tag = "lot",
            placeholder = "A24-0312",
            value = draft.lot.orEmpty(),
            onChange = { onDraft(draft.copy(lot = it.ifBlank { null })) },
        )

        Field(
            label = "Хранение",
            tag = "location",
            placeholder = "Холодильник, полка 2",
            value = draft.locationRu.orEmpty(),
            onChange = { onDraft(draft.copy(locationRu = it.ifBlank { null })) },
        )
    }
}
