package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/** «Доза растёт: 0,25 мг → 0,5 мг», and the day it happens. */
data class TitrationStep(
    val week: Int,
    val from: Dose,
    val to: Dose,
    val date: LocalDate,
)

/**
 * Every dose change this item makes, earliest first.
 *
 * The prototype writes `TITRATION_STEPS` as a pair of hand-written entries
 * whose dates are `addDays(CYCLE_START, 4 * 7)` and `8 * 7` — computed from one
 * frozen constant, which is correct for exactly one protocol starting on
 * exactly one day. Both the doses and the dates are functions of the phases and
 * `protocol.startDate`.
 *
 * One list rather than a second reading of `phases` beside `doseBands`: the
 * chart's dashed marks and the schedule screen's «следующий шаг» are the same
 * events asked about differently, and written twice they disagree the first
 * time a band moves — which is `phaseDose`'s argument, applied to the boundaries
 * instead of to the interiors.
 *
 * A boundary between two phases carrying the *same* dose is not a step. §03
 * lets a course split its table that way — «hold 0,5 for another four weeks» —
 * and «0,5 мг → 0,5 мг» is a dose change that did not happen. A cancelled
 * course has no steps at all, for `phaseDose`'s reason.
 *
 * Steps falling past the course's last prescribed day are dropped here rather
 * than at each reader: §03 joins `to_week` on the phase to `weeks` on the
 * protocol with nothing, so a phase reaching week 20 of a twelve-week course is
 * representable — and a step on such a date is one the chart draws over no
 * band, and the schedule screen promises outside the protocol.
 */
fun titrationSteps(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
): List<TitrationStep> {
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return emptyList()
    val phases = plan.phases[itemId]?.sortedBy { it.fromWeek } ?: return emptyList()

    return phases
        .zipWithNext()
        .filter { (from, to) -> from.dose != to.dose }
        .map { (from, to) ->
            TitrationStep(
                week = to.fromWeek,
                from = from.dose,
                to = to.dose,
                date = plan.protocol.weekStart(to.fromWeek),
            )
        }.filter { it.date <= plan.protocol.lastPrescribedDay }
}

/**
 * The next dose change after [today], or null if the protocol has none left.
 *
 * Bounded by the cycle rather than by the calendar: a date outside the
 * protocol's window has no «next step», the same way it has no cycle week.
 */
fun titrationStepAfter(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
    today: LocalDate,
): TitrationStep? {
    val week = cycleWeek(plan.protocol, today) ?: return null
    return titrationSteps(plan, itemId).firstOrNull { it.week > week }
}
