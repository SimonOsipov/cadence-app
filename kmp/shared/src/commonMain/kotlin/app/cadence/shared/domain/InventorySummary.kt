package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * The cabinet, split the four ways «Аптечка» browses it.
 *
 * The groups are **not disjoint**, and that is the prototype's own shape:
 * `inventorySummary` in `inventory/data.ts` puts an opened expiring vial in
 * both `active` and `expiring`, and an unopened one in both `sealed` and
 * `expiring`. A patient asking «что у меня активно» and a patient asking «что
 * скоро истечёт» are asking about the same vial from two sides, and a
 * partition would drop it out of one of the lists they browse.
 *
 * Nothing here is stored. Every group is a filter over [vialStatus], and
 * [reorder] is [reorderHint] asked once per protocol item — so the cabinet and
 * the Today screen's warning cannot disagree, because they are the same call.
 */
data class InventorySummary(
    val active: List<Vial>,
    val sealed: List<Vial>,
    val expiring: List<Vial>,
    val low: List<Vial>,
    val reorder: List<ReorderHint>,
)

/**
 * What «Аптечка» shows, for one day.
 *
 * [today] is a parameter rather than a clock read: which vials are expiring is
 * a question about a date, and a function that fetched its own «now» could not
 * be asked about any other one.
 */
fun inventorySummary(
    plan: ProtocolPlan,
    vials: List<Vial>,
    events: List<DoseEvent>,
    today: LocalDate,
): InventorySummary {
    // A vial the patient threw away is history, not stock. The prototype has no
    // disposed state and so never had to decide; counting one would tell a
    // patient they have doses they do not have.
    val live = vials.filter { it.disposedAt == null }
    val status = live.associateWith { vialStatus(it, events, today) }

    return InventorySummary(
        active = live.filter { it.openedAt != null },
        sealed = live.filter { it.openedAt == null },
        expiring = live.filter { status[it] == VialStatus.EXPIRING },
        low = live.filter { status[it] == VialStatus.LOW },
        // One hint per prescribed compound, and none for a vial of something
        // the patient is not on: «weeks left» is stock divided by a rate, and
        // an unprescribed compound has no rate to divide by.
        reorder = plan.items.mapNotNull { reorderHint(it, vials, events) }.distinctBy { it.compoundId },
    )
}
