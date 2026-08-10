package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * A vial being added, before it is one.
 *
 * **No dose field, and that is the project rule rather than an omission.** The
 * prototype's «Добавить флакон» asks for «Дозировка · 0,25 мг» and stores it on
 * the vial, which is the same number the protocol's phase already decides — so
 * a vial carrying its own copy is a derived value stored, and the two go out of
 * step the first time a doctor titrates. What §03's `vials` holds is a
 * concentration label, which is a fact about the glass rather than about the
 * prescription.
 *
 * There is no remaining count either, for the same reason and one more: a vial
 * nothing has been drawn from has all its doses, and `remaining` is
 * `totalDoses − count(events)` on every read.
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
     * Whether «Сохранить» is live.
     *
     * Three facts a vial cannot exist without: what is in it, how many doses it
     * holds, and when it stops being usable. The lot and the shelf are how a
     * patient tells two vials apart, and a patient who does not know them yet
     * should still be able to record the vial in front of them.
     *
     * An expiry already past is refused rather than saved: stock that cannot be
     * used is not stock, and a cabinet that accepted it would count doses the
     * patient must not take.
     */
    fun canSave(today: LocalDate): Boolean =
        compoundId != null && totalDoses > 0 && expiresOn != null && expiresOn >= today
}
