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

/**
 * What a check-in offers the day — everything optional, because step 4 of the wizard and the
 * diary sheet are both «всё по желанию». An unnamed field means «skipped», never «erase what
 * the morning said»; the merge is what turns that into a stored entry.
 *
 * It carries no source: which of the two a write is comes from the path taken, not from the
 * draft, so a caller cannot label its own write.
 */
data class CheckInDraft(
    val entryDate: LocalDate,
    val mood: Int? = null,
    val energy: Int? = null,
    val sleep: Int? = null,
    val tags: List<JournalTag> = emptyList(),
    val note: String? = null,
) {
    /** Nothing named at all. A written-empty entry would put an unfilled day into the feed and the heatmap. */
    val saysNothing: Boolean
        get() = mood == null && energy == null && sleep == null && tags.isEmpty() && note.isNullOrBlank()

    /**
     * Every named reading is on the 1..5 scale. Off it, the value is refused rather than
     * stored: [MoodLevel.of] answers `null` outside the range, so a stored 7 reads back as
     * «no answer» and the loss is silent.
     */
    val readingsAreOnTheScale: Boolean
        get() = listOfNotNull(mood, energy, sleep).all { it in SCALE }

    private companion object {
        val SCALE = 1..5
    }
}

/**
 * §03's `mood smallint 1..5` with the word the patient reads for it.
 *
 * The prototype carries two scales for this one number — «Никак / Слабо / …» in the dose
 * wizard (`log-dose/components.tsx:513`) and «Тяжело / Так себе / …» in the journal
 * (`journal/data.ts:46-53`). The journal's wording wins: it is where the number is read
 * back, in the feed and in the heatmap legend, and a scale that renames itself between the
 * screen that writes and the screen that reads is not a scale. The colour each level is
 * drawn in belongs to the design system, not here — this module has no Compose.
 */
enum class MoodLevel(
    val value: Int,
    val labelRu: String,
) {
    HARD(1, "Тяжело"),
    POOR(2, "Так себе"),
    EVEN(3, "Ровно"),
    GOOD(4, "Хорошо"),
    BRIGHT(5, "Светло"),
    ;

    companion object {
        /** `null` for anything outside 1..5 — a row this app did not write says nothing. */
        fun of(value: Int?): MoodLevel? = entries.firstOrNull { it.value == value }
    }
}
