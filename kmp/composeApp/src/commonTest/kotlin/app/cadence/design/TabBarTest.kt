package app.cadence.design

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals

@OptIn(ExperimentalTestApi::class)
class TabBarTest {
    @Test
    fun everyDestinationIsReachableAndTheActionIsNamed() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceTabBar(active = CadenceDestination.TODAY, onSelect = {}, onLog = {})
                }
            }

            CadenceDestination.entries.forEach {
                onNodeWithText(it.label).assertIsDisplayed()
            }
            // The centre action is a raised circle with no visible label, so it
            // is announced by its accessibility name or by nothing at all.
            onNodeWithContentDescription(CadenceLogAction.ACCESSIBILITY_LABEL).assertIsDisplayed()
        }

    @Test
    fun theDestinationsAreDrawnInTheOrderTheProtoypeUses() {
        // Addressing every other assertion by text means a reordered enum would
        // pass all of them while moving the raised action across the bar.
        assertEquals(
            listOf("Сегодня", "Аптечка", "Тренды", "Питание"),
            CadenceDestination.entries.map { it.label },
        )
    }

    @Test
    fun onlyTheCurrentDestinationReportsItselfSelected() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceTabBar(active = CadenceDestination.TRENDS, onSelect = {}, onLog = {})
                }
            }

            // Asserted through semantics rather than a colour: a colour cannot
            // be read from the tree, which is how a bar that highlighted nothing
            // passed a suite claiming to check exactly this. It also makes the
            // current tab announceable — VoiceOver had no way to say it before.
            onNodeWithText("Тренды").assertIsSelected()
            CadenceDestination.entries.filterNot { it == CadenceDestination.TRENDS }.forEach {
                onNodeWithText(it.label).assertIsNotSelected()
            }
        }

    @Test
    fun everyDestinationReportsTheOneItStandsFor() =
        runComposeUiTest {
            val taps = mutableListOf<CadenceDestination>()
            setContent {
                CadenceTheme {
                    CadenceTabBar(
                        active = CadenceDestination.TODAY,
                        onSelect = { taps += it },
                        onLog = {},
                    )
                }
            }

            // Tapped out of declaration order, so a bar reporting by position
            // rather than by identity fails.
            onNodeWithText("Питание").performClick()
            onNodeWithText("Аптечка").performClick()
            onNodeWithText("Сегодня").performClick()
            onNodeWithText("Тренды").performClick()

            assertEquals(
                listOf(
                    CadenceDestination.NUTRITION,
                    CadenceDestination.INVENTORY,
                    CadenceDestination.TODAY,
                    CadenceDestination.TRENDS,
                ),
                taps,
            )
        }

    @Test
    fun theCentreActionReportsSeparatelyFromEveryDestination() =
        runComposeUiTest {
            var logs = 0
            val selections = mutableListOf<CadenceDestination>()
            setContent {
                CadenceTheme {
                    CadenceTabBar(
                        active = CadenceDestination.TODAY,
                        onSelect = { selections += it },
                        onLog = { logs++ },
                    )
                }
            }

            onNodeWithContentDescription(CadenceLogAction.ACCESSIBILITY_LABEL).performClick()

            // Logging a dose pushes a wizard; it does not change which
            // destination is current. When both went through one callback, the
            // showcase set `active` to the action and the bar highlighted
            // nothing — the state this bar's own docs called impossible.
            assertEquals(1, logs, "the action did not report")
            assertEquals(emptyList(), selections, "the action reported as a destination change")
        }
}
