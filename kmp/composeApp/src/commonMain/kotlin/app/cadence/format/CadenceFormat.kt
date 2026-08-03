package app.cadence.format

import app.cadence.shared.domain.Dose

/*
 * The one place this surface turns numbers into Russian text.
 *
 * Numbers are data and formatting is presentation — the project rule — so
 * nothing upstream of here ever holds «1 240» as a string, and nothing
 * downstream builds one of its own.
 *
 * Hand-rolled rather than taken from a locale API: Kotlin/Native carries no
 * ICU, so there is no `toLocaleString` to call on both platforms, and the two
 * rules the product needs fit in thirty lines that a test can pin exactly.
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

private const val TEEN_FLOOR = 11
private const val TEEN_CEILING = 14
private const val FEW_FLOOR = 2
private const val FEW_CEILING = 4
private const val TENS = 100
private const val DECIMAL_BASE = 10L
private const val DOSE_DIGITS = 2
private const val HALF = 0.5
private const val UNITS = 10

/**
 * «приём» / «приёма» / «приёмов».
 *
 * The prototype writes `count === 1 ? 'приём' : count < 5 ? 'приёма' :
 * 'приёмов'`, which is right through 20 and wrong from 21 up. Every count the
 * sheet can actually reach today is inside the range where the two agree, so
 * this changes nothing on screen — what it changes is that the next screen
 * counting something copies a rule that holds rather than one that happens to.
 */
fun pluralMeals(count: Int): String = russianPlural(count, "приём", "приёма", "приёмов")

/**
 * «неделю» / «недели» / «недель».
 *
 * The reorder card repeats the meal card's approximation in the prototype
 * (`weeksLeft < 5 ? 'недели' : 'недель'`) and is wrong from 21 up in the same
 * way. One rule serves both nouns.
 */
fun pluralWeeks(count: Int): String = russianPlural(count, "неделю", "недели", "недель")

/**
 * The rule itself, so the next noun does not copy a `when` block.
 *
 * The whole reason for fixing the prototype's version was «the next screen
 * counting something copies a rule that holds instead of one that happens to» —
 * which only works if the rule is callable.
 */
private fun russianPlural(
    count: Int,
    one: String,
    few: String,
    many: String,
): String {
    val n = if (count < 0) -count else count
    val lastTwo = n % TENS
    val last = n % UNITS

    return when {
        lastTwo in TEEN_FLOOR..TEEN_CEILING -> many
        last == 1 -> one
        last in FEW_FLOOR..FEW_CEILING -> few
        else -> many
    }
}

/**
 * A decimal with the Russian comma and a fixed number of digits.
 *
 * Fixed, not trimmed: a weight that dropped its trailing zero would jump
 * between «110,0» and «110» as the patient loses a kilogram, and a column of
 * numbers that changes width is a column that jitters.
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
 * A dose as its two runs: «0,25» and «мг».
 *
 * Trailing zeroes are dropped here and nowhere else, because a dose is spoken
 * the way it is written on the box — «1 мг», not «1,0 мг» — while a weight is
 * not. Returned as a pair because `CadenceNumber` takes them apart: the value
 * is tabular mono, the unit is not.
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
 * «↓ 0,4 кг» — how far the last reading moved, or null if there is nothing to
 * compare it to.
 *
 * One reading is where every patient starts, and `points[size - 2]` on a
 * one-point series throws; the guard is the reason this is a function rather
 * than an expression repeated at two call sites, which is what it was.
 *
 * Zero is «→», not «↑». Two identical readings are a plateau, and rendering one
 * as a gain is a claim the data does not make.
 */
fun formatDelta(
    points: List<Double>,
    unit: String,
    digits: Int = 1,
): String? {
    if (points.size < 2) return null
    val delta = points.last() - points[points.size - 2]
    val arrow =
        when {
            delta < 0 -> "↓"
            delta > 0 -> "↑"
            else -> "→"
        }
    val magnitude = if (delta < 0) -delta else delta
    return "$arrow ${formatDecimal(magnitude, digits)} $unit"
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
