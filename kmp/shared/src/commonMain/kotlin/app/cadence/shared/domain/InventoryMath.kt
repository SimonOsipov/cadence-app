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
 * §03's third correction: `remaining = total_doses − count(dose_events.vial_id)`. No
 * counter to drift, because there is no counter.
 */
fun remainingDoses(
    vial: Vial,
    events: List<DoseEvent>,
): Int = (vial.totalDoses - events.count { it.vialId == vial.id }).coerceAtLeast(0)

/**
 * Computed, per §03's L10. Declaration order is precedence: disposed is a fact about the
 * vial, expiry has a deadline, low stock is the softest of the three.
 */
enum class VialStatus { DISPOSED, EXPIRING, SEALED, LOW, ACTIVE }

fun vialStatus(
    vial: Vial,
    events: List<DoseEvent>,
    today: LocalDate,
): VialStatus {
    val daysToExpiry = today.daysUntil(vial.expiresOn)
    val remaining = remainingDoses(vial, events)

    return when {
        vial.disposedAt != null -> VialStatus.DISPOSED

        // Before SEALED, deliberately: unopened stock about to expire is exactly the vial
        // worth warning about. An earlier order read SEALED first and said nothing.
        daysToExpiry <= EXPIRING_DAYS -> VialStatus.EXPIRING

        vial.openedAt == null -> VialStatus.SEALED

        remaining < vial.totalDoses * LOW_FRACTION -> VialStatus.LOW

        else -> VialStatus.ACTIVE
    }
}

/**
 * §03 sets both conditions: «0 sealed spares & ≤4 weeks supply» — either alone is a hint
 * people learn to ignore. Takes [today] to exclude stock that's already expired: an earlier
 * signature dropped the date, and an expired sealed vial made `hasSealedSpare` true, showing
 * a patient two doses from nothing no reorder card at all.
 */
data class ReorderHint(
    val compoundId: CompoundId,
    val weeksLeft: Int,
)

fun reorderHint(
    item: ProtocolItem,
    vials: List<Vial>,
    events: List<DoseEvent>,
    today: LocalDate,
): ReorderHint? {
    // Compound and rate off the same item, so they can't disagree: passed separately, BPC's
    // rate against semaglutide's stock reads as «0 weeks left», silently and plausibly.
    val compoundId = item.compoundId ?: return null
    val dosesPerWeek = item.dosesPerWeek()

    // One compound at a time: without the filter, an unopened vial of anything else
    // counted as the sealed spare that suppresses the hint.
    val live =
        vials.filter {
            it.disposedAt == null && it.compoundId == compoundId && it.expiresOn >= today
        }
    val hasSealedSpare = live.any { it.openedAt == null }
    if (dosesPerWeek <= 0.0 || live.isEmpty() || hasSealedSpare) return null

    val weeksLeft = (live.sumOf { remainingDoses(it, events) } / dosesPerWeek).toInt()

    return if (weeksLeft > REORDER_WEEKS) null else ReorderHint(compoundId, weeksLeft)
}
