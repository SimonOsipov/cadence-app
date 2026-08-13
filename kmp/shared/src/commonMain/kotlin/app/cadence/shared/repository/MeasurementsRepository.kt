package app.cadence.shared.repository

import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.Metric

/**
 * One metric's recent history. §11: the Biomarker Sheet reads «latest + 7-pt series per
 * metric», and the Today screen's headline glance draws the same points as a sparkline —
 * one read, not two. Ordered oldest first, the direction a chart draws in; `latest` is the
 * last point rather than a separate wire field, since two fields could disagree and §03's
 * rule is «latest reading wins».
 */
data class MetricSeries(
    val metric: Metric,
    val points: List<Measurement>,
) {
    val latest: Measurement? get() = points.lastOrNull()
}

/** §11: `GET /me/trends` and the biomarker sheet's slice of it. */
interface MeasurementsRepository {
    suspend fun series(
        metric: Metric,
        points: Int = DEFAULT_POINTS,
    ): MetricSeries

    companion object {
        /** «7-pt series per metric», §11. */
        const val DEFAULT_POINTS: Int = 7
    }
}
