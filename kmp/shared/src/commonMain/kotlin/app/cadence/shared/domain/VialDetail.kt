package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil

/** One line of «Последние записи» — a dose, as this vial's sheet lists it. */
data class VialDose(
    val date: LocalDate,
    val dose: Dose,
    val site: InjectionSite?,
)

/**
 * Everything the detail sheet shows about one vial.
 *
 * The row is the same one the cabinet's card drew, not a second resolution: a
 * sheet that worked out its own compound and remaining count could disagree
 * with the card the patient tapped to open it.
 *
 * The prototype seeds `recent` and `usage` per vial, so its lists cannot be
 * wrong and cannot move. Here both are filters over the one fact stream, which
 * is what makes «what came out of this vial» a question with an answer rather
 * than a decoration.
 */
data class VialDetail(
    val row: VialRow,
    /** Newest first — «Последние записи» opens with the last one. */
    val recent: List<VialDose>,
    /** Doses per week since the vial was opened, oldest week first. */
    val dosesPerWeek: List<Int>,
)

private const val DAYS_PER_WEEK = 7

fun vialDetail(
    plan: ProtocolPlan,
    vial: Vial,
    events: List<DoseEvent>,
    today: LocalDate,
    compounds: List<Compound>,
): VialDetail {
    val drawn = events.filter { it.vialId == vial.id }

    return VialDetail(
        row = vialRow(plan, vial, events, today, compounds),
        recent =
            drawn
                .sortedByDescending { it.injectedAt }
                .map { VialDose(date = it.scheduledForDate, dose = it.dose, site = it.site) },
        dosesPerWeek = weeklyUsage(vial, drawn, today),
    )
}

/**
 * How many doses came out of the vial in each week it has been open.
 *
 * Counted from the day it was opened rather than from the calendar's weeks: the
 * question the chart answers is «как я его расходую», and a vial opened on a
 * Wednesday has its own weeks. A vial that was never opened has none.
 */
private fun weeklyUsage(
    vial: Vial,
    drawn: List<DoseEvent>,
    today: LocalDate,
): List<Int> {
    val opened = vial.openedAt ?: return emptyList()
    if (drawn.isEmpty()) return emptyList()

    val lastDay = vial.disposedAt ?: today
    val weeks = opened.daysUntil(lastDay) / DAYS_PER_WEEK + 1

    return (0 until weeks).map { week ->
        drawn.count { opened.daysUntil(it.scheduledForDate) / DAYS_PER_WEEK == week }
    }
}
