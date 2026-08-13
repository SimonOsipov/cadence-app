package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime

/**
 * §03's seventh correction: the prototype hardcodes this strip as a literal array with the
 * dose spelled «0,25 мг» for all twelve weeks. `todayStatus` is null when the item isn't due
 * today at all — a claim about nothing. When due more than once (BPC-157's two daily times),
 * it's the *least complete* of the day's slots, not the first — see [leastComplete].
 */
data class ProtocolRow(
    val itemId: ProtocolItemId,
    val kind: ProtocolItemKind,
    /**
     * Resolved here, not left as an id: §11's Today row needs the name and isn't allowed a
     * repository to look it up in.
     */
    val compound: Compound?,
    val dose: Dose?,
    val times: List<LocalTime>,
    val cadence: ProtocolCadence,
    val todayStatus: OccurrenceStatus?,
    /**
     * §03's `protocol_items.loggable`: the strip draws every item, but the dose wizard
     * offers only loggable ones — a weigh-in isn't a dose.
     */
    val loggable: Boolean,
)

/**
 * Built from `plan.items`, not the day's occurrences: the strip is the week's prescription,
 * so the weekly injection belongs on it every day, not just Sundays. Today's state comes
 * from [occurrencesFor] — the same call Schedule makes, so the two can't disagree by construction.
 */
fun weekProtocolRows(
    plan: ProtocolPlan,
    compounds: List<Compound>,
    events: List<DoseEvent>,
    today: LocalDate,
): List<ProtocolRow> {
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return emptyList()
    if (cycleWeek(plan.protocol, today) == null) return emptyList()
    val todays = occurrencesFor(plan, events, today, today)

    return plan.items.map { item ->
        ProtocolRow(
            itemId = item.id,
            kind = item.kind,
            compound = compounds.firstOrNull { it.id == item.compoundId },
            dose = phaseDose(plan, item.id, today),
            times = item.times,
            cadence = item.cadence,
            // A logged dose greys the row for the same reason it greys the calendar cell.
            todayStatus = leastComplete(todays.filter { it.itemId == item.id }),
            loggable = item.loggable,
        )
    }
}

/**
 * `firstOrNull` was here once, and collapsed a multi-slot item to whichever slot came first:
 * logging BPC-157's 08:00 dose read «Сегодня · записано» while the 20:00 injection was still
 * due — the defect [statusOf] prevents, reintroduced one layer up. DONE only when every slot
 * is done, else whatever the first non-done slot says.
 */
private fun leastComplete(occurrences: List<Occurrence>): OccurrenceStatus? =
    when {
        occurrences.isEmpty() -> null
        occurrences.all { it.status == OccurrenceStatus.DONE } -> OccurrenceStatus.DONE
        else -> occurrences.first { it.status != OccurrenceStatus.DONE }.status
    }
