package app.cadence.shared.domain

import kotlin.math.abs

/** One metric's row on the list: what it is, and what it did in the window. */
data class MetricTrend(
    val meta: MetricMeta,
    val series: TrendSeries,
) {
    /**
     * How far the metric moved, as a share of where the window opened.
     *
     * Null when there is nothing to compare — fewer than two readings — or when
     * the base is zero, which no metric §03 has can honestly be but a division
     * would not care.
     *
     * Relative rather than absolute, because the point of it is to rank metrics
     * against each other: eight milliseconds of HRV on a base of fifty is a
     * bigger change than two kilograms on a base of a hundred, and comparing
     * the raw numbers would sort by unit.
     */
    val movement: Double?
        get() {
            val base = series.base ?: return null
            val delta = series.delta ?: return null
            return if (base == 0.0) null else abs(delta / base)
        }
}

/**
 * Every metric of §03, for one window.
 *
 * The whole set, in `Metric.entries` order, including the ones with nothing to
 * show: a list that dropped an unmeasured metric would leave a patient unable
 * to find out that it *is* unmeasured, and «метрика без измерений говорит об
 * этом» is one of the screen's criteria.
 */
data class TrendsOverview(
    val window: TrendWindow,
    val range: TrendRange,
    val metrics: List<MetricTrend>,
) {
    /**
     * The card at the top of the screen.
     *
     * The first metric of the set — weight — rather than the last one opened.
     * «What I looked at last time» is a second kind of memory for the app to
     * keep, and there is nowhere to keep it until the profile block; the screen
     * stays a function of its data. Recorded as a divergence: the prototype
     * features whichever biomarker was opened last.
     */
    val hero: MetricTrend? get() = metrics.firstOrNull()

    /** The two-up grid below the hero. */
    val rest: List<MetricTrend> get() = metrics.drop(1)
}

/** Three, as the prototype shows — enough to be a summary, few enough to read. */
const val NOTABLE_SHIFTS: Int = 3

/**
 * The metrics that moved most across the window, largest first.
 *
 * Derived, which is the whole difference from the prototype. Its «Заметные
 * сдвиги» are three hand-written cards — «HRV вырос на 17 мс · С момента старта
 * BPC-157», «Глубокий сон +24 мин», «Метаболизм стабилен» — and two of the
 * three name things §03 has no metric for at all. Only the shape of the first
 * survives the port: a metric, and how far it moved.
 *
 * Metrics with nothing to compare are left out rather than ranked last. A
 * section called «заметные сдвиги» listing a metric that has not been measured
 * twice is naming a shift that did not happen.
 */
fun TrendsOverview.notableShifts(limit: Int = NOTABLE_SHIFTS): List<MetricTrend> =
    metrics
        .filter { it.movement != null }
        // By id after the movement, so two metrics that moved by the same share
        // do not swap places between two reads of the same data.
        .sortedWith(compareByDescending<MetricTrend> { it.movement }.thenBy { it.meta.metric.code })
        .take(limit)
