package app.cadence.screens.nutrition

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CADENCE_STEPPER_MINUS_TAG
import app.cadence.design.CADENCE_STEPPER_PLUS_TAG
import app.cadence.design.CadenceTheme
import app.cadence.format.formatInteger
import app.cadence.shared.domain.Macros
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.domain.MealItem
import app.cadence.shared.domain.MealSource
import app.cadence.shared.parsing.MealParseResult
import app.cadence.shared.parsing.mealSamplePrompts
import kotlinx.datetime.LocalDateTime
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val NOW = LocalDateTime(2026, 5, 31, 8, 5)

/** `MockSeed.targets` — the same 1800/140/200/60 the spec puts in exactly one place. */
private val TARGETS = Macros(kcal = 1800, proteinG = 140, carbsG = 200, fatG = 60)

private val LUNCH_ITEMS =
    listOf(
        MealItem("Куриная грудка", 150, MacrosTenths(2400, 450, 0, 60)),
        MealItem("Бурый рис", 120, MacrosTenths(1450, 30, 300, 10)),
    )

private val LUNCH_PARSE =
    MealParseResult.Parsed(mealName = "Обед", transcript = "Курица с рисом.", items = LUNCH_ITEMS)

/**
 * grams = 15, protein = 1,4 г (14 tenths). Chosen, not copied from the seed:
 * scaling this item down 10 g and back up is the case where rescaling from
 * the *previous* current value (the prototype's bug, `meal/data.ts:140-151`)
 * and rescaling from the true original disagree on the **displayed**, rounded
 * whole gram — not merely on an internal tenths digit no test could see.
 *
 * −10 g lands exactly on the floor (15 − 10 = 5), and +10 g returns to 15:
 *   - correct (`rescaleMealItem(original, …)` both times): 14 tenths → 1 г
 *     both before and after, because the up-edit's scale factor is
 *     `15 / 15 == 1` regardless of what happened in between.
 *   - wrong (scaling from the −10 result, 5 g / 5 tenths, back up to 15 g):
 *     `5 * 15 / 5` rounds to 15 tenths → displays as 2 г, not 1 г.
 */
private val RESCALE_ITEM = MealItem("Тестовый гарнир", 15, MacrosTenths(100, 14, 0, 0))
private val RESCALE_PARSE =
    MealParseResult.Parsed(mealName = "Перекус", transcript = "Гарнир.", items = listOf(RESCALE_ITEM))

/** A parser stub: hands back a fixed queue of answers, one per call, in order. */
private fun queuedParser(vararg answers: MealParseResult): suspend (String) -> MealParseResult {
    var next = 0
    return {
        val answer = answers[next.coerceAtMost(answers.size - 1)]
        next++
        answer
    }
}

@OptIn(ExperimentalTestApi::class)
class LogMealScreenTest {
    private fun ComposeUiTest.chatField() = onNodeWithTag(LOG_MEAL_CHAT_FIELD_TAG, useUnmergedTree = true)

    private fun ComposeUiTest.itemRows() = onAllNodesWithTag(LOG_MEAL_ITEM_ROW_TAG, useUnmergedTree = true)

    private fun ComposeUiTest.itemsHeader() = onNodeWithTag(LOG_MEAL_ITEMS_HEADER_TAG, useUnmergedTree = true)

    private fun ComposeUiTest.deleteButtons() = onAllNodesWithContentDescription("Удалить позицию")

    // Merged tree, not unmerged: the tag sits on the same clickable node that
    // merges its own label `BasicText` child upward, the same reason the
    // grams pill is read this way in `steppingGramsDownThenUpRestoresTheOriginalMacros`.
    private fun ComposeUiTest.saveFooter() = onNodeWithTag(LOG_MEAL_SAVE_TAG)

    private fun ComposeUiTest.parseAndAwait(text: String = "курица с рисом") {
        chatField().performTextReplacement(text)
        waitForIdle()
        onNodeWithText("Разобрать →").performClick()
        waitForIdle()
    }

