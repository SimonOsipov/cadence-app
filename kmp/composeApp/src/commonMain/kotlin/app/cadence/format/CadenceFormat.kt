package app.cadence.format

import app.cadence.shared.domain.Dose

/*
 * The one place this surface turns numbers into Russian text. Numbers are data,
 * formatting is presentation — the project rule — so nothing upstream holds
 * «1 240» as a string and nothing downstream builds one of its own.
 *
 * Hand-rolled, not a locale API: Kotlin/Native carries no ICU, so there is no
 * `toLocaleString` on both platforms, and the two rules the product needs fit in
 * thirty lines a test can pin exactly.
 */

/** Russian groups thousands with U+00A0, so a value never wraps mid-number. */
private const val GROUP_SEPARATOR = ' '
private const val GROUP_SIZE = 3

/** `toLocaleString('ru-RU')` for a whole number: 1240 → «1 240». */
fun formatInteger(value: Int): String {
    val negative = value < 0
    // Widened to Long before negating: -Int.MIN_VALUE is Int.MIN_VALUE, so the
    // obvious `-value` returns it unchanged and renders the most negative
    // number this type holds as a positive one.
    val digits =
        value
            .toLong()
            .let { if (negative) -it else it }
            .toString()

    val grouped =
        buildString {
            digits.forEachIndexed { index, digit ->
                if (index > 0 && (digits.length - index) % GROUP_SIZE == 0) append(GROUP_SEPARATOR)
                append(digit)
            }
        }

    return if (negative) "-$grouped" else grouped
}

private const val DECIMAL_BASE = 10L
private const val DOSE_DIGITS = 2
private const val HALF = 0.5
private const val TENTHS_PER_UNIT = 10.0

/** «960 ккал» — a whole kcal value with its unit. `MealHero.kt`'s remaining-line and totals row. */
fun formatKcal(value: Int): String = "${formatInteger(value)} ккал"

/** «80 г» — a whole-gram value with its unit. `MealHero.kt`'s remaining-line. */
fun formatGrams(value: Int): String = "${formatInteger(value)} г"

/**
 * A [app.cadence.shared.domain.MacrosTenths] kcal field, rounded for display without a
 * second `toMacros()` — that conversion stays the single path to a whole
 * [app.cadence.shared.domain.Macros] at the `TodaySummary`/`NutritionDay.totals`
 * boundary (`Nutrition.kt:62-77,197`); mirrors the file-private `tenthsLabel` idiom in
 * `NutritionMealFeed.kt` and `LogMealItemsList.kt`.
 */
fun formatKcalTenths(kcalTenths: Int): String = "${formatDecimal(kcalTenths / TENTHS_PER_UNIT, digits = 0)} ккал"

/**
 * A decimal with the Russian comma and a fixed digit count — not trimmed, since a
 * weight that dropped its trailing zero would jump between «110,0» and «110», and a
 * column that changes width is a column that jitters.
 */
fun formatDecimal(
    value: Double,
    digits: Int,
): String {
    val negative = value < 0
    val scale = generateSequence(1L) { it * DECIMAL_BASE }.elementAt(digits)
    // floor(x + 0,5), not round(x): Kotlin's round breaks ties to even, so
    // 0,25 at one digit gives «0,2». A patient reading a dose expects the half
    // to go up.
    val scaled = kotlin.math.floor(kotlin.math.abs(value) * scale + HALF).toLong()
    val whole = formatInteger((scaled / scale).toInt())
    val fraction = (scaled % scale).toString().padStart(digits, '0')

    val body = if (digits == 0) whole else "$whole,$fraction"
    return if (negative) "-$body" else body
}

/**
 * A dose as its two runs: «0,25» and «мг». Trailing zeroes are dropped here only,
 * since a dose reads the way it's written on the box — «1 мг», not «1,0 мг» — while
 * a weight is not. Returned as a pair because `CadenceNumber` renders the value in
 * tabular mono and the unit in a different face.
 */
fun formatDose(dose: Dose): Pair<String, String> {
    val whole = dose.value == kotlin.math.floor(dose.value)
    // Trim only past the comma. `trimEnd('0')` on a whole number turns 250 мкг
    // into 25 мкг, which is a tenth of a dose.
    val value =
        if (whole) {
            formatDecimal(dose.value, digits = 0)
        } else {
            formatDecimal(dose.value, DOSE_DIGITS).trimEnd('0').trimEnd(',')
        }
    return value to dose.unit.code
}

/**
 * «↓ 0,4 кг» — how far the *last* reading moved, vs. `TrendSeries.delta`'s «since the
 * window opened»: on 101,2 → 99,0 → 98,8 → 98,4 the two answer −0,4 and −2,8, so one
 * name over both invited reaching for whichever was in scope. Null with nothing to
 * compare — `points[size - 2]` on a one-point series throws, which is why this is a
 * function rather than an expression repeated at two call sites.
 */
fun formatDeltaSincePrevious(
    points: List<Double>,
    unit: String,
    digits: Int = 1,
): String? {
    if (points.size < 2) return null
    return formatSignedDelta(points.last() - points[points.size - 2], unit, digits)
}

/**
 * A delta someone else already computed, as «↓ 2,8 кг» — the sole `when` deciding
 * the arrow, so a trends screen handing over a `TrendSeries.delta` with no points
 * left to subtract never grows a second copy of this rule.
 *
 * Zero reads «→», not «↑»: two identical readings are a plateau, not a gain. The
 * arrow follows what is *printed*, not what was computed — a window delta can be
 * −0,04 kg (imported weights are not rounded onto tenths), so «↓ 0,0» must read off
 * the rendered string rather than be re-derived, or it could disagree with a true
 * plateau's «→ 0,0».
 */
fun formatSignedDelta(
    delta: Double,
    unit: String,
    digits: Int = 1,
): String {
    val magnitude = if (delta < 0) -delta else delta
    val shown = formatDecimal(magnitude, digits)
    val arrow =
        when {
            shown.none { it in '1'..'9' } -> "→"
            delta < 0 -> "↓"
            else -> "↑"
        }
    return "$arrow $shown $unit"
}

/** §03 stores the wire unit («kg», «ms»); the screen shows Russian. */
fun unitRu(unit: String): String =
    when (unit) {
        "kg" -> "кг"
        "ms" -> "мс"
        "bpm" -> "уд/мин"
        "cm" -> "см"
        else -> unit
    }
