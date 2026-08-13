package app.cadence.format

/*
 * Russian noun declension by count — split out of `CadenceFormat.kt` once step 14
 * added a fourth noun and tripped detekt's `TooManyFunctions`. Hand-rolled for the
 * same reason as that file: no ICU plural rules on Kotlin/Native.
 */

private const val TEEN_FLOOR = 11
private const val TEEN_CEILING = 14
private const val FEW_FLOOR = 2
private const val FEW_CEILING = 4
private const val TENS = 100
private const val UNITS = 10

/**
 * «приём» / «приёма» / «приёмов». The prototype's `count === 1 ? 'приём' : count < 5
 * ? 'приёма' : 'приёмов'` is right through 20 and wrong from 21 up; every count the
 * sheet reaches today falls where the two agree, so this changes nothing on screen —
 * only that the next counting screen copies a rule that holds, not one that happens to.
 */
fun pluralMeals(count: Int): String = russianPlural(count, "приём", "приёма", "приёмов")

/** «5 флаконов», «21 флакон» — the same rule, for the cabinet. */
fun pluralVials(count: Int): String = russianPlural(count, "флакон", "флакона", "флаконов")

/**
 * «неделю» / «недели» / «недель». The reorder card repeats the meal card's
 * approximation in the prototype (`weeksLeft < 5 ? 'недели' : 'недель'`), wrong
 * from 21 up the same way — one rule serves both nouns.
 */
fun pluralWeeks(count: Int): String = russianPlural(count, "неделю", "недели", "недель")

/**
 * «позиция» / «позиции» / «позиций» — a logged meal's item count.
 * `NutritionMealFeed.kt`'s meal row, `LogMealItemsList.kt`'s header and
 * `TodayMeals`' recent-meal row all read this rather than three unlinked inline
 * copies of `russianPlural(…, "позиция", "позиции", "позиций")`.
 */
fun pluralItems(count: Int): String = russianPlural(count, "позиция", "позиции", "позиций")

/**
 * The rule itself, so the next noun does not copy a `when` block — the whole point
 * of fixing the prototype's version was a rule callers could call rather than copy.
 */
fun russianPlural(
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
