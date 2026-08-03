package app.cadence.format

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
fun pluralMeals(count: Int): String {
    val n = if (count < 0) -count else count
    val lastTwo = n % TENS
    val last = n % UNITS

    return when {
        lastTwo in TEEN_FLOOR..TEEN_CEILING -> "приёмов"
        last == 1 -> "приём"
        last in FEW_FLOOR..FEW_CEILING -> "приёма"
        else -> "приёмов"
    }
}
