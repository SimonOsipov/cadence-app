package app.cadence.shared.domain

import kotlinx.datetime.LocalTime

/**
 * The half of «Воскресенье, утро» that is not the weekday.
 *
 * The prototype writes the whole greeting as a literal, frozen at a Sunday
 * morning. It is a function of the clock, and it lives here rather than on the
 * screen because it is a rule about time, not about layout — and because this
 * is the half of the codebase the gate runs.
 *
 * Boundaries are the ones Russian actually uses rather than even quarters:
 * «ночь» runs to five, «утро» to noon, «день» to six, «вечер» to midnight.
 */
enum class PartOfDay(
    val ru: String,
) {
    NIGHT("ночь"),
    MORNING("утро"),
    AFTERNOON("день"),
    EVENING("вечер"),
}

private const val MORNING_FROM = 5
private const val AFTERNOON_FROM = 12
private const val EVENING_FROM = 18

fun partOfDay(time: LocalTime): PartOfDay =
    when (time.hour) {
        in MORNING_FROM until AFTERNOON_FROM -> PartOfDay.MORNING
        in AFTERNOON_FROM until EVENING_FROM -> PartOfDay.AFTERNOON
        in EVENING_FROM..LAST_HOUR -> PartOfDay.EVENING
        else -> PartOfDay.NIGHT
    }

private const val LAST_HOUR = 23
