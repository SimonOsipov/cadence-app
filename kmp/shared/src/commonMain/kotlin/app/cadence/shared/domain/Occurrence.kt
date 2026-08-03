package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.daysUntil

private const val DAYS_PER_WEEK = 7

/**
 * What the calendar shows for one item, on one date, at one time.
 *
 * Keyed by the time as well as the date because §03 gives an item a `times[]`:
 * BPC-157 at 08:00 and 20:00 is two occurrences on one day, and a patient who
 * logged the morning one has not logged the evening one.
 */
data class Occurrence(
    val itemId: ProtocolItemId,
    val kind: ProtocolItemKind,
    val date: LocalDate,
    val time: LocalTime,
    val dose: Dose?,
    val status: OccurrenceStatus,
)

/**
 * Computed by comparing generated occurrences against logged events, never
 * stored — §03's L10 and the project's «nothing derived is stored».
 */
enum class OccurrenceStatus { DONE, PENDING, MISSED, SCHEDULED }

/**
 * Which week of the cycle a date falls in, or null if it falls outside.
 *
 * Week 1 is the seven days from the protocol's start. The prototype hardcodes
 * «week 4» against a frozen 31 May; §03's second correction is that the answer
 * is this subtraction, so a protocol that began elsewhere gives a different
 * answer for the same date.
 */
fun cycleWeek(
    protocol: Protocol,
    date: LocalDate,
): Int? {
    val offset = protocol.startDate.daysUntil(date)
    if (offset < 0) return null
    val week = offset / DAYS_PER_WEEK + 1
    return if (week > protocol.weeks) null else week
}

/**
 * A protocol and everything needed to read it.
 *
 * One type rather than three parameters that always travel together: they come
 * from one fetch, they are meaningless apart, and detekt counting the arguments
 * was right about what that list was turning into.
 */
data class ProtocolPlan(
    val protocol: Protocol,
    val items: List<ProtocolItem>,
    val phases: Map<ProtocolItemId, List<ProtocolPhase>>,
)

/**
 * The schedule for one date, generated.
 *
 * §03: «There is no materialized schedule table. The calendar is generated on
 * demand from protocol_items × protocol_phases in the patient's timezone. Only
 * actual events — a logged dose, a measurement — are rows.» So this is the
 * whole schedule: nothing to keep in sync when a doctor edits a protocol, and
 * the Today strip and the Schedule screen render the same call, which is §03's
 * seventh correction.
 *
 * `today` is passed rather than read from a clock: the status of a date depends
 * on where it sits relative to now, and a function that fetched its own «now»
 * could not be tested against a calendar.
 */
fun occurrencesFor(
    plan: ProtocolPlan,
    events: List<DoseEvent>,
    date: LocalDate,
    today: LocalDate,
): List<Occurrence> {
    val week = cycleWeek(plan.protocol, date) ?: return emptyList()

    return plan.items
        .filter { it.fallsOn(date) }
        .flatMap { item ->
            val dose = plan.phases[item.id]?.firstOrNull { it.covers(week) }?.dose
            item.times.map { time ->
                Occurrence(
                    itemId = item.id,
                    kind = item.kind,
                    date = date,
                    time = time,
                    dose = dose,
                    status = statusOf(item, date, time, events, today),
                )
            }
        }
}

/** §03's `cadence` and `days_of_week[]`, read together. */
private fun ProtocolItem.fallsOn(date: LocalDate): Boolean =
    when (cadence) {
        ProtocolCadence.DAILY -> true
        ProtocolCadence.WEEKLY, ProtocolCadence.N_PER_WEEK -> date.dayOfWeek in daysOfWeek
    }

/**
 * A logged event wins; otherwise the date decides.
 *
 * The event has to match the date as well as the item — matching on the item
 * alone would mark every remaining Sunday done after the first injection.
 */
private fun statusOf(
    item: ProtocolItem,
    date: LocalDate,
    time: LocalTime,
    events: List<DoseEvent>,
    today: LocalDate,
): OccurrenceStatus {
    val logged =
        events.any { event ->
            event.protocolItemId == item.id &&
                event.scheduledForDate == date &&
                (event.scheduledForTime == null || event.scheduledForTime == time)
        }
    if (logged) return OccurrenceStatus.DONE

    return when {
        date < today -> OccurrenceStatus.MISSED
        date == today -> OccurrenceStatus.PENDING
        else -> OccurrenceStatus.SCHEDULED
    }
}
