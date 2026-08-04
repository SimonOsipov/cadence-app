package app.cadence.screens.dose

import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.VialRow

/**
 * One row of «Что вы приняли?» — a loggable item, as the compound step draws it.
 *
 * The dose is the one in force today rather than the compound's own constant:
 * the prototype's `COMPOUNDS` carry `default: '0.25'` frozen, and a patient in
 * week six is offered week one's number.
 */
data class DoseOption(
    val itemId: ProtocolItemId,
    val nameRu: String,
    // What kind of item it is, so the site step knows whether a zone is
    // required. Hardcoding INJECTION here would undo the reason `DoseDraft`
    // carries a kind at all: a supplement has no zone, and a wizard that
    // demanded one could not log it.
    val kind: ProtocolItemKind,
    val dose: Dose?,
    /**
     * Where the plunger sits, in insulin units — or null when nobody knows.
     *
     * A mg→units reading is a property of the vial's reconstitution, and the
     * vial picker is not ported yet. The prototype computes
     * `(dose / compound.default) * syringeFill`, which draws the ratio to the
     * prescription rather than a volume; that is wrong for a barrel a patient
     * pulls to a mark, so nothing is drawn until something knows.
     */
    val syringeUnits: Float? = null,
    /**
     * The open vials of this compound, fullest first.
     *
     * The dose step picks from these. Sealed stock is not offered: a vial
     * nobody has opened is not one this dose came out of.
     */
    val vials: List<VialRow> = emptyList(),
    /** «п/к · еженедельно» — route and cadence, rendered. */
    val modeRu: String,
    /** The «сегодня» badge: this item has an occurrence today. */
    val dueToday: Boolean,
)
