package app.cadence.format

import app.cadence.shared.domain.PartOfDay
import kotlinx.datetime.DayOfWeek
import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil
import kotlinx.datetime.isoDayNumber

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

private val MONTHS_GENITIVE =
    listOf(
        "января",
        "февраля",
        "марта",
        "апреля",
        "мая",
        "июня",
        "июля",
        "августа",
        "сентября",
        "октября",
        "ноября",
        "декабря",
    )

/** «7 июня» — the genitive the date reads in, not the nominative «Июнь». */
fun dayAndMonth(date: LocalDate): String = "${date.day} ${MONTHS_GENITIVE[date.month.ordinal]}"

private val WEEKDAYS_SHORT =
    listOf("Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс")

/** The calendar's column headings, Monday first as the prototype draws them. */
fun weekdayHeadings(): List<String> = WEEKDAYS_SHORT

/** How many blanks precede the first of the month in a Monday-first grid. */
fun leadingBlanks(firstOfMonth: LocalDate): Int = firstOfMonth.dayOfWeek.isoDayNumber - 1

/** «сегодня» / «вчера» / «3 дн назад» / «2 нед назад» — the prototype's own scale. */
fun relativeDay(
    date: LocalDate,
    today: LocalDate,
): String {
    val days = date.daysUntil(today)

    return when {
        days <= 0 -> "сегодня"

        days == 1 -> "вчера"

        days < DAYS_PER_WEEK -> "$days дн назад"

        // Rounded, like the prototype's `Math.round(-day / 7)`: a patient
        // reading «2 нед назад» is placing a dose, not measuring it.
        else -> "${(days + DAYS_PER_WEEK / 2) / DAYS_PER_WEEK} нед назад"
    }
}

private const val DAYS_PER_WEEK = 7
