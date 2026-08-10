package app.cadence.shared.repository

import app.cadence.shared.domain.InventorySummary
import app.cadence.shared.domain.VialDetail
import app.cadence.shared.domain.VialDraft
import app.cadence.shared.domain.VialId

/** What adding a vial did. */
sealed interface AddVialResult {
    data class Added(
        val id: VialId,
    ) : AddVialResult

    /**
     * The draft is missing something a vial cannot exist without, or its expiry
     * has already passed — see `VialDraft.canSave`.
     *
     * One case rather than two: the screen's own button is dead until the draft
     * is savable, so a caller reaching this has bypassed the form, and telling
     * it *which* rule it broke would be the form's knowledge restated where
     * nothing reads it.
     */
    data object Rejected : AddVialResult
}

/**
 * §11: `GET /me/vials`, `POST /me/vials`.
 *
 * The cabinet is read whole rather than as a list of vials: the groups and the
 * reorder hints are computed from the same events the remaining counts are, and
 * a screen assembling them itself could disagree with the Today warning about
 * the same compound.
 */
interface InventoryRepository {
    /** The cabinet as «Аптечка» browses it, for today. */
    suspend fun cabinet(): InventorySummary

    /** One vial with its records and its weeks, or null if it is not the patient's. */
    suspend fun vial(id: VialId): VialDetail?

    suspend fun addVial(draft: VialDraft): AddVialResult
}
