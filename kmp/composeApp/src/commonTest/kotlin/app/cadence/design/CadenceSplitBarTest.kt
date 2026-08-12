package app.cadence.design

import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class CadenceSplitBarTest {
    @Test
    fun theSplitIsByCaloriesAndNotByGrams() {
        // 10 g of fat is 90 kcal against 10 g of protein's 40: a split drawn by
        // grams would give three equal thirds and look right on a fixture where
        // the grams happen to match. This is the mutation the test exists for.
        val (p, c, f) = splitShares(proteinG = 10.0, carbsG = 10.0, fatG = 10.0)

        assertTrue(abs(p - 40f / 170f) < BAR_FRACTION_TOLERANCE, "protein is $p, not 40 of 170 kcal")
        assertTrue(abs(c - 40f / 170f) < BAR_FRACTION_TOLERANCE, "carbs are $c")
        assertTrue(abs(f - 90f / 170f) < BAR_FRACTION_TOLERANCE, "fat is $f, not 90 of 170 kcal")
    }

    @Test
    fun anEmptyMealSplitsIntoNothingRatherThanDividing() {
        assertEquals(Triple(0f, 0f, 0f), splitShares(0.0, 0.0, 0.0))
    }

    @Test
    fun theFatSegmentIsAsWideAsItsShareOfTheBar() =
        runComposeUiTest {
            // Measured, not described — same reasoning as theMacroFillIsAsWideAsItsFraction.
            // Fat is the segment that a by-grams mutation gets most wrong (90 of
            // 170 kcal vs. an equal third), so it is the one this test measures.
            setContent {
                CadenceTheme {
                    CadenceSplitBar(proteinG = 10.0, carbsG = 10.0, fatG = 10.0)
                }
            }

            val track = onNodeWithTag(CADENCE_SPLIT_TRACK_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val fat =
                onNodeWithTag(splitSegmentTag("fat"), useUnmergedTree = true).fetchSemanticsNode().boundsInRoot

            val fatRatio = fat.width / track.width
            assertTrue(abs(fatRatio - 90f / 170f) < BAR_FRACTION_TOLERANCE, "fat is $fatRatio of the bar, not 90/170")
        }

    @Test
    fun theProteinSegmentPaintsItsOwnColourNotFats() =
        runComposeUiTest {
            // Nothing in this file measures a segment's own colour — only its
            // width — so protein's `forest700` collapsing to fat's own
            // `sand700` (two of three segments becoming indistinguishable)
            // survived every test here.
            setContent {
                CadenceTheme {
                    CadenceSplitBar(proteinG = 10.0, carbsG = 10.0, fatG = 10.0)
                }
            }

            val protein = onNodeWithTag(splitSegmentTag("protein"), useUnmergedTree = true).captureToImage()
            val fat = onNodeWithTag(splitSegmentTag("fat"), useUnmergedTree = true).captureToImage()
            val proteinPixel = protein.toPixelMap()[protein.width / 2, protein.height / 2]
            val fatPixel = fat.toPixelMap()[fat.width / 2, fat.height / 2]

            assertEquals(CadenceColors.forest700, proteinPixel, "protein's own segment is not painted forest700")
            assertNotEquals(proteinPixel, fatPixel, "protein and fat segments paint the same colour")
        }
}
