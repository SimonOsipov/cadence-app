package app.cadence.shared.domain

import kotlin.time.Instant

/**
 * §03's eight metrics, in one table.
 *
 * The fifth correction: the dashboard shows {hrv, rhr, sleep} plus weight and
 * mobile shows all eight, but they are «same rows, different projections» —
 * not two datasets. `SLEEP` is Cadence's derived «Сон /100», computed by the
 * API from imported sessions rather than read from a device.
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
         * The metric a wire code names, or null if none does.
         *
         * `CadenceRoute.TrendDetail` carries a `String` — it is the prototype's
         * `RootStackParamList` parameter, and the codes line up because the
         * prototype keys `TREND_DATA` by the same names §03 stores — so this is
         * the only way back. Null rather than a default: the prototype has a
         * `thigh` that §03 does not, and a screen opened on it has to say so
         * instead of quietly showing somebody's weight.
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
