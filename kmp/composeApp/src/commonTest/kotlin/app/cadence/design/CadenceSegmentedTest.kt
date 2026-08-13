package app.cadence.design

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class CadenceSegmentedTest {
    @Test
    fun theSegmentedControlReportsTheOptionAndNotItsIndex() =
        runComposeUiTest {
            var picked = "текст"
            setContent {
                CadenceTheme {
                    CadenceSegmented(
                        options = listOf("текст", "фото", "голос"),
                        selected = "текст",
                        onSelect = { picked = it },
                        label = { it },
                    )
                }
            }

            onNodeWithText("голос").performClick()

            assertEquals("голос", picked, "the control reported a position rather than a value")
        }

    @Test
    fun theSegmentedControlReportsAnyClickedOptionNotJustTheLastOne() =
        runComposeUiTest {
            // theSegmentedControlReportsTheOptionAndNotItsIndex only ever
            // clicks «голос», which happens to be this fixture's *last*
            // option too — so `onSelect(options[options.size - 1])` passes
            // that test as well as the correct implementation does.
            // Clicking the middle option is the only interaction that tells
            // «reports what was clicked» apart from «always reports the
            // last one».
            var picked = "текст"
            setContent {
                CadenceTheme {
                    CadenceSegmented(
                        options = listOf("текст", "фото", "голос"),
                        selected = "текст",
                        onSelect = { picked = it },
                        label = { it },
                    )
                }
            }

            onNodeWithText("фото").performClick()

            assertEquals("фото", picked, "the control reported the last option rather than the one clicked")
        }

    @Test
    fun aDisabledSegmentedControlReportsNothing() =
        runComposeUiTest {
            // «За сколько напоминать» is inert while its parent toggle is off
            // (`SettingsScreen.tsx:358`), and the recipe card's toggle is live —
            // one control, two states.
            var picked = "на порцию"
            setContent {
                CadenceTheme {
                    CadenceSegmented(
                        options = listOf("на порцию", "всё"),
                        selected = "на порцию",
                        onSelect = { picked = it },
                        label = { it },
                        enabled = false,
                    )
                }
            }

            onNodeWithText("всё").performClick()

            assertEquals("на порцию", picked, "a disabled control still reported")
        }

    @Test
    fun theDisabledTrackDimsAlongWithItsSegments() =
        runComposeUiTest {
            // CadenceControls.kt:132-138's precedent chains alpha *before*
            // `.background(...)` so the background sits inside the dimmed
            // layer. Chaining alpha *after* `.background(palette.sunk)`
            // instead — the mutation this test catches — leaves the track
            // painted at full strength underneath the alpha, indistinguishable
            // to every semantics-based test in this file.
            //
            // Enabled and disabled pixels of the same node are compared
            // directly, not one pixel against a hardcoded `CadenceColors.linen`
            // — this test's first draft did that, and passed on *any* pixel
            // lighter than raw linen, including the selected segment's own
            // `paper` fill (0.9529 vs. linen's 0.8392): the wrong pixel,
            // sampled before [CADENCE_SEGMENTED_TRACK_TAG] moved ahead of
            // `.padding(...)` in the chain (a tag after padding reports the
            // padding-reduced content box, cropping the track's own edge out
            // of the capture). Enabled-vs-disabled needs no palette constant
            // and holds regardless of which way a future alpha blend shifts.
            var enabled by mutableStateOf(true)
            setContent {
                CadenceTheme {
                    CadenceSegmented(
                        options = listOf("на порцию", "всё"),
                        selected = "на порцию",
                        onSelect = {},
                        label = { it },
                        enabled = enabled,
                    )
                }
            }

            fun trackPixel(): Color {
                val track = onNodeWithTag(CADENCE_SEGMENTED_TRACK_TAG, useUnmergedTree = true).captureToImage()
                return track.toPixelMap()[2, track.height / 2]
            }

            val enabledPixel = trackPixel()
            runOnIdle { enabled = false }
            val disabledPixel = trackPixel()

            assertTrue(
                enabledPixel != disabledPixel,
                "the track's pixel is unchanged between enabled ($enabledPixel) and disabled " +
                    "($disabledPixel); the dim is not reaching the track",
            )
        }

    @Test
    fun theSelectedSegmentPaintsPaperOverTheTrack() =
        runComposeUiTest {
            // theDisabledTrackDimsAlongWithItsSegments's probe samples pixel
            // (2, h/2), inside the track's CadenceSpacing.xxs padding, never
            // reaching a segment — so `if (isSelected) palette.paper else
            // Color.Transparent` always returning `Color.Transparent` (the
            // active pill gone) leaves every test in this file green.
            // Follows this file's pixel-probe precedent (and
            // theFieldPaintsABorderDistinctFromItsPaperFill in
            // CadenceTextFieldTest): compare against a palette token, not a
            // hardcoded hex.
            var paperColor = Color.Unspecified
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    paperColor = Cadence.palette.paper
                    CadenceSegmented(
                        options = listOf("на порцию", "всё"),
                        selected = "на порцию",
                        onSelect = {},
                        label = { it },
                    )
                }
            }

            val track = onNodeWithTag(CADENCE_SEGMENTED_TRACK_TAG, useUnmergedTree = true).captureToImage()
            val pixels = track.toPixelMap()
            val paddingPx = with(density) { CadenceSpacing.xxs.toPx() }
            val segmentWidthPx = (track.width - 2 * paddingPx) / 2
            val selectedCenterX = (paddingPx + segmentWidthPx / 2).toInt()

            assertEquals(
                paperColor,
                pixels[selectedCenterX, track.height / 2],
                "the selected segment's own pixel is not the palette's paper fill",
            )
        }

    @Test
    fun theChosenSegmentIsSelectedAndItsNeighbourIsNot() =
        runComposeUiTest {
            // Same reasoning as theTodayColumnIsSelectedAndItsNeighbourIsNot
            // for CadenceWeekBars: an existence check on a tag can't tell
            // «this one is on» from «every segment looks the same», so both
            // halves below are measured. Against `selected = false` (every
            // segment reads not-selected), assertIsSelected on «фото» is the
            // half that catches it; against the opposite constant,
            // `selected = true`, assertIsNotSelected on «текст» catches it —
            // assertIsSelected alone couldn't tell that mutation from correct,
            // since «фото» reads selected either way.
            setContent {
                CadenceTheme {
                    CadenceSegmented(
                        options = listOf("текст", "фото", "голос"),
                        selected = "фото",
                        onSelect = {},
                        label = { it },
                    )
                }
            }

            onNodeWithText("фото").assertIsSelected()
            onNodeWithText("текст").assertIsNotSelected()
        }

    @Test
    fun theSegmentedControlStaysOneLineEvenWithATooLongLabel() =
        runComposeUiTest {
            // A probe measured this before the fix: the same long string, same
            // width, rendered 140.dp tall with only overflow = Ellipsis, and
            // 14.dp with maxLines = 1 beside it — Ellipsis alone has nothing to
            // truncate against, so the label wraps instead. Comparing two live
            // tracks at the same width pins the fix without a line-height
            // constant a font change would break.
            val longLabel = "Очень длинное название режима, которое совершенно точно не влезет в один сегмент"

            setContent {
                CadenceTheme {
                    Column(Modifier.width(160.dp)) {
                        CadenceSegmented(
                            options = listOf("текст", "фото"),
                            selected = "текст",
                            onSelect = {},
                            label = { it },
                        )
                        CadenceSegmented(
                            options = listOf(longLabel, "фото"),
                            selected = longLabel,
                            onSelect = {},
                            label = { it },
                        )
                    }
                }
            }

            val tracks =
                onAllNodesWithTag(CADENCE_SEGMENTED_TRACK_TAG, useUnmergedTree = true).fetchSemanticsNodes()
            val shortLabelHeight = tracks[0].boundsInRoot.height
            val longLabelHeight = tracks[1].boundsInRoot.height

            assertEquals(
                shortLabelHeight,
                longLabelHeight,
                "the too-long label's track ($longLabelHeight) is taller than the short-label " +
                    "track ($shortLabelHeight) — the label wrapped instead of ellipsizing to one line",
            )
        }
}
