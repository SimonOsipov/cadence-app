package app.cadence.shared.parsing

import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealItem
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs

/**
 * Trigger-word inputs below are the prototype's own placeholder text
 * (`mobile/src/features/meal/data.ts:79,93,107`), not the bare label a
 * canned parse is named after: the id `'lunch-bowl'`'s trigger list has
 * `bowl` (Latin) and `рис`/`капуста`/`тахини` (Cyrillic), not the Cyrillic
 * word `боул` itself, so a lone `"боул"` would score zero everywhere and
 * fall through to breakfast rather than lunch. The full placeholder phrase
 * carries `рис` (inside `рисом`) and `тахини`, which is what actually wins
 * the lunch parse under `pickSampleForInput`'s literal scoring
 * (`meal/data.ts:119-127`).
 */
class MockMealParserTest {
    @Test
    fun snackKeywordTvorogGivesTheSnackParse() =
        runTest {
            // meal/data.ts:107 — the snack sample's own placeholder text.
            val result = MockMealParser().parse("«Творог с яблоком»")

            val parsed = assertIs<MealParseResult.Parsed>(result)
            assertEquals("Перекус", parsed.mealName)
            assertEquals(
                "Творог с нарезанным яблоком и каплей миндальной пасты.",
                parsed.transcript,
            )
            assertEquals(
                listOf(
                    MealItem("Творог", 150, MacrosTenths(1450, 210, 60, 40)),
                    MealItem("Яблоко", 180, MacrosTenths(950, 0, 250, 0)),
                    MealItem("Миндальная паста", 15, MacrosTenths(950, 30, 30, 80)),
                ),
                parsed.items,
            )
        }

    @Test
    fun lunchKeywordBowlGivesTheLunchParse() =
        runTest {
            // meal/data.ts:93 — the lunch-bowl sample's own placeholder text,
            // matching on рис/тахини, not on the Cyrillic word боул itself.
            val result = MockMealParser().parse("«Куриная боул-чашка с рисом, капустой и тахини»")

            val parsed = assertIs<MealParseResult.Parsed>(result)
            assertEquals("Обед", parsed.mealName)
            assertEquals(
                "Боул с куриной грудкой: бурый рис, кейл, печёный батат и капля тахини.",
                parsed.transcript,
            )
            assertEquals(
                listOf(
                    MealItem("Куриная грудка", 150, MacrosTenths(2400, 450, 0, 60)),
                    MealItem("Бурый рис", 120, MacrosTenths(1450, 30, 300, 10)),
                    MealItem("Кейл с заправкой", 80, MacrosTenths(600, 30, 60, 30)),
                    MealItem("Батат", 100, MacrosTenths(900, 20, 210, 0)),
                    MealItem("Тахини", 15, MacrosTenths(900, 30, 30, 80)),
                ),
                parsed.items,
            )
        }

    @Test
    fun unknownTextFallsBackToTheBreakfastParse() =
        runTest {
            val result = MockMealParser().parse("Привет, как прошёл день?")

            val parsed = assertIs<MealParseResult.Parsed>(result)
            assertEquals("Завтрак", parsed.mealName)
            assertEquals(
                "Греческий йогурт с ягодами, два варёных яйца и кусок ржаного хлеба с авокадо.",
                parsed.transcript,
            )
            assertEquals(
                listOf(
                    MealItem("Греческий йогурт", 150, MacrosTenths(1300, 170, 60, 40)),
                    MealItem("Ягодная смесь", 80, MacrosTenths(450, 10, 110, 0)),
                    MealItem("Варёные яйца", 100, MacrosTenths(1550, 130, 10, 110)),
                    MealItem("Ржаной хлеб", 40, MacrosTenths(1100, 40, 220, 10)),
                    MealItem("Авокадо", 60, MacrosTenths(960, 10, 50, 90)),
                ),
                parsed.items,
            )
        }

    @Test
    fun tiedZeroScoresBreakToTheFirstEntryNotTheLastOne() =
        runTest {
            // Every trigger scores zero against this text, so the winner comes
            // entirely from the tie-break rule: `score > bestScore`, seeded at
            // -1, replaces only on a strictly higher score. Breakfast is
            // checked first and wins every all-zero tie; a `>=` mutant would
            // keep replacing through the whole list and land on snack, the
            // last entry, instead.
            val result = MockMealParser().parse("совершенно нейтральный текст без единого совпадения")

            val parsed = assertIs<MealParseResult.Parsed>(result)
            assertEquals("Завтрак", parsed.mealName)
        }

