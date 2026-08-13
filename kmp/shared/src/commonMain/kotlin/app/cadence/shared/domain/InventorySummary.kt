package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * The groups are **not disjoint**, matching the prototype: an opened expiring vial is in
 * both `active` and `expiring`. Nothing here is stored — every group is a filter over
 * [vialStatus], and [reorder] is [reorderHint] per protocol item, so the cabinet and the
 * Today screen's warning are the same call.
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
 * Resolved here, not left as ids, same reason as [ProtocolRow]. `remaining` is still a
 * subtraction behind the scenes: [inventorySummary] computes it every read, and [Vial] has
 * no field for it.
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

/** [today] is a parameter, not a clock read, so this can be asked about a date other than now. */
fun inventorySummary(
    plan: ProtocolPlan,
    vials: List<Vial>,
    events: List<DoseEvent>,
    today: LocalDate,
    // No default: an optional list means a caller that forgets it gets a cabinet of unnamed
    // vials and no error — exactly what the first version of this screen shipped.
    compounds: List<Compound>,
): InventorySummary {
    val rows = vials.filter { it.disposedAt == null }.map { vialRow(plan, it, events, today, compounds) }

    return InventorySummary(
        active = rows.filter { it.openedAt != null },
        sealed = rows.filter { it.openedAt == null },
        expiring = rows.filter { it.status == VialStatus.EXPIRING },
        low = rows.filter { it.status == VialStatus.LOW },
        // One hint per prescribed compound: an unprescribed one has no rate to divide stock by.
        reorder = plan.items.mapNotNull { reorderHint(it, vials, events, today) }.distinctBy { it.compoundId },
    )
}

/**
 * Shared by the cabinet and the detail sheet on purpose: two resolutions of the same vial
 * are two chances to answer «сколько осталось» differently.
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
