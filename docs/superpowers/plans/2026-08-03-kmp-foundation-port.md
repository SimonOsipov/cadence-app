# KMP Foundation Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring `kmp/composeApp`'s design system from a token showcase to the full component set the frozen prototype's screens will need, and put the three real typefaces behind it.

**Architecture:** The design system already carries the palette, the dimension scales, and 16 composables. This plan adds the eight the prototype has and Compose does not, wires Compose Multiplatform resources so the three chosen typefaces replace the platform defaults, and starts the divergence registry that step 11 of the block will finish. Nothing here knows about navigation, data, or screens — those are steps 2 and 3.

**Tech Stack:** Kotlin Multiplatform 2.4.10, Compose Multiplatform 1.11.1, `compose.components.resources` for fonts, `kotlin.test` + `compose.ui.test` for tests, ktlint + detekt through `scripts/gate/kmp.sh`.

## Global Constraints

- **The prototype is the specification.** `mobile/src/components/{primitives,shared}.tsx` and `mobile/src/theme/index.ts` are read, never edited. Divergence from them is a deliberate decision with a comment on the spot and an entry in the registry — invariant 1 of the `kmp-app` note.
- **No new colours or fonts.** Everything comes from `CadenceColors` / `CadencePalette` / `CadenceTypography`. A screen needing a colour the palette lacks is a design question, not a coding one.
- **No Material.** `libs.versions.toml` states it: `BasicText`, `Box`, `Row`, `Column` and the project's own primitives only. No ripple — press feedback is the 0.98 scale in `Modifier.pressable`.
- **RU is the product language.** Every user-visible string is Russian. Code, comments, and commit messages are English.
- **Numbers are data, formatting is presentation.** A dose is `{value, unit}`; `«0,25 мг»` is assembled by a formatter, never baked into a string.
- **Compose UI tests run on the iOS simulator target only.** The Android target deliberately has no host-test builder (`kmp-wiring` spec, "What already exists"). A test that must prove rendering runs there; anything provable without a runtime is a plain `kotlin.test`.
- **The gate is `./scripts/gate/kmp.sh`** — ktlint, detekt, `testAndroidHostTest`, `:androidApp:assembleDebug`. It must be green at the end of every task.
- **Minimum Android SDK is 28**, which is what makes variable-font weight selection available at all (`FontVariation` needs API 26+).

---

## File Structure

**Created:**
- `kmp/composeApp/src/commonMain/composeResources/font/GolosText.ttf` — body face, variable weight axis
- `kmp/composeApp/src/commonMain/composeResources/font/CormorantGaramond.ttf` — display face
- `kmp/composeApp/src/commonMain/composeResources/font/CormorantGaramondItalic.ttf` — display italic
- `kmp/composeApp/src/commonMain/composeResources/font/JetBrainsMono.ttf` — mono face
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceLayout.kt` — `CadenceSection`, `CadenceListRow`
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceHeaders.kt` — `CadenceAppHeader`, `CadenceScreenHeader`
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceCharts.kt` — `CadenceSpark`, `CadenceCycleRing`
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTabBar.kt` — `CadenceTabBar`, `CadenceTab`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/LayoutTest.kt`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/HeadersTest.kt`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/ChartsTest.kt`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/TabBarTest.kt`
- `docs/prototype-divergences.md` — the registry step 11 finishes

**Modified:**
- `kmp/gradle/libs.versions.toml` — add the `compose-resources` library alias
- `kmp/composeApp/build.gradle.kts` — the resources dependency and the `compose.resources` block
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTypography.kt` — `CadenceFonts` becomes composable-provided; the open-question comment becomes a recorded decision
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTheme.kt` — provides the typography built from real fonts
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceText.kt` — add `CadenceTitleEmphasis`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt` — the composable count assertion moves from 16

**Read, never edited:** everything under `mobile/`.

**Not in scope, and why:** icons. The prototype's `icon-paths.ts` and `CadenceIcons.kt` were compared name by name on 2026-08-03 — 41 against 41, identical sets, no gap. `CadenceColors.kt` likewise matches `mobile/src/theme/index.ts` value for value; the only divergence is against the *web* CSS, and it is recorded rather than fixed (Task 2).

---

### Task 1: The three real typefaces

Closes the tail task `6h8w8Q5C8P9CfCrq`. Today `CadenceFonts` points at `FontFamily.Serif`/`SansSerif`/`Monospace` with a comment saying the choice is open. The choice was made on 2026-07-28: Golos Text for body, because DM Sans ships no Cyrillic and every string the product shows is Russian.

