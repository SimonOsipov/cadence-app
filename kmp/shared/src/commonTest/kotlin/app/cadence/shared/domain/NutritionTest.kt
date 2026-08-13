package app.cadence.shared.domain

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotEquals
import kotlin.time.Instant

private val PATIENT = UserId("p-1")
private val EATEN_AT = Instant.parse("2026-05-31T12:00:00Z")

private fun mealItem(
    name: String,
    grams: Int,
    macros: MacrosTenths,
) = MealItem(name = name, grams = grams, macros = macros)

private fun meal(
    id: String,
    items: List<MealItem>,
) = Meal(
    id = MealId(id),
    patientId = PATIENT,
    eatenAt = EATEN_AT,
    name = "Обед",
    source = MealSource.AI_TEXT,
    recipeId = null,
    items = items,
)

/**
 * One serving of `chicken-bowl`, servings halved for a single logged portion. Each item's
 * macros are the per-100g value scaled to that item's grams, rounded once to the nearest
 * tenth — [MealItem.macros]'s storage precision — never further to a whole gram.
 */
private fun chickenBowlOneServing(): Meal =
    meal(
        "chicken-bowl-serving",
        listOf(
            // chicken 150 g: 31 g/100g protein → 46.5 g exactly.
            mealItem("Куриная грудка", 150, MacrosTenths(2475, 465, 0, 54)),
            // rice 120 g: 2.7 g/100g protein → 3.24 g → nearest tenth 3.2 g.
            mealItem("Бурый рис", 120, MacrosTenths(1476, 32, 300, 12)),
            // broccoli 80 g: 2.8 g/100g protein → 2.24 g → nearest tenth 2.2 g.
            mealItem("Брокколи", 80, MacrosTenths(272, 22, 56, 3)),
            // sweetpot 100 g: 1.6 g/100g protein → 1.6 g exactly.
            mealItem("Батат", 100, MacrosTenths(860, 16, 200, 1)),
            // tahini 15 g: 17 g/100g protein → 2.55 g → tie, rounds up to 2.6 g.
            mealItem("Тахини", 15, MacrosTenths(893, 26, 32, 81)),
            // oliveoil 10 g halved to 5 g: no protein at all.
            mealItem("Оливковое масло", 5, MacrosTenths(442, 0, 0, 50)),
        ),
    )

class NutritionTest {
    /**
     * Exact sum then one rounding: 46.5+3.2+2.2+1.6+2.6+0 = 56.1 → 56. Kills the mutation
     * of rounding each item to whole grams before summing, which gives 57 (47+3+2+2+3+0).
     */
    @Test
    fun chickenBowlOneServingProteinSumsPreciselyThenRoundsOnce() {
        val totals = chickenBowlOneServing().totals.toMacros()

        assertEquals(56, totals.proteinG)
    }

    /**
     * Two meals each tied at 0,5g protein: summed precisely first, the day is exactly 1,0g
     * → 1. Rounded per meal first (0,5 → 1 each), the day would read 2.
     */
    @Test
    fun dayTotalsAcrossMealsSumPreciselyThenRoundOnce() {
        val mealA = meal("m-a", listOf(mealItem("A", 50, MacrosTenths(0, 5, 0, 0))))
        val mealB = meal("m-b", listOf(mealItem("B", 50, MacrosTenths(0, 5, 0, 0))))

        val dayTotals = listOf(mealA, mealB).map { it.totals }.sumTenths().toMacros()

        assertEquals(1, dayTotals.proteinG)
    }

    /**
     * The 100g → 90g step is chosen so protein lands on a tie: 1,5g × 90/100 = 13,5 tenths
     * exactly. A fixture that merely *differs* at 90g would pass under a truncating
     * `scaleRounded` too (13 instead of 14) — asserting the exact tenths pins round-half-up.
     */
    @Test
    fun rescaleFromOriginalRoundTripsExactly() {
        val original = mealItem("Овсянка", 100, MacrosTenths(2000, 15, 400, 50))

        val steppedDown = rescaleMealItem(original, original.grams - 10)
        val steppedBackUp = rescaleMealItem(original, steppedDown.grams + 10)

        // 90/100 of (2000, 15, 400, 50): protein's 13,5-tenths tie rounds up to 14.
        assertEquals(MacrosTenths(1800, 14, 360, 45), steppedDown.macros)
        assertEquals(original.grams, steppedBackUp.grams)
        assertEquals(original.macros, steppedBackUp.macros)
    }

    /**
     * [dayTotalsAcrossMealsSumPreciselyThenRoundOnce]'s tie is consumed by the exact fold
     * before it reaches [MacrosTenths.toMacros] — two 0,5g terms sum to an untied 1,0g. This
     * puts the tie directly at the boundary.
     */
    @Test
    fun toMacrosRoundsHalfUpAtExactTie() {
        val halfGram = MacrosTenths(0, 5, 0, 0)

        assertEquals(1, halfGram.toMacros().proteinG)
    }

    /** A rate off a zero base is unsatisfiable, so it is refused rather than silently computed. */
    @Test
    fun rescaleMealItemRejectsZeroGramOriginal() {
        val zeroGramOriginal = mealItem("Пустая позиция", 0, MacrosTenths(0, 0, 0, 0))

        assertFailsWith<IllegalArgumentException> { rescaleMealItem(zeroGramOriginal, 50) }
    }

    /**
     * A negative target isn't an error — the floor is a UI concern — but must land somewhere
     * defined: coerced to 0g, every field zero. Diverges from the prototype, which clamps
     * grams but leaves macros untouched; zeroed macros are the honest reading of a
     * zero-gram portion.
     */
    @Test
    fun rescaleMealItemCoercesNegativeGramsToZero() {
        val original = mealItem("Овсянка", 100, MacrosTenths(2000, 15, 400, 50))

        val rescaled = rescaleMealItem(original, -20)

        assertEquals(0, rescaled.grams)
        assertEquals(MacrosTenths.ZERO, rescaled.macros)
    }

    /**
     * 80g carries exactly 1g protein; a 10g step is ratio 90/80 = 1,125. At tenths precision
     * that rounds to 1,1g and the badge moves; at whole-gram precision it rounds to 1g and freezes.
     */
    @Test
    fun tenGramStepMovesBerryMixProteinAtTenthsPrecision() {
        val original = mealItem("Ягодная смесь", 80, MacrosTenths(450, 10, 110, 0))

        val steppedUp = rescaleMealItem(original, original.grams + 10)

        assertEquals(11, steppedUp.macros.proteinGTenths)
        assertNotEquals(original.macros.proteinGTenths, steppedUp.macros.proteinGTenths)
    }
}
