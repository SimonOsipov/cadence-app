package app.cadence.shared.repository

import app.cadence.shared.domain.DoseBand
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.ProtocolMark
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview

/**
 * One metric, with everything drawn beside it.
 *
 * The bands and the marks travel with the series rather than being fetched
 * separately, because they are the same answer to «what happened in this
 * window»: §11's trends row reads «series by metric and window with baselines,
 * protocol events overlaid, and dose bands», one call.
 */
data class MetricDetail(
    val trend: MetricTrend,
    val bands: List<DoseBand>,
    val marks: List<ProtocolMark>,
)

/**
 * §11's `GET /me/trends`.
 *
 * Two calls rather than one, because the two screens ask different questions:
 * the list wants every metric shallowly, the detail wants one metric with its
 * protocol overlay. A single call returning both would send eight metrics'
 * worth of bands to a screen that draws none of them.
 *
 * The window is a parameter on both, not state held here — a repository that
 * remembered which window was chosen would be a second place the answer lives,
 * and the screen already owns it.
 */
interface TrendsRepository {
    suspend fun overview(window: TrendWindow): TrendsOverview

    /**
     * Not nullable: the parameter is the enum, so «no such metric» cannot reach
     * here. Resolving a route's `String` is `Metric.fromCode`'s job, and the
     * screen that opened on an unknown code says so before it asks for data —
     * a nullable return would put an unreachable branch in every caller.
     */
    suspend fun metric(
        metric: Metric,
        window: TrendWindow,
    ): MetricDetail
}