**Files:**
- Create: `kmp/composeApp/src/commonMain/composeResources/font/{GolosText,CormorantGaramond,CormorantGaramondItalic,JetBrainsMono}.ttf`
- Modify: `kmp/gradle/libs.versions.toml`
- Modify: `kmp/composeApp/build.gradle.kts:33-49`
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTypography.kt:11-33,64-124`
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTheme.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `@Composable fun cadenceTypography(): CadenceTypography` in `CadenceTypography.kt`, replacing the top-level `val CadenceDefaultTypography`. `Cadence.typography` keeps its current type and call sites are unchanged — only the construction site moves into composition, because Compose Multiplatform's `Font()` for a bundled resource is a `@Composable` function and cannot run at object-init time.

- [ ] **Step 1: Fetch the four typefaces**

All four are OFL and shipped by Google Fonts as variable fonts with a weight axis; static instances are not published for these families, which is why the weight is selected through `FontVariation` rather than by picking a file.

```bash
cd kmp/composeApp/src/commonMain
mkdir -p composeResources/font
base=https://raw.githubusercontent.com/google/fonts/main/ofl
curl -fsSL "$base/golostext/GolosText%5Bwght%5D.ttf"                        -o composeResources/font/GolosText.ttf
curl -fsSL "$base/cormorantgaramond/CormorantGaramond%5Bwght%5D.ttf"        -o composeResources/font/CormorantGaramond.ttf
curl -fsSL "$base/cormorantgaramond/CormorantGaramond-Italic%5Bwght%5D.ttf" -o composeResources/font/CormorantGaramondItalic.ttf
curl -fsSL "$base/jetbrainsmono/JetBrainsMono%5Bwght%5D.ttf"                -o composeResources/font/JetBrainsMono.ttf
file composeResources/font/*.ttf   # expect "TrueType Font data" four times, not HTML
```

- [ ] **Step 2: Declare the resources dependency**

In `kmp/gradle/libs.versions.toml`, under `[libraries]`, after the `compose-ui-test` line:

```toml
compose-resources = { module = "org.jetbrains.compose.components:components-resources", version.ref = "compose-multiplatform" }
```

In `kmp/composeApp/build.gradle.kts`, add to `commonMain.dependencies` (after `libs.compose.ui.backhandler`):

```kotlin
            implementation(libs.compose.resources)
```

and after the closing brace of the `kotlin { }` block:

```kotlin
compose.resources {
    // Internal: the generated accessor is an implementation detail of the
    // design system, not part of the module's surface.
    publicResClass = false
    packageOfResClass = "app.cadence.design.generated"
}
```

- [ ] **Step 3: Run the gate to verify the resources generate**

Run: `./scripts/gate/kmp.sh`
Expected: PASS. The build now generates `app.cadence.design.generated.Res`. If it fails with "unresolved reference: Res", the resource directory name is wrong — it must be exactly `composeResources/font`.

- [ ] **Step 4: Write the failing test**

Add to `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt`:

```kotlin
    @Test
    fun typographyUsesTheProductTypefacesRatherThanPlatformDefaults() =
        runComposeUiTest {
            var typography: CadenceTypography? = null
            setContent {
                CadenceTheme {
                    typography = Cadence.typography
                }
            }

            val resolved = requireNotNull(typography)
            // A platform default is a generic FontFamily singleton; a bundled
            // face is a FontListFontFamily. Comparing against the three
            // singletons is what makes the assertion fail today and pass once
            // the real faces are loaded.
            val defaults = setOf(FontFamily.Serif, FontFamily.SansSerif, FontFamily.Monospace)
            assertTrue(resolved.title.fontFamily !in defaults, "display face is still a platform default")
            assertTrue(resolved.body.fontFamily !in defaults, "body face is still a platform default")
            assertTrue(resolved.number.fontFamily !in defaults, "mono face is still a platform default")
        }
```

- [ ] **Step 5: Run the test to verify it fails**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "*typographyUsesTheProductTypefaces*"`
Expected: FAIL with "display face is still a platform default".

- [ ] **Step 6: Replace the platform defaults**

In `CadenceTypography.kt`, replace the `CadenceFonts` object (lines 11–33) with:

```kotlin
import androidx.compose.runtime.Composable
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontVariation
import app.cadence.design.generated.Res
import app.cadence.design.generated.cormorantgaramond
import app.cadence.design.generated.cormorantgaramonditalic
import app.cadence.design.generated.golostext
import app.cadence.design.generated.jetbrainsmono
import org.jetbrains.compose.resources.Font

/**
 * The three faces of the product, resolved once per composition.
 *
 * DECIDED 2026-07-28 — the body face is Golos Text, not the prototype's DM
 * Sans: DM Sans ships latin and latin-ext only, and every string this product
 * shows a user is Russian. The prototype hit the same wall on the display face
 * (Instrument Serif has no Cyrillic) and fell back to Cormorant Garamond;
 * nobody had run the check on the body face. Golos Text was designed for
 * Cyrillic rather than extended into it, is OFL, and sits close to DM Sans in
 * width and tone, so the prototype's layouts do not shift.
 *
 * All three ship as variable fonts — Google Fonts publishes no static
 * instances for these families — so a weight is a variation setting on one
 * file rather than a file of its own.
 */
@Composable
private fun face(
    resource: org.jetbrains.compose.resources.FontResource,
    weight: FontWeight,
): FontFamily =
    FontFamily(
        Font(
            resource = resource,
            weight = weight,
            variationSettings = FontVariation.Settings(FontVariation.weight(weight.weight)),
        ),
    )

@Composable
internal fun displayFace(): FontFamily = face(Res.font.cormorantgaramond, FontWeight.Medium)

@Composable
internal fun displayItalicFace(): FontFamily = face(Res.font.cormorantgaramonditalic, FontWeight.Medium)

@Composable
internal fun bodyFace(weight: FontWeight): FontFamily = face(Res.font.golostext, weight)

@Composable
internal fun monoFace(weight: FontWeight): FontFamily = face(Res.font.jetbrainsmono, weight)
```

- [ ] **Step 7: Turn the typography into a composable factory**

In the same file, replace `val CadenceDefaultTypography = CadenceTypography(...)` (lines 64–124) with a function of the same body. Only the two lines below change per style — each `fontFamily = CadenceFonts.x` becomes a call:

```kotlin
@Composable
fun cadenceTypography(): CadenceTypography {
    val display = displayFace()
    val displayItalic = displayItalicFace()
    val body = bodyFace(FontWeight.Normal)
    val bodyMedium = bodyFace(FontWeight.Medium)
    val mono = monoFace(FontWeight.Medium)

    return CadenceTypography(
        eyebrow =
            TextStyle(
                fontFamily = bodyMedium,
                fontWeight = FontWeight.Medium,
                fontSize = 11.sp,
                letterSpacing = 0.14.em,
            ),
        title =
            TextStyle(
                fontFamily = display,
                fontSize = CadenceTitleSize,
                lineHeight = 1.08.em,
                letterSpacing = (-0.018).em,
            ),
        titleEmphasis =
            TextStyle(
                fontFamily = displayItalic,
                fontStyle = FontStyle.Italic,
                fontSize = CadenceTitleSize,
                lineHeight = 1.08.em,
                letterSpacing = (-0.018).em,
            ),
        body =
            TextStyle(
                fontFamily = body,
                fontSize = CadenceBodySize,
                lineHeight = 1.5.em,
            ),
        meta =
            TextStyle(
                fontFamily = body,
                fontSize = 12.sp,
            ),
        number =
            TextStyle(
                fontFamily = mono,
                fontWeight = FontWeight.Medium,
                fontSize = CadenceNumberSize,
                lineHeight = 1.05.em,
                letterSpacing = (-0.03).em,
            ),
        numberUnit =
            TextStyle(
                fontFamily = displayItalic,
                fontStyle = FontStyle.Italic,
                fontSize = CadenceNumberSize * CADENCE_NUMBER_UNIT_RATIO,
            ),
        label =
            TextStyle(
                fontFamily = bodyMedium,
                fontWeight = FontWeight.Medium,
                fontSize = 14.sp,
            ),
    )
}
```

Then in `CadenceTheme.kt`, replace the reference to `CadenceDefaultTypography` with a call to `cadenceTypography()`. Read the file first — it is 49 lines and the provider is a single `CompositionLocalProvider` entry.

- [ ] **Step 8: Run the test to verify it passes**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "*typographyUsesTheProductTypefaces*"`
Expected: PASS.