    @Test
    fun matchingIsCaseInsensitive() =
        runTest {
            val result = MockMealParser().parse("ТВОРОГ С ЯБЛОКОМ")

            val parsed = assertIs<MealParseResult.Parsed>(result)
            assertEquals("Перекус", parsed.mealName)
        }

    @Test
    fun blankTextDoesNotStartAParse() =
        runTest {
            assertEquals(MealParseResult.Unavailable, MockMealParser().parse(""))
            assertEquals(MealParseResult.Unavailable, MockMealParser().parse("   "))
        }

    @Test
    fun theUnavailableModeNeverReturnsACannedParseEvenForMatchingText() =
        runTest {
            val parser = MockMealParser(available = false)

            val result = parser.parse("«Творог с яблоком»")

            assertEquals(MealParseResult.Unavailable, result)
        }

    @Test
    fun cannedParsesCarryThePrototypesPlaceholdersVerbatim() {
        // meal/data.ts:79,93,107 — ported whole, not just the fields `parse`
        // hands back, so a step-5 example-cycling control has real text to
        // read rather than reconstructing it from the transcript.
        assertEquals(
            listOf(
                "«Греческий йогурт с ягодами, два варёных яйца и тост с авокадо»",
                "«Куриная боул-чашка с рисом, капустой и тахини»",
                "«Творог с яблоком»",
            ),
            CANNED_MEAL_PARSES.map { it.placeholder },
        )
    }

    @Test
    fun cannedParsesCarryThePrototypesTriggerWordsVerbatim() {
        // meal/data.ts:78,92,106 — the actual scoring input; a corrupted
        // trigger silently resolves real text ("chicken bowl", "yogurt") to
        // the wrong canned meal without any test noticing, since every other
        // fixture here only exercises the winning entry's own triggers.
        assertEquals(
            listOf(
                listOf(
                    "yogurt",
                    "eggs",
                    "sourdough",
                    "avocado",
                    "berries",
                    "oats",
                    "oatmeal",
                    "йогурт",
                    "яйц",
                    "хлеб",
                    "авокадо",
                    "ягод",
                ),
                listOf(
                    "bowl",
                    "chicken",
                    "rice",
                    "salmon",
                    "quinoa",
                    "salad",
                    "lunch",
                    "обед",
                    "курица",
                    "рис",
                    "батат",
                    "тахини",
                    "капуста",
                ),
                listOf(
                    "snack",
                    "cottage",
                    "almond",
                    "apple",
                    "peanut",
                    "перекус",
                    "творог",
                    "яблок",
                    "миндаль",
                ),
            ),
            CANNED_MEAL_PARSES.map { it.triggerWords },
        )
    }

    @Test
    fun theSamplePromptsCarryOnlyThePlaceholderAndTranscriptInOrder() {
        // The step-5 accessor's whole point: composeApp reads placeholder and
        // transcript without seeing triggerWords, mealName or items.
        assertEquals(
            CANNED_MEAL_PARSES.map { MealSamplePrompt(it.placeholder, it.transcript) },
            mealSamplePrompts(),
        )
    }

    @Test
    fun aContestedScoreEndsWithTheHigherScoringParseNotTheFirstMatch() =
        runTest {
            // "йогурт" scores breakfast 1; "рис" (inside "рисом") and "тахини"
            // score lunch-bowl 2; snack scores 0. Counting, not existence, is
            // what `pickSampleForInput` actually does (meal/data.ts:122-124):
            // `triggerWords.reduce((acc, w) => acc + (t.includes(w) ? 1 : 0), 0)`,
            // one point per matching trigger, not one point for having any.
            // An `any`-based mutant would tie breakfast and lunch-bowl at 1
            // each and, checked first, land on breakfast — this input asserts
            // Обед and so catches that. A `>=` tie-break mutant does not
            // change this particular result (2 still beats 1 either way) —
            // `tiedZeroScoresBreakToTheFirstEntryNotTheLastOne` above is what
            // catches that mutant instead.
            val result = MockMealParser().parse("йогурт с рисом и тахини")

            val parsed = assertIs<MealParseResult.Parsed>(result)
            assertEquals("Обед", parsed.mealName)
        }
}
