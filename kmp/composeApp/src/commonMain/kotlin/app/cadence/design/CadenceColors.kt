package app.cadence.design

import androidx.compose.runtime.Immutable
import androidx.compose.ui.graphics.Color

/**
 * The raw palette, carried over value for value from mobile/src/theme/index.ts,
 * which in turn was ported from web/prototype/design-system/colors_and_type.css.
 *
 * This is the only place a colour literal belongs. A screen that needs a colour
 * the palette does not have is a design question, not a coding one.
 */
object CadenceColors {
    val paper = Color(0xFFFBF8F3)
    val cream = Color(0xFFF6F1EA)
    val linen = Color(0xFFEDE5D6)
    val bone = Color(0xFFE4DAC6)
    val border = Color(0xFFCDC1A8)

    val ink900 = Color(0xFF1A1A1A)
    val ink800 = Color(0xFF2A2A2A)
    val ink700 = Color(0xFF3F3B35)
    val ink600 = Color(0xFF5C5852)
    val ink500 = Color(0xFF8A857D)
    val ink400 = Color(0xFFB3AD9F)
    val ink300 = Color(0xFFD6CFBF)

    val forest900 = Color(0xFF142C1F)
    val forest800 = Color(0xFF1F4530)
    val forest700 = Color(0xFF2D5F3F)
    val forest600 = Color(0xFF3D7A52)
    val forest500 = Color(0xFF4A8161)
    val forest300 = Color(0xFFA6C2AF)
    val forest200 = Color(0xFFCADCCF)
    val forest100 = Color(0xFFD8E5DB)
    val forest50 = Color(0xFFEAF0EB)

    val sand900 = Color(0xFF6B4A25)
    val sand700 = Color(0xFFB8895A)

    // Not named in mobile/src/theme/index.ts — the prototype writes this shade as
    // the literal #a5773d at its one call site (NutritionScreen.tsx carb macro bar).
    // Added here so that call site has a token to port to.
    val sand600 = Color(0xFFA5773D)
    val sand500 = Color(0xFFD4A574)
    val sand300 = Color(0xFFE8D4B8)
    val sand100 = Color(0xFFF3E8D6)

    val danger = Color(0xFFB8503C)
    val dangerBg = Color(0xFFF4DFD6)
    val dangerFg = Color(0xFF6B2818)
    val warning = Color(0xFFC2780A)
    val warningBg = Color(0xFFFBEED1)
    val warningFg = Color(0xFF7A4A06)
    val info = Color(0xFF4A6B7A)
    val infoBg = Color(0xFFDFE6E9)

    val hairline = Color(0xFF1A1A1A).copy(alpha = 0.08f)

    // The prototype's theme names only `glassDark: 'rgba(20,44,31,.35)'` and
    // writes the toast's `rgba(20,44,31,.25)` inline in ConfirmToast.tsx. Same
    // ink at two weights — named once here so neither is a literal at a call
    // site, which is the whole point of this file.
    private val forestScrim = Color(0xFF142C1F)
    val glassDark = forestScrim.copy(alpha = 0.35f)
    val glassSoft = forestScrim.copy(alpha = 0.25f)
}

/**
 * The semantic layer: what a colour is *for*, not what it is. Screens read this
 * and never CadenceColors, so a palette change is one edit.
 *
 * Mirrors `pal` in the prototype, which is the light "cream" scheme. A dark
 * scheme would be a second instance of this type, not a second set of names.
 */
@Immutable
data class CadencePalette(
    val bg: Color,
    val paper: Color,
    val sunk: Color,
    val bone: Color,
    val border: Color,
    val ink: Color,
    val ink2: Color,
    val muted: Color,
    val subtle: Color,
    val placeholder: Color,
    val forestBg: Color,
    val forestDeep: Color,
    val forestFg: Color,
    val forestPillBg: Color,
    val forestPillFg: Color,
    val sand: Color,
    val sand3: Color,
    val sand1: Color,
    val sand7: Color,
    val glassDark: Color,
    val glassSoft: Color,
    val hairline: Color,
    val onForest: Color,
)

val CadenceLightPalette =
    CadencePalette(
        bg = CadenceColors.cream,
        paper = CadenceColors.paper,
        sunk = CadenceColors.linen,
        bone = CadenceColors.bone,
        border = CadenceColors.border,
        ink = CadenceColors.ink900,
        ink2 = CadenceColors.ink800,
        muted = CadenceColors.ink600,
        subtle = CadenceColors.ink500,
        placeholder = CadenceColors.ink400,
        forestBg = CadenceColors.forest800,
        forestDeep = CadenceColors.forest900,
        forestFg = CadenceColors.cream,
        forestPillBg = CadenceColors.forest50,
        forestPillFg = CadenceColors.forest800,
        sand = CadenceColors.sand500,
        sand3 = CadenceColors.sand300,
        sand1 = CadenceColors.sand100,
        sand7 = CadenceColors.sand700,
        glassDark = CadenceColors.glassDark,
        glassSoft = CadenceColors.glassSoft,
        hairline = CadenceColors.hairline,
        onForest = CadenceColors.cream,
    )
