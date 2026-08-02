// The product's type scale, over the three bundled typefaces.
//
// DECIDED 2026-07-28 — the body face is **Golos Text**, not the prototype's DM
// Sans. DM Sans publishes `latin` and `latin-ext` only, and every string this
// product shows a user is Russian, so DM Sans would hand all of it to a system
// fallback. The prototype had already hit this on the display face (Instrument
// Serif has no Cyrillic) and fell back to Cormorant Garamond; nobody re-ran the
// check on the body face. Golos Text was designed for Cyrillic rather than
// extended into it, and sits close to DM Sans in width and tone, so the
// prototype's layouts do not shift.
//
// Three families, four files: the display italic is a file of its own because
// the bundled files carry a `wght` axis only. Licences and copyright notices
// live in kmp/composeApp/licenses/ — the OFL requires them to travel with the
// fonts, and they sit outside composeResources/ because the resource generator
// would turn a licence into a resource accessor.
//
// Axis ranges, read from each file's `fvar`: Golos Text 400–900, Cormorant
// Garamond 300–700, JetBrains Mono 100–800. A weight outside its range is
// clamped silently, so asking Golos Text for Light gets Regular.

package app.cadence.design

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.remember
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import app.cadence.design.generated.CormorantGaramond
import app.cadence.design.generated.CormorantGaramondItalic
import app.cadence.design.generated.GolosText
import app.cadence.design.generated.JetBrainsMono
import app.cadence.design.generated.Res
import org.jetbrains.compose.resources.Font
import org.jetbrains.compose.resources.FontResource

/**
 * One face of one family at one weight.
 *
 * `variationSettings` is deliberately not passed: the library's default for it
 * is derived from [weight] and [style] and covers both the `wght` and the
 * `ital` axes, whereas naming only `wght` here would quietly drop the second.
 * Measured on the simulator — a string set at Medium is the same width with the
 * explicit setting and without it (248px either way), and genuinely different
 * at Light (246px) and Bold (252px), so the axis is applied by the default.
 */
@Composable
private fun face(
    resource: FontResource,
    weight: FontWeight,
    style: FontStyle = FontStyle.Normal,
): Font = Font(resource = resource, weight = weight, style = style)

/**
 * A family holds every weight the design system offers in that face.
 *
 * One `Font` per family would make `fontWeight` inert: the resolver would
 * return the only member whatever was asked for, and Compose would synthesize
 * the difference — a fake bold over a file that carries a real one.
 */
@Composable
private fun displayFace(): FontFamily = FontFamily(face(Res.font.CormorantGaramond, FontWeight.Medium))

@Composable
private fun displayItalicFace(): FontFamily =
    FontFamily(face(Res.font.CormorantGaramondItalic, FontWeight.Medium, FontStyle.Italic))

/** Regular, Medium and SemiBold — the three the prototype's `F` exposes. */
@Composable
private fun bodyFace(): FontFamily =
    FontFamily(
        face(Res.font.GolosText, FontWeight.Normal),
        face(Res.font.GolosText, FontWeight.Medium),
        face(Res.font.GolosText, FontWeight.SemiBold),
    )

/** Regular and Medium, as `F.mono` and `F.monoMedium` in the prototype. */
@Composable
private fun monoFace(): FontFamily =
    FontFamily(
        face(Res.font.JetBrainsMono, FontWeight.Normal),
        face(Res.font.JetBrainsMono, FontWeight.Medium),
    )

/**
 * Type styles, ported from the primitives in mobile/src/components.
 *
 * The prototype derives line height and tracking from the font size rather than
 * fixing them — every one of its Title call sites passes an explicit size — so
 * the ratios are what is stored here, in `em`. Overriding `fontSize` on any of
 * these styles rescales the leading and the tracking with it, which an absolute
 * `sp` line height would not: a 34sp title would keep 30sp leading and overlap.
 *
 * That applies to the styles that set a `lineHeight` — title, titleEmphasis,
 * body and number. The single-line styles (eyebrow, meta, label, numberUnit)
 * take their leading from the font's own metrics, which already scale.
 */
@Immutable
data class CadenceTypography internal constructor(
    val eyebrow: TextStyle,
    val title: TextStyle,
    val titleEmphasis: TextStyle,
    val body: TextStyle,
    val meta: TextStyle,
    val number: TextStyle,
    val numberUnit: TextStyle,
    val label: TextStyle,
)

/** Default sizes. Call sites override them; the ratios below follow along. */
val CadenceTitleSize = 28.sp
val CadenceBodySize = 14.sp
val CadenceNumberSize = 44.sp

/** The unit beside a number is set at 0.36x the number, as in the prototype. */
const val CADENCE_NUMBER_UNIT_RATIO = 0.36f

/**
 * The metrics both display styles share.
 *
 * Held as one style copied twice rather than a helper taking a face and a slant:
 * a helper with those two as separate arguments makes the upright face plus a
 * slant flag representable, which asks for a synthetic oblique over the file
 * that has a drawn italic.
 */
private val displayMetrics =
    TextStyle(
        fontWeight = FontWeight.Medium,
        fontSize = CadenceTitleSize,
        lineHeight = 1.08.em,
        letterSpacing = (-0.018).em,
    )

/**
 * Builds the type scale over the bundled faces.
 *
 * A function rather than a constant because the faces are resources, and
 * reading one is a composition-time operation that a top-level `val` cannot do.
 */
@Composable
fun cadenceTypography(): CadenceTypography {
    val display = displayFace()
    val displayItalic = displayItalicFace()
    val body = bodyFace()
    val mono = monoFace()

    return remember(display, displayItalic, body, mono) {
        CadenceTypography(
            // Uppercase micro-label. The prototype uppercases the text itself; do
            // that at the call site so the original string stays readable in code.
            eyebrow =
                TextStyle(
                    fontFamily = body,
                    fontWeight = FontWeight.Medium,
                    fontSize = 11.sp,
                    letterSpacing = 0.14.em,
                ),
            title = displayMetrics.copy(fontFamily = display),
            // Serif italic emphasis inside a title, e.g. «Выберите ритм.»
            titleEmphasis =
                displayMetrics.copy(fontFamily = displayItalic, fontStyle = FontStyle.Italic),
            body =
                TextStyle(
                    fontFamily = body,
                    fontWeight = FontWeight.Normal,
                    fontSize = CadenceBodySize,
                    lineHeight = 1.5.em,
                ),
            meta =
                TextStyle(
                    fontFamily = body,
                    fontWeight = FontWeight.Normal,
                    fontSize = 12.sp,
                ),
            // Tabular mono for every measured value: a dose, a weight, a count.
            number =
                TextStyle(
                    fontFamily = mono,
                    fontWeight = FontWeight.Medium,
                    fontSize = CadenceNumberSize,
                    lineHeight = 1.05.em,
                    letterSpacing = (-0.03).em,
                ),
            // The unit is set in serif italic at 0.36x the number — «мг», «см».
            numberUnit =
                TextStyle(
                    fontFamily = displayItalic,
                    fontWeight = FontWeight.Medium,
                    fontStyle = FontStyle.Italic,
                    fontSize = CadenceNumberSize * CADENCE_NUMBER_UNIT_RATIO,
                ),
            label =
                TextStyle(
                    fontFamily = body,
                    fontWeight = FontWeight.Medium,
                    fontSize = 14.sp,
                ),
        )
    }
}
