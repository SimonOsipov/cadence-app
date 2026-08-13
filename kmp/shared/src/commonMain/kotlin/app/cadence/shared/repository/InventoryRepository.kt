package app.cadence.shared.repository

import app.cadence.shared.domain.InventorySummary
import app.cadence.shared.domain.VialDetail
import app.cadence.shared.domain.VialDraft
import app.cadence.shared.domain.VialId

sealed interface AddVialResult {
    data class Added(
        val id: VialId,
    ) : AddVialResult

    /**
     * Fails `VialDraft.canSave`. One case, not two: the screen's own save button is already
     * dead until the draft passes, so a caller reaching this bypassed the form — naming
     * which rule broke would just restate the form's own knowledge where nothing reads it.
     */
    data object Rejected : AddVialResult
}

/**
 * §11: `GET /me/vials`, `POST /me/vials`. Read whole, not as a list of vials — the groups
 * and reorder hints come from the same events the remaining counts do, and a screen
 * assembling them itself could disagree with the Today warning about the same compound.
 */
interface InventoryRepository {
    /** The cabinet as «Аптечка» browses it, for today. */
    suspend fun cabinet(): InventorySummary

    /** One vial with its records and its weeks, or null if it is not the patient's. */
    suspend fun vial(id: VialId): VialDetail?

    suspend fun addVial(draft: VialDraft): AddVialResult
}
