package app.cadence.shared.domain

import kotlinx.datetime.LocalTime

/**
 * The half of «Воскресенье, утро» that isn't the weekday, computed here (not on the screen)
 * as a rule about time. Boundaries are the ones Russian actually uses, not even quarters:
 * «ночь» to five, «утро» to noon, «день» to six, «вечер» to midnight.
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
