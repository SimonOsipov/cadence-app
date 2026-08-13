package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime
import kotlinx.datetime.daysUntil

private const val DAYS_PER_WEEK = 7

/**
 * Keyed by time as well as date: §03's `times[]` makes BPC-157 at 08:00 and 20:00 two
 * occurrences, and logging one doesn't log the other.
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
 * Computed by comparing generated occurrences against logged events, never stored — §03's
 * L10 and «nothing derived is stored».
 */
enum class OccurrenceStatus { DONE, PENDING, MISSED, SCHEDULED }

/**
 * Week 1 is the seven days from the protocol's start; §03's second correction replaces the
 * prototype's hardcoded «week 4» against a frozen 31 May with this subtraction.
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

/** One type, not three parameters that always travel together: they come from one fetch and are meaningless apart. */
data class ProtocolPlan(
    val protocol: Protocol,
    val items: List<ProtocolItem>,
    val phases: Map<ProtocolItemId, List<ProtocolPhase>>,
)

/**
 * §03: no materialized schedule table; generated on demand from protocol_items ×
 * protocol_phases. This is the whole schedule — Today and Schedule render the same call
 * (§03's seventh correction). `today` is a parameter, not read from a clock, so this can be
 * tested against a calendar.
 */
fun occurrencesFor(
    plan: ProtocolPlan,
    events: List<DoseEvent>,
    date: LocalDate,
    today: LocalDate,
): List<Occurrence> {
    // CANCELLED only, and the narrowing matters: a COMPLETED course is every patient after
    // twelve weeks, and blanking it would erase the dots from days they actually injected.
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return emptyList()

    if (cycleWeek(plan.protocol, date) == null) return emptyList()

    return plan.items
        .filter { it.fallsOn(date) }
        .flatMap { item ->
            val dose = phaseDose(plan, item.id, date)
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

/**
 * Null has three causes, one answer: cancelled, outside the window, or no phase covers
 * that week — offering a number in any case is the defect the step-4 review found on the
 * hero. One function, not two readings of `phases`: written twice, wizard and calendar
 * disagree the first time a band moves.
 */
fun phaseDose(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
    date: LocalDate,
): Dose? {
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return null
    val week = cycleWeek(plan.protocol, date) ?: return null
    return plan.phases[itemId]?.firstOrNull { it.covers(week) }?.dose
}

/** Derived, not seeded: a stored copy goes stale the first time a protocol is edited. */
fun ProtocolItem.dosesPerWeek(): Double = scheduledDaysPerWeek() * times.size

/** Shared with [fallsOn], so the two readings of `cadence` cannot drift apart. */
private fun ProtocolItem.scheduledDaysPerWeek(): Double =
    when (cadence) {
        ProtocolCadence.DAILY -> DAYS_PER_WEEK.toDouble()
        ProtocolCadence.WEEKLY, ProtocolCadence.N_PER_WEEK -> daysOfWeek.size.toDouble()
    }

/** §03's `cadence` and `days_of_week[]`, read together. */
private fun ProtocolItem.fallsOn(date: LocalDate): Boolean =
    when (cadence) {
        ProtocolCadence.DAILY -> true
        ProtocolCadence.WEEKLY, ProtocolCadence.N_PER_WEEK -> date.dayOfWeek in daysOfWeek
    }

/**
 * Match must include item, date **and slot**: matching on item alone would mark every
 * remaining Sunday done after the first injection; without the slot, one BPC-157 event with
 * a null time closed both 08:00 and 20:00, telling a patient the evening dose was already done.
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
                event.scheduledForTime == time
        }
    if (logged) return OccurrenceStatus.DONE

    return when {
        date < today -> OccurrenceStatus.MISSED
        date == today -> OccurrenceStatus.PENDING
        else -> OccurrenceStatus.SCHEDULED
    }
}
