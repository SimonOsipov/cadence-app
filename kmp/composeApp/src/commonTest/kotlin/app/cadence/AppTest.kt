package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.currentPlatform
import kotlin.test.Test

@OptIn(ExperimentalTestApi::class)
class AppTest {
    @Test
    fun appShowsTheBrand() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithText("Cadence").assertIsDisplayed()
        }

    @Test
    fun appShowsThePlatformItRunsOn() =
        runComposeUiTest {
            setContent { App() }

            // Proves :shared is linked into the UI, not just into the module graph.
            onNodeWithText(currentPlatform().name).assertIsDisplayed()
        }

    @Test
    fun theShowcaseRendersEveryComponentItIsSupposedTo() =
        runComposeUiTest {
            setContent { App() }

            // The showcase is where a regression is seen before it reaches a
            // screen. A component that silently stops rendering fails here.
            listOf(
                "СЕГОДНЯ" to "the eyebrow",
                "Ваша аптечка" to "the emphasised title",
                "0,25" to "the measured value",
                "мг" to "its unit",
                "по расписанию" to "the pill",
                "Неделя" to "the chip",
                "Открыть лист" to "the button",
                "Сегодня" to "the tab bar",
            ).forEach { (text, _) ->
                onNodeWithText(text).assertIsDisplayed()
            }

            // The sparkline draws no text, so without a handle deleting it from
            // the showcase leaves this whole suite green — which it did.
            onNodeWithTag(SHOWCASE_SPARK_TAG).assertIsDisplayed()
        }

    @Test
    fun theShowcaseTabBarReportsBothADestinationAndTheAction() =
        runComposeUiTest {
            setContent { App() }

            // The previous version of this test clicked a tab and asserted the
            // tab's own label was displayed — which it is either way, because
            // the bar renders every label unconditionally. It passed against
            // `onSelect = {}`. The showcase now shows its state, so the
            // assertion has something a tap can actually change.
            onNodeWithText("Сегодня · записей: 0").assertIsDisplayed()

            onNodeWithText("Тренды").performClick()
            onNodeWithText("Тренды · записей: 0").assertIsDisplayed()

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Тренды · записей: 1").assertIsDisplayed()
            // And it did not double as a destination change.
            onNodeWithText("Тренды").assertIsSelected()
        }
}
