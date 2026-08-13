package app.cadence.screens.dose

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import app.cadence.design.CADENCE_MOOD_LABELS
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceBodyMap
import app.cadence.design.CadenceBodyView
import app.cadence.design.CadenceBodyZone
import app.cadence.design.CadenceChip
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceDoseStepper
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceMoodSlider
import app.cadence.design.CadencePill
import app.cadence.design.CadencePillTone
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceSyringeBar
import app.cadence.design.CadenceTextField
import app.cadence.design.CadenceTitle
import app.cadence.format.formatDose
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.SideEffect

/** «Тошнота», «Усталость» … — the prototype's seven, in its order. */
val SIDE_EFFECT_LABELS_RU: Map<SideEffect, String> =
    mapOf(
        SideEffect.NAUSEA to "Тошнота",
        SideEffect.FATIGUE to "Усталость",
        SideEffect.HEADACHE to "Голова",
        SideEffect.BLOATING to "Вздутие",
        SideEffect.INSOMNIA to "Бессонница",
        SideEffect.SITE to "Шишка",
        SideEffect.APPETITE to "Нет аппетита",
    )

/** «Шаг 1 · Препарат» — one row per loggable item. */
@Composable
fun CompoundStep(
    options: List<DoseOption>,
    draft: DoseDraft,
    onDraft: (DoseDraft) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        options.forEach { option ->
            val chosen = option.itemId == draft.itemId

            Row(
                Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(CadenceRadius.md))
                    .background(if (chosen) Cadence.palette.paper else Cadence.palette.bg)
                    .border(
                        width = HAIRLINE,
                        color = if (chosen) CadenceColors.forest700 else Cadence.palette.border,
                        shape = RoundedCornerShape(CadenceRadius.md),
                    ).clickable {
                        // Re-tapping the chosen row is a no-op in `selectItem`;
                        // here the dose comes off the option, which already
                        // holds the phase in force rather than a constant.
                        if (!chosen) {
                            onDraft(
                                draft.copy(
                                    itemId = option.itemId,
                                    kind = option.kind,
                                    dose = option.dose,
                                ),
                            )
                        }
                    }.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
                horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                RadioDot(chosen)

                Column(Modifier.weight(1f)) {
                    Row(
                        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        CadenceBody(option.nameRu)
                        if (option.dueToday) CadencePill("сегодня", tone = CadencePillTone.SAND)
                    }
                    CadenceMeta(option.dose?.let { doseLine(it, option.modeRu) } ?: option.modeRu)
                }
            }
        }
    }
}

private fun doseLine(
    dose: Dose,
    mode: String,
): String {
    val (value, unit) = formatDose(dose)

    return "$value $unit · $mode"
}

@Composable
private fun RadioDot(selected: Boolean) {
    Box(
        Modifier
            .size(RADIO_OUTER)
            .clip(CircleShape)
            .border(
                width = RADIO_RING,
                color = if (selected) CadenceColors.forest700 else Cadence.palette.border,
                shape = CircleShape,
            ),
        contentAlignment = Alignment.Center,
    ) {
        if (selected) {
            Box(
                Modifier
                    .size(RADIO_INNER)
                    .clip(CircleShape)
                    .background(CadenceColors.forest700),
            )
        }
    }
}

private val HAIRLINE = 1.dp
private val RADIO_OUTER = 20.dp
private val RADIO_INNER = 10.dp
private val RADIO_RING = 1.5.dp

/** «Шаг 2 · Доза» — the stepper and the barrel under it. */
@Composable
fun DoseAmountStep(
    draft: DoseDraft,
    option: DoseOption?,
    onDraft: (DoseDraft) -> Unit,
) {
    val dose = draft.dose ?: return

    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxl)) {
        Column(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(CadenceRadius.xl))
                .background(Cadence.palette.paper)
                .border(HAIRLINE, Cadence.palette.hairline, RoundedCornerShape(CadenceRadius.xl))
                .padding(CadenceSpacing.lg),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxl),
        ) {
            CadenceDoseStepper(dose = dose, onChange = { onDraft(draft.copy(dose = it)) })

            // Only when something knows the reading. A barrel drawn from a
            // guessed concentration is an instruction telling a patient how far
            // to pull a plunger, and three of the prototype's four compounds
            // would be shown a wrong or a saturated one. §03 stores
            // `concentration_label` as a label — «1 мг/мл» — so even with the
            // vial in hand there is no number to divide by yet.
            option?.syringeUnits?.let {
                Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
                    CadenceEyebrow("На шприце ${SYRINGE_MAX.toInt()} ед.")
                    CadenceSyringeBar(units = it, max = SYRINGE_MAX)
                }
            }
        }

        VialPicker(option, draft, onDraft)
    }
}

