package app.cadence.shared.repository

import app.cadence.shared.domain.CheckInDraft
import app.cadence.shared.domain.JournalEntry
import kotlinx.datetime.LocalDate

/**
 * A sealed answer rather than a nullable entry: «you named nothing» and «that reading is off
 * the scale» are two things a sheet says differently, and one `null` makes them one.
 */
sealed interface JournalSaveResult {
    /** Carries the stored entry, not the draft: the merge is exactly where the two differ. */
    data class Written(
        val entry: JournalEntry,
    ) : JournalSaveResult

    sealed interface Rejected : JournalSaveResult {
        /** Nothing was named. See [CheckInDraft.saysNothing]. */
        data object Empty : Rejected

        /** A reading outside 1..5. See [CheckInDraft.readingsAreOnTheScale]. */
        data object OffTheScale : Rejected
    }
}

/**
 * §03's `journal_entries`. `PUT /me/journal/{date}` updates the day's entry rather than adding
 * a second — the date is the identity, so there is no id to hand back.
 */
interface JournalRepository {
    /** The one entry for that day, or null if the patient has written nothing. */
    suspend fun entry(date: LocalDate): JournalEntry?

    /**
     * A check-in written by hand. The dose wizard's own check-in goes through
     * [DoseLogRepository.submit], which merges by the same rule — one merge, two paths.
     */
    suspend fun save(draft: CheckInDraft): JournalSaveResult
}
