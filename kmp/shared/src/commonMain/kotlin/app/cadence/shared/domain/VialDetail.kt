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
 * Row is the same one the cabinet's card drew, not a second resolution: a sheet working out
 * its own compound and remaining count could disagree with the card the patient tapped.
 * Prototype seeds `recent`/`usage` per vial; here both are filters over the one fact stream.
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
 * Counted from the day the vial was opened, not the calendar's weeks: a vial opened on a
 * Wednesday has its own weeks. Never opened means none.
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
