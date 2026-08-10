package app.cadence.screens.inventory

import app.cadence.shared.domain.InventorySummary
import app.cadence.shared.domain.VialRow

/** The five ways «Аптечка» can be sliced, in the prototype's order. */
enum class VialFilter(
    val labelRu: String,
) {
    ALL("Все"),
    ACTIVE("Активные"),
    EXPIRING("Истекают"),
    LOW("На исходе"),
}

/** The vials this filter shows. */
internal fun InventorySummary.of(filter: VialFilter): List<VialRow> =
    when (filter) {
        VialFilter.ALL -> all
        VialFilter.ACTIVE -> active
        VialFilter.EXPIRING -> expiring
        VialFilter.LOW -> low
    }
