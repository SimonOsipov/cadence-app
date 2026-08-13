package app.cadence.shared.repository

import app.cadence.shared.domain.DoseBand
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.ProtocolMark
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview

/**
 * One metric, with everything drawn beside it. The bands and marks travel with the
 * series rather than being fetched separately — they're the same answer to «what
 * happened in this window» per §11's trends row.
 */
data class MetricDetail(
    val trend: MetricTrend,
    val bands: List<DoseBand>,
    val marks: List<ProtocolMark>,
)

/**
 * §11's `GET /me/trends`. Two calls, not one: the list wants every metric shallowly,
 * the detail wants one with its protocol overlay — one call would send every metric's
 * bands to a screen that draws none of them. The window is a parameter, not state held
 * here, since the screen already owns it.
 */
interface TrendsRepository {
    suspend fun overview(window: TrendWindow): TrendsOverview

    /**
     * Not nullable: the parameter is the enum, so «no such metric» cannot reach here —
     * resolving a route's `String` is `Metric.fromCode`'s job.
     */
    suspend fun metric(
        metric: Metric,
        window: TrendWindow,
    ): MetricDetail
}
