package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.plus

/** §03's `compounds`. */
data class Compound(
    val id: CompoundId,
    val code: String,
    val nameRu: String,
    val defaultUnit: DoseUnit,
    val route: String,
    // Data, not presentation: which glyph a compound gets is the clinic's choice.
    // `CadenceIcons.byName` adapts this kebab string into path data.
    val icon: String,
)

enum class ProtocolStatus { ACTIVE, COMPLETED, CANCELLED }

/**
 * §03's `protocols`. `startDate` is what cycle week derives from — §03's second correction
 * over the prototype's hardcoded «week 4».
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
 * §03's `protocol_items`. `times` is a list because §03 makes it one: BPC-157 is 08:00 and
 * 20:00, why an occurrence is keyed (item, date, time), not (item, date).
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

private const val DAYS_IN_WEEK = 7

/**
 * The one place this arithmetic lives: written out three times before (titration dates,
 * dose bands, «весь цикл» window), three copies of «(N − 1) × 7» were three chances to disagree.
 */
internal fun Protocol.weekStart(week: Int): LocalDate = startDate.plus(DatePeriod(days = (week - 1) * DAYS_IN_WEEK))

/** The last day the protocol prescribes anything — day `weeks × 7 − 1`. */
internal val Protocol.lastPrescribedDay: LocalDate
    get() = startDate.plus(DatePeriod(days = weeks * DAYS_IN_WEEK - 1))