    @Test
    fun changingModeErasesWhatWasParsed() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE)) }
            }

            parseAndAwait()

            itemsHeader().assertExists()
            onNodeWithText("Услышали").assertExists()

            onNodeWithText("Фото · скоро").performClick()
            waitForIdle()

            itemsHeader().assertDoesNotExist()
            onNodeWithText("Услышали").assertDoesNotExist()
        }

    @Test
    fun photoAndVoiceProduceNoItems() =
        runComposeUiTest {
            // The parser would succeed on any call — proving Photo/Voice never
            // reach it is what makes this fixture meaningful; a parser that
            // always answers Unavailable would pass even if the segments did
            // call it.
            setContent {
                CadenceTheme {
                    LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE, LUNCH_PARSE))
                }
            }

            onNodeWithText("Фото · скоро").performClick()
            waitForIdle()

            itemsHeader().assertDoesNotExist()
            onNodeWithText("Распознавание снимка пока не работает — опишите еду текстом").assertExists()

            onNodeWithText("Голос · скоро").performClick()
            waitForIdle()

            itemsHeader().assertDoesNotExist()
            onNodeWithText("Распознавание голоса пока не работает — опишите еду текстом").assertExists()
        }

    @Test
    fun unavailableLeavesTheScreenAliveAndWhatWasParsedStillEditable() =
        runComposeUiTest {
            var cancelled = false
            setContent {
                CadenceTheme {
                    LogMealScreen(
                        now = NOW,
                        targets = TARGETS,
                        parse = queuedParser(LUNCH_PARSE, MealParseResult.Unavailable),
                        onCancel = { cancelled = true },
                    )
                }
            }

            parseAndAwait()
            itemRows().assertCountEquals(LUNCH_ITEMS.size)

            // Second attempt on the same screen fails.
            parseAndAwait("другой текст")

            onNodeWithText("Не получилось разобрать — можно попробовать ещё раз.").assertExists()
            // The failed attempt did not touch what the first attempt parsed.
            itemRows().assertCountEquals(LUNCH_ITEMS.size)
            // The field is still there to retry from, and the screen still
            // responds to its own controls rather than being torn down.
            chatField().assertExists()
            onNodeWithContentDescription("Закрыть").performClick()
            waitForIdle()

            assertTrue(cancelled, "the close control stopped responding after a failed parse")
        }

    /**
     * Against "draw the total" (both rows would read 385 — `2 400 + 1 450`
     * tenths kcal, rounded) and "draw a constant" (both rows would read the
     * same fabricated number regardless of which item they belong to).
     *
     * Extended to cover deletion (`LogMealScreen.kt`'s `deleteItem`, unmeasured
     * before this round): removing one row must leave the *other* item behind
     * — not a blank slot and not a stale kcal figure — and the header count
     * and footer total must follow the smaller list.
     */
    @Test
    fun twoItemsWithDifferentKcalDrawTwoDifferentNumbers() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE)) }
            }

            parseAndAwait()

            onNodeWithTag(logMealItemKcalTag(0), useUnmergedTree = true).assertTextEquals("240 ккал")
            onNodeWithTag(logMealItemKcalTag(1), useUnmergedTree = true).assertTextEquals("145 ккал")

            deleteButtons()[0].performClick()
            waitForIdle()

            itemRows().assertCountEquals(1)
            onNodeWithTag(logMealItemKcalTag(0), useUnmergedTree = true).assertTextEquals("145 ккал")
            // CadenceEyebrow uppercases its own text (`CadenceText.kt:29`).
            onNodeWithText("ОБЕД · 1 ПОЗИЦИЯ").assertExists()
            saveFooter().assertTextEquals("Сохранить · 145 ккал")
        }

    @Test
    fun savingWritesAiTextSource() =
        runComposeUiTest {
            var saved: MealDraft? = null
            setContent {
                CadenceTheme {
                    LogMealScreen(
                        now = NOW,
                        targets = TARGETS,
                        parse = queuedParser(LUNCH_PARSE),
                        onSave = { saved = it },
                    )
                }
            }

            parseAndAwait()
            saveFooter().performClick()
            waitForIdle()

            assertEquals(MealSource.AI_TEXT, saved?.source)
            assertEquals("Обед", saved?.name)
            assertEquals(LUNCH_ITEMS, saved?.items)
        }

    /**
     * The four totals-strip columns against `TARGETS` — its only witness
     * before this round was arithmetic read by eye, and a transposed column
     * (protein rendering fat's number, say) would have shipped silently.
     * 385/48/30/7 are the two-item lunch fixture's exact folded kcal / protein
     * / carbs / fat, each read off `logMealTotalTag`'s own merged node so a
     * value and its caption are asserted together, not two separate lookups
     * that could each pass against the wrong column.
     */
    @Test
    fun totalsStripShowsFourColumnsAgainstTargets() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE)) }
            }

            parseAndAwait()

            onNodeWithTag(logMealTotalTag("kcal")).assertTextEquals("385", "/ ${formatInteger(TARGETS.kcal)}")
            onNodeWithTag(
                logMealTotalTag("protein"),
            ).assertTextEquals("48", "/ ${formatInteger(TARGETS.proteinG)} г")
            onNodeWithTag(logMealTotalTag("carbs")).assertTextEquals("30", "/ ${formatInteger(TARGETS.carbsG)} г")
            onNodeWithTag(logMealTotalTag("fat")).assertTextEquals("7", "/ ${formatInteger(TARGETS.fatG)} г")
        }

    /**
     * The trap the step names explicitly: rescaling must always start from
     * the position's *parsed* values, never from whatever the previous edit
     * left behind. −10 г lands exactly on the floor; +10 г returns to the
     * start — see [RESCALE_ITEM]'s own KDoc for why this specific item
     * catches a base-on-current implementation and a base-on-original one
     * would not both pass it. Also exercises the floor itself: a second
     * decrement at 5 г must not go lower — `LOG_MEAL_GRAM_FLOOR` had no test
     * that could tell it apart from a floor of 0 before this round.
     */
    @Test
    fun steppingGramsDownThenUpRestoresTheOriginalMacros() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(RESCALE_PARSE)) }
            }

            parseAndAwait("гарнир")

            val proteinBadge = onNodeWithText("Белок 1 г", substring = false)
            proteinBadge.assertExists()

            // `CadenceChip`'s own text merges up onto its (clickable) tagged
            // node — the same reason `onNodeWithText("Разобрать →").performClick()`
            // works above without `useUnmergedTree` — so the grams pill's
            // number is read off the merged tree, not the raw one.
            onNodeWithTag(logMealItemGramsTag(0)).performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_STEPPER_MINUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            onNodeWithTag(logMealItemGramsTag(0)).assertTextEquals("5 г")

            // The floor holds: a second decrement at 5 г does not go lower.
            onNodeWithTag(CADENCE_STEPPER_MINUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()
            onNodeWithTag(logMealItemGramsTag(0)).assertTextEquals("5 г")

            onNodeWithTag(CADENCE_STEPPER_PLUS_TAG, useUnmergedTree = true).performClick()
            waitForIdle()

            onNodeWithTag(logMealItemGramsTag(0)).assertTextEquals("15 г")
            onNodeWithText("Белок 1 г", substring = false).assertExists()
        }

    /**
     * Reproduces the round-1 defect exactly: parsing, deleting every
     * position, and then re-parsing the *same* text (a structurally equal
     * `MealItem` list) must still repopulate the list. Keying the editable
     * state on `parseState.items` instead of a per-attempt generation left
     * the screen a dead end here — the list stayed empty and the footer
     * stuck on «Добавьте что-нибудь» — with mode-switching the only escape.
     */
    @Test
    fun reParsingTheSameTextAfterDeletingEverythingRepopulatesTheList() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE, LUNCH_PARSE))
                }
            }

            parseAndAwait()
            itemRows().assertCountEquals(LUNCH_ITEMS.size)

            deleteButtons()[0].performClick()
            waitForIdle()
            deleteButtons()[0].performClick()
            waitForIdle()
            itemRows().assertCountEquals(0)
            saveFooter().assertTextEquals("Добавьте что-нибудь")

            parseAndAwait()

            itemRows().assertCountEquals(LUNCH_ITEMS.size)
            saveFooter().assertTextEquals("Сохранить · 385 ккал")
        }

    /**
     * The recorded divergence from the prototype (`LogMealScreen.tsx:476-507`
     * starts a parse on an empty field and substitutes breakfast): both
     * halves of the guard — the button's own `enabled` and `runParse`'s own
     * early return — had nothing pinning them before this round.
     */
    @Test
    fun anEmptyFieldDoesNotStartAParse() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE)) }
            }

            // No text typed — the field starts and stays empty.
            onNodeWithText("Разобрать →").performClick()
            waitForIdle()

            itemsHeader().assertDoesNotExist()
            saveFooter().assertTextEquals("Добавьте что-нибудь")
        }

    /** «Пример» must write the transcript into the field, not just cycle the placeholder. */
    @Test
    fun cyclingTheSampleFillsTheFieldWithItsTranscript() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS) }
            }

            onNodeWithText("Пример").performClick()
            waitForIdle()

            chatField().assertTextEquals(mealSamplePrompts()[1].transcript)
        }

    /**
     * The header chip's own reason for existing (`headerChipText`'s KDoc):
     * every part read from the clock, none of it the prototype's literal
     * «08:42 · вс 24 мая». Nothing asserted the rendered string before this
     * round — replacing the whole function body with that literal left every
     * other test green.
     */
    @Test
    fun headerChipReadsTheClockNotALiteral() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS) }
            }

            onNodeWithText("08:05 · Вс 31 мая").assertExists()
        }
}