- [ ] **Step 9: Run the gate**

Run: `./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 10: Commit**

```bash
git add kmp/gradle/libs.versions.toml kmp/composeApp/build.gradle.kts \
        kmp/composeApp/src/commonMain/composeResources kmp/composeApp/src/commonMain/kotlin/app/cadence/design \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt
git commit -m "feat(kmp): the type is the product's own, and Cyrillic no longer falls back"
```

**Not proven by this task, and it must be said:** that the glyphs render in the chosen faces on a device rather than merely resolving to a non-default `FontFamily`. That needs a run on an emulator and a simulator, which is the tail task's own acceptance criterion ("checked by running, not only by building"). Record it as outstanding in the step report.

---

### Task 2: The divergence registry, opened with what is already known

Invariant 1 of the `kmp-app` note requires every divergence from the prototype to be a deliberate decision rather than an accident. Step 11 of the block assembles the registry; it is opened here, because the first three entries already exist and would otherwise be reconstructed from memory in a month.

**Files:**
- Create: `docs/prototype-divergences.md`
- Test: none — this is a document, and the assertions that back its entries live in the tasks that create them.

**Interfaces:**
- Consumes: nothing.
- Produces: a document later tasks append to. The format is one `##` heading per divergence with **What the prototype does**, **What we do**, **Why**.

- [ ] **Step 1: Write the registry with the three entries that are already true**

```markdown
# Divergences from the frozen prototype

`mobile/` is the visual specification for the patient app. A difference between
it and `kmp/` is a decision, not an accident — invariant 1 of the `kmp-app`
architecture note. Every one of them is written down here, with the reason.

Step 11 of the port block reviews this document against a side-by-side run.

## Body typeface: Golos Text, not DM Sans

**What the prototype does:** `F.body` is `DMSans_400Regular`.
**What we do:** Golos Text, at the same three weights.
**Why:** DM Sans publishes `latin` and `latin-ext` only. Every string this
product shows a user is Russian, so DM Sans would fall back to a system face
for all of it. The prototype had already hit this on the display face
(Instrument Serif → Cormorant Garamond) and did not re-run the check on the
body face. Decision recorded 2026-07-28 in `specs/dashboard-skeleton.md`.

## Card shadows: the step survives, the warm tint does not

**What the prototype does:** `SHADOW` in `theme/index.ts` is a warm umbra,
`rgba(46,38,24,…)`, with an explicit colour, opacity, radius, and offset.
**What we do:** `CadenceElevation` carries the step only; Compose takes an
elevation and derives the shadow per platform.
**Why:** Compose's `Modifier.shadow` has no tint parameter that behaves the
same on both platforms. Re-implementing the umbra by hand would mean drawing
our own shadow on every card. Accepted at the design-system port (BST-05).

## `border` and `ink700`: the mobile theme wins, the web CSS diverges

**What the prototype does:** `mobile/src/theme/index.ts` sets `border` to
`#cdc1a8` and defines `ink700` as `#3f3b35`.
**What we do:** exactly the same — `CadenceColors.kt` matches the mobile theme
value for value.
**Why it is listed at all:** the *web* prototype's `colors_and_type.css` sets
`--border` to `#e4dac6` (our `bone`) and has no `ink-700` at all; `#cdc1a8` is
its `--border-strong`. So the two prototypes disagree with each other, and the
mobile one is canon for the mobile app. Nothing to change here; the web side
resolves it in its own token port (`dashboard-skeleton`, step 2). Verified
2026-08-03.
```

- [ ] **Step 2: Commit**

```bash
git add docs/prototype-divergences.md
git commit -m "docs(kmp): open the divergence registry with what is already decided"
```

---

### Task 3: `CadenceTitleEmphasis` — italic emphasis inside a title

The prototype's `TitleEm` is not `Title` with `italic=true`: it is an inline run set in serif italic and forest green *within* a surrounding title, as in «Выберите ритм.». `CadenceTitle(italic = true)` italicises the whole line and cannot express it.

**Files:**
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceText.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt`

**Interfaces:**
- Consumes: `Cadence.typography.titleEmphasis`, `CadenceTitleSize` (Task 1).
- Produces: `@Composable fun CadenceTitleEmphasis(text: String, modifier: Modifier = Modifier, size: TextUnit = CadenceTitleSize, color: Color = CadenceColors.forest700)`.

- [ ] **Step 1: Write the failing test**

```kotlin
    @Test
    fun titleEmphasisRendersItsText() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceTitleEmphasis("ритм")
                }
            }
            onNodeWithText("ритм").assertIsDisplayed()
        }
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "*titleEmphasisRendersItsText*"`
Expected: FAIL — "unresolved reference: CadenceTitleEmphasis".

- [ ] **Step 3: Implement it**

Append to `CadenceText.kt`:

```kotlin
/**
 * A serif-italic run set inside a title — «Выберите ритм.» in the prototype.
 *
 * Distinct from `CadenceTitle(italic = true)`, which italicises the whole
 * line: this is the emphasis word alone, and it carries the forest accent.
 */
@Composable
fun CadenceTitleEmphasis(
    text: String,
    modifier: Modifier = Modifier,
    size: TextUnit = CadenceTitleSize,
    color: Color = CadenceColors.forest700,
) {
    BasicText(
        text = text,
        modifier = modifier,
        style = Cadence.typography.titleEmphasis.copy(color = color, fontSize = size),
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
    )
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "*titleEmphasisRendersItsText*"`
Expected: PASS.

- [ ] **Step 5: Run the gate and commit**

```bash
./scripts/gate/kmp.sh
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceText.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt
git commit -m "feat(kmp): a title can emphasise one word without italicising the line"
```

---

### Task 4: `CadenceSection` and `CadenceListRow`

The two layout primitives every screen repeats. `Section` is an eyebrow with an optional trailing action; `ListRow` is a tinted icon tile, a title with an optional subtitle, and an optional right-hand value with its own subtitle.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceLayout.kt`
- Create: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/LayoutTest.kt`

**Interfaces:**
- Consumes: `CadenceEyebrow`, `CadenceIcon`, `Cadence.palette`, `Cadence.typography`, `Modifier.pressable` (internal, `CadenceControls.kt`), `CadenceSpacing`, `CadenceRadius`.
- Produces:
  - `@Composable fun CadenceSection(modifier: Modifier = Modifier, title: String? = null, action: String? = null, onAction: (() -> Unit)? = null, content: @Composable ColumnScope.() -> Unit)`
  - `enum class CadenceRowTone { FOREST, SAND, LINEN, DANGER }`
  - `@Composable fun CadenceListRow(icon: List<String>, title: String, modifier: Modifier = Modifier, tone: CadenceRowTone = CadenceRowTone.FOREST, subtitle: String? = null, trailing: String? = null, trailingSubtitle: String? = null, onClick: (() -> Unit)? = null)`

