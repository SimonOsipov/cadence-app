package app.cadence.shared.repository

import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.VialId
import kotlinx.datetime.LocalDate

/**
 * A sealed answer, not a nullable id: «day rolled over», «already logged» and «missing a
 * zone» are three different things for a screen to say, and a single `null` makes them one.
 */
sealed interface DoseLogResult {
    /**
     * §03's "one action, two facts" — the journal entry has no id of its own; §03 keys it
     * `UNIQUE(patient, date)`, so the date *is* the identity.
     */
    data class Written(
        val eventId: DoseEventId,
        val journalDate: LocalDate,
        /** Read back off the event, not the draft — the only way anything currently observes `DoseEvent.dose`. */
        val dose: Dose,
        /**
         * Null when the patient has none of that compound. Observable on purpose: §03's
         * third correction makes the vial what a remaining count is subtracted from.
         */
        val vialId: VialId?,
    ) : DoseLogResult

    /** The draft cannot make a dose event — see `DoseDraft.canSubmit`. */
    data object Incomplete : DoseLogResult

    /** The item has no occurrence today; the day rolled over under an open app. */
    data object NotScheduledToday : DoseLogResult

    /**
     * "Every", not "the one": §03 gives BPC-157 two daily times, so a morning dose leaves
     * the evening one still to record.
     */
    data object AlreadyLogged : DoseLogResult
}

/**
 * §11: `POST /me/dose-events`. One call: §03's mood/sides check-in also writes the journal
 * entry — "one action, two facts" — so a screen making two calls could half-fail.
 * The occurrence this settles and the vial drawn from are resolved here, against the same
 * schedule the calendar reads — see docs/prototype-divergences.md for the missing vial picker.
 */
interface DoseLogRepository {
    suspend fun submit(draft: DoseDraft): DoseLogResult
}
