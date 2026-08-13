package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * §03: «tags[] (7 fixed)» — [SideEffect], not a second set. An earlier version declared
 * seven duplicate members alongside [SideEffect] with a test forbidding them to differ,
 * which is the type system saying there is one type; the alias keeps the column's name at
 * the call sites.
 */
typealias JournalTag = SideEffect

/** §03: `source manual|dose` — the dose wizard's check-in writes one of these. */
enum class JournalSource(
    val code: String,
) {
    MANUAL("manual"),
    DOSE("dose"),
}

/** §03's `journal_entries`, `UNIQUE(patient, date)` — one entry per day, written by upsert. */
data class JournalEntry(
    val patientId: UserId,
    val entryDate: LocalDate,
    val mood: Int?,
    val energy: Int?,
    val sleep: Int?,
    val tags: List<JournalTag>,
    val note: String?,
    val source: JournalSource,
)
