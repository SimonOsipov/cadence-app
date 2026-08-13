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
 * One serving of `chicken-bowl` (`mobile/src/features/recipe/data.ts:107-120`,
 * per-100g table at `:57-81`), servings halved for a single logged portion.
 * Each item's macros are the ingredient's per-100g value scaled to that item's
 * grams and rounded once to the nearest tenth — the storage precision
 * [MealItem.macros] carries — never further to a whole gram.
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
     * Exact sum then one rounding: the stored (tenth-rounded) items add to
     * 46.5 + 3.2 + 2.2 + 1.6 + 2.6 + 0 = 56.1 → 56. The mutation this kills is
     * rounding each position to whole grams before summing, which the spec's
     * step-2 text gives as 57 (47 + 3 + 2 + 2 + 3 + 0) — verified against the
     * prototype's per-100g table by hand before this test was written.
     */
    @Test
    fun chickenBowlOneServingProteinSumsPreciselyThenRoundsOnce() {
        val totals = chickenBowlOneServing().totals.toMacros()

        assertEquals(56, totals.proteinG)
    }

    /**
     * Two meals whose protein totals are each a tie at the whole-gram
     * boundary: 0,5 g and 0,5 g. Summed precisely first, the day is exactly
     * 1,0 g → 1. Rounded per meal first (0,5 → 1 each, round half up), the
     * day would read 2 — the mutation this test exists to catch.
     */
    @Test
    fun dayTotalsAcrossMealsSumPreciselyThenRoundOnce() {
        val mealA = meal("m-a", listOf(mealItem("A", 50, MacrosTenths(0, 5, 0, 0))))
        val mealB = meal("m-b", listOf(mealItem("B", 50, MacrosTenths(0, 5, 0, 0))))

        val dayTotals = listOf(mealA, mealB).map { it.totals }.sumTenths().toMacros()

        assertEquals(1, dayTotals.proteinG)
    }

    /**
     * «−10 г, then +10 г» from the same original — `rescaleMealItem` always
     * takes the base explicitly, so this is not testing luck: the scale
     * factor at the second call is exactly `original.grams / original.grams`,
     * which is one, for every field, every time.
     *
     * The 100 g → 90 g step is chosen so protein actually lands on a tie:
     * 1,5 g × 90/100 = 13,5 tenths exactly. A fixture that merely *differs*
     * from the original at 90 g would pass under a truncating `scaleRounded`
     * too (13 instead of 14) — asserting the intermediate's exact tenths is
     * what pins round-half-up here, not just "it moved".
     */
    @Test
    fun rescaleFromOriginalRoundTripsExactly() {
        val original = mealItem("Овсянка", 100, MacrosTenths(2000, 15, 400, 50))

        val steppedDown = rescaleMealItem(original, original.grams - 10)
        val steppedBackUp = rescaleMealItem(original, steppedDown.grams + 10)

        // 90/100 of (2000, 15, 400, 50): kcal and carbs and fat divide evenly;
        // protein's 13,5-tenths tie rounds up to 14, not down to 13.
        assertEquals(MacrosTenths(1800, 14, 360, 45), steppedDown.macros)
        assertEquals(original.grams, steppedBackUp.grams)
        assertEquals(original.macros, steppedBackUp.macros)
    }

    /**
     * `toMacros` is the one conversion boundary in the whole nutrition
     * context, and a tie there is not exercised by
     * [dayTotalsAcrossMealsSumPreciselyThenRoundOnce]: that test's tie is
     * consumed by the exact fold before it ever reaches [MacrosTenths.toMacros]
     * — two 0,5 g terms sum to an untied 1,0 g. This test puts the tie
     * directly at the boundary: 0,5 g rounds up to 1 g, not down to 0.
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
     * A negative target is not an error — the floor is a UI concern, and the
     * prototype's stepper puts it at 5 g (`LogMealScreen.tsx:855`,
     * `Math.max(5, item.grams - 10)`) — but it has to land somewhere defined
     * rather than somewhere merely untested: coerced to 0 g, with every field
     * at zero. This diverges from the prototype, which clamps grams and leaves
     * the macros untouched (`meal/data.ts:141`); zeroed macros are the honest
     * reading of a zero-gram portion, and nothing consumes the difference until
     * the inline stepper lands in step-5.
     */
    @Test
    fun rescaleMealItemCoercesNegativeGramsToZero() {
        val original = mealItem("Овсянка", 100, MacrosTenths(2000, 15, 400, 50))

        val rescaled = rescaleMealItem(original, -20)

        assertEquals(0, rescaled.grams)
        assertEquals(MacrosTenths.ZERO, rescaled.macros)
    }

    /**
     * «Ягодная смесь» (`mobile/src/features/meal/data.ts:84`): 80 g carries
     * exactly 1 g of protein. A 10 g step is a ratio of 90/80 = 1,125, so the
     * true value is 1,125 g — at tenths precision that rounds to 1,1 g and the
     * badge moves. At whole-gram precision the same edit rounds to 1 g and the
     * badge is frozen — the mutation this test exists to catch.
     */
    @Test
    fun tenGramStepMovesBerryMixProteinAtTenthsPrecision() {
        val original = mealItem("Ягодная смесь", 80, MacrosTenths(450, 10, 110, 0))

        val steppedUp = rescaleMealItem(original, original.grams + 10)

        assertEquals(11, steppedUp.macros.proteinGTenths)
        assertNotEquals(original.macros.proteinGTenths, steppedUp.macros.proteinGTenths)
    }
}
