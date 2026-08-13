package app.cadence.format

/*
 * Russian noun declension by count — split out of `CadenceFormat.kt` once
 * step 14 of the nutrition block added a fourth noun and the file reached
 * detekt's `TooManyFunctions` ceiling. Same reasoning as that file's own
 * header: hand-rolled because Kotlin/Native carries no ICU plural rules.
 */

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
fun pluralMeals(count: Int): String = russianPlural(count, "приём", "приёма", "приёмов")

/** «5 флаконов», «21 флакон» — the same rule, for the cabinet. */
fun pluralVials(count: Int): String = russianPlural(count, "флакон", "флакона", "флаконов")

/**
 * «неделю» / «недели» / «недель».
 *
 * The reorder card repeats the meal card's approximation in the prototype
 * (`weeksLeft < 5 ? 'недели' : 'недель'`) and is wrong from 21 up in the same
 * way. One rule serves both nouns.
 */
fun pluralWeeks(count: Int): String = russianPlural(count, "неделю", "недели", "недель")

/**
 * «позиция» / «позиции» / «позиций» — a logged meal's item count.
 *
 * `NutritionMealFeed.kt` and `LogMealItemsList.kt` already call
 * `russianPlural(…, "позиция", "позиции", "позиций")` inline at their own meal
 * and item rows; `TodayMeals`' recent-meal row is a third call site, named
 * here rather than repeated a third time.
 */
fun pluralItems(count: Int): String = russianPlural(count, "позиция", "позиции", "позиций")

/**
 * The rule itself, so the next noun does not copy a `when` block.
 *
 * The whole reason for fixing the prototype's version was «the next screen
 * counting something copies a rule that holds instead of one that happens to» —
 * which only works if the rule is callable.
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
