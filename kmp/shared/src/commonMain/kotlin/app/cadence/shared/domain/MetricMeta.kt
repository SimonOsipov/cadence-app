package app.cadence.shared.domain

/** Which way a metric has to move for the patient to be getting better. */
enum class MetricDirection { UP, DOWN }

/**
 * Whether a screen may offer «Добавить замер» for this metric.
 *
 * Named for the decision it makes rather than for where readings come from —
 * that is [MeasurementSource]'s job, and the two are not the same axis. A
 * weight arrives from a smart scale *and* from the patient's own hand, and
 * `measurements.md` invariant 6 says manual entry stays first-class everywhere:
 * the import is a reinforcement, never a replacement. So [BY_HAND] does not
 * claim a metric is never imported, only that a hand can write it.
 *
 * The [BY_HAND] half is the prototype's own `editable` flag
 * (`mobile/src/features/body/data.ts:37-41`): weight, body fat and the three
 * tape measurements. The other half is an inference from absence — HRV,
 * resting pulse and sleep carry no such flag because they are not on that
 * screen at all, and the prototype offers no control anywhere that types one
 * in. `SLEEP` is [DEVICE_ONLY] even though the API scores it: the sessions
 * behind the score are the watch's, and no hand enters them.
 */
enum class MetricEntry { BY_HAND, DEVICE_ONLY }

/**
 * The colour family a metric is drawn in.
 *
 * Data rather than a `when` on the screen, for the same reason §03 puts an
 * `icon` on a compound: which family a metric belongs to is a product decision.
 * The enum names a family, not a colour — the palette stays on the surface.
 */
enum class MetricAccent { FOREST, SAND }

/**
 * A sanity cap, not a limit `formatDecimal` imposes.
 *
 * That function indexes a `Long` power-of-ten sequence, so it copes with far
 * more than this; what it cannot cope with is a negative count, which walks off
 * the front. Four is «no clinical measurement is finer than this», and a table
 * asking for more has a typo rather than a requirement.
 */
private const val MAX_DECIMALS = 4

/**
 * Everything about a metric that is not a reading.
 *
 * The prototype keeps these fields on the same object as the numbers
 * (`TREND_DATA` in `mobile/src/features/trends/data.ts`), which is what makes
 * its baselines and its series two copies of one truth. Here the readings are
 * §03's rows and this is the table beside them.
 *
 * [unit] is the wire unit, because that is what the readings carry and what the
 * project's «numbers are data, formatting is presentation» rule leaves to the
 * surface; `unitRu` turns «kg» into «кг». The prototype stores the Russian
 * directly, so four of these eight — «kg», «ms», «bpm», «cm» — are deliberately
 * *not* its strings. The other four («%», «/100») are the same either way.
 *
 * [decimals] is a rendering hint held as data, which is the one field here that
 * sits awkwardly in the domain. It is kept because §03 already moves the
 * neighbouring per-metric constants server-side — «Thresholds (HRV ≥58 · RHR
 * ≤60 · Sleep ≥75) move from client code to one constants module in the API so
 * both surfaces agree» — and this table is standing in for that module until it
 * exists.
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
        // The same guard `TrendRange` carries, for the same reason: a row with
        // a blank label draws a nameless card, and a negative `decimals`
        // indexes off the front of `formatDecimal`'s scale sequence.
        require(label.isNotBlank() && eyebrow.isNotBlank() && unit.isNotBlank()) {
            "${metric.code} needs a label, an eyebrow and a unit"
        }
        require(decimals in 0..MAX_DECIMALS) {
            "${metric.code} asks for $decimals decimal places"
        }
    }
}

/**
 * The row for this metric.
 *
 * A `when` rather than a map lookup: the compiler then owns «every metric has a
 * row», so a ninth metric fails to build instead of failing a test — or, worse,
 * throwing out of a composable that asked for the row it forgot. The rows are
 * declared once below rather than constructed here, so reading this in a list
 * does not allocate eight objects per frame.
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

// Six rows are the trends prototype's own copy, taken as written.
//
// `HIP` and `CHEST` are not in that module — but they are not invented either.
// The prototype's *body* screen carries them
// (`mobile/src/features/body/data.ts:40-41`, drawn by `BodyScreen.tsx:723`)
// with the labels «Бёдра» and «Грудь» and with `editable: true`, so [label] and
// [MetricEntry] are ported like the rest. What that screen has no field for is
// an eyebrow, a direction or an accent — those three are inferred from `WAIST`,
// the tape measurement both modules describe.
//
// `decimals` for the tape is the one place the two prototype modules disagree:
// the body screen says `dec: 0` for waist, hip and chest, the trends module
// says `decimals: 1` for waist. This table follows trends, because the seeded
// tape series are in tenths (`MockSeed`'s `TENTHS` grid) and rounding them to
// whole centimetres flattens the chart: the hip's seven weekly readings step
// six times at one decimal place and three times at none — half its history
// becomes a plateau it never had. The waist loses one step of six.
//
// `THIGH` is in neither table here: the prototype has it, §03 does not, and the
// data model is canon.

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
        // A score with no unit of its own. Without the «/100» the card reads
        // «Сон 86» — a number whose scale the patient has to guess, next to a
        // pulse and a weight that carry theirs.
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
        // Invented: the body screen has no eyebrow. It follows «Талия» /
        // «Обхват талии» rather than the trends module's thigh, whose label and
        // eyebrow are the same string.
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
        // Down with the rest of the tape, and the body screen's own seeded
        // history agrees (`SEED_HIST.chest` runs 112 → 105). A course built to
        // add mass would want the other answer, and this is one of the fields
        // the constants module will eventually own.
        direction = MetricDirection.DOWN,
        entry = MetricEntry.BY_HAND,
        accent = MetricAccent.FOREST,
    )
