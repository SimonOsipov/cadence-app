package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.minus

/**
 * A *prescription*, not a history: no `DoseEvent` reaches either function in this file. What
 * the patient injected is the dots on the chart; what the doctor asked for is the strip
 * underneath — a band vanishing because a week was missed would tell a patient their
 * protocol changed when it hadn't. [range] is already clipped to the window it was asked about.
 */
data class DoseBand(
    val dose: Dose,
    val range: TrendRange,
)

/** §11 lists «protocol event overlays, dose spans» for the trend screens. */
enum class ProtocolMarkKind { START, TITRATION }

/**
 * [from] is null on [ProtocolMarkKind.START]: there's no dose to have come up from. The
 * prototype's third kind (`'add'`, day 49) isn't here — not because the day is underivable
 * ([protocolMarks] derives it for any item whose first phase opens after week 1), but because
 * the seed's BPC has a single phase across all twelve weeks, so its start *is* day 0.
 */
data class ProtocolMark(
    val kind: ProtocolMarkKind,
    val date: LocalDate,
    val from: Dose?,
    val to: Dose,
)

/**
 * Built from `plan.phases`, not `occurrencesFor`: a band is a phase — a first week, a last
 * week and a dose. Cancelled draws nothing, same reason as `phaseDose`. Phases are clipped
 * to the protocol's own last day as well as to [range]: §03 leaves `to_week` and `weeks`
 * unjoined, so a band reaching week 20 of a twelve-week course is representable.
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
 * Changed, not rose: §03 lets a course taper as readily as it titrates, and a mark that only
 * fired upwards would leave a reduction unexplained. The start sits on the first day of the
 * *first phase*: the protocol's own start date when dosed from day one, or the phase-opening
 * date when introduced mid-course (§03 leaves `from_week` unconstrained). Marks are clipped
 * to the course as well as [range], same reason [doseBands] clips its bands.
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
