package app.cadence.design

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.math.abs
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

/**
 * Summed channel distance, so an antialiased glyph counts as its own colour. Measured, not
 * picked: the nearest pixel of «Жиры» at this size is 0,08 from pure red, while the default
 * this test exists to catch — `subtle` — is 1,47 away. The gate sits an order of magnitude
 * below the confusion it has to prevent.
 */
private const val COLOUR_TOLERANCE = 0.15f

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

    /**
     * The dark card variant (`MacroBarDark`, `RecipeBuilderScreen.tsx:20-75`): on
     * `forest800` the default `sunk` (linen) track reads as a *filled* bar rather than an
     * empty one — the prototype's own is cream at 14% (`:39`) — so the track is a parameter.
     * Measured on an all-zero split, the one state where the track is not covered by its
     * own segments, and with `sand300`, which no segment paints: `forest700` here would
     * also be the answer if a zero share wrongly drew a full-width protein segment.
     */
    @Test
    fun anEmptyTrackPaintsTheTrackColourItWasGiven() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceSplitBar(proteinG = 0.0, carbsG = 0.0, fatG = 0.0, trackColor = CadenceColors.sand300)
                }
            }

            val track = onNodeWithTag(CADENCE_SPLIT_TRACK_TAG, useUnmergedTree = true).captureToImage()

            assertEquals(CadenceColors.sand300, track.toPixelMap()[track.width / 2, track.height / 2])
        }

    /**
     * The other two parameters exist for the same dark card, and nothing measured them:
     * ignoring them and keeping `subtle`/`ink2` left the whole suite green while the legend
     * stayed unreadable on `forest800`. Glyphs are antialiased, so this asks whether the
     * colour appears anywhere in the node rather than at one chosen pixel.
     *
     * Label and value are probed as separate nodes, and all three entries are probed: a
     * whole-entry probe stays green when the two colours are swapped, and a protein-only
     * probe stays green when carbs and fat keep the hardcoded defaults.
     */
    @Test
    fun everyLegendEntryPaintsTheLabelAndValueColoursItWasGiven() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceSplitBar(
                        proteinG = 30.0,
                        carbsG = 20.0,
                        fatG = 10.0,
                        labelColor = Color.Red,
                        valueColor = Color.Blue,
                    )
                }
            }

            listOf("protein", "carbs", "fat").forEach { macro ->
                assertPaints(splitLegendLabelTag(macro), Color.Red, "$macro's label")
                assertPaints(splitLegendValueTag(macro), Color.Blue, "$macro's grams")
            }
        }

    /**
     * Nearest painted colour rather than an exact match: glyphs are antialiased, and
     * whether any pixel lands on the text colour exactly depends on the word — «Белок» and
     * «Углев» have such a pixel at this size, «Жиры» does not (measured).
     */
    private fun ComposeUiTest.assertPaints(
        tag: String,
        expected: Color,
        what: String,
    ) {
        val pixels = onNodeWithTag(tag, useUnmergedTree = true).captureToImage().toPixelMap()
        val distances =
            buildList {
                for (x in 0 until pixels.width) {
                    for (y in 0 until pixels.height) add(pixels[x, y] to distance(pixels[x, y], expected))
                }
            }
        val (closest, gap) = distances.minBy { it.second }

        assertTrue(gap < COLOUR_TOLERANCE, "$what is painted $closest, $gap away from the $expected it was given")
    }

    private fun distance(
        a: Color,
        b: Color,
    ): Float = abs(a.red - b.red) + abs(a.green - b.green) + abs(a.blue - b.blue)

    @Test
    fun anEmptyTrackDefaultsToSunk() =
        runComposeUiTest {
            var sunk = Color.Unspecified
            setContent {
                CadenceTheme {
                    sunk = Cadence.palette.sunk
                    CadenceSplitBar(proteinG = 0.0, carbsG = 0.0, fatG = 0.0)
                }
            }

            val track = onNodeWithTag(CADENCE_SPLIT_TRACK_TAG, useUnmergedTree = true).captureToImage()

            assertEquals(sunk, track.toPixelMap()[track.width / 2, track.height / 2])
        }
}