- [ ] **Step 1: Write the failing tests**

Create `LayoutTest.kt`:

```kotlin
package app.cadence.design

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals

@OptIn(ExperimentalTestApi::class)
class LayoutTest {
    @Test
    fun sectionShowsItsTitleUppercasedAndFiresItsAction() =
        runComposeUiTest {
            var taps = 0
            setContent {
                CadenceTheme {
                    CadenceSection(title = "сегодня", action = "всё", onAction = { taps++ }) {
                        CadenceBody("тело секции")
                    }
                }
            }
            // The eyebrow uppercases at render, as it does everywhere else.
            onNodeWithText("СЕГОДНЯ").assertIsDisplayed()
            onNodeWithText("тело секции").assertIsDisplayed()
            onNodeWithText("всё").performClick()
            assertEquals(1, taps)
        }

    @Test
    fun sectionWithoutATitleOrActionStillShowsItsContent() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceSection {
                        CadenceBody("только тело")
                    }
                }
            }
            onNodeWithText("только тело").assertIsDisplayed()
        }

    @Test
    fun listRowShowsEveryTextSlotItWasGiven() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceListRow(
                        icon = CadenceIcons.byName.getValue("beaker"),
                        title = "Семаглутид",
                        subtitle = "вскрыт 12 дней назад",
                        trailing = "0,25",
                        trailingSubtitle = "мг",
                    )
                }
            }
            onNodeWithText("Семаглутид").assertIsDisplayed()
            onNodeWithText("вскрыт 12 дней назад").assertIsDisplayed()
            onNodeWithText("0,25").assertIsDisplayed()
            onNodeWithText("мг").assertIsDisplayed()
        }

    @Test
    fun listRowIsTappableOnlyWhenGivenAnOnClick() =
        runComposeUiTest {
            var taps = 0
            setContent {
                CadenceTheme {
                    CadenceListRow(
                        icon = CadenceIcons.byName.getValue("beaker"),
                        title = "Семаглутид",
                        onClick = { taps++ },
                    )
                }
            }
            onNodeWithText("Семаглутид").performClick()
            assertEquals(1, taps)
        }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.LayoutTest"`
Expected: FAIL — "unresolved reference: CadenceSection".

- [ ] **Step 3: Implement both**

Create `CadenceLayout.kt`:

```kotlin
package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// Section and list row, ported from mobile/src/components/primitives.tsx.

/**
 * A titled block: an eyebrow, an optional trailing action, and content.
 *
 * The action is a plain text link rather than a button — that is what the
 * prototype draws, and a button here would compete with the content.
 */
@Composable
fun CadenceSection(
    modifier: Modifier = Modifier,
    title: String? = null,
    action: String? = null,
    onAction: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (title != null || action != null) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.xxs),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.Bottom,
            ) {
                if (title != null) CadenceEyebrow(title) else Box(Modifier)
                if (action != null && onAction != null) {
                    val interactionSource = remember { MutableInteractionSource() }
                    BasicText(
                        text = action,
                        modifier = Modifier.pressable(onAction, interactionSource),
                        style =
                            Cadence.typography.label.copy(
                                color = CadenceColors.forest700,
                                fontSize = 12.sp,
                            ),
                        maxLines = 1,
                    )
                }
            }
        }
        content()
    }
}

enum class CadenceRowTone { FOREST, SAND, LINEN, DANGER }

private data class RowColors(val background: Color, val foreground: Color)

/**
 * A row with a tinted icon tile, a title, and an optional measured value.
 *
 * The trailing value is set in tabular mono, like every measured value in the
 * product — it lines up down a list, which is the whole reason it is mono.
 */
@Composable
fun CadenceListRow(
    icon: List<String>,
    title: String,
    modifier: Modifier = Modifier,
    tone: CadenceRowTone = CadenceRowTone.FOREST,
    subtitle: String? = null,
    trailing: String? = null,
    trailingSubtitle: String? = null,
    onClick: (() -> Unit)? = null,
) {
    val colors =
        when (tone) {
            CadenceRowTone.FOREST -> RowColors(CadenceColors.forest50, CadenceColors.forest700)
            CadenceRowTone.SAND -> RowColors(CadenceColors.sand100, CadenceColors.sand900)
            CadenceRowTone.LINEN -> RowColors(CadenceColors.linen, CadenceColors.ink700)
            CadenceRowTone.DANGER -> RowColors(CadenceColors.dangerBg, CadenceColors.danger)
        }
    val interactionSource = remember { MutableInteractionSource() }

    Row(
        modifier =
            modifier
                .then(if (onClick != null) Modifier.pressable(onClick, interactionSource) else Modifier)
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = CadenceSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier =
                Modifier
                    .size(40.dp)
                    .background(colors.background, RoundedCornerShape(CadenceRadius.md)),
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(paths = icon, size = 20.dp, tint = colors.foreground)
        }

        Column(modifier = Modifier.weight(1f)) {
            BasicText(
                text = title,
                style = Cadence.typography.label.copy(color = Cadence.palette.ink),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (subtitle != null) {
                CadenceMeta(subtitle, Modifier.padding(top = 1.dp), maxLines = 1)
            }
        }

        if (trailing != null) {
            Column(horizontalAlignment = Alignment.End) {
                BasicText(
                    text = trailing,
                    style =
                        Cadence.typography.number.copy(
                            color = Cadence.palette.ink2,
                            fontSize = 13.sp,
                        ),
                    maxLines = 1,
                )
                if (trailingSubtitle != null) {
                    BasicText(
                        text = trailingSubtitle,
                        style =
                            Cadence.typography.meta.copy(
                                color = Cadence.palette.subtle,
                                fontSize = 11.sp,
                            ),
                        maxLines = 1,
                    )
                }
            }
        }
    }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.LayoutTest"`
Expected: PASS, 4 tests.

- [ ] **Step 5: Run the gate and commit**

```bash
./scripts/gate/kmp.sh
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceLayout.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/LayoutTest.kt
git commit -m "feat(kmp): sections and list rows, the two shapes every screen repeats"
```

---

### Task 5: `CadenceAppHeader` and `CadenceScreenHeader`

