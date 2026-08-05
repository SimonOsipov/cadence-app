package app.cadence.shared.domain

import kotlinx.datetime.DatePeriod
import kotlinx.datetime.LocalDate
import kotlinx.datetime.daysUntil
import kotlinx.datetime.minus

/**
 * The four spans the trends screens switch between.
 *
 * Three of them are lengths. `CYCLE` is not: it is bounded by the protocol —
 * it opens on `startDate` and closes on the last prescribed day — so the same
 * calendar day answered against two different courses gives two different
 * windows.
 *
 * The prototype hides this. It writes every window as a literal span ending at
 * a frozen «day 84», and gives «3 месяца» and «цикл» the *same* 84-day span
 * (`mobile/src/features/trends/data.ts:144-145`), so switching between those two
 * changed nothing on screen. Porting that would draw a patient in week two the
 * same chart as a patient in week eleven.
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
 * Twelve weeks, counted inclusively.
 *
 * The prototype's axis runs day 0 to day 84, which is 85 days by this file's
 * convention; twelve weeks is 84, and that is what «3 месяца» means here.
 */
private const val THREE_MONTHS_DAYS = 84

/**
 * The days a window covers, both ends included.
 *
 * A type rather than a pair of parameters because the range outlives the
 * question that produced it: the chart will draw its axis from these two dates,
 * and `doseBands(plan, itemId, range)` takes this very type. Two call sites
 * deriving the same edges separately is how an axis and the bands under it
 * drift apart.
 *
 * A range always holds at least one day. A window with no days at all is not an
 * inverted range but `null` — see [rangeOn]. The distinction matters because
 * [from] and [through] are read directly by the chart and by the bands, and only
 * [days] and [contains] would notice a pair that had quietly swapped ends.
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
     * Where a day sits across this window, as a `0f..1f` span.
     *
     * A day is a span rather than a point because that is what a day is on an
     * axis: day `i` of `n` occupies `[i/n, (i+1)/n]`. A dose band runs from the
     * opening edge of its first day to the closing edge of its last, so a band
     * covering the whole window fills it; a protocol mark stands at the middle
     * of its own day, where it is inside the band it opens rather than on the
     * seam before it.
     *
     * Clamped, because the caller draws with the result. A date outside the
     * window has no place on the axis, and an unclamped fraction would put it
     * outside the canvas instead of at its edge.
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
 * Which days this window covers, ending no later than [today] — or null when it
 * covers none.
 *
 * The right edge is never in the future: a window longer than the patient has
 * lived draws what was lived rather than empty space to the right of it. The
 * left edge is *not* clipped to the first reading — that is what makes «3
 * месяца» on a three-week course visibly partial instead of quietly looking
 * full.
 *
 * `CYCLE` is bounded at both ends by the course itself, which is what keeps it
 * agreeing with [cycleWeek]: that function returns null once a date passes
 * `weeks`, and a «весь цикл» window that kept growing past the last prescribed
 * day would put dates on the axis the rest of this package considers outside the
 * protocol. Null follows the same precedent — a course that starts next week has
 * no cycle to show yet, exactly as [cycleWeek] has no week to name.
 *
 * Status is deliberately not consulted. [cycleWeek] does not consult it either:
 * which days belong to a course is geometry, and whether a cancelled course
 * should be drawn at all is the screen's question, not this one's.
 *
 * [today] is a parameter for the reason every date in this module is one: a
 * function that read its own clock could not be asked about another day.
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
    // `lastPrescribedDay` rather than the same arithmetic written again: the
    // dose bands are clipped by it too, and an axis that outlived the strip
    // beneath it is what one copy drifting from the other looks like.
    val through = minOf(today, protocol.lastPrescribedDay)
    return if (through < protocol.startDate) null else TrendRange(protocol.startDate, through)
}
