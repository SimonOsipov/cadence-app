package app.cadence.screens.nutrition

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.MacrosTenths
import app.cadence.shared.domain.MealItem
import app.cadence.shared.parsing.MealParseResult
import kotlinx.datetime.LocalDateTime
import kotlin.test.Test
import kotlin.test.assertTrue

private val NOW = LocalDateTime(2026, 5, 31, 8, 5)

private val LUNCH_ITEMS =
    listOf(
        MealItem("Куриная грудка", 150, MacrosTenths(2400, 450, 0, 60)),
        MealItem("Бурый рис", 120, MacrosTenths(1450, 30, 300, 10)),
    )

private val LUNCH_PARSE =
    MealParseResult.Parsed(mealName = "Обед", transcript = "Курица с рисом.", items = LUNCH_ITEMS)

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

    private fun ComposeUiTest.itemsStub() = onNodeWithTag(LOG_MEAL_ITEMS_STUB_TAG, useUnmergedTree = true)

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
                CadenceTheme { LogMealScreen(now = NOW, parse = queuedParser(LUNCH_PARSE)) }
            }

            parseAndAwait()

            itemsStub().assertExists()
            onNodeWithText("Услышали").assertExists()

            onNodeWithText("Фото · скоро").performClick()
            waitForIdle()

            itemsStub().assertDoesNotExist()
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
                CadenceTheme { LogMealScreen(now = NOW, parse = queuedParser(LUNCH_PARSE, LUNCH_PARSE)) }
            }

            onNodeWithText("Фото · скоро").performClick()
            waitForIdle()

            itemsStub().assertDoesNotExist()
            onNodeWithText("Распознавание снимка пока не работает — опишите еду текстом").assertExists()

            onNodeWithText("Голос · скоро").performClick()
            waitForIdle()

            itemsStub().assertDoesNotExist()
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
                        parse = queuedParser(LUNCH_PARSE, MealParseResult.Unavailable),
                        onCancel = { cancelled = true },
                    )
                }
            }

            parseAndAwait()
            itemsStub().assertTextEquals(LUNCH_ITEMS.size.toString())

            // Second attempt on the same screen fails.
            parseAndAwait("другой текст")

            onNodeWithText("Не получилось разобрать — можно попробовать ещё раз.").assertExists()
            // The failed attempt did not touch what the first attempt parsed.
            itemsStub().assertTextEquals(LUNCH_ITEMS.size.toString())
            // The field is still there to retry from, and the screen still
            // responds to its own controls rather than being torn down.
            chatField().assertExists()
            onNodeWithContentDescription("Закрыть").performClick()
            waitForIdle()

            assertTrue(cancelled, "the close control stopped responding after a failed parse")
        }
}