The two headers. The app header is the home screen's greeting, large serif name, bell, and avatar; the screen header is the back chevron with an eyebrow and title used by every pushed screen.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceHeaders.kt`
- Create: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/HeadersTest.kt`

**Interfaces:**
- Consumes: `CadenceTitle`, `CadenceMeta`, `CadenceEyebrow`, `CadenceIconButton`, `CadenceIcons`, `Modifier.pressable`.
- Produces:
  - `@Composable fun CadenceAppHeader(greeting: String, name: String, avatarInitial: String, onAvatarClick: () -> Unit, onBellClick: () -> Unit, modifier: Modifier = Modifier)`
  - `@Composable fun CadenceScreenHeader(title: String, onBack: () -> Unit, modifier: Modifier = Modifier, eyebrow: String? = null, trailing: (@Composable () -> Unit)? = null)`

**Note on the avatar initial:** it is a parameter, not derived here. `identity`'s note states initials are derived from `full_name` by one module per surface and are never stored; that module does not exist yet, so the header takes what it is given and the derivation lands with the profile data in a later step.

- [ ] **Step 1: Write the failing tests**

Create `HeadersTest.kt`:

```kotlin
package app.cadence.design

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals

@OptIn(ExperimentalTestApi::class)
class HeadersTest {
    @Test
    fun appHeaderShowsGreetingNameAndReachesBothControls() =
        runComposeUiTest {
            var bells = 0
            var avatars = 0
            setContent {
                CadenceTheme {
                    CadenceAppHeader(
                        greeting = "Доброе утро",
                        name = "Марина",
                        avatarInitial = "М",
                        onAvatarClick = { avatars++ },
                        onBellClick = { bells++ },
                    )
                }
            }
            onNodeWithText("Доброе утро").assertIsDisplayed()
            onNodeWithText("Марина").assertIsDisplayed()
            onNodeWithContentDescription("Напоминания").performClick()
            onNodeWithText("М").performClick()
            assertEquals(1, bells)
            assertEquals(1, avatars)
        }

    @Test
    fun screenHeaderShowsEyebrowAndTitleAndGoesBack() =
        runComposeUiTest {
            var backs = 0
            setContent {
                CadenceTheme {
                    CadenceScreenHeader(
                        title = "Аптечка",
                        eyebrow = "запас",
                        onBack = { backs++ },
                    )
                }
            }
            onNodeWithText("ЗАПАС").assertIsDisplayed()
            onNodeWithText("Аптечка").assertIsDisplayed()
            onNodeWithContentDescription("Назад").performClick()
            assertEquals(1, backs)
        }

    @Test
    fun screenHeaderWithoutAnEyebrowStillShowsItsTitle() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceScreenHeader(title = "Профиль", onBack = {})
                }
            }
            onNodeWithText("Профиль").assertIsDisplayed()
        }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.HeadersTest"`
Expected: FAIL — "unresolved reference: CadenceAppHeader".

- [ ] **Step 3: Implement both**

Create `CadenceHeaders.kt`:

```kotlin
package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// The two headers, ported from mobile/src/components/shared.tsx.

/**
 * The home screen's header: a greeting, the patient's name in display type,
 * and the two controls that sit beside it.
 *
 * [avatarInitial] is passed in rather than derived. Initials are a derived
 * value owned by one module per surface (`identity` note) and that module does
 * not exist yet — deriving it here would create the second source the note
 * forbids.
 */
@Composable
fun CadenceAppHeader(
    greeting: String,
    name: String,
    avatarInitial: String,
    onAvatarClick: () -> Unit,
    onBellClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }

    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(
                    start = CadenceSpacing.xl,
                    end = CadenceSpacing.xl,
                    top = CadenceSpacing.sm,
                    bottom = 18.dp,
                ),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        verticalAlignment = Alignment.Bottom,
    ) {
        Column(modifier = Modifier.weight(1f)) {
            CadenceMeta(greeting, Modifier.padding(bottom = CadenceSpacing.xxs), maxLines = 1)
            CadenceTitle(name, size = 32.sp, maxLines = 1)
        }

        Row(
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            CadenceIconButton(
                icon = CadenceIcons.byName.getValue("bell"),
                contentDescription = "Напоминания",
                onClick = onBellClick,
                background = Cadence.palette.sunk,
            )
            Box(
                modifier =
                    Modifier
                        .pressable(onAvatarClick, interactionSource)
                        .size(40.dp)
                        .background(CadenceColors.forest700, RoundedCornerShape(CadenceRadius.pill)),
                contentAlignment = Alignment.Center,
            ) {
                BasicText(
                    text = avatarInitial,
                    style =
                        Cadence.typography.titleEmphasis.copy(
                            color = CadenceColors.cream,
                            fontSize = 18.sp,
                        ),
                    maxLines = 1,
                )
            }
        }
    }
}

/** The header every pushed screen wears: back, an optional eyebrow, a title. */
@Composable
fun CadenceScreenHeader(
    title: String,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    eyebrow: String? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .padding(
                    horizontal = CadenceSpacing.md,
                    vertical = 0.dp,
                ).padding(top = CadenceSpacing.xs, bottom = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CadenceIconButton(
            icon = CadenceIcons.byName.getValue("chevron-left"),
            contentDescription = "Назад",
            onClick = onBack,
            background = Cadence.palette.sunk,
        )
        Column(modifier = Modifier.weight(1f)) {
            if (eyebrow != null) {
                CadenceEyebrow(eyebrow, Modifier.padding(bottom = 2.dp))
            }
            CadenceTitle(title, size = 24.sp, maxLines = 1)
        }
        trailing?.invoke()
    }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.HeadersTest"`
Expected: PASS, 3 tests. If `chevron-left` or `bell` throws `NoSuchElementException`, the icon name differs — list `CadenceIcons.byName.keys` and use the actual key.

- [ ] **Step 5: Run the gate and commit**

```bash
./scripts/gate/kmp.sh
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceHeaders.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/HeadersTest.kt
git commit -m "feat(kmp): the two headers every screen lands under"
```

---

### Task 6: `CadenceSpark` and `CadenceCycleRing`

The two drawn shapes in the foundation. Both are `Canvas` — the prototype uses SVG, which has no Compose equivalent, so the geometry is ported as drawing commands. `ScrubChart` is *not* here: it belongs to step 7 of the block, and it needs a gesture as well as a canvas.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceCharts.kt`
- Create: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/ChartsTest.kt`