/**
 * «Из вашей аптечки» — which vial this dose comes out of. Nothing is drawn with one
 * open vial of the compound: a choice with one option is not a choice, and the write
 * makes the same one. Two open at once is the case §03 allows, the seed contains and
 * the prototype's own `VIALS` has — the only one where the app cannot know.
 */
@Composable
private fun VialPicker(
    option: DoseOption?,
    draft: DoseDraft,
    onDraft: (DoseDraft) -> Unit,
) {
    val vials = option?.vials.orEmpty()
    if (vials.size < 2) return

    // Into the draft, not just onto the screen: shown but unrecorded, the picker's
    // «fullest open vial» and the write's own are two implementations of one rule
    // that agree until one of them changes.
    LaunchedEffect(vials.first().id) {
        if (draft.vialId == null) onDraft(draft.copy(vialId = vials.first().id))
    }

    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        CadenceEyebrow("Из вашей аптечки")

        vials.forEach { vial ->
            val chosen = vial.id == draft.vialId

            Row(
                Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(CadenceRadius.md))
                    .background(if (chosen) Cadence.palette.paper else Cadence.palette.bg)
                    .border(
                        width = HAIRLINE,
                        color = if (chosen) CadenceColors.forest700 else Cadence.palette.border,
                        shape = RoundedCornerShape(CadenceRadius.md),
                        // `selectable` rather than `clickable`: which row is chosen
                        // is drawn only in a border colour and a dot, so without
                        // the semantics neither a screen reader nor a test can tell
                        // the vials apart — and «the wizard opened on the vial the
                        // patient had in their hand» is exactly the thing a test
                        // has to be able to say.
                    ).selectable(selected = chosen) { onDraft(draft.copy(vialId = vial.id)) }
                    .padding(CadenceSpacing.md),
                horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                RadioDot(chosen)
                CadenceBody(vial.lot ?: "—", modifier = Modifier.weight(1f))
                CadenceMeta("${vial.remaining} доз")
            }
        }
    }
}

/** «На шприце 100 ед.» — the pen the prototype draws. */
private const val SYRINGE_MAX = 100f

/** «Шаг 3 · Место» — the diagram, opened on the rotation's suggestion. */
@Composable
fun SiteStep(
    draft: DoseDraft,
    suggested: InjectionSite,
    lastUsed: List<InjectionSite>,
    onDraft: (DoseDraft) -> Unit,
) {
    // Which way round the body is showing is view state, not part of the
    // draft — but it has to be state *somewhere*. Left on the default, the
    // «Сзади» toggle draws and does nothing, and the four zones behind it
    // cannot be chosen at all: the rotation would suggest a glute and then
    // refuse to accept it, and the patient would record an injection
    // somewhere they did not make one.
    var view by remember { mutableStateOf(CadenceBodyView.FRONT) }

    CadenceBodyMap(
        selected = draft.site,
        suggested = suggested,
        view = view,
        lastUsed = lastUsed,
        onView = { view = it },
        onSelect = { onDraft(draft.copy(site = it)) },
    )
}

/** «Шаг 4 · Контекст» — mood, what is bothering them, a note, a photo slot. */
@Composable
fun ContextStep(
    draft: DoseDraft,
    onDraft: (DoseDraft) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxl)) {
        Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
            CadenceEyebrow("Энергия · сегодня")
            CadenceMoodSlider(value = draft.mood, onChange = { onDraft(draft.copy(mood = it)) })
        }

        Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
            CadenceEyebrow("Что-то беспокоит?")
            SideEffectChips(draft, onDraft)
        }

        Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
            CadenceEyebrow("Заметки")
            NoteField(draft, onDraft)
        }

        PhotoSlot(draft, onDraft)
    }
}

