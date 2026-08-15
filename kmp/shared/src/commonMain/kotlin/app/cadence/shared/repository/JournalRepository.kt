package app.cadence.shared.repository

import app.cadence.shared.domain.CheckInDraft
import app.cadence.shared.domain.JournalEntry
import kotlinx.datetime.LocalDate

/** Sealed rather than nullable: a sheet says «you named nothing» and «that is off the scale» differently. */
sealed interface JournalSaveResult {
    /** The stored entry, not the draft — the merge is where the two differ. */
    data class Written(
        val entry: JournalEntry,
    ) : JournalSaveResult

    sealed interface Rejected : JournalSaveResult {
        /** Nothing was named — [CheckInDraft.saysNothing]. */
        data object Empty : Rejected

        /** A reading outside 1..5 — [CheckInDraft.readingsAreOnTheScale]. */
        data object OffTheScale : Rejected
    }
}

/** §03's `journal_entries`. The date is the identity, so a write updates the day rather than adding a second. */
interface JournalRepository {
    /** The one entry for that day, or null if the patient has written nothing. */
    suspend fun entry(date: LocalDate): JournalEntry?

    /** A check-in written by hand; the wizard's goes through [DoseLogRepository.submit] and merges by the same rule. */
    suspend fun save(draft: CheckInDraft): JournalSaveResult
}
