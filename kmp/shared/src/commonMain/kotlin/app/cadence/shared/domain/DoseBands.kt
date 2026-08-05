package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.minus

/**
 * One dose, held for a stretch of days.
 *
 * The band is a *prescription*, not a history: no `DoseEvent` reaches either
 * function in this file, and that is the point. What the patient actually
 * injected is the dots on the chart; what the doctor asked for is the strip
 * underneath, and a band that vanished because a week was missed would tell a
 * patient their protocol had changed when it had not.
 *
 * [range] is already clipped to the window it was asked about, so a screen can
 * lay it out against the same two dates it drew the axis from.
 */
data class DoseBand(
    val dose: Dose,
    val range: TrendRange,
)

/** §11 lists «protocol event overlays, dose spans» for the trend screens. */
enum class ProtocolMarkKind { START, TITRATION }

/**
 * A dashed line on the chart and the words beside it.
 *
 * [from] is null on a [ProtocolMarkKind.START]: there is no dose to have come
 * up from, and «0,25 мг → 0,25 мг» on day one is a change that did not happen.
 *
 * The prototype has a third kind — `{ kind: 'add', '+ BPC-157' }` on day 49 of
 * `mobile/src/features/trends/data.ts` — and it is not here. Not because the
 * day is underivable: [protocolMarks] derives exactly that day for any item
 * whose first phase opens after week 1. It is absent because the seed's BPC has
 * a single phase across all twelve weeks, so its start *is* day 0, and the
 * prototype's day 49 is a literal about a course this data does not describe.
 * An item that really began mid-course gets a [START] mark on the day it began.
 */
data class ProtocolMark(
    val kind: ProtocolMarkKind,
    val date: LocalDate,
    val from: Dose?,
    val to: Dose,
)

/**
 * The dose bands for one item, clipped to [range].
 *
 * Built from `plan.phases` rather than from `occurrencesFor`: that function
 * answers about one day and returns nothing outside the protocol window, while
 * a band is a phase — a first week, a last week and a dose — and
 * `Protocol.weekStart` already turns a week into a date.
 *
 * A cancelled course draws nothing, which is `phaseDose`'s answer for the same
 * reason: §03 gives `protocols` no `cancelled_at` to bound a withdrawn
 * prescription with, so there is no defensible span to draw.
 *
 * Phases are clipped to the protocol's own last day as well as to [range]. §03
 * stores `to_week` on the phase and `weeks` on the protocol with nothing
 * joining them, so a band reaching week 20 of a twelve-week course is
 * representable — and would put a prescription under dates `cycleWeek` calls
 * outside the protocol.
 */
fun doseBands(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
    range: TrendRange,
): List<DoseBand> {
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return emptyList()
    val phases = plan.phases[itemId] ?: return emptyList()

    return phases
        .sortedBy { it.fromWeek }
        .mapNotNull { phase ->
            val opens = plan.protocol.weekStart(phase.fromWeek)
            // The last day of `toWeek` is the day before `toWeek + 1` opens.
            val closes = minOf(plan.protocol.weekStart(phase.toWeek + 1).minusDay(), plan.protocol.lastPrescribedDay)
            val from = maxOf(opens, range.from)
            val through = minOf(closes, range.through)
            if (from > through) null else DoseBand(phase.dose, TrendRange(from, through))
        }
}

/**
 * Where this item's prescription began and every day its dose changed, inside
 * [range].
 *
 * Changed, not rose: §03 lets a course taper as readily as it titrates, and a
 * mark that only fired upwards would leave a reduction unexplained on the
 * chart.
 *
 * The start sits on the first day of the *first phase*, which is the protocol's
 * own start date whenever the item is dosed from day one — every dosed item in
 * the seed. It is not the same day when the first phase opens later: §03
 * constrains `from_week` to nothing, so a first phase opening in week 2 is
 * representable, and this codebase reads that shape as an item introduced after
 * the course began. Marking such an item on a day it was not yet prescribed
 * would put «Старт · 0,25 мг» a week before the first injection; dropping the
 * mark instead would leave a band on the chart with nothing saying what opened
 * it.
 *
 * Marks are clipped to the course as well as to [range], for the reason
 * [doseBands] clips its bands: a dashed «доза выросла» line on a date
 * `cycleWeek` calls outside the protocol would stand over no band at all.
 */
fun protocolMarks(
    plan: ProtocolPlan,
    itemId: ProtocolItemId,
    range: TrendRange,
): List<ProtocolMark> {
    if (plan.protocol.status == ProtocolStatus.CANCELLED) return emptyList()
    val phases = plan.phases[itemId]?.sortedBy { it.fromWeek } ?: return emptyList()

    val start =
        phases.firstOrNull()?.let {
            ProtocolMark(
                kind = ProtocolMarkKind.START,
                date = plan.protocol.weekStart(it.fromWeek),
                from = null,
                to = it.dose,
            )
        }

    val titrations =
        titrationSteps(plan, itemId).map {
            ProtocolMark(ProtocolMarkKind.TITRATION, it.date, from = it.from, to = it.to)
        }

    return (listOfNotNull(start) + titrations)
        .filter { it.date <= plan.protocol.lastPrescribedDay && it.date in range }
}

private fun LocalDate.minusDay(): LocalDate = minus(DatePeriod(days = 1))