/**
 * The slot renders and reports a tap; nothing uploads. §03 routes photos straight to
 * object storage under a path convention — the one documented client→data exception
 * — so the upload lands with the storage work; omitting the slot entirely would leave
 * `DoseDraft.photoAttached` with no writer.
 */
@Composable
private fun PhotoSlot(
    draft: DoseDraft,
    onDraft: (DoseDraft) -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.md))
            .background(Cadence.palette.paper)
            .border(HAIRLINE, Cadence.palette.border, RoundedCornerShape(CadenceRadius.md))
            .clickable { onDraft(draft.copy(photoAttached = !draft.photoAttached)) }
            .padding(CadenceSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIcon(paths = CadenceIcons.camera, tint = Cadence.palette.muted)
        CadenceBody(if (draft.photoAttached) "Фото прикреплено" else "Добавить фото")
    }
}

@Composable
private fun SideEffectChips(
    draft: DoseDraft,
    onDraft: (DoseDraft) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        SIDE_EFFECT_LABELS_RU.entries.chunked(CHIPS_PER_ROW).forEach { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
                row.forEach { (effect, label) ->
                    CadenceChip(
                        label = label,
                        active = effect in draft.sideEffects,
                        onClick = {
                            val next =
                                if (effect in draft.sideEffects) {
                                    draft.sideEffects - effect
                                } else {
                                    draft.sideEffects + effect
                                }
                            onDraft(draft.copy(sideEffects = next))
                        },
                    )
                }
            }
        }
    }
}

private const val CHIPS_PER_ROW = 3

@Composable
private fun NoteField(
    draft: DoseDraft,
    onDraft: (DoseDraft) -> Unit,
) {
    CadenceTextField(
        value = draft.note.orEmpty(),
        // Empty means «не написали», not «написали пустое»: a blank note
        // would render an empty row in the review.
        onValueChange = { onDraft(draft.copy(note = it.ifBlank { null })) },
        placeholder = "Что-то важное про эту дозу?",
    )
}

/** «Шаг 5 · Проверка» — the hero card and the rows under it. */
@Composable
fun ReviewStep(
    draft: DoseDraft,
    option: DoseOption?,
) {
    Column(verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
        Column(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(CadenceRadius.xxl))
                .background(Cadence.palette.forestBg)
                .padding(CadenceSpacing.xxl),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
        ) {
            CadenceTitle(option?.nameRu ?: "—", color = CadenceColors.cream)

            // Two runs, not one string: the number and the unit are set in
            // different faces, and a dose is never a rendered whole.
            draft.dose?.let {
                val (value, unit) = formatDose(it)

                Row(
                    horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
                    verticalAlignment = Alignment.Bottom,
                ) {
                    CadenceTitle(value, color = CadenceColors.sand300)
                    CadenceBody(unit, color = CadenceColors.sand300)
                }
            }

            CadenceBody(
                draft.site?.let { CadenceBodyZone.of(it)?.labelRu } ?: "—",
                modifier = Modifier.padding(top = CadenceSpacing.sm),
                color = CadenceColors.cream,
            )
        }

        Column(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(CadenceRadius.lg))
                .background(Cadence.palette.paper)
                .border(HAIRLINE, Cadence.palette.hairline, RoundedCornerShape(CadenceRadius.lg)),
        ) {
            ReviewRow("Энергия", draft.mood?.let { CADENCE_MOOD_LABELS.getOrNull(it - 1) } ?: "—")
            ReviewRow(
                "Заметки",
                draft.sideEffects
                    .mapNotNull { SIDE_EFFECT_LABELS_RU[it] }
                    .joinToString(" · ")
                    .ifEmpty { "—" },
            )
            draft.note?.let { ReviewRow("Заметка", it) }
        }
    }
}

@Composable
private fun ReviewRow(
    label: String,
    value: String,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
    ) {
        CadenceMeta(label, modifier = Modifier.weight(1f))
        CadenceBody(value, modifier = Modifier.weight(WIDE_COLUMN))
    }
}

private const val WIDE_COLUMN = 2f
