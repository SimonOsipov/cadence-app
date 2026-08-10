package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.LocalTime

/**
 * One line of «Протокол этой недели».
 *
 * The prototype writes these as a literal array of three — `protocolRows` in
 * `TodayScreen.tsx` — with the dose spelled «0,25 мг» for all twelve weeks and
 * the state hand-wired to a boolean. That array is the strip §03 names in its
 * seventh correction: «Today-screen protocol strip hardcoded, diverges from
 * schedule data · both render the same generated occurrences endpoint».
 *
 * `todayStatus` is null when the item is not due today at all — a Wednesday has
 * no weekly injection, and «сегодня · ждёт» on a day with no occurrence would
 * be a claim about nothing.
 *
 * When the item is due more than once — BPC-157 is `times = [08:00, 20:00]` —
 * `todayStatus` is the *least complete* of the day's slots, not the first. See
 * [leastComplete] for why the first is a wrong answer with a clinical cost.
 */
data class ProtocolRow(
    val itemId: ProtocolItemId,
    val kind: ProtocolItemKind,
    /**
     * Resolved here rather than left as an id: §11's Today row reads
     * «Семаглутид · 0,25 мг», and a screen that had to look the name up would
     * need a repository it is not allowed to have.
     */
    val compound: Compound?,
    val dose: Dose?,
    val times: List<LocalTime>,
    val cadence: ProtocolCadence,
    val todayStatus: OccurrenceStatus?,
    /**
     * §03's `protocol_items.loggable`. The strip draws every item; the dose
     * wizard may only offer the ones a patient can record — a weigh-in is on
     * the protocol and is not a dose.
     */
    val loggable: Boolean,
)

/**
 * The week's protocol, from the same source the calendar draws.
 *
 * Built from `plan.items` rather than from the day's occurrences: the strip is
 * the week's prescription, so the weekly injection belongs on it every day, not
 * only on Sundays. Today's state comes from [occurrencesFor] — the same call
 * the Schedule screen makes — so the two cannot disagree by construction, which
 * is the whole point of putting both screens in one step.
 */
fun weekProtocolRows(
    plan: ProtocolPlan,
    compounds: List<Compound>,
    events: List<DoseEvent>,
    today: LocalDate,
): List<ProtocolRow> {
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return emptyList()
    // A date outside the protocol's window has no week and so no strip; the
    // dose each row carries is [phaseDose]'s answer for the same day.
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
            // The day's own occurrences decide, so a logged dose greys the row
            // for the same reason it greys the calendar cell.
            todayStatus = leastComplete(todays.filter { it.itemId == item.id }),
            loggable = item.loggable,
        )
    }
}

/**
 * The status of a whole day's slots for one item.
 *
 * `firstOrNull` was here, and it collapsed a multi-slot item to whichever slot
 * came first: log BPC-157's 08:00 dose and the row read «Сегодня · записано»
 * while the 20:00 injection was still due. That is the defect [statusOf] was
 * written to prevent — its own KDoc describes it — reintroduced one layer up,
 * and the strip is the surface a patient checks to decide whether they have
 * finished for the day.
 *
 * So: DONE only when every slot is done, and otherwise whatever the first slot
 * that is *not* done actually says. No arm invents a status the occurrences did
 * not already carry — `weekProtocolRows` asks about today and only today, so
 * DONE and PENDING are the only two reachable here, and a `MISSED` branch would
 * be a guess about a call that cannot happen.
 */
private fun leastComplete(occurrences: List<Occurrence>): OccurrenceStatus? =
    when {
        occurrences.isEmpty() -> null
        occurrences.all { it.status == OccurrenceStatus.DONE } -> OccurrenceStatus.DONE
        else -> occurrences.first { it.status != OccurrenceStatus.DONE }.status
    }
