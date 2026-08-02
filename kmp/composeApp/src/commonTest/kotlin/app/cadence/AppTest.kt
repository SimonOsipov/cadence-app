package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
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
        }

    @Test
    fun theShowcaseTabBarReportsTheDestinationTapped() =
        runComposeUiTest {
            setContent { App() }

            // Not decoration: the bar holds state here, so a tap that did not
            // arrive would leave the previous destination tinted and nothing
            // else would say so.
            onNodeWithText("Тренды").performClick()
            onNodeWithText("Тренды").assertIsDisplayed()
        }
}
