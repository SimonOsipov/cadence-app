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
    val active: List<VialRow>,
    val sealed: List<VialRow>,
    val expiring: List<VialRow>,
    val low: List<VialRow>,
    val reorder: List<ReorderHint>,
) {
    /** Every live vial, once, in the order the cabinet holds them. */
    val all: List<VialRow> get() = (active + sealed).distinctBy { it.id }
}

/**
 * One vial as a screen reads it.
 *
 * Resolved here rather than left as ids, for the same reason [ProtocolRow] is:
 * a card that had to look a compound's name up, or count events to find what is
 * left, would need a repository it is not allowed to have — and could disagree
 * with the Today screen about the same number.
 *
 * `remaining` is a value on this row and a *subtraction* behind it. There is
 * still nothing stored: [inventorySummary] computes it on every read, and
 * [Vial] has no field to put it in.
 */
data class VialRow(
    val id: VialId,
    val compound: Compound?,
    /** The dose in force for this compound today, or null if it is not prescribed. */
    val dose: Dose?,
    val remaining: Int,
    val totalDoses: Int,
    val status: VialStatus,
    val openedAt: LocalDate?,
    val expiresOn: LocalDate,
    val lot: String?,
    val locationRu: String?,
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
    // No default. An optional list means a caller that forgets it gets a
    // cabinet of unnamed vials and no error — which is exactly what the first
    // version of this screen shipped.
    compounds: List<Compound>,
): InventorySummary {
    // A vial the patient threw away is history, not stock. The prototype has no
    // disposed state and so never had to decide; counting one would tell a
    // patient they have doses they do not have.
    val rows = vials.filter { it.disposedAt == null }.map { vialRow(plan, it, events, today, compounds) }

    return InventorySummary(
        active = rows.filter { it.openedAt != null },
        sealed = rows.filter { it.openedAt == null },
        expiring = rows.filter { it.status == VialStatus.EXPIRING },
        low = rows.filter { it.status == VialStatus.LOW },
        // One hint per prescribed compound, and none for a vial of something
        // the patient is not on: «weeks left» is stock divided by a rate, and
        // an unprescribed compound has no rate to divide by.
        reorder = plan.items.mapNotNull { reorderHint(it, vials, events) }.distinctBy { it.compoundId },
    )
}

/**
 * One vial, resolved.
 *
 * Shared by the cabinet and the detail sheet on purpose: two resolutions of the
 * same vial are two chances to answer «сколько осталось» differently on two
 * screens the patient moves between with one tap.
 */
fun vialRow(
    plan: ProtocolPlan,
    vial: Vial,
    events: List<DoseEvent>,
    today: LocalDate,
    compounds: List<Compound>,
): VialRow =
    VialRow(
        id = vial.id,
        compound = compounds.firstOrNull { it.id == vial.compoundId },
        dose =
            plan.items
                .firstOrNull { it.compoundId == vial.compoundId }
                ?.let { phaseDose(plan, it.id, today) },
        remaining = remainingDoses(vial, events),
        totalDoses = vial.totalDoses,
        status = vialStatus(vial, events, today),
        openedAt = vial.openedAt,
        expiresOn = vial.expiresOn,
        lot = vial.lot,
        locationRu = vial.locationRu,
    )
