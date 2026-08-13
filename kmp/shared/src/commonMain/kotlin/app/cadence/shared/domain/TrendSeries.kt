package app.cadence.shared.domain

import kotlinx.datetime.LocalDate
import kotlinx.datetime.TimeZone
import kotlinx.datetime.toLocalDateTime

/**
 * Everything below the points is a `get()` — «nothing derived is stored» applied to a chart,
 * since §03 already calls trend series and timeframes «queries, not tables». The nulls are
 * the point: a metric not measured in this window has no base, delta or average, not zeros
 * — zero is a reading, «нет данных» is not.
 */
data class TrendSeries(
    val metric: Metric,
    val range: TrendRange,
    val points: List<Measurement>,
    val zone: TimeZone,
) {
    /**
     * The zone travels with the series so the days a chart clips by are the days it draws
     * by; in a second zone, a reading admitted at the window's edge is drawn beyond it.
     */
    fun dayOf(point: Measurement): LocalDate = point.measuredAt.toLocalDateTime(zone).date

    /** Where the window opened, which is not where the patient's history did. */
    val base: Double? get() = points.firstOrNull()?.value

    val latest: Double? get() = points.lastOrNull()?.value

    /**
     * A different question from `formatDeltaSincePrevious`'s «how far did the last reading
     * move» (100 → 98,4 → 98,8 is +0,4) — this answers «how far since the window opened»
     * (−1,2 on the same readings). Null below two readings, not zero: a zero would render
     * as a plateau the data never claimed.
     */
    val delta: Double? get() = if (points.size < 2) null else points.last().value - points.first().value

    val average: Double? get() = if (points.isEmpty()) null else points.sumOf { it.value } / points.size

    val minimum: Double? get() = points.minOfOrNull { it.value }

    val maximum: Double? get() = points.maxOfOrNull { it.value }
}

/**
 * Sorted here, not trusted: §03 specifies no ordering, and the seed puts one weight out of
 * list order on purpose. Ties broken by id so «the last one» doesn't depend on arrival order.
 * [zone], not UTC, same reason `CadenceClock.today` takes one — travels into the result so
 * whoever draws these points reads days the same way this filter did. Filtering is by metric
 * and date only; the caller owns narrowing by patient (§03 keys every reading by `patient_id`).
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
        zone = zone,
    )
