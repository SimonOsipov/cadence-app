package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * §03: «tags[] (7 fixed)». A closed set, so a client cannot invent one.
 *
 * The seven are the prototype's (`TAGS` in
 * `mobile/src/features/journal/data.ts`, whose own comment reads «tags —
 * side-effect ids»), and they are deliberately the same seven as [SideEffect].
 * §03 requires it: «The wizard's mood/sides check-in also writes today's
 * journal_entry with source dose — one action, two facts». A first version of
 * this enum invented `MOOD`, `ENERGY` and `SLEEP`, which duplicated the three
 * 1–5 columns on the row and left the wizard's side effects with nowhere to go.
 */
enum class JournalTag(
    val code: String,
) {
    NAUSEA("nausea"),
    FATIGUE("fatigue"),
    HEADACHE("headache"),
    BLOATING("bloating"),
    INSOMNIA("insomnia"),
    SITE("site"),
    APPETITE("appetite"),
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
