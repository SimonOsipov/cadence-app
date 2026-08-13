package app.cadence.shared.domain

import kotlin.math.abs

/** One metric's row on the list: what it is, and what it did in the window. */
data class MetricTrend(
    val meta: MetricMeta,
    val series: TrendSeries,
) {
    /**
     * Relative, not absolute: the point is ranking metrics against each other, and 8ms of
     * HRV on a base of 50 is a bigger change than 2kg on a base of 100 — raw numbers would
     * sort by unit. Null below two readings or a zero base.
     */
    val movement: Double?
        get() {
            val base = series.base ?: return null
            val delta = series.delta ?: return null
            return if (base == 0.0) null else abs(delta / base)
        }
}

/**
 * The whole set, in `Metric.entries` order, including unmeasured ones: dropping one would
 * leave a patient unable to find out it *is* unmeasured.
 */
data class TrendsOverview(
    val window: TrendWindow,
    val range: TrendRange,
    val metrics: List<MetricTrend>,
) {
    /**
     * The first metric — weight — not the last one opened: no memory to keep that in yet.
     * Divergence: the prototype features whichever was opened last.
     */
    val hero: MetricTrend? get() = metrics.firstOrNull()

    /** The two-up grid below the hero. */
    val rest: List<MetricTrend> get() = metrics.drop(1)
}

/** Three, as the prototype shows — enough to be a summary, few enough to read. */
const val NOTABLE_SHIFTS: Int = 3

/**
 * Derived, unlike the prototype's three hand-written «Заметные сдвиги» cards, two of which
 * name things §03 has no metric for. Only the shape of the first survives: a metric and how
 * far it moved. Metrics with nothing to compare are left out, not ranked last.
 */
fun TrendsOverview.notableShifts(limit: Int = NOTABLE_SHIFTS): List<MetricTrend> =
    metrics
        .filter { it.movement != null }
        // By id after movement, so two metrics that moved by the same share don't swap
        // places between two reads of the same data.
        .sortedWith(compareByDescending<MetricTrend> { it.movement }.thenBy { it.meta.metric.code })
        .take(limit)
