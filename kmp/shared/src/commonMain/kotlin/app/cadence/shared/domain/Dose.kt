package app.cadence.shared.domain

/** §03: `default_unit мг|мкг`. */
enum class DoseUnit(
    val code: String,
) {
    MG("мг"),
    MCG("мкг"),
}

/**
 * A dose, as §03 resolves it: «numeric + unit; RU formatting ("0,25 мг") is
 * presentation».
 *
 * `Double` rather than a decimal type: these are protocol doses with two
 * significant figures, they are never summed, and Kotlin Multiplatform has no
 * common `BigDecimal`. If a total is ever needed, the type changes here, once.
 */
data class Dose(
    val value: Double,
    val unit: DoseUnit,
)
