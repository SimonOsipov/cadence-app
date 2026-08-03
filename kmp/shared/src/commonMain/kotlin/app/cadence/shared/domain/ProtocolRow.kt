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
 */
data class ProtocolRow(
    val itemId: ProtocolItemId,
    val kind: ProtocolItemKind,
    val compoundId: CompoundId?,
    val dose: Dose?,
    val times: List<LocalTime>,
    val cadence: ProtocolCadence,
    val todayStatus: OccurrenceStatus?,
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
    events: List<DoseEvent>,
    today: LocalDate,
): List<ProtocolRow> {
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return emptyList()
    val week = cycleWeek(plan.protocol, today) ?: return emptyList()
    val todays = occurrencesFor(plan, events, today, today)

    return plan.items.map { item ->
        ProtocolRow(
            itemId = item.id,
            kind = item.kind,
            compoundId = item.compoundId,
            dose = plan.phases[item.id]?.firstOrNull { it.covers(week) }?.dose,
            times = item.times,
            cadence = item.cadence,
            // The day's own occurrences decide, so a logged dose greys the row
            // for the same reason it greys the calendar cell.
            todayStatus = todays.firstOrNull { it.itemId == item.id }?.status,
        )
    }
}
