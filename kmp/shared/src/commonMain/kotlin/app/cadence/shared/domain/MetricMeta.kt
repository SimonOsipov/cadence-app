package app.cadence.shared.domain

/** Which way a metric has to move for the patient to be getting better. */
enum class MetricDirection { UP, DOWN }

/**
 * Whether a screen may offer «Добавить замер», not where readings come from (that's
 * [MeasurementSource]'s axis) — a weight arrives from a scale *and* the patient's hand, and
 * import never replaces manual entry (measurements.md invariant 6). [BY_HAND] is the
 * prototype's `editable` flag; the rest is inferred from absence. `SLEEP` is [DEVICE_ONLY]
 * even though the API scores it: the sessions behind the score are the watch's.
 */
enum class MetricEntry { BY_HAND, DEVICE_ONLY }

/**
 * Data, not a `when` on the screen — a product decision, same reason §03 puts an `icon` on
 * a compound. Names a family, not a colour; the palette stays on the surface.
 */
enum class MetricAccent { FOREST, SAND }

/**
 * A sanity cap, not `formatDecimal`'s own limit — a negative count walks off the front of
 * its scale sequence; four is "no clinical measurement is finer than this".
 */
private const val MAX_DECIMALS = 4

/**
 * The prototype keeps these fields on the same object as the numbers, making baselines and
 * series two copies of one truth; here the readings are §03's rows and this is the table
 * beside them. [unit] is the wire unit — `unitRu` turns «kg» into «кг» — deliberately not
 * the prototype's own Russian strings for four of the eight. [decimals] sits awkwardly in
 * the domain as a rendering hint, standing in for the constants module §03 eventually moves these to.
 */
data class MetricMeta(
    val metric: Metric,
    val label: String,
    val eyebrow: String,
    val unit: String,
    val decimals: Int,
    val direction: MetricDirection,
    val entry: MetricEntry,
    val accent: MetricAccent,
) {
    init {
        // Same guard as `TrendRange`: a blank label draws a nameless card, a negative
        // `decimals` indexes off the front of `formatDecimal`'s scale sequence.
        require(label.isNotBlank() && eyebrow.isNotBlank() && unit.isNotBlank()) {
            "${metric.code} needs a label, an eyebrow and a unit"
        }
        require(decimals in 0..MAX_DECIMALS) {
            "${metric.code} asks for $decimals decimal places"
        }
    }
}

/**
 * A `when`, not a map lookup: the compiler then owns «every metric has a row», so a ninth
 * metric fails to build rather than throwing out of a composable. Rows declared once below,
 * not constructed here, so reading this in a list doesn't allocate eight objects per frame.
 */
val Metric.meta: MetricMeta
    get() =
        when (this) {
            Metric.WEIGHT -> WEIGHT_META
            Metric.HRV -> HRV_META
            Metric.RHR -> RHR_META
            Metric.SLEEP -> SLEEP_META
            Metric.BODY_FAT -> BODY_FAT_META
            Metric.WAIST -> WAIST_META
            Metric.HIP -> HIP_META
            Metric.CHEST -> CHEST_META
        }

// Six rows are the trends prototype's own copy. `HIP` and `CHEST` aren't in that module, but
// come from the body screen (label + `editable: true`); eyebrow/direction/accent are
// inferred from `WAIST`. `decimals` follows trends' `1` for the tape, not the body screen's
// `0`: the seeded tape series are in tenths, and rounding to whole cm flattens the chart —
// the hip's seven readings would step six times at one decimal and three times at none.
// `THIGH` is in neither table: the prototype has it, §03 doesn't.

private val WEIGHT_META =
    MetricMeta(
        metric = Metric.WEIGHT,
        label = "Вес",
        eyebrow = "Масса тела",
        unit = "kg",
        decimals = 1,
        direction = MetricDirection.DOWN,
        entry = MetricEntry.BY_HAND,
        accent = MetricAccent.FOREST,
    )

private val HRV_META =
    MetricMeta(
        metric = Metric.HRV,
        label = "HRV",
        eyebrow = "Вариабельность сердца",
        unit = "ms",
        decimals = 0,
        direction = MetricDirection.UP,
        entry = MetricEntry.DEVICE_ONLY,
        accent = MetricAccent.FOREST,
    )

private val RHR_META =
    MetricMeta(
        metric = Metric.RHR,
        label = "ЧСС покоя",
        eyebrow = "Пульс в покое",
        unit = "bpm",
        decimals = 0,
        direction = MetricDirection.DOWN,
        entry = MetricEntry.DEVICE_ONLY,
        accent = MetricAccent.FOREST,
    )

private val SLEEP_META =
    MetricMeta(
        metric = Metric.SLEEP,
        label = "Сон",
        eyebrow = "Качество сна",
        // Without «/100» the card reads «Сон 86» — a number with no scale to guess against.
        unit = "/100",
        decimals = 0,
        direction = MetricDirection.UP,
        entry = MetricEntry.DEVICE_ONLY,
        accent = MetricAccent.SAND,
    )

private val BODY_FAT_META =
    MetricMeta(
        metric = Metric.BODY_FAT,
        label = "% жира",
        eyebrow = "Состав тела",
        unit = "%",
        decimals = 1,
        direction = MetricDirection.DOWN,
        entry = MetricEntry.BY_HAND,
        accent = MetricAccent.FOREST,
    )

private val WAIST_META =
    MetricMeta(
        metric = Metric.WAIST,
        label = "Талия",
        eyebrow = "Обхват талии",
        unit = "cm",
        decimals = 1,
        direction = MetricDirection.DOWN,
        entry = MetricEntry.BY_HAND,
        accent = MetricAccent.FOREST,
    )

private val HIP_META =
    MetricMeta(
        metric = Metric.HIP,
        label = "Бёдра",
        // Invented: the body screen has no eyebrow. Follows «Талия»/«Обхват талии», not the
        // trends module's thigh (whose label and eyebrow are the same string).
        eyebrow = "Обхват бёдер",
        unit = "cm",
        decimals = 1,
        direction = MetricDirection.DOWN,
        entry = MetricEntry.BY_HAND,
        accent = MetricAccent.FOREST,
    )

private val CHEST_META =
    MetricMeta(
        metric = Metric.CHEST,
        label = "Грудь",
        eyebrow = "Обхват груди",
        unit = "cm",
        decimals = 1,
        // Down with the rest of the tape; the body screen's own seeded history agrees
        // (SEED_HIST.chest runs 112 → 105). One of the fields the constants module will own.
        direction = MetricDirection.DOWN,
        entry = MetricEntry.BY_HAND,
        accent = MetricAccent.FOREST,
    )
