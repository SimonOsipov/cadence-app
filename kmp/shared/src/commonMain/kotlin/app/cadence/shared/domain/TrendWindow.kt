package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil
import kotlinx.datetime.minus

/**
 * Three are lengths; `CYCLE` is bounded by the protocol (`startDate` to last prescribed day),
 * so the same calendar day gives different windows for different courses. The prototype
 * hides this — it gives «3 месяца» and «цикл» the *same* 84-day span, so switching between
 * them changes nothing; porting that would draw week two and week eleven identically.
 */
enum class TrendWindow {
    WEEK,
    FOUR_WEEKS,
    THREE_MONTHS,
    CYCLE,
}

/** Both edges included, because a window is a set of days and not a subtraction. */
private const val TODAY_COUNTS_AS_ONE = 1

private const val WEEK_DAYS = 7
private const val FOUR_WEEKS_DAYS = 28

/**
 * Twelve weeks, counted inclusively. The prototype's axis runs day 0 to day 84 (85 days by
 * this file's convention); twelve weeks is 84.
 */
private const val THREE_MONTHS_DAYS = 84

/**
 * A type, not a pair of parameters: the range outlives the question that produced it, and
 * `doseBands(plan, itemId, range)` takes this same type, so an axis and the bands under it
 * can't drift apart by deriving the same edges twice. A range always holds at least one day
 * — an empty window is `null`, not an inverted range — see [rangeOn].
 */
data class TrendRange(
    val from: LocalDate,
    val through: LocalDate,
) {
    init {
        require(from <= through) {
            "a range runs forwards: $from is after $through"
        }
    }

    val days: Int get() = from.daysUntil(through) + TODAY_COUNTS_AS_ONE

    operator fun contains(date: LocalDate): Boolean = date >= from && date <= through

    /**
     * A day is a span, not a point: day `i` of `n` occupies `[i/n, (i+1)/n]`. A dose band
     * runs from the opening edge of its first day to the closing edge of its last; a
     * protocol mark sits at the middle of its own day. Clamped so a date outside the window
     * lands at the canvas edge rather than off it.
     */
    fun fractionOf(date: LocalDate): ClosedFloatingPointRange<Float> {
        val index = from.daysUntil(date)
        val start = (index.toFloat() / days).coerceIn(0f, 1f)
        val end = ((index + TODAY_COUNTS_AS_ONE).toFloat() / days).coerceIn(0f, 1f)
        return start..end
    }
}

/** The middle of a day's span — where a single-day mark stands. */
fun TrendRange.middleOf(date: LocalDate): Float {
    val span = fractionOf(date)
    return (span.start + span.endInclusive) / 2f
}

/**
 * Right edge never in the future; left edge is *not* clipped to the first reading, which is
 * what makes «3 месяца» on a three-week course visibly partial rather than quietly full.
 * `CYCLE` is bounded at both ends by the course itself, agreeing with [cycleWeek]. Status is
 * deliberately not consulted — which days belong to a course is geometry, not the screen's question.
 */
fun TrendWindow.rangeOn(
    plan: ProtocolPlan,
    today: LocalDate,
): TrendRange? =
    when (this) {
        TrendWindow.WEEK -> endingOn(today, WEEK_DAYS)

        TrendWindow.FOUR_WEEKS -> endingOn(today, FOUR_WEEKS_DAYS)

        TrendWindow.THREE_MONTHS -> endingOn(today, THREE_MONTHS_DAYS)

        // The anchor, not a length.
        TrendWindow.CYCLE -> cycleRange(plan.protocol, today)
    }

private fun endingOn(
    today: LocalDate,
    days: Int,
): TrendRange = TrendRange(today.minus(DatePeriod(days = days - TODAY_COUNTS_AS_ONE)), today)

private fun cycleRange(
    protocol: Protocol,
    today: LocalDate,
): TrendRange? {
    // `lastPrescribedDay`, not the same arithmetic written again: dose bands are clipped
    // by it too, so this can't drift into an axis that outlives the strip beneath it.
    val through = minOf(today, protocol.lastPrescribedDay)
    return if (through < protocol.startDate) null else TrendRange(protocol.startDate, through)
}
