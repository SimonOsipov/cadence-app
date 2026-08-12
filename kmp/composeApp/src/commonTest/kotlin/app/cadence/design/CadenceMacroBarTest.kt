package app.cadence.design

import androidx.compose.foundation.layout.Column
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class CadenceMacroBarTest {
    @Test
    fun theMacroFractionIsClampedAtBothEndsAndSurvivesAZeroGoal() {
        assertEquals(0.5f, macroFraction(value = 70.0, goal = 140.0))
        assertEquals(1f, macroFraction(value = 200.0, goal = 140.0), "a goal beaten is not a bar overflowing")
        assertEquals(0f, macroFraction(value = -1.0, goal = 140.0))
        assertEquals(0f, macroFraction(value = 70.0, goal = 0.0), "a zero goal divides")
    }

    @Test
    fun theMacroFillIsAsWideAsItsFraction() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceMacroBar(
                        "protein",
                        "белок",
                        value = 35.0,
                        goal = 140.0,
                        unit = "г",
                        color = CadenceColors.forest700,
                    )
                }
            }

            val track =
                onNodeWithTag(
                    macroTrackTag("protein"),
                    useUnmergedTree = true,
                ).fetchSemanticsNode().boundsInRoot
            val fill = onNodeWithTag(macroFillTag("protein"), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            assertTrue(
                abs(fill.width / track.width - 0.25f) < BAR_FRACTION_TOLERANCE,
                "35 of 140 is not a quarter of the track",
            )
        }

    @Test
    fun theMacroBarLabelReadsValueOverGoalNotValueOverItself() =
        runComposeUiTest {
            // No test in this file asserts this component's text at all, so
            // `formatDecimal(goal, 0)` collapsing to `formatDecimal(value, 0)`
            // — rendering «35 / 35 г» instead of «35 / 140 г» — survived.
            setContent {
                CadenceTheme {
                    CadenceMacroBar(
                        "protein",
                        "белок",
                        value = 35.0,
                        goal = 140.0,
                        unit = "г",
                        color = CadenceColors.forest700,
                    )
                }
            }

            onNodeWithText("35 / 140 г").assertExists()
        }

    @Test
    fun threeCoexistingMacroBarsOwnThreeDistinctTrackTags() =
        runComposeUiTest {
            // The nutrition screen renders protein, carbs and fat side by side, which
            // is exactly what no other test in this file does — every other fixture
            // above renders a single bar, so `macroTrackTag(id) = "cadence-macro-track-$id"`
            // collapsing back to the single constant `"cadence-macro-track"` it replaced
            // would leave them all green: onNodeWithTag only throws once more than one
            // bar is on screen and two of them share a tag. Three ids are queried by
            // their own tag and their semantics node ids are collected into a Set —
            // under the collapse, the first of the three onNodeWithTag calls below
            // already fails, because it now matches all three tracks instead of one.
            setContent {
                CadenceTheme {
                    Column {
                        CadenceMacroBar(
                            "protein",
                            "белок",
                            value = 35.0,
                            goal = 140.0,
                            unit = "г",
                            color = CadenceColors.forest700,
                        )
                        CadenceMacroBar(
                            "carbs",
                            "углеводы",
                            value = 60.0,
                            goal = 200.0,
                            unit = "г",
                            color = CadenceColors.sand600,
                        )
                        CadenceMacroBar(
                            "fat",
                            "жиры",
                            value = 20.0,
                            goal = 70.0,
                            unit = "г",
                            color = CadenceColors.sand700,
                        )
                    }
                }
            }

            val trackNodeIds =
                listOf("protein", "carbs", "fat").map { id ->
                    onNodeWithTag(macroTrackTag(id), useUnmergedTree = true).fetchSemanticsNode().id
                }

            assertEquals(3, trackNodeIds.toSet().size, "three macro bars did not own three distinct track tags")
        }
}