**Interfaces:**
- Consumes: `Cadence.palette`, `CadenceColors`.
- Produces:
  - `@Composable fun CadenceSpark(data: List<Float>, modifier: Modifier = Modifier, color: Color = CadenceColors.forest700, fill: Color? = null, width: Dp = 120.dp, height: Dp = 36.dp)`
  - `@Composable fun CadenceCycleRing(week: Int, modifier: Modifier = Modifier, total: Int = 12, size: Dp = 132.dp, stroke: Dp = 10.dp, color: Color = CadenceColors.forest700)`

**The geometry, copied from the prototype so it is not re-derived:**
- Spark: 2px padding on all sides; `x[i] = pad + i * (width - 2*pad) / (n - 1)`; `y[i] = pad + (max - v) / (max - min || 1) * (height - 2*pad)`; a 2px round-capped polyline; a 3px dot on the last point; the fill, when given, closes the path down to `height - pad` at 0.4 alpha.
- Ring: radius `(size - stroke) / 2`, centred; a full track circle in `palette.bone`; an arc of `week / total` of the circumference in `color`, round-capped, starting at 12 o'clock; `total` tick marks at `palette.subtle`, 1px, 0.5 alpha, drawn between `r - stroke/2 - 4` and `r - stroke/2 - 1`.

- [ ] **Step 1: Write the failing tests**

A drawn shape has no text to query, so the tests assert what is assertable without pixels: that the composable exists and lays out at the size it was given, and that degenerate inputs do not crash it. Visual fidelity is step 11's side-by-side.

Create `ChartsTest.kt`:

```kotlin
package app.cadence.design

import androidx.compose.foundation.layout.Box
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertHeightIsEqualTo
import androidx.compose.ui.test.assertWidthIsEqualTo
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.runComposeUiTest
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import kotlin.test.Test

@OptIn(ExperimentalTestApi::class)
class ChartsTest {
    @Test
    fun sparkOccupiesTheSizeItWasGiven() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceSpark(
                        data = listOf(0.3f, 0.5f, 0.4f, 0.6f, 0.55f, 0.7f, 0.8f),
                        modifier = Modifier.testTag("spark"),
                        width = 120.dp,
                        height = 36.dp,
                    )
                }
            }
            onNodeWithTag("spark").assertWidthIsEqualTo(120.dp).assertHeightIsEqualTo(36.dp)
        }

    @Test
    fun sparkSurvivesAFlatSeriesAndASinglePoint() =
        runComposeUiTest {
            // A flat series makes (max - min) zero, and one point makes (n - 1)
            // zero. Both divide in the prototype's formula; neither may crash.
            setContent {
                CadenceTheme {
                    Box {
                        CadenceSpark(data = listOf(0.5f, 0.5f, 0.5f), modifier = Modifier.testTag("flat"))
                        CadenceSpark(data = listOf(0.5f), modifier = Modifier.testTag("single"))
                        CadenceSpark(data = emptyList(), modifier = Modifier.testTag("empty"))
                    }
                }
            }
            onNodeWithTag("flat").assertWidthIsEqualTo(120.dp)
            onNodeWithTag("single").assertWidthIsEqualTo(120.dp)
            onNodeWithTag("empty").assertWidthIsEqualTo(120.dp)
        }

    @Test
    fun cycleRingOccupiesTheSizeItWasGiven() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceCycleRing(week = 4, modifier = Modifier.testTag("ring"), size = 132.dp)
                }
            }
            onNodeWithTag("ring").assertWidthIsEqualTo(132.dp).assertHeightIsEqualTo(132.dp)
        }

    @Test
    fun cycleRingClampsAWeekOutsideTheProtocol() =
        runComposeUiTest {
            // Week 0 and week 14 of 12 both arrive from real data during a
            // protocol change; neither may draw an arc longer than the circle
            // or shorter than nothing.
            setContent {
                CadenceTheme {
                    Box {
                        CadenceCycleRing(week = 0, total = 12, modifier = Modifier.testTag("zero"))
                        CadenceCycleRing(week = 14, total = 12, modifier = Modifier.testTag("over"))
                    }
                }
            }
            onNodeWithTag("zero").assertWidthIsEqualTo(132.dp)
            onNodeWithTag("over").assertWidthIsEqualTo(132.dp)
        }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.ChartsTest"`
Expected: FAIL — "unresolved reference: CadenceSpark".

- [ ] **Step 3: Implement both**

Create `CadenceCharts.kt`:

