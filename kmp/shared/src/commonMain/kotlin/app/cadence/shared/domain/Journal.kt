package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/** §03: «tags[] (7 fixed)». A closed set, so a client cannot invent one. */
enum class JournalTag(
    val code: String,
) {
    ENERGY("energy"),
    APPETITE("appetite"),
    SLEEP("sleep"),
    MOOD("mood"),
    DIGESTION("digestion"),
    TRAINING("training"),
    STRESS("stress"),
}

/** §03: `source manual|dose` — the dose wizard's check-in writes one of these. */
enum class JournalSource(
    val code: String,
) {
    MANUAL("manual"),
    DOSE("dose"),
}

/**
 * §03's `journal_entries`, `UNIQUE(patient, date)` — one entry per day, written
 * by upsert. The heatmap and the mood series are queries over these, not
 * stored aggregates.
 */
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
