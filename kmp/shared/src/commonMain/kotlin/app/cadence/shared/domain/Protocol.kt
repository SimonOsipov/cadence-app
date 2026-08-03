package app.cadence.shared.domain

import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime

// protocol & scheduling — «the prescription: what, when, how much, titrating to
// where» (§03).

/** §03's `compounds`. */
data class Compound(
    val id: CompoundId,
    val code: String,
    val nameRu: String,
    val defaultUnit: DoseUnit,
    val route: String,
    // §03 carries an `icon` on the compound, and it is data rather than
    // presentation: which glyph a compound gets is the clinic's choice and
    // arrives from the server. `CadenceIcons.byName` is the adapter that turns
    // this kebab string into path data, which is the reason that map exists.
    val icon: String,
)

enum class ProtocolStatus { ACTIVE, COMPLETED, CANCELLED }

/**
 * §03's `protocols`.
 *
 * `startDate` is what the cycle week derives from. The prototype hardcodes
 * «week 4» against a frozen calendar; §03's second correction is that the
 * answer is a subtraction from this field.
 */
data class Protocol(
    val id: ProtocolId,
    val patientId: UserId,
    val startDate: LocalDate,
    val weeks: Int,
    val status: ProtocolStatus,
    val createdBy: UserId?,
    val notes: String?,
)

/** §03: `kind injection|supplement|weigh_in`. */
enum class ProtocolItemKind { INJECTION, SUPPLEMENT, WEIGH_IN }

/** §03: `cadence weekly|daily|n_per_week`. */
enum class ProtocolCadence { WEEKLY, DAILY, N_PER_WEEK }

/**
 * §03's `protocol_items`.
 *
 * `times` is a list because §03 makes it one: BPC-157 is 08:00 and 20:00, which
 * is why an occurrence is keyed by (item, date, time) and not by (item, date).
 */
data class ProtocolItem(
    val id: ProtocolItemId,
    val protocolId: ProtocolId,
    val kind: ProtocolItemKind,
    val compoundId: CompoundId?,
    val cadence: ProtocolCadence,
    val daysOfWeek: List<DayOfWeek>,
    val times: List<LocalTime>,
    val loggable: Boolean,
)

/**
 * §03's `protocol_phases` — the titration band: 0,25 (w1–4) → 0,5 (w5–8) → 1,0
 * (w9–12).
 */
data class ProtocolPhase(
    val fromWeek: Int,
    val toWeek: Int,
    val dose: Dose,
) {
    fun covers(week: Int): Boolean = week in fromWeek..toWeek
}
