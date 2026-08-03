package app.cadence.design

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontListFontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.TextUnitType
import androidx.compose.ui.unit.isUnspecified
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class DesignSystemTest {
    @Test
    fun aDisabledButtonSaysSoAndSwallowsTheTap() =
        runComposeUiTest {
            // Both halves. `assertIsNotEnabled` checks a semantics property and
            // nothing else, so a button that merely *claims* to be disabled
            // while still firing its handler passes it — and the dose wizard's
            // «Сохранить дозу» would submit an incomplete draft.
            var clicks = 0

            setContent {
                CadenceTheme { CadenceButton(label = "Дальше", onClick = { clicks++ }, enabled = false) }
            }

            onNodeWithText("Дальше").assertIsNotEnabled()
            onNodeWithText("Дальше").performClick()

            assertEquals(0, clicks, "a disabled button fired its handler")
        }

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
    fun leadingAndTrackingFollowTheSizeOnEveryStyleThatSetsThem() =
        // The scale resolves inside composition, so it is read through the theme.
        runComposeUiTest {
            val typography = themedTypography()

            // Stored as ratios, so an override rescales them. Absolute sp leading
            // would leave a 34sp title with 30sp leading, and its lines overlap.
            //
            // `number` is checked too, and not for symmetry: CadenceNumber
            // overrides its fontSize on every call site, so it is the second
            // style where this contract is actually exercised.
            listOf(
                "title" to typography.title,
                "titleEmphasis" to typography.titleEmphasis,
                "body" to typography.body,
                "number" to typography.number,
            ).forEach { (role, style) ->
                assertEquals(
                    TextUnitType.Em,
                    style.lineHeight.type,
                    "$role line height is not a ratio",
                )
            }

            listOf("title" to typography.title, "number" to typography.number)
                .forEach { (role, style) ->
                    assertEquals(
                        TextUnitType.Em,
                        style.letterSpacing.type,
                        "$role tracking is not a ratio",
                    )
                }
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
    fun anEmphasisedTitleIsOneRunOfTextRatherThanThreePieces() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    // As the prototype writes it: «Кому напишем?», with only the
                    // middle word in the italic display face.
                    CadenceTitle(cadenceEmphasisedTitle(prefix = "Кому ", emphasis = "напишем", suffix = "?"))
                }
            }

            // One node, not three. Three side-by-side composables would neither
            // wrap as a paragraph nor be found by the whole string.
            onNodeWithText("Кому напишем?").assertIsDisplayed()
        }

    @Test
    fun theEmphasisedRunTakesTheDrawnItalicFaceAndNothingElseDoes() =
        runComposeUiTest {
            var italicFace: FontFamily? = null
            var uprightFace: FontFamily? = null
            setContent {
                CadenceTheme {
                    italicFace = Cadence.typography.titleEmphasis.fontFamily
                    uprightFace = Cadence.typography.title.fontFamily
                    CadenceTitle(cadenceEmphasisedTitle(prefix = "Ваша ", emphasis = "аптечка"))
                }
            }

            // Read back out of the composed tree, not out of the builder. Read
            // from the builder, this passed even when the title overload threw
            // the spans away before drawing.
            val text =
                onNodeWithText("Ваша аптечка")
                    .fetchSemanticsNode()
                    .config[SemanticsProperties.Text]
                    .single()
            val spans = text.spanStyles
            assertEquals(1, spans.size, "an emphasised title carries exactly one span")

            // The span covers the emphasis word and nothing around it.
            assertEquals("аптечка", text.text.substring(spans[0].start, spans[0].end))

            // Sharing the upright family would leave Compose to slant the
            // emphasis synthetically instead of using the drawn italic file.
            assertEquals(italicFace, spans[0].item.fontFamily, "the emphasis is not the italic face")
            assertNotEquals(uprightFace, spans[0].item.fontFamily, "the emphasis shares the upright face")
            // The rest of the span, which a hand-picked three properties lost:
            // the accent colour, and the weight without which Cormorant falls
            // back to its own Light default instance.
            assertEquals(CadenceColors.forest700, spans[0].item.color, "the emphasis lost its accent")
            assertEquals(FontWeight.Medium, spans[0].item.fontWeight, "the emphasis lost its weight")
            // Unspecified on purpose: the size belongs to the call site, and a
            // baked 28sp would sit a fat word inside a 22sp line.
            assertTrue(spans[0].item.fontSize.isUnspecified, "the emphasis pins its own size")
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
    fun aStaticCardStillRendersItsContent() =
        runComposeUiTest {
            // The `onClick == null` branch exists to avoid starting an
            // interaction collector per item in a list — a deliberate
            // optimisation whose only exercise was the showcase. The audit
            // that replaced the showcase went component by component and so
            // missed the parameter combinations it had been executing.
            setContent {
                CadenceTheme {
                    CadenceCard { CadenceBody("Статическая карточка") }
                }
            }

            // The click action is the assertion: `clickable` publishes one and
            // merges descendants, so a card given a no-op handler instead of
            // the null branch is distinguishable after all — the first version
            // of this test recorded that as an accepted limit and was wrong.
            onNodeWithText("Статическая карточка").assertIsDisplayed().assertHasNoClickAction()
        }

    @Test
    fun aButtonWithAnIconAndFullWidthStillRendersBoth() =
        runComposeUiTest {
            // Same gap: `if (icon != null)` measures the icon through
            // LocalDensity and composes a CadenceIcon, and after the showcase
            // went, no test composed that branch at all.
            setContent {
                CadenceTheme {
                    CadenceButton(
                        label = "Открыть лист",
                        onClick = {},
                        icon = CadenceIcons.plus,
                        fillWidth = true,
                    )
                }
            }

            onNodeWithText("Открыть лист").assertIsDisplayed()
        }

    @Test
    fun everyPillToneRendersItsLabel() =
        runComposeUiTest {
            // The showcase in App.kt used to be the only place a pill was
            // rendered under test. It became the shell in step 2 of the port,
            // and this is where that assertion went — found by auditing what
            // the showcase alone was covering, before deleting it.
            setContent {
                CadenceTheme {
                    Column {
                        CadencePillTone.entries.forEach { CadencePill(it.name, tone = it) }
                    }
                }
            }

            CadencePillTone.entries.forEach {
                onNodeWithText(it.name).assertIsDisplayed()
            }
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
    fun everyRoleIsSetInABundledFaceRatherThanASystemOne() =
        runComposeUiTest {
            val typography = themedTypography()

            // Asserted positively, as a type. A blacklist of the generic
            // singletons would have to name all five — Default, SansSerif,
            // Serif, Monospace, Cursive — and FontFamily.Default is the one a
            // careless revert produces, so the blacklist that names three is
            // exactly the one that lets the likeliest regression through.
            //
            // The cast also gives the weight and the resource to assert below.
            allRoles(typography).forEach { (role, family) ->
                assertNotNull(family, "the $role role has no family at all")
                assertTrue(
                    family is FontListFontFamily,
                    "the $role role is set in a system family — Cyrillic would fall back",
                )
            }
        }

    @Test
    fun theFourFacesStayBoundToTheRolesTheyWereChosenFor() =
        runComposeUiTest {
            val typography = themedTypography()

            // Every role above is a bundled face, which says nothing about
            // *which* one. Pointing the mono face at Golos would set every dose
            // and weight in a proportional face — the column of numbers on the
            // inventory screen stops lining up, and no assertion above notices.
            val distinct =
                listOf(
                    typography.title.fontFamily,
                    typography.body.fontFamily,
                    typography.number.fontFamily,
                    typography.numberUnit.fontFamily,
                )
            assertEquals(
                distinct.size,
                distinct.toSet().size,
                "two roles collapsed onto one face",
            )

            // The display family carries one weight and the prototype pins it at
            // Medium. Cormorant Garamond's own default instance is Light, so a
            // dropped weight renders every title lighter than the design with
            // nothing failing.
            val display = typography.title.fontFamily as FontListFontFamily
            assertEquals(
                FontWeight.Medium,
                display.fonts.single().weight,
                "the display face lost its weight and falls back to the file's Light default",
            )

            // The body family carries three, so `fontWeight` still selects
            // rather than being synthesized over a single member.
            val body = typography.body.fontFamily as FontListFontFamily
            assertEquals(
                listOf(FontWeight.Normal, FontWeight.Medium, FontWeight.SemiBold),
                body.fonts.map { it.weight },
                "the body family stopped offering the weights the prototype uses",
            )
        }

    @Test
    fun typographyReadOutsideTheThemeFailsInsteadOfServingAFallback() =
        runComposeUiTest {
            // The contract this guards is the reason the local has no default.
            // Without a test, the next person whose composable reads a token
            // outside the theme restores a default value, and the Cyrillic
            // fallback comes back silently.
            assertFailsWith<IllegalStateException> {
                setContent { Cadence.typography }
                waitForIdle()
            }
        }

    /**
     * The scale as the theme publishes it.
     *
     * Every typography assertion needs the same seven lines — the scale resolves
     * inside composition now, so it cannot be read from a constant.
     */
    private fun ComposeUiTest.themedTypography(): CadenceTypography {
        var seen: CadenceTypography? = null
        setContent {
            CadenceTheme {
                seen = Cadence.typography
            }
        }
        return assertNotNull(seen, "CadenceTheme did not publish a typography")
    }

    /** All eight roles, so a new one cannot be added without being checked. */
    private fun allRoles(typography: CadenceTypography): List<Pair<String, FontFamily?>> =
        listOf(
            "eyebrow" to typography.eyebrow.fontFamily,
            "title" to typography.title.fontFamily,
            "titleEmphasis" to typography.titleEmphasis.fontFamily,
            "body" to typography.body.fontFamily,
            "meta" to typography.meta.fontFamily,
            "number" to typography.number.fontFamily,
            "numberUnit" to typography.numberUnit.fontFamily,
            "label" to typography.label.fontFamily,
        )

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
