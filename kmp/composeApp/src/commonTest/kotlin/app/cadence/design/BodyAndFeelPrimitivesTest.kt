package app.cadence.design

import androidx.compose.foundation.layout.width
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

private const val TOLERANCE = 1e-3f

/**
 * A fraction that matches nothing else the body screen draws — not the weight, not
 * the lean mass, not a round quarter of the ring. A sweep computed from the wrong
 * input would have to land on this by coincidence.
 */
private const val FAT_PERCENT = 27.4

@OptIn(ExperimentalTestApi::class)
class BodyAndFeelPrimitivesTest {
    @Test
    fun theArcIsTheFatShareOfTheWholeCircle() {
        assertEquals(98.64f, compositionSweep(FAT_PERCENT), TOLERANCE)
    }

    @Test
    fun aShareOutsideTheScaleIsClampedRatherThanWoundRoundTwice() {
        // A bad reading is a drawing problem, not an arithmetic one: 140% would
        // otherwise draw a full ring plus a second arc over its own start.
        assertEquals(0f, compositionSweep(-3.0), TOLERANCE)
        assertEquals(FULL_TURN, compositionSweep(140.0), TOLERANCE)
    }

    @Test
    fun theRingCarriesTheWeightAtItsCentre() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceCompositionRing(
                        weightKg = 84.3,
                        fatPercent = FAT_PERCENT,
                        modifier = Modifier.width(343.dp),
                    )
                }
            }

            onNodeWithText("84,3").assertExists()
            onNodeWithText("кг").assertExists()
        }

    @Test
    fun tappingTheThirdBarFillsThreeAndNotOnlyTheThird() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    var value by remember { mutableStateOf(1) }

                    CadenceStepDots(
                        value = value,
                        accent = CadenceStepAccent.ENERGY,
                        onChange = { value = it },
                    )
                }
            }

            onNodeWithTag(cadenceStepDotTag(1)).assertIsSelected()
            onNodeWithTag(cadenceStepDotTag(2)).assertIsNotSelected()

            onNodeWithTag(cadenceStepDotTag(3)).performClick()

            // Cumulative, not a single highlight: one, two and three are all filled.
            (1..3).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsSelected() }
            (4..5).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsNotSelected() }
        }

    @Test
    fun theTwoAccentsAreNotTheSameColour() {
        val energy = stepAccentColor(CadenceStepAccent.ENERGY)
        val sleep = stepAccentColor(CadenceStepAccent.SLEEP)

        assertEquals(CadenceColors.forest700, energy)
        assertEquals(CadenceColors.sand700, sleep)
        assertNotEquals(energy, sleep)
    }

    @Test
    fun nothingIsFilledBeforeTheFirstAnswer() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceStepDots(value = 0, accent = CadenceStepAccent.SLEEP, onChange = {})
                }
            }

            // Zero is «not answered yet», and it has to look unlike «the worst there is».
            (1..5).forEach { onNodeWithTag(cadenceStepDotTag(it)).assertIsNotSelected() }
        }

    @Test
    fun theRingLeavesTheLeanShareToWhatIsUnderTheArc() {
        // Fat and lean are one circle, so the lean share is never computed twice:
        // the base ring is drawn whole and the fat arc covers its own part of it.
        val fat = compositionSweep(FAT_PERCENT)

        assertTrue(fat < FULL_TURN, "the fat arc covers the whole ring at $fat degrees")
        assertEquals(FULL_TURN, fat + compositionSweep(100.0 - FAT_PERCENT), TOLERANCE)
    }
}
