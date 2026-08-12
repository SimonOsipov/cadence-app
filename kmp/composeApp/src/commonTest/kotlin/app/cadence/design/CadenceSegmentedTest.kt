package app.cadence.design

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
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
            // CadenceControls.kt:132-138's own precedent for a disabled
            // control chains its alpha *before* `.background(...)`, so the
            // background sits inside the dimmed layer. Chaining `.then(if
            // (enabled) ... else Modifier.alpha(...))` *after*
            // `.background(palette.sunk)` instead — the mutation this test
            // exists to catch — leaves the track painted at full strength
            // underneath the alpha, indistinguishable from the enabled
            // track to every semantics-based test in this file.
            //
            // The same node is captured both enabled and disabled and the
            // two pixels are compared directly, rather than one pixel
            // against a hardcoded `CadenceColors.linen` in one direction —
            // this test's own first draft. That comparison passed on *any*
            // pixel lighter than raw linen, including the selected
            // segment's own `paper` fill (blue 0.9529 vs. linen's 0.8392),
            // which is exactly the wrong pixel the first draft sampled
            // before [CADENCE_SEGMENTED_TRACK_TAG] was moved ahead of
            // `.padding(...)` in the modifier chain — a tag placed after
            // padding reports the padding-reduced content box as its own
            // bounds, cropping a capture to the segments and hiding the
            // track's own edge. Comparing enabled against disabled needs no
            // palette constant, assumes nothing about which colour is
            // lighter, and holds regardless of which way a future
            // palette's alpha blend shifts.
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
            // theDisabledTrackDimsAlongWithItsSegments's own probe samples
            // pixel (2, h/2), which sits inside the track's own
            // CadenceSpacing.xxs padding and never reaches a segment — so
            // `if (isSelected) palette.paper else Color.Transparent` always
            // returning `Color.Transparent` (the active pill, the control's
            // whole visible purpose, gone) leaves every test in this file
            // green. Follows this file's own two pixel-probe precedents
            // (this test and theFieldPaintsABorderDistinctFromItsPaperFill
            // in CadenceTextFieldTest): compare against a palette token, not
            // a hardcoded hex.
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
            // for CadenceWeekBars: an existence check on a tag cannot tell
            // «this one is on» from «every segment looks the same», so both
            // halves below are measured, not just one. Against the mutation
            // `selected = false` (every segment reads as not selected):
            // assertIsSelected on «фото» is the half that catches it — its
            // failure reads «Failed to assert … (Selected = 'true')» on
            // that node. assertIsNotSelected on «текст» instead catches the
            // opposite constant, `selected = true` (every segment marks
            // itself) — a mutation assertIsSelected alone could not tell
            // apart from correct, since «фото» would still read selected
            // either way.
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
}
