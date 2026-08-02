package app.cadence.design

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals

@OptIn(ExperimentalTestApi::class)
class TabBarTest {
    @Test
    fun everyDestinationIsReachableByNameOrByLabel() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceTabBar(active = CadenceTab.TODAY, onSelect = {})
                }
            }

            onNodeWithText("Сегодня").assertIsDisplayed()
            onNodeWithText("Аптечка").assertIsDisplayed()
            onNodeWithText("Тренды").assertIsDisplayed()
            onNodeWithText("Питание").assertIsDisplayed()
            // The centre action is a raised circle with no visible label, so it
            // is announced by its accessibility name or by nothing at all.
            onNodeWithContentDescription("Записать").assertIsDisplayed()
        }

    @Test
    fun everyDestinationReportsTheTabItStandsFor() =
        runComposeUiTest {
            val taps = mutableListOf<CadenceTab>()
            setContent {
                CadenceTheme {
                    CadenceTabBar(active = CadenceTab.TODAY, onSelect = { taps += it })
                }
            }

            // Tapped in a different order than they are declared, so a bar that
            // reported by position rather than by identity would fail.
            onNodeWithText("Питание").performClick()
            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Аптечка").performClick()
            onNodeWithText("Сегодня").performClick()
            onNodeWithText("Тренды").performClick()

            assertEquals(
                listOf(
                    CadenceTab.NUTRITION,
                    CadenceTab.LOG,
                    CadenceTab.INVENTORY,
                    CadenceTab.TODAY,
                    CadenceTab.TRENDS,
                ),
                taps,
            )
        }

    @Test
    fun theActiveDestinationIsDrawnDifferentlyFromTheRest() =
        runComposeUiTest {
            // Without this the bar renders identically whatever is active, and
            // the patient loses the only cue for where they are. Asserted
            // through the tint the component resolves, since a colour is not
            // queryable from the tree.
            assertEquals(
                CadenceColors.forest700,
                cadenceTabTint(tab = CadenceTab.TRENDS, active = CadenceTab.TRENDS, subtle = CadenceColors.ink500),
                "the active destination is not tinted forest",
            )
            assertEquals(
                CadenceColors.ink500,
                cadenceTabTint(tab = CadenceTab.TRENDS, active = CadenceTab.TODAY, subtle = CadenceColors.ink500),
                "an inactive destination is not tinted subtle",
            )
        }
}
