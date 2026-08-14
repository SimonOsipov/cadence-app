package app.cadence.screens.nutrition

import androidx.compose.foundation.layout.size
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
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
 * grams = 15, protein = 1,4 г (14 tenths). Chosen, not copied from the seed: scaling this item
 * down 10 g and back up (to the floor, 5 g, and back to 15) is the case where rescaling from the
 * *previous* current value (the prototype's bug, `meal/data.ts:140-151`) and rescaling from the
 * true original disagree on the **displayed** rounded gram — correct stays at 1 г throughout
 * (scale factor `15/15 == 1`), wrong rounds `5 * 15 / 5` to 2 г.
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

    // Merged tree: the tag sits on the clickable node that merges its label `BasicText` child
    // upward — same reason as the grams pill in `steppingGramsDownThenUpRestoresTheOriginalMacros`.
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
            // The parser would succeed on any call, so proving Photo/Voice never reach it is
            // what makes this fixture meaningful — an always-Unavailable parser would pass
            // even if the segments did call it.
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

    /**
     * "Уже разобранные позиции остаются правимыми" (spec line 295): the *edits* must survive a
     * failed retry, not merely the row count — `parseState.items` never changes on `Unavailable`,
     * so a row-count check alone can't tell a kept edit from a silent re-seed from the original.
     * Deleting a row before the failed retry and asserting *which* one remains is `generation`'s
     * only witness: `afterParseResult`'s `Unavailable` branch deliberately does not bump it, and
     * `loggedItems`/`editingIndex` are keyed on that generation for exactly this reason.
     */
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

            // An edit made to this attempt's parsed items, before the retry.
            deleteButtons()[0].performClick()
            waitForIdle()
            itemRows().assertCountEquals(1)

            // Second attempt on the same screen fails.
            parseAndAwait("другой текст")

            onNodeWithText("Не получилось разобрать — можно попробовать ещё раз.").assertExists()
            // A `generation` bump on the `Unavailable` branch would re-seed `loggedItems` from
            // `parseState.items` (still both original rows) and restore the deleted position.
            itemRows().assertCountEquals(1)
            onNodeWithTag(logMealItemKcalTag(0), useUnmergedTree = true).assertTextEquals("145 ккал")
            // The screen still responds to its own controls rather than being torn down.
            chatField().assertExists()
            onNodeWithContentDescription("Закрыть").performClick()
            waitForIdle()

            assertTrue(cancelled, "the close control stopped responding after a failed parse")
        }

    /**
     * Against "draw the total" (both rows would read 385, the rounded sum) and "draw a
     * constant" (both rows would read the same fabricated number). Also covers deletion
     * (`LogMealScreen.kt`'s `deleteItem`, previously unmeasured): the remaining row, header
     * count and footer total must all follow the smaller list.
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
            // `CadenceEyebrow` uppercases its own text (`CadenceText.kt:29`).
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
     * The four totals-strip columns against `TARGETS` — previously only checked by arithmetic
     * read by eye, so a transposed column (protein rendering fat's number) shipped silently.
     * 385/48/30/7 are the fixture's exact folded kcal/protein/carbs/fat, each read off
     * `logMealTotalTag`'s merged node, which also carries the column's Russian label — so a
     * label swap between two columns reddens the same assertion as a transposed number, not
     * just a lookup that could pass against the wrong column.
     */
    @Test
    fun totalsStripShowsFourColumnsAgainstTargets() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE)) }
            }

            parseAndAwait()

            onNodeWithTag(logMealTotalTag("kcal")).assertTextEquals("ККАЛ", "385", "/ ${formatInteger(TARGETS.kcal)}")
            onNodeWithTag(
                logMealTotalTag("protein"),
            ).assertTextEquals("БЕЛОК", "48", "/ ${formatInteger(TARGETS.proteinG)} г")
            onNodeWithTag(
                logMealTotalTag("carbs"),
            ).assertTextEquals("УГЛЕВ", "30", "/ ${formatInteger(TARGETS.carbsG)} г")
            onNodeWithTag(logMealTotalTag("fat")).assertTextEquals("ЖИРЫ", "7", "/ ${formatInteger(TARGETS.fatG)} г")
        }

    /**
     * The trap the step names explicitly: rescaling must always start from the position's
     * *parsed* values, not whatever the previous edit left behind — see [RESCALE_ITEM]'s KDoc
     * for why this item catches a base-on-current implementation. Also exercises the floor
     * itself: a second decrement at 5 г must not go lower — `LOG_MEAL_GRAM_FLOOR` had no test
     * distinguishing it from a floor of 0 before this.
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

            // `CadenceChip`'s text merges up onto its clickable tagged node (same reason
            // `onNodeWithText("Разобрать →").performClick()` works above unmerged), so the
            // grams pill's number is read off the merged tree.
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
     * Reproduces the round-1 defect: re-parsing the *same* text after deleting every position
     * must still repopulate the list. Keying editable state on `parseState.items` instead of a
     * per-attempt generation left the screen a dead end here — empty list, stuck footer, with
     * mode-switching the only escape.
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
     * Recorded divergence from the prototype (`LogMealScreen.tsx:476-507` starts a parse on an
     * empty field and substitutes breakfast): the guard has two halves — the button's `enabled`
     * and `runParse`'s early return — and only their conjunction was pinned before this.
     * `assertIsNotEnabled` pins `enabled` on its own, via the disabled `CadenceButton`'s own
     * semantics node (`CadenceControls.kt:136`), so a mutation dropping `enabled` alone (a
     * button a tap can't reach, but visually identical) reddens here even though the click
     * below never fires.
     */
    @Test
    fun anEmptyFieldDoesNotStartAParse() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS, parse = queuedParser(LUNCH_PARSE)) }
            }

            // No text typed — the field starts and stays empty.
            onNodeWithText("Разобрать →").assertIsNotEnabled()
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
     * `headerChipText`'s own reason for existing: every part reads from the clock, none of it
     * the prototype's literal «08:42 · вс 24 мая». Nothing asserted the rendered string before
     * this — replacing the whole function body with that literal left every other test green.
     */
    @Test
    fun headerChipReadsTheClockNotALiteral() =
        runComposeUiTest {
            setContent {
                CadenceTheme { LogMealScreen(now = NOW, targets = TARGETS) }
            }

            onNodeWithText("08:05 · Вс 31 мая").assertExists()
        }

    /**
     * The prototype pins «Сохранить» to the bottom over a gradient and reserves 130dp of
     * scroll for it (`LogMealScreen.tsx:97-99,344-394`); drawn in flow instead, it sits
     * below the fold as soon as a parse fills the list, and the screen's own primary action
     * has to be hunted for.
     *
     * Measured on `positionInRoot + size`, never `boundsInRoot`: bounds are clipped by every
     * parent, so «the footer ends inside the screen» written with them is an identity no
     * layout can fail. The window is constrained to a phone, or the harness's own 711dp of
     * height hides the defect.
     */
    @Test
    fun theSaveFooterStaysOnScreenOnceTheParseFillsTheList() =
        runComposeUiTest {
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    LogMealScreen(
                        now = NOW,
                        targets = TARGETS,
                        parse = queuedParser(LUNCH_PARSE),
                        modifier = Modifier.size(PHONE_WIDTH, PHONE_HEIGHT),
                    )
                }
            }

            parseAndAwait()

            val footer = saveFooter().fetchSemanticsNode()
            val bottom = with(density) { (footer.positionInRoot.y + footer.size.height).toDp() }

            assertTrue(
                bottom <= PHONE_HEIGHT,
                "the save footer ends at $bottom on a $PHONE_HEIGHT screen — it is below the fold",
            )
        }
}

/**
 * The column ends in a spacer of [app.cadence.screens.nutrition.FOOTER_CLEARANCE]; a bar
 * taller than that hides its own last content instead of clearing it. `size`, not
 * `boundsInRoot`: a clipped height can only ever make this pass.
 */
@OptIn(ExperimentalTestApi::class)
class LogMealFooterClearanceTest {
    @Test
    fun theSaveFooterFitsInsideItsOwnClearance() =
        runComposeUiTest {
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    LogMealScreen(now = NOW, targets = TARGETS, modifier = Modifier.size(PHONE_WIDTH, PHONE_HEIGHT))
                }
            }

            val bar = onNodeWithTag(LOG_MEAL_SAVE_TAG).fetchSemanticsNode()
            val height = with(density) { bar.size.height.toDp() }

            assertTrue(
                height <= FOOTER_CLEARANCE,
                "the save bar is $height tall but the column only clears $FOOTER_CLEARANCE",
            )
        }
}

/** An iPhone SE's own box: the narrowest and shortest phone this app targets. */
private val PHONE_WIDTH = 375.dp
private val PHONE_HEIGHT = 667.dp
