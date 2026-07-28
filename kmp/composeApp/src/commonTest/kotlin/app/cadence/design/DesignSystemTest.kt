package app.cadence.design

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.unit.TextUnitType
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class DesignSystemTest {
    @Test
    fun buttonShowsItsLabelAndReportsClicks() =
        runComposeUiTest {
            var clicks = 0
            setContent {
                CadenceTheme {
                    CadenceButton(label = "Зафиксировать", onClick = { clicks++ })
                }
            }

            onNodeWithText("Зафиксировать").assertIsDisplayed().performClick()

            assertEquals(1, clicks, "the button did not report the click")
        }

    @Test
    fun chipTogglesTheStateItIsGiven() =
        runComposeUiTest {
            var active by mutableStateOf(false)
            setContent {
                CadenceTheme {
                    CadenceChip(label = "Неделя", active = active, onClick = { active = !active })
                }
            }

            onNodeWithText("Неделя").performClick()

            // Assert the state actually flipped, not merely that the label is
            // still there — a chip that ignored `active` would pass that.
            assertTrue(active, "the chip did not report the click")
            onNodeWithText("Неделя").assertIsDisplayed()
        }

    @Test
    fun cardReportsClicksWhenItIsGivenOne() =
        runComposeUiTest {
            var clicks = 0
            setContent {
                CadenceTheme {
                    CadenceCard(onClick = { clicks++ }) {
                        CadenceBody("Карточка")
                    }
                }
            }

            onNodeWithText("Карточка").performClick()

            assertEquals(1, clicks, "a tappable card swallowed the click")
        }

    @Test
    fun iconButtonCarriesAnAccessibleName() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceIconButton(
                        icon = CadenceIcons.bell,
                        contentDescription = "Напоминания",
                        onClick = {},
                    )
                }
            }

            // An icon-only control is announced by this name or by nothing.
            onNodeWithContentDescription("Напоминания").assertIsDisplayed()
        }

    @Test
    fun anUnknownIconNameDrawsNothingInsteadOfThrowing() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceIcon(name = "no-such-icon")
                    CadenceBody("После иконки")
                }
            }

            // The composition survived the miss; that is the whole contract.
            onNodeWithText("После иконки").assertIsDisplayed()
        }

    @Test
    fun titleLeadingAndTrackingFollowTheSize() {
        // Stored as ratios, so an override rescales them. Absolute sp leading
        // would leave a 34sp title with 30sp leading, and its lines overlap.
        val style = CadenceDefaultTypography.title
        assertEquals(TextUnitType.Em, style.lineHeight.type, "line height is not a ratio")
        assertEquals(TextUnitType.Em, style.letterSpacing.type, "tracking is not a ratio")
    }

    @Test
    fun sheetComposesNothingWhileClosed() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceSheet(open = false, onDismiss = {}) {
                        Box(Modifier.testTag("sheet-body"))
                    }
                }
            }

            onNodeWithTag("sheet-body").assertDoesNotExist()
            onNodeWithTag(CADENCE_SHEET_SCRIM_TAG).assertDoesNotExist()
        }

    @Test
    fun sheetShowsItsContentAndDismissesFromTheScrim() =
        runComposeUiTest {
            var open by mutableStateOf(true)
            setContent {
                CadenceTheme {
                    CadenceSheet(open = open, onDismiss = { open = false }) {
                        CadenceBody("Тело листа")
                    }
                }
            }

            onNodeWithText("Тело листа").assertIsDisplayed()
            onNodeWithTag(CADENCE_SHEET_SCRIM_TAG).performClick()
            onNodeWithText("Тело листа").assertDoesNotExist()
        }

    @Test
    fun numberShowsValueAndUnitAsSeparateRuns() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    // A dose is {value, unit}: two pieces of data, never one
                    // pre-formatted string. The comma is a formatter's decision.
                    CadenceNumber(value = "0,25", unit = "мг")
                }
            }

            onNodeWithText("0,25").assertIsDisplayed()
            onNodeWithText("мг").assertIsDisplayed()
        }

    @Test
    fun eyebrowUppercasesWithoutTouchingTheSourceString() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceEyebrow("сегодня")
                }
            }

            onNodeWithText("СЕГОДНЯ").assertIsDisplayed()
        }

    @Test
    fun themePublishesTheProtocolPalette() =
        runComposeUiTest {
            var seen: CadencePalette? = null
            setContent {
                CadenceTheme {
                    seen = LocalCadencePalette.current
                }
            }

            val palette = assertNotNull(seen, "CadenceTheme did not publish a palette")
            // Ported values, not invented ones: these two come straight from
            // mobile/src/theme/index.ts.
            assertEquals(Color(0xFFFBF8F3), palette.paper, "paper drifted from the prototype")
            assertEquals(Color(0xFF142C1F), palette.forestDeep, "forest900 drifted from the prototype")
        }

    @Test
    fun everyPrototypeIconParses() {
        // The 41 icons came over from mobile/src/components/icon-paths.ts as
        // data. A path that fails to parse draws nothing at runtime and nothing
        // says so — which is why they are parsed here rather than trusted.
        assertEquals(41, CadenceIcons.byName.size, "the icon set changed size")

        CadenceIcons.byName.forEach { (name, paths) ->
            assertTrue(paths.isNotEmpty(), "icon '$name' has no path data")
            paths.forEach { data ->
                val parsed = PathParser().parsePathString(data).toPath()
                assertFalse(parsed.isEmpty, "icon '$name' parsed to an empty path")
            }
        }
    }
}
