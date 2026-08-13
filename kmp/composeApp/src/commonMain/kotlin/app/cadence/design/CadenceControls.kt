package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.disabled
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// Buttons, chips and pills, ported from mobile/src/components/primitives.tsx.
// Every pressable scales to 0.98 while held (the prototype's `.press`); nothing
// ripples, since this design language has no Material ink.

internal const val PRESSED_SCALE = 0.98f
private val PILL_SHAPE = RoundedCornerShape(CadenceRadius.pill)

/** The press feedback shared by every control here. */
@Composable
internal fun Modifier.pressable(
    onClick: () -> Unit,
    interactionSource: MutableInteractionSource,
): Modifier {
    val pressed by interactionSource.collectIsPressedAsState()

    return this
        .scale(if (pressed) PRESSED_SCALE else 1f)
        .clickable(
            interactionSource = interactionSource,
            indication = null,
            onClick = onClick,
        )
}

enum class CadenceButtonKind { PRIMARY, SECONDARY, GHOST, DARK, DANGER }

/** `opacity: nextDisabled ? 0.65 : 1` on the prototype's wizard footer. */
private const val DISABLED_ALPHA = 0.65f

enum class CadenceButtonSize { SMALL, MEDIUM, LARGE }

private data class ButtonColors(
    val background: Color,
    val foreground: Color,
    val outline: Color,
)

private data class ButtonMetrics(
    val vertical: Dp,
    val horizontal: Dp,
    val fontSize: TextUnit,
)

@Composable
private fun colorsFor(kind: CadenceButtonKind): ButtonColors {
    val palette = Cadence.palette
    return when (kind) {
        CadenceButtonKind.PRIMARY -> {
            ButtonColors(CadenceColors.forest700, palette.forestFg, Color.Transparent)
        }

        CadenceButtonKind.SECONDARY -> {
            ButtonColors(palette.sunk, palette.ink, palette.bone)
        }

        CadenceButtonKind.GHOST -> {
            ButtonColors(Color.Transparent, CadenceColors.forest700, Color.Transparent)
        }

        CadenceButtonKind.DARK -> {
            ButtonColors(palette.ink, palette.sand3, Color.Transparent)
        }

        CadenceButtonKind.DANGER -> {
            ButtonColors(Color.Transparent, CadenceColors.danger, CadenceColors.danger)
        }
    }
}

private fun metricsFor(size: CadenceButtonSize): ButtonMetrics =
    when (size) {
        CadenceButtonSize.SMALL -> ButtonMetrics(8.dp, 14.dp, 13.sp)
        CadenceButtonSize.MEDIUM -> ButtonMetrics(12.dp, 20.dp, 14.sp)
        CadenceButtonSize.LARGE -> ButtonMetrics(15.dp, 22.dp, 15.sp)
    }

@Composable
fun CadenceButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    kind: CadenceButtonKind = CadenceButtonKind.PRIMARY,
    size: CadenceButtonSize = CadenceButtonSize.MEDIUM,
    icon: List<String>? = null,
    fillWidth: Boolean = false,
    // Disabled state is drawn distinctly, not just ignored: an inert-looking
    // «Дальше» that still accepts taps is indistinguishable from a working one.
    enabled: Boolean = true,
) {
    val colors = colorsFor(kind)
    val metrics = metricsFor(size)
    val interactionSource = remember { MutableInteractionSource() }

    Row(
        modifier =
            modifier
                .then(if (fillWidth) Modifier.fillMaxWidth() else Modifier)
                .then(if (enabled) Modifier.pressable(onClick, interactionSource) else Modifier)
                // Merged so the button answers as one node — the label is a
                // child `BasicText`, and unmerged a test would find it instead.
                .semantics(mergeDescendants = true) { if (!enabled) disabled() }
                .then(if (enabled) Modifier else Modifier.alpha(DISABLED_ALPHA))
                .background(colors.background, PILL_SHAPE)
                .border(1.dp, colors.outline, PILL_SHAPE)
                .padding(horizontal = metrics.horizontal, vertical = metrics.vertical),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            val iconSize = with(LocalDensity.current) { metrics.fontSize.toDp() + 3.dp }
            CadenceIcon(paths = icon, size = iconSize, tint = colors.foreground)
        }
        BasicText(
            text = label,
            style =
                Cadence.typography.label.copy(
                    color = colors.foreground,
                    fontSize = metrics.fontSize,
                ),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

/** A round, borderless icon button — the 40dp tap target the prototype uses. */
@Composable
fun CadenceIconButton(
    icon: List<String>,
    // Required, not optional: an icon-only control with no name is announced
    // as an unlabelled button. Russian, like every other user-facing string.
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    iconSize: Dp = 20.dp,
    background: Color = Color.Transparent,
    tint: Color = Cadence.palette.ink2,
) {
    val interactionSource = remember { MutableInteractionSource() }
    Box(
        modifier =
            modifier
                .pressable(onClick, interactionSource)
                .size(40.dp)
                .background(background, PILL_SHAPE)
                .semantics { this.contentDescription = contentDescription },
        contentAlignment = Alignment.Center,
    ) {
        CadenceIcon(paths = icon, size = iconSize, tint = tint)
    }
}

/** A filter chip: outlined when idle, inverted to ink when active. */
@Composable
fun CadenceChip(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    active: Boolean = false,
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val background = if (active) palette.ink else Color.Transparent
    val outline = if (active) palette.ink else palette.border
    val foreground = if (active) CadenceColors.cream else CadenceColors.ink700

    Box(
        modifier =
            modifier
                .pressable(onClick, interactionSource)
                .background(background, PILL_SHAPE)
                .border(1.dp, outline, PILL_SHAPE)
                .padding(horizontal = 14.dp, vertical = 7.dp),
    ) {
        BasicText(
            text = label,
            style = Cadence.typography.label.copy(color = foreground, fontSize = 13.sp),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

enum class CadencePillTone { FOREST, SAND, WARNING, DANGER, NEUTRAL, DARK }

private data class PillColors(
    val background: Color,
    val foreground: Color,
    val dot: Color,
)

/** A status pill: a coloured dot plus a short label. Read-only, never tappable. */
@Composable
fun CadencePill(
    label: String,
    modifier: Modifier = Modifier,
    tone: CadencePillTone = CadencePillTone.FOREST,
) {
    val colors =
        when (tone) {
            CadencePillTone.FOREST -> {
                PillColors(CadenceColors.forest50, CadenceColors.forest800, CadenceColors.forest700)
            }

            CadencePillTone.SAND -> {
                PillColors(CadenceColors.sand100, CadenceColors.sand900, CadenceColors.sand700)
            }

            CadencePillTone.WARNING -> {
                PillColors(CadenceColors.warningBg, CadenceColors.warningFg, CadenceColors.warning)
            }

            CadencePillTone.DANGER -> {
                PillColors(CadenceColors.dangerBg, CadenceColors.dangerFg, CadenceColors.danger)
            }

            CadencePillTone.NEUTRAL -> {
                PillColors(CadenceColors.linen, CadenceColors.ink800, CadenceColors.ink500)
            }

            CadencePillTone.DARK -> {
                PillColors(CadenceColors.ink900, CadenceColors.sand300, CadenceColors.sand500)
            }
        }

    Row(
        modifier =
            modifier
                .background(colors.background, PILL_SHAPE)
                .padding(horizontal = 10.dp, vertical = CadenceSpacing.xxs),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(6.dp).background(colors.dot, PILL_SHAPE))
        BasicText(
            text = label,
            style = Cadence.typography.label.copy(color = colors.foreground, fontSize = 11.sp),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}
