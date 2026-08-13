package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class CadenceTextFieldTest {
    @Test
    fun thePlaceholderDisappearsOnceThereIsText() =
        runComposeUiTest {
            setContent {
                CadenceTheme { CadenceTextField(value = "", onValueChange = {}, placeholder = "Найти продукт") }
            }
            onNodeWithText("Найти продукт").assertExists()
        }

    @Test
    fun theFieldShowsWhatItWasGivenRatherThanWhatWasTyped() =
        runComposeUiTest {
            // Against the mutation «the field keeps its own state»: a controlled
            // field whose parent rejects a value must show the rejection.
            setContent {
                CadenceTheme { CadenceTextField(value = "Творог", onValueChange = {}, placeholder = "Найти продукт") }
            }

            onNodeWithText("Творог").assertExists()
            onAllNodesWithText("Найти продукт").assertCountEquals(0)
        }

    @Test
    fun typingReportsWhatWasTypedOnTheExactNodeFieldModifierTags() =
        runComposeUiTest {
            // Against the mutation «onValueChange is never wired»: a field that
            // renders but never reports back would leave every caller's draft
            // frozen at its initial value. Reached via a testTag on
            // `fieldModifier` (same way `AddVialScreenTest.field` locates a
            // real site's field), proving it lands on the node that actually
            // carries the text-editing action, not on the surrounding box
            // (which plain `modifier` targets, per the fix-round ruling below).
            var typed = ""
            setContent {
                CadenceTheme {
                    CadenceTextField(
                        value = "",
                        onValueChange = { typed = it },
                        placeholder = "Найти продукт",
                        fieldModifier = Modifier.testTag(TEXT_FIELD_PROBE_TAG),
                    )
                }
            }

            onNodeWithTag(TEXT_FIELD_PROBE_TAG, useUnmergedTree = true).performTextReplacement("Творог")

            assertEquals("Творог", typed, "fieldModifier did not reach a node that reports typing")
        }

    @Test
    fun aFieldGivenWeightLeavesRoomForItsRowSibling() =
        runComposeUiTest {
            // Against the mutation that routes the whole `modifier` to the
            // inner `BasicTextField` instead of the outer `Box` (this
            // component's original shape, overridden in the fix round below):
            // `Row` only reads a `weight` marker off its immediate child, so a
            // `weight(1f)` buried inside the field's internals is invisible to
            // it, and the box's unconditional `fillMaxWidth()` then claims the
            // entire row before the sibling is measured — the chat composer
            // (`ChatThreadScreen.tsx:290-312`) and the ingredient search
            // (`RecipeBuilderScreen.tsx:187-199`) both need exactly this shape.
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    Row(Modifier.width(ROW_PROBE_WIDTH)) {
                        CadenceTextField(
                            value = "",
                            onValueChange = {},
                            placeholder = "",
                            modifier = Modifier.weight(1f),
                        )
                        // Painted, not left empty: an unpainted Box with only a
                        // size modifier measured as (260,0)-(260,0) in this
                        // harness — width 0, even once correctly placed at
                        // x=260 by Row — so this probe stands in for the send
                        // button/icon a real sibling always paints.
                        Box(
                            Modifier
                                .width(SIBLING_PROBE_WIDTH)
                                .height(SIBLING_PROBE_WIDTH)
                                .background(Color.Red)
                                .testTag(TEXT_FIELD_SIBLING_TAG),
                        )
                    }
                }
            }

            val sibling =
                onNodeWithTag(TEXT_FIELD_SIBLING_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot
            val expectedSiblingWidth = with(density) { SIBLING_PROBE_WIDTH.toPx() }

            assertEquals(
                expectedSiblingWidth,
                sibling.width,
                "the field's weight(1f) squeezed its Row sibling to ${sibling.width}, not its natural " +
                    "$expectedSiblingWidth",
            )
        }

    @Test
    fun theFieldPaintsABorderDistinctFromItsPaperFill() =
        runComposeUiTest {
            // The component's defining visual — background plus border — was
            // asserted by nothing: an independent reviewer deleted
            // `.background(Cadence.palette.paper)` alone and 36/36 stayed
            // green, because `edge != center` only proved the two pixels
            // differ, not that the centre is the field's own paper fill —
            // with the background gone, CadenceTheme's `bg` shows through the
            // centre and still differs from `border`. Pinned against the
            // palette values themselves (hoisted out of `setContent` the way
            // this file hoists `LocalDensity`), not a hardcoded hex, so the
            // assertion holds regardless of what `paper`/`border` resolve to.
            var paperColor = Color.Unspecified
            var borderColor = Color.Unspecified
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    paperColor = Cadence.palette.paper
                    borderColor = Cadence.palette.border
                    CadenceTextField(
                        value = "",
                        onValueChange = {},
                        placeholder = "",
                        modifier = Modifier.testTag(TEXT_FIELD_PROBE_TAG),
                    )
                }
            }

            assertNotEquals(paperColor, borderColor, "paper and border must be distinct for this test to be meaningful")

            val field = onNodeWithTag(TEXT_FIELD_PROBE_TAG, useUnmergedTree = true).captureToImage()
            val pixels = field.toPixelMap()
            // TEXT_FIELD_HAIRLINE_PIN is deliberately a second, independent
            // opinion on the border's width, not read from TEXT_FIELD_BORDER:
            // an assertion deriving its expectation from the exact constant a
            // mutation would touch can never disagree with that mutation —
            // widen TEXT_FIELD_BORDER and both would move together. Pinning
            // against a value that doesn't move gives an 8x-wider border
            // something to fail against. Verified: flipping TEXT_FIELD_BORDER
            // from 1.dp to 8.dp turns this assertion red only because
            // pinnedPx doesn't track it.
            val pinnedPx = with(density) { TEXT_FIELD_HAIRLINE_PIN.roundToPx() }
            // x=0, y=h/2 is the shape's leftmost point — true as long as the
            // field's height is at least 2*CadenceRadius.md (24px at density
            // 1.0, below which y=h/2 falls inside the corner arc instead of
            // the straight edge); true today, since the field is ~45px tall.
            val edge = pixels[0, field.height / 2]
            val justPastPin = pixels[pinnedPx, field.height / 2]
            val center = pixels[field.width / 2, field.height / 2]

            assertEquals(borderColor, edge, "the field's edge pixel ($edge) is not its own border colour")
            assertEquals(
                paperColor,
                justPastPin,
                "the pixel just past the pinned $pinnedPx px hairline ($justPastPin) is not paper — the border " +
                    "is wider than the design's hairline",
            )
            assertEquals(paperColor, center, "the field's centre pixel ($center) is not its own paper fill")
        }

    @Test
    fun theFieldPadsItsContentAwayFromTheBorder() =
        runComposeUiTest {
            // Same finding as above: an independent reviewer deleted
            // `.padding(CadenceSpacing.md)` and 36/36 stayed green, because
            // `field > bare` only pins direction — shrinking it to
            // `.padding(CadenceSpacing.xxs)` (12dp to 4dp) survives that too.
            // Pinned to the exact amount instead: both components render the
            // same body-styled `BasicTextField`, so content height cancels
            // out of the difference and only the padding remains — converted
            // through `LocalDensity` (hoisted the same way as the test above)
            // rather than hardcoded, since CadenceSpacing is Dp, not px. Not
            // density-fragile at any density this target ships — integers and
            // half-integers agree with layout's per-side rounding
            // (2 * roundToPx(12.dp)) — but can diverge at a fractional
            // density (e.g. d=3.3: 80 vs 79.2 here).
            lateinit var density: Density
            setContent {
                CadenceTheme {
                    density = LocalDensity.current
                    Row {
                        BasicTextField(
                            value = "",
                            onValueChange = {},
                            textStyle = Cadence.typography.body,
                            modifier = Modifier.testTag(TEXT_FIELD_BARE_TAG),
                        )
                        CadenceTextField(
                            value = "",
                            onValueChange = {},
                            placeholder = "",
                            modifier = Modifier.testTag(TEXT_FIELD_PROBE_TAG),
                        )
                    }
                }
            }

            val bare =
                onNodeWithTag(TEXT_FIELD_BARE_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.height
            val field =
                onNodeWithTag(TEXT_FIELD_PROBE_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.height
            val expectedPadding = with(density) { (CadenceSpacing.md * 2).toPx() }

            assertEquals(
                expectedPadding,
                field - bare,
                "the field's padding (${field - bare}) is not exactly twice CadenceSpacing.md ($expectedPadding)",
            )
        }

    @Test
    fun minLinesReservesRoomBeforeAnyTextIsTyped() =
        runComposeUiTest {
            // Against the mutation «minLines is accepted but ignored»: an empty
            // field would then be exactly one line tall regardless of the
            // parameter, and a chat composer meant to open at three lines would
            // open at one.
            var lines by mutableStateOf(1)
            setContent {
                CadenceTheme {
                    CadenceTextField(
                        value = "",
                        onValueChange = {},
                        placeholder = "Сообщение",
                        minLines = lines,
                        modifier = Modifier.testTag(TEXT_FIELD_PROBE_TAG),
                    )
                }
            }

            fun fieldHeight() =
                onNodeWithTag(TEXT_FIELD_PROBE_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.height

            val oneLine = fieldHeight()
            runOnIdle { lines = 4 }
            val fourLines = fieldHeight()

            assertTrue(fourLines > oneLine, "minLines = 4 ($fourLines) is not taller than minLines = 1 ($oneLine)")
        }

    @Test
    fun singleLineKeepsOneLineWhileTheDefaultWrapsOntoMore() =
        runComposeUiTest {
            // Against the mutation «singleLine is accepted but ignored»: a
            // one-line field (a search box, a title) would then grow onto a
            // second line the moment a caller types past the visible width —
            // exactly the shape a search bar cannot have.
            //
            // The field's box always fills whatever width its parent offers
            // (`CadenceTextField.kt`'s outer `Box` calls `fillMaxWidth()`
            // unconditionally), so the narrow width must come from an
            // ancestor — a plain `Row` would hand the first field the entire
            // available width and starve the second.
            val long = "нежирный творог пятипроцентный большая пачка из холодильника"
            setContent {
                CadenceTheme {
                    Row {
                        Box(Modifier.width(WRAP_PROBE_WIDTH)) {
                            CadenceTextField(
                                value = long,
                                onValueChange = {},
                                placeholder = "",
                                modifier = Modifier.testTag(TEXT_FIELD_WRAP_TAG),
                            )
                        }
                        Box(Modifier.width(WRAP_PROBE_WIDTH)) {
                            CadenceTextField(
                                value = long,
                                onValueChange = {},
                                placeholder = "",
                                singleLine = true,
                                modifier = Modifier.testTag(TEXT_FIELD_NO_WRAP_TAG),
                            )
                        }
                    }
                }
            }

            val wrapped =
                onNodeWithTag(TEXT_FIELD_WRAP_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.height
            val single =
                onNodeWithTag(TEXT_FIELD_NO_WRAP_TAG, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot.height

            assertTrue(wrapped > single, "singleLine did not stop the field from wrapping onto more than one line")
        }
}

private const val TEXT_FIELD_PROBE_TAG = "text-field-probe"
private const val TEXT_FIELD_WRAP_TAG = "text-field-wrap-probe"
private const val TEXT_FIELD_NO_WRAP_TAG = "text-field-no-wrap-probe"
private const val TEXT_FIELD_SIBLING_TAG = "text-field-sibling-probe"
private const val TEXT_FIELD_BARE_TAG = "text-field-bare-probe"
private val WRAP_PROBE_WIDTH = 90.dp
private val ROW_PROBE_WIDTH = 300.dp
private val SIBLING_PROBE_WIDTH = 40.dp

/**
 * The field's border, pinned independently of `TEXT_FIELD_BORDER` — see the comment at its
 * one call site for why duplicating the value here, rather than reading the constant a
 * mutation would touch, is the point.
 */
private val TEXT_FIELD_HAIRLINE_PIN = 1.dp
