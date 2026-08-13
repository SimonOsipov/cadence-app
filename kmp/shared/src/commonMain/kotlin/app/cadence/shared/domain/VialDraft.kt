package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * **No dose field, and that's the project rule, not an omission.** The prototype's «Добавить
 * флакон» stores the dose on the vial — the same number the protocol's phase already
 * decides, a derived value going stale the first time a doctor titrates. §03's `vials` holds
 * a concentration label, a fact about the glass, not the prescription. No remaining count
 * either: `remaining` is `totalDoses − count(events)` on every read.
 */
data class VialDraft(
    val compoundId: CompoundId? = null,
    /** §03's `concentration_label` — «1 мг/мл», as printed on the vial. */
    val concentrationLabel: String? = null,
    val totalDoses: Int = 0,
    val expiresOn: LocalDate? = null,
    val lot: String? = null,
    val locationRu: String? = null,
) {
    /**
     * Three facts a vial can't exist without: what's in it, how many doses, when it expires.
     * Lot and shelf are optional — a patient who doesn't know them yet can still record the
     * vial. Already-expired is refused: a cabinet accepting it would count doses the patient
     * must not take.
     */
    fun canSave(today: LocalDate): Boolean =
        compoundId != null && totalDoses > 0 && expiresOn != null && expiresOn >= today
}