```kotlin
package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlin.math.PI
import kotlin.math.cos
import kotlin.math.sin

// The two drawn shapes of the foundation, ported from
// mobile/src/components/shared.tsx. The prototype draws them in SVG; Compose
// has no SVG, so the geometry is ported as drawing commands and the numbers
// are the prototype's, not re-derived.

private const val SPARK_PAD = 2f
private const val SPARK_FILL_ALPHA = 0.4f

/**
 * An inline trend sparkline: a polyline with a dot on the latest point.
 *
 * Degenerate series are the normal case, not an edge one — a patient with one
 * measurement, or three identical ones — so a flat series and a single point
 * both draw rather than divide by zero.
 */
@Composable
fun CadenceSpark(
    data: List<Float>,
    modifier: Modifier = Modifier,
    color: Color = CadenceColors.forest700,
    fill: Color? = null,
    width: Dp = 120.dp,
    height: Dp = 36.dp,
) {
    Canvas(modifier = modifier.size(width, height)) {
        if (data.isEmpty()) return@Canvas

        val max = data.max()
        val min = data.min()
        val span = if (max == min) 1f else max - min
        val innerWidth = size.width - SPARK_PAD * 2
        val innerHeight = size.height - SPARK_PAD * 2
        val step = if (data.size == 1) 0f else innerWidth / (data.size - 1)

        val points =
            data.mapIndexed { index, value ->
                Offset(
                    x = SPARK_PAD + index * step,
                    y = SPARK_PAD + (max - value) / span * innerHeight,
                )
            }

        if (fill != null && points.size > 1) {
            val area =
                Path().apply {
                    moveTo(points.first().x, points.first().y)
                    points.drop(1).forEach { lineTo(it.x, it.y) }
                    lineTo(points.last().x, size.height - SPARK_PAD)
                    lineTo(SPARK_PAD, size.height - SPARK_PAD)
                    close()
                }
            drawPath(area, fill, alpha = SPARK_FILL_ALPHA)
        }

        if (points.size > 1) {
            val line =
                Path().apply {
                    moveTo(points.first().x, points.first().y)
                    points.drop(1).forEach { lineTo(it.x, it.y) }
                }
            drawPath(line, color, style = Stroke(width = 2f, cap = StrokeCap.Round))
        }

        drawCircle(color, radius = 3f, center = points.last())
    }
}

/**
 * Week N of a protocol as a ring, with a tick per week.
 *
 * The week is clamped: a protocol change can hand this a week outside the
 * range, and an arc longer than the circle reads as a bug in the data rather
 * than in the drawing.
 */
@Composable
fun CadenceCycleRing(
    week: Int,
    modifier: Modifier = Modifier,
    total: Int = 12,
    size: Dp = 132.dp,
    stroke: Dp = 10.dp,
    color: Color = CadenceColors.forest700,
) {
    val palette = Cadence.palette

    Canvas(modifier = modifier.size(size)) {
        if (total <= 0) return@Canvas

        val strokePx = stroke.toPx()
        val radius = (this.size.width - strokePx) / 2f
        val centre = Offset(this.size.width / 2f, this.size.height / 2f)
        val topLeft = Offset(centre.x - radius, centre.y - radius)
        val arcSize = Size(radius * 2, radius * 2)
        val progress = week.coerceIn(0, total).toFloat() / total

        drawCircle(
            color = palette.bone,
            radius = radius,
            center = centre,
            style = Stroke(width = strokePx),
        )

        if (progress > 0f) {
            drawArc(
                color = color,
                // -90 puts the start at twelve o'clock, as the prototype's
                // rotate(-90) does.
                startAngle = -90f,
                sweepAngle = 360f * progress,
                useCenter = false,
                topLeft = topLeft,
                size = arcSize,
                style = Stroke(width = strokePx, cap = StrokeCap.Round),
            )
        }

        val inner = radius - strokePx / 2f - 4f
        val outer = radius - strokePx / 2f - 1f
        repeat(total) { index ->
            val angle = (index.toFloat() / total) * 2f * PI.toFloat() - PI.toFloat() / 2f
            drawLine(
                color = palette.subtle,
                start = Offset(centre.x + cos(angle) * inner, centre.y + sin(angle) * inner),
                end = Offset(centre.x + cos(angle) * outer, centre.y + sin(angle) * outer),
                strokeWidth = 1f,
                alpha = 0.5f,
            )
        }
    }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.ChartsTest"`
Expected: PASS, 4 tests.

- [ ] **Step 5: Run the gate and commit**

```bash
./scripts/gate/kmp.sh
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceCharts.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/ChartsTest.kt
git commit -m "feat(kmp): the sparkline and the cycle ring, drawn rather than vectored"
```

---

### Task 7: `CadenceTabBar`

The five-tab bar with the raised primary action in the middle. It is built here as a presentational component — it takes the active tab and reports taps — because `shared.tsx` is this step's file. Step 2 of the block wires it into navigation.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTabBar.kt`
- Create: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/TabBarTest.kt`

**Interfaces:**
- Consumes: `CadenceIcon`, `CadenceIcons`, `Cadence.palette`, `Modifier.pressable`.
- Produces:
  - `enum class CadenceTab(val icon: String, val label: String) { TODAY("home", "Сегодня"), INVENTORY("beaker", "Аптечка"), LOG("plus", "Записать"), TRENDS("chart-bar", "Тренды"), NUTRITION("cake", "Питание") }`
  - `@Composable fun CadenceTabBar(active: CadenceTab?, onSelect: (CadenceTab) -> Unit, modifier: Modifier = Modifier)`

**Divergence to record:** the prototype's bar sits on a three-stop vertical gradient fading cream to transparent. Compose Multiplatform has `Brush.verticalGradient` and this ports directly, so there is no divergence — but the prototype also positions it `absolute` over the content, which is a layout decision belonging to the shell. Here it is an ordinary composable; step 2 places it.

- [ ] **Step 1: Write the failing tests**

Create `TabBarTest.kt`:

```kotlin
package app.cadence.design

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals

@OptIn(ExperimentalTestApi::class)
class TabBarTest {
    @Test
    fun tabBarShowsEveryLabelExceptThePrimaryAction() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    CadenceTabBar(active = CadenceTab.TODAY, onSelect = {})
                }
            }
            onNodeWithText("Сегодня").assertIsDisplayed()
            onNodeWithText("Аптечка").assertIsDisplayed()
            onNodeWithText("Тренды").assertIsDisplayed()
            onNodeWithText("Питание").assertIsDisplayed()
            // The centre action is a raised icon button with no visible label,
            // so it is reachable by its accessibility name instead.
            onNodeWithContentDescription("Записать").assertIsDisplayed()
        }

    @Test
    fun tabBarReportsEveryTabItWasTapped() =
        runComposeUiTest {
            val taps = mutableListOf<CadenceTab>()
            setContent {
                CadenceTheme {
                    CadenceTabBar(active = CadenceTab.TODAY, onSelect = { taps += it })
                }
            }
            onNodeWithText("Аптечка").performClick()
            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Питание").performClick()
            assertEquals(
                listOf(CadenceTab.INVENTORY, CadenceTab.LOG, CadenceTab.NUTRITION),
                taps,
            )
        }

    @Test
    fun tabBarWithNoActiveTabStillRenders() =
        runComposeUiTest {
            // The shell shows the bar over a pushed screen where nothing is
            // active; a null must not be a crash.
            setContent {
                CadenceTheme {
                    CadenceTabBar(active = null, onSelect = {})
                }
            }
            onNodeWithText("Сегодня").assertIsDisplayed()
        }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.TabBarTest"`
Expected: FAIL — "unresolved reference: CadenceTabBar".

- [ ] **Step 3: Implement it**

Create `CadenceTabBar.kt`:

