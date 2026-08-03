package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.plus

/** «Доза растёт: 0,25 мг → 0,5 мг», and the day it happens. */
data class TitrationStep(
    val week: Int,
    val from: Dose,
    val to: Dose,
    val date: LocalDate,
)

/**
 * The next dose change after [today], or null if the protocol has none left.
 *
 * The prototype writes `TITRATION_STEPS` as a literal pair with literal dates,
 * which is correct for exactly one protocol starting on exactly one day. Both
 * the doses and the date are functions of the phases and `protocol.startDate`.
 *
 * The date is the first day of the band that is stepping up: week N begins
 * `(N - 1) × 7` days after the start.
 */
fun titrationStepAfter(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
    today: LocalDate,
): TitrationStep? {
    val week = cycleWeek(plan.protocol, today) ?: return null
    val phases = plan.phases[itemId]?.sortedBy { it.fromWeek } ?: return null

    return phases
        .zipWithNext()
        .firstOrNull { (_, next) -> next.fromWeek > week }
        ?.let { (from, to) ->
            TitrationStep(
                week = to.fromWeek,
                from = from.dose,
                to = to.dose,
                date = plan.protocol.startDate.plus(DatePeriod(days = (to.fromWeek - 1) * DAYS_IN_WEEK)),
            )
        }
}

private const val DAYS_IN_WEEK = 7
