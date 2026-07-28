package app.cadence.design

import androidx.compose.runtime.Immutable
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp

/**
 * The three faces of the product, and the single place they are chosen.
 *
 * OPEN QUESTION — the prototype's body face is DM Sans, which ships no Cyrillic
 * subset, and every string this product shows a user is Russian. The prototype
 * already hit the same wall on the display face (Instrument Serif has no
 * Cyrillic) and fell back to Cormorant Garamond; nobody ran the check on the
 * body face. Until a replacement is agreed the platform defaults stand in, so
 * the layout is honest about metrics without pretending to be the final type.
 *
 * When the decision lands, it is these three values that change and nothing
 * else: every style below and every screen reads them from here.
 */
object CadenceFonts {
    /** Cormorant Garamond in the design; has Cyrillic. */
    val display: FontFamily = FontFamily.Serif

    /** DM Sans in the design; NO Cyrillic — see the note above. */
    val body: FontFamily = FontFamily.SansSerif

    /** JetBrains Mono in the design; has Cyrillic. */
    val mono: FontFamily = FontFamily.Monospace
}

/**
 * Type styles, ported from the primitives in mobile/src/components.
 *
 * The prototype derives line height and tracking from the font size rather than
 * fixing them — every one of its Title call sites passes an explicit size — so
 * the ratios are what is stored here, in `em`. Overriding `fontSize` on any of
 * these styles rescales the leading and the tracking with it, which an absolute
 * `sp` line height would not: a 34sp title would keep 30sp leading and overlap.
 */
@Immutable
data class CadenceTypography(
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

val CadenceDefaultTypography =
    CadenceTypography(
        // Uppercase micro-label. The prototype uppercases the text itself; do
        // that at the call site so the original string stays readable in code.
        eyebrow =
            TextStyle(
                fontFamily = CadenceFonts.body,
                fontWeight = FontWeight.Medium,
                fontSize = 11.sp,
                letterSpacing = 0.14.em,
            ),
        title =
            TextStyle(
                fontFamily = CadenceFonts.display,
                fontSize = CadenceTitleSize,
                lineHeight = 1.08.em,
                letterSpacing = (-0.018).em,
            ),
        // Serif italic emphasis inside a title, e.g. «Выберите ритм.»
        titleEmphasis =
            TextStyle(
                fontFamily = CadenceFonts.display,
                fontStyle = FontStyle.Italic,
                fontSize = CadenceTitleSize,
                lineHeight = 1.08.em,
                letterSpacing = (-0.018).em,
            ),
        body =
            TextStyle(
                fontFamily = CadenceFonts.body,
                fontSize = CadenceBodySize,
                lineHeight = 1.5.em,
            ),
        meta =
            TextStyle(
                fontFamily = CadenceFonts.body,
                fontSize = 12.sp,
            ),
        // Tabular mono for every measured value: a dose, a weight, a count.
        number =
            TextStyle(
                fontFamily = CadenceFonts.mono,
                fontWeight = FontWeight.Medium,
                fontSize = CadenceNumberSize,
                lineHeight = 1.05.em,
                letterSpacing = (-0.03).em,
            ),
        // The unit is set in serif italic at 0.36x the number — «мг», «см».
        numberUnit =
            TextStyle(
                fontFamily = CadenceFonts.display,
                fontStyle = FontStyle.Italic,
                fontSize = CadenceNumberSize * CADENCE_NUMBER_UNIT_RATIO,
            ),
        label =
            TextStyle(
                fontFamily = CadenceFonts.body,
                fontWeight = FontWeight.Medium,
                fontSize = 14.sp,
            ),
    )
