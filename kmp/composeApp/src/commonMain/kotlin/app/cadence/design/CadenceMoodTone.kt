package app.cadence.design

import androidx.compose.ui.graphics.Color
import app.cadence.shared.domain.MoodLevel

/**
 * How each [MoodLevel] is drawn — the fill for a dot or a heatmap cell, and the soft ground
 * behind it. `journal/data.ts:46-53` carries the pair as literals; every one of them is an
 * existing token here, so the port maps rather than declares (`#b8503c` is `danger`,
 * `#3d7a52` is `forest600`, `#ede5d6` is `linen`, and so on). The one exception is level 5's
 * soft, `#dcebe0`, which the palette already had as [CadenceColors.forest100].
 *
 * Lives in the design system, not in the domain, for the reason [CadenceBodyZone] does: a
 * shared module that has no Compose cannot hold a `Color`.
 */
enum class CadenceMoodTone(
    val level: MoodLevel,
    val color: Color,
    val soft: Color,
) {
    HARD(MoodLevel.HARD, CadenceColors.danger, CadenceColors.dangerBg),
    POOR(MoodLevel.POOR, CadenceColors.warning, CadenceColors.warningBg),
    EVEN(MoodLevel.EVEN, CadenceColors.ink500, CadenceColors.linen),
    GOOD(MoodLevel.GOOD, CadenceColors.forest600, CadenceColors.forest50),
    BRIGHT(MoodLevel.BRIGHT, CadenceColors.forest700, CadenceColors.forest100),
    ;

    companion object {
        private val byLevel = entries.associateBy { it.level }

        /** The drawing for a level. Null only if the scale and this table drift apart. */
        fun of(level: MoodLevel): CadenceMoodTone? = byLevel[level]
    }
}
