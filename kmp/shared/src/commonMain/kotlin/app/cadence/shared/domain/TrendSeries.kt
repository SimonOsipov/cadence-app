package app.cadence.shared.domain

import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime

/**
 * One metric, clipped to one window.
 *
 * Everything below the points is a `get()`, and that is the project's «nothing
 * derived is stored» applied to a chart: a base and a delta held as fields could
 * disagree with the readings they were taken from, and §03 already calls trend
 * series and timeframes «queries, not tables».
 *
 * The nulls are the point of the type. A metric the patient has not measured in
 * this window has no base, no delta and no average — not zeros. Zero is a
 * reading; «нет данных» is not, and rendering one as the other tells a patient
 * their heart rate variability collapsed.
 */
data class TrendSeries(
    val metric: Metric,
    val range: TrendRange,
    val points: List<Measurement>,
) {
    /** Where the window opened, which is not where the patient's history did. */
    val base: Double? get() = points.firstOrNull()?.value

    val latest: Double? get() = points.lastOrNull()?.value

    /**
     * How far the metric moved across the window — the last reading less the
     * base.
     *
     * A different question from the one «Сегодня» asks. `formatDeltaSincePrevious`
     * answers «how far did the last reading move», which on 100 → 98,4 → 98,8
     * is +0,4; this answers «how far has the patient come since the window
     * opened», which on the same three readings is −1,2. Both are true and
     * neither substitutes for the other, so they are named apart.
     *
     * Null below two readings rather than zero: one reading has nothing to
     * compare itself to, and a zero would render as «→ 0,0» — a plateau the
     * data never claimed.
     */
    val delta: Double? get() = if (points.size < 2) null else points.last().value - points.first().value

    val average: Double? get() = if (points.isEmpty()) null else points.sumOf { it.value } / points.size

    val minimum: Double? get() = points.minOfOrNull { it.value }

    val maximum: Double? get() = points.maxOfOrNull { it.value }
}

/**
 * The readings for one metric inside one window, oldest first.
 *
 * Sorted here rather than trusted: §03 specifies no ordering for `measurements`
 * at all, so none may be assumed, the seed puts one of its weights out of list
 * order on purpose, and a base read off an unsorted list is whichever row
 * arrived first. Two readings sharing an instant are broken apart by id, so that
 * «the last one» does not depend on the order they happened to arrive in.
 *
 * [zone] rather than UTC, for the same reason `CadenceClock.today` takes one: a
 * reading taken at half past one in the morning belongs to the day the patient
 * took it on, while in UTC it is still the previous evening — and a comparison
 * done there drops it out of a window that opens that morning. The caller keeps
 * the zone: placing these points back on an axis built from [TrendRange] has to
 * use the same one, or a reading admitted at the edge is drawn beyond it.
 *
 * Filtering is by metric and date only. A list holding two patients' rows would
 * merge into one plausible-looking series, so the caller owns that narrowing —
 * §03 keys every reading by `patient_id` and the repository is where that lives.
 */
fun trendSeries(
    measurements: List<Measurement>,
    metric: Metric,
    range: TrendRange,
    zone: TimeZone,
): TrendSeries =
    TrendSeries(
        metric = metric,
        range = range,
        points =
            measurements
                .filter { it.metric == metric && it.measuredAt.toLocalDateTime(zone).date in range }
                .sortedWith(compareBy({ it.measuredAt }, { it.id.raw })),
    )