```kotlin
package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * The five destinations of the patient app, ported from `TABS` in
 * mobile/src/components/shared.tsx. The labels are product copy and stay
 * Russian; the icon names index [CadenceIcons].
 */
enum class CadenceTab(
    val icon: String,
    val label: String,
) {
    TODAY("home", "Сегодня"),
    INVENTORY("beaker", "Аптечка"),
    LOG("plus", "Записать"),
    TRENDS("chart-bar", "Тренды"),
    NUTRITION("cake", "Питание"),
}

/**
 * The bottom bar. Presentational only: it reports taps and does not know what
 * a destination is — the shell owns that.
 *
 * The centre tab is a raised circle rather than an icon and a label, which is
 * why it is announced by its accessibility name instead of a visible one.
 */
@Composable
fun CadenceTabBar(
    active: CadenceTab?,
    onSelect: (CadenceTab) -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    // The prototype fades cream upward over the scrolling content so rows do
    // not collide with the bar; the three stops are its own.
    val fade =
        Brush.verticalGradient(
            0.0f to palette.bg.copy(alpha = 0f),
            0.4f to palette.bg.copy(alpha = 0.85f),
            1.0f to palette.bg,
        )

    Row(
        modifier =
            modifier
                .fillMaxWidth()
                .background(fade)
                .windowInsetsPadding(WindowInsets.navigationBars)
                .padding(horizontal = CadenceSpacing.sm, vertical = CadenceSpacing.sm),
        verticalAlignment = Alignment.Bottom,
    ) {
        CadenceTab.entries.forEach { tab ->
            if (tab == CadenceTab.LOG) {
                Box(modifier = Modifier.weight(1f), contentAlignment = Alignment.Center) {
                    val interactionSource = remember { MutableInteractionSource() }
                    Box(
                        modifier =
                            Modifier
                                .pressable({ onSelect(tab) }, interactionSource)
                                .size(52.dp)
                                .shadow(CadenceElevation.md, RoundedCornerShape(CadenceRadius.pill))
                                .background(
                                    CadenceColors.forest700,
                                    RoundedCornerShape(CadenceRadius.pill),
                                ).semantics { contentDescription = tab.label },
                        contentAlignment = Alignment.Center,
                    ) {
                        CadenceIcon(
                            paths = CadenceIcons.byName.getValue(tab.icon),
                            size = 24.dp,
                            tint = CadenceColors.cream,
                        )
                    }
                }
            } else {
                val selected = tab == active
                val tint = if (selected) CadenceColors.forest700 else palette.subtle
                val interactionSource = remember { MutableInteractionSource() }

                Column(
                    modifier =
                        Modifier
                            .weight(1f)
                            .pressable({ onSelect(tab) }, interactionSource)
                            .padding(vertical = CadenceSpacing.xs),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(2.dp),
                ) {
                    CadenceIcon(
                        paths = CadenceIcons.byName.getValue(tab.icon),
                        size = 22.dp,
                        tint = tint,
                    )
                    BasicText(
                        text = tab.label,
                        style = Cadence.typography.label.copy(color = tint, fontSize = 10.sp),
                        maxLines = 1,
                    )
                }
            }
        }
    }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.design.TabBarTest"`
Expected: PASS, 3 tests.

- [ ] **Step 5: Update the composable count**

`DesignSystemTest.kt` pins the number of public composables. Eight were added — `CadenceTitleEmphasis`, `CadenceSection`, `CadenceListRow`, `CadenceAppHeader`, `CadenceScreenHeader`, `CadenceSpark`, `CadenceCycleRing`, `CadenceTabBar` — so the assertion moves from 16 to 24. Update the number and the comment that explains the counting rule.

- [ ] **Step 6: Run the gate and commit**

```bash
./scripts/gate/kmp.sh
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTabBar.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/TabBarTest.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt
git commit -m "feat(kmp): the five destinations, drawn but not yet wired"
```

---

### Task 8: The showcase catches up

`App.kt` is a placeholder exercising the design system. It currently shows nine of the sixteen composables. Leaving eight new ones with no place to be looked at means the first time anybody sees them is inside a screen, which is the wrong place to notice that a padding is wrong.

**Files:**
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/App.kt`
- Modify: `kmp/composeApp/src/commonTest/kotlin/app/cadence/AppTest.kt`

**Interfaces:**
- Consumes: everything produced by Tasks 3–7.
- Produces: nothing. This is the last task that touches `App.kt` — step 2 of the block replaces it with a navigation host, and `AppTest.kt` is rewritten there. That is already named in the `mobile-sign-in` spec's step 1, so it is expected rather than a surprise.

- [ ] **Step 1: Read `App.kt` and `AppTest.kt` in full before editing**

This task is deliberately the only one written without literal code: `App.kt` is 79 lines whose exact structure the implementer must see to extend it without restating it wrongly. Read both files first.

- [ ] **Step 2: Add the new composables to the showcase**

Extend the existing `Column` with one instance of each of the eight, using Russian copy consistent with what is already there. Keep it a flat list — this is a showcase, not a screen. Suggested copy, so the strings are not invented twice:

```kotlin
CadenceTitleEmphasis("ритм")
CadenceSection(title = "сегодня", action = "всё", onAction = {}) {
    CadenceListRow(
        icon = CadenceIcons.byName.getValue("beaker"),
        title = "Семаглутид",
        subtitle = "вскрыт 12 дней назад",
        trailing = "0,25",
        trailingSubtitle = "мг",
    )
}
CadenceScreenHeader(title = "Аптечка", eyebrow = "запас", onBack = {})
CadenceSpark(data = listOf(0.3f, 0.5f, 0.4f, 0.6f, 0.55f, 0.7f, 0.8f))
CadenceCycleRing(week = 4)
CadenceTabBar(active = CadenceTab.TODAY, onSelect = {})
```

`CadenceAppHeader` goes at the top of the column rather than in the list, because it is a header and putting it mid-column would misrepresent how it sits.

- [ ] **Step 3: Extend the placeholder test**

`AppTest.kt` asserts the showcase's contents with `onNodeWithText(...).assertIsDisplayed()`. Add one assertion per new component, using a string from the copy above — "ритм", "СЕГОДНЯ", "Семаглутид", "Аптечка", "Сегодня" — so a component that stops rendering fails a test rather than silently disappearing. The two drawn shapes have no text; give them `Modifier.testTag` and assert the tags.

- [ ] **Step 4: Run the gate**

Run: `./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 5: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/App.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/AppTest.kt
git commit -m "feat(kmp): the showcase shows all of it, so a regression is visible"
```

---

## What this step does not deliver

Named so the step report is honest rather than optimistic:

- **Rendering proven on a device.** Every test here runs on the iOS simulator target and asserts structure, not pixels. That the Golos Text glyphs actually appear, that the ring's arc starts at twelve o'clock, and that the spark's fill sits under its line are all step 11's side-by-side against a running prototype — plus the font tail task's own criterion, which asks for a run rather than a build.
- **Android UI tests.** The Android target has no host-test builder by design (`kmp-wiring`). These components get Android coverage when that builder arrives, which is that spec's step 1.
- **`ScrubChart`.** Canvas plus `pointerInput`, and it belongs to the trends section — step 7 of the block.
- **The initials module.** `CadenceAppHeader` takes an initial rather than deriving one, because the derivation is owned by one module per surface and that module arrives with profile data.
