package app.cadence.shared.repository

import app.cadence.shared.domain.JournalEntry
import kotlinx.datetime.LocalDate

/**
 * §03's `journal_entries`, read side. Narrow on purpose: the diary screen and its heatmap
 * arrive in step 7 of the block — this exists now only so the second fact of a check-in is
 * observable, since a write nothing can read back is a write nobody can show works.
 */
interface JournalRepository {
    /** The one entry for that day, or null if the patient has written nothing. */
    suspend fun entry(date: LocalDate): JournalEntry?
}
