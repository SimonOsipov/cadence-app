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
 * The prototype hardcodes `TITRATION_STEPS` from one frozen constant; here doses and dates
 * are functions of the phases and `protocol.startDate`. Written once, not read twice beside
 * `doseBands`, so the chart's marks and the schedule's «следующий шаг» can't disagree. A
 * boundary between two phases with the *same* dose isn't a step (§03 allows holding a dose).
 * Steps past the last prescribed day are dropped here, same reason as [doseBands]'s clipping.
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
 * Bounded by the cycle, not the calendar: a date outside the protocol's window has no
 * «next step», the same way it has no cycle week.
 */
fun titrationStepAfter(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
    today: LocalDate,
): TitrationStep? {
    val week = cycleWeek(plan.protocol, today) ?: return null
    return titrationSteps(plan, itemId).firstOrNull { it.week > week }
}
