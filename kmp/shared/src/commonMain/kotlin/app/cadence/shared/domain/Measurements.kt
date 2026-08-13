package app.cadence.shared.domain

import kotlin.time.Instant

/**
 * §03's eight metrics, one table. The fifth correction: dashboard and mobile show different
 * subsets, but «same rows, different projections», not two datasets. `SLEEP` is Cadence's
 * derived «Сон /100», computed by the API from imported sessions, not read from a device.
 */
enum class Metric(
    val code: String,
) {
    WEIGHT("weight"),
    HRV("hrv"),
    RHR("rhr"),
    SLEEP("sleep"),
    BODY_FAT("bodyfat"),
    WAIST("waist"),
    HIP("hip"),
    CHEST("chest"),
    ;

    companion object {
        /**
         * Null, not a default: the prototype has a `thigh` §03 doesn't, and a screen opened
         * on it has to say so, not quietly show somebody's weight.
         */
        fun fromCode(code: String): Metric? = entries.firstOrNull { it.code == code }
    }
}

/** §03: `source manual|healthkit|health_connect`. Latest reading wins, whatever the source. */
enum class MeasurementSource(
    val code: String,
) {
    MANUAL("manual"),
    HEALTH_KIT("healthkit"),
    HEALTH_CONNECT("health_connect"),
}

/** §03's `measurements`. */
data class Measurement(
    val id: MeasurementId,
    val patientId: UserId,
    val metric: Metric,
    val value: Double,
    val unit: String,
    val measuredAt: Instant,
    val source: MeasurementSource,
    val externalId: String?,
    val note: String?,
)

/** §03's `body_photos`. BMI is derived from height, so it is not a field. */
data class BodyPhoto(
    val patientId: UserId,
    val storagePath: String,
    val takenAt: Instant,
    val weightKgAt: Double?,
)
