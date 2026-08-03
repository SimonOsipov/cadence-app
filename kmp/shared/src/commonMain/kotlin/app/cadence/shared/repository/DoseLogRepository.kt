package app.cadence.shared.repository

import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseDraft
import app.cadence.shared.domain.DoseEventId
import kotlinx.datetime.LocalDate

/**
 * What one check-in did.
 *
 * A sealed answer rather than a nullable id: «the day rolled over», «this
 * occurrence is already logged» and «the draft is missing a zone» are three
 * different things for a screen to say, and a single `null` makes them one.
 */
sealed interface DoseLogResult {
    /**
     * §03's «one action, two facts» — the ids of both.
     *
     * The journal entry has no id of its own: §03 keys it `UNIQUE(patient,
     * date)`, so the date *is* the identity.
     */
    data class Written(
        val eventId: DoseEventId,
        val journalDate: LocalDate,
        /**
         * What was recorded, read back off the event rather than off the draft.
         *
         * The prototype's warm confirmation sheet names the dose it just saved,
         * and a confirmation that echoed the input rather than the record would
         * confirm nothing. It is also the only way anything can currently
         * observe `DoseEvent.dose`: no screen reads the event stream yet.
         */
        val dose: Dose,
    ) : DoseLogResult

    /** The draft cannot make a dose event — see `DoseDraft.canSubmit`. */
    data object Incomplete : DoseLogResult

    /** The item has no occurrence today; the day rolled over under an open app. */
    data object NotScheduledToday : DoseLogResult

    /**
     * Every slot this item has today is already logged.
     *
     * «Every», not «the one»: §03 gives BPC-157 `times = [08:00, 20:00]`, so a
     * patient who took the morning dose has an evening one still to record.
     */
    data object AlreadyLogged : DoseLogResult
}

/**
 * §11: `POST /me/dose-events`.
 *
 * One call, because §03 says one action writes two facts: «The wizard's
 * mood/sides check-in also writes today's journal_entry with source dose — one
 * action, two facts, matching the prototype's journal seed data.» A screen
 * making two calls in sequence can half-fail, and a patient whose dose is
 * recorded without the side effect they reported is worse than one whose
 * check-in failed outright.
 *
 * The draft carries the patient's choices. The occurrence this settles and the
 * vial it came out of are resolved here, against the same schedule the calendar
 * reads — see `docs/prototype-divergences.md` for what the missing vial picker
 * costs.
 */
interface DoseLogRepository {
    suspend fun submit(draft: DoseDraft): DoseLogResult
}
