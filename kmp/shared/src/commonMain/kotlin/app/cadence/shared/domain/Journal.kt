package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * §03: «tags[] (7 fixed)» — and they are [SideEffect], not a second set.
 *
 * The prototype's own comment reads «tags — side-effect ids», and §03 requires
 * «one action, two facts»: the dose wizard's check-in writes a dose event and a
 * journal entry at once. A first version invented `MOOD`, `ENERGY` and `SLEEP`,
 * which duplicated three columns already on the row and left the side effects
 * nowhere to go; a second declared seven identical members alongside
 * [SideEffect] with a test forbidding them to differ — which is the type system
 * saying there is one type. §03 has two columns; the client has one vocabulary,
 * and the alias keeps the column's name at the call sites.
 */
typealias JournalTag = SideEffect

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
