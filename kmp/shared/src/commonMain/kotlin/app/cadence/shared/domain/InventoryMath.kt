package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil

/** §03: «low <25%». */
private const val LOW_FRACTION = 0.25

/** §03: «expiring ≤14 d». */
private const val EXPIRING_DAYS = 14

/** §03: «≤4 weeks supply». */
private const val REORDER_WEEKS = 4

/**
 * How many doses a vial has left.
 *
 * §03's third correction, in one line: «The prototype ships two disconnected
 * vial datasets and logging never decrements stock. Fixed structurally:
 * `remaining = total_doses − count(dose_events.vial_id)`.» There is no counter
 * to drift, because there is no counter — the answer is recomputed on every
 * read, which is what «nothing derived is stored» buys.
 */
fun remainingDoses(
    vial: Vial,
    events: List<DoseEvent>,
): Int = (vial.totalDoses - events.count { it.vialId == vial.id }).coerceAtLeast(0)

/**
 * What the «Аптечка» chip says.
 *
 * Computed, per §03's L10. The order is a precedence and not a coincidence:
 * disposed is a fact about the vial, expiry is the one condition with a
 * deadline attached, and low stock is the softest of the three.
 */
enum class VialStatus { DISPOSED, SEALED, EXPIRING, LOW, ACTIVE }

fun vialStatus(
    vial: Vial,
    events: List<DoseEvent>,
    today: LocalDate,
): VialStatus {
    val daysToExpiry = today.daysUntil(vial.expiresOn)
    val remaining = remainingDoses(vial, events)

    // One `when`, so the precedence is a list that can be read top to bottom
    // rather than a sequence of early exits whose order is implicit.
    return when {
        vial.disposedAt != null -> VialStatus.DISPOSED

        // Expiry before sealed, deliberately: unopened stock with days left on
        // it is exactly the vial worth warning about, because it is about to be
        // wasted. An earlier order read SEALED and said nothing.
        daysToExpiry <= EXPIRING_DAYS -> VialStatus.EXPIRING

        vial.openedAt == null -> VialStatus.SEALED

        remaining < vial.totalDoses * LOW_FRACTION -> VialStatus.LOW

        else -> VialStatus.ACTIVE
    }
}

/**
 * «Пора заказать» — or nothing at all.
 *
 * §03 sets both conditions: «0 sealed spares & ≤4 weeks supply». Either alone
 * is not a reason to tell a patient on a protocol to order more, and a hint
 * that fires on one of them is a hint people learn to ignore.
 *
 * `dosesPerWeek` comes from the protocol rather than from the vial, because how
 * fast stock runs out is a property of the prescription.
 *
 * Takes no date. An earlier signature did, and detekt caught it unused: §03
 * defines supply by doses, not by shelf life, so a vial expiring next week
 * still counts as supply here while [vialStatus] flags it separately. Whether
 * those two should be one answer is a product question, not a coding one.
 */
data class ReorderHint(
    val compoundId: CompoundId,
    val weeksLeft: Int,
)

fun reorderHint(
    compoundId: CompoundId,
    vials: List<Vial>,
    events: List<DoseEvent>,
    dosesPerWeek: Double,
): ReorderHint? {
    // One compound at a time. Without the filter an unopened vial of anything
    // else counted as the sealed spare that suppresses the hint, its doses
    // counted as this compound's supply, and the hint named whichever vial
    // happened to come first — a patient about to run out was told nothing.
    val live = vials.filter { it.disposedAt == null && it.compoundId == compoundId }
    val hasSealedSpare = live.any { it.openedAt == null }
    if (dosesPerWeek <= 0.0 || live.isEmpty() || hasSealedSpare) return null

    val weeksLeft = (live.sumOf { remainingDoses(it, events) } / dosesPerWeek).toInt()

    return if (weeksLeft > REORDER_WEEKS) null else ReorderHint(compoundId, weeksLeft)
}
