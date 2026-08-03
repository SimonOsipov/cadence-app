package app.cadence.format

import app.cadence.shared.domain.PartOfDay
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate

// Russian calendar names. Hand-written for the same reason the number
// formatting is: Kotlin/Native carries no ICU, so there is no locale API that
// answers on both platforms, and these are two closed lists.

private val WEEKDAYS_NOMINATIVE =
    mapOf(
        DayOfWeek.MONDAY to "Понедельник",
        DayOfWeek.TUESDAY to "Вторник",
        DayOfWeek.WEDNESDAY to "Среда",
        DayOfWeek.THURSDAY to "Четверг",
        DayOfWeek.FRIDAY to "Пятница",
        DayOfWeek.SATURDAY to "Суббота",
        DayOfWeek.SUNDAY to "Воскресенье",
    )

/** «Воскресенье». */
fun weekdayNominative(day: DayOfWeek): String = WEEKDAYS_NOMINATIVE.getValue(day)

/** «4-я неделя» — the ordinal suffix Russian uses for a feminine noun. */
fun cycleWeekLabel(week: Int): String = "$week-я неделя"

/**
 * «Воскресенье, утро · 4-я неделя».
 *
 * The prototype writes this whole line as a literal. Every part of it is a
 * function of the clock and the protocol, and assembling it here keeps the
 * screen free of both.
 */
fun greeting(
    date: LocalDate,
    partOfDay: PartOfDay,
    cycleWeek: Int?,
): String {
    val head = "${weekdayNominative(date.dayOfWeek)}, ${partOfDay.ru}"
    return if (cycleWeek == null) head else "$head · ${cycleWeekLabel(cycleWeek)}"
}
