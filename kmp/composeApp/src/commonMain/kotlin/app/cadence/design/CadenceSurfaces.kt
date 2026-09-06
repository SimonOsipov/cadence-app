package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.backhandler.BackHandler
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** Test handle for the sheet's scrim — the only way to hit it from a UI test. */
const val CADENCE_SHEET_SCRIM_TAG = "cadence-sheet-scrim"

enum class CadenceCardTone { PAPER, CREAM, LINEN, FOREST, SAND, OUTLINE }

/**
 * A card, ported from mobile/src/components/primitives.tsx.
 *
 * The prototype's warm umbra shadow does not survive the port — Compose takes
 * an elevation and derives the shadow per platform — so what carries over is
 * the step, not the tint. See [CadenceElevation].
 */
@Composable
fun CadenceCard(
    modifier: Modifier = Modifier,
    tone: CadenceCardTone = CadenceCardTone.PAPER,
    elevation: Dp = CadenceElevation.sm,
    padding: Dp = CadenceSpacing.lg,
    radius: Dp = CadenceRadius.lg,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    val palette = Cadence.palette
    val background =
        when (tone) {
            CadenceCardTone.PAPER, CadenceCardTone.OUTLINE -> palette.paper
            CadenceCardTone.CREAM -> palette.bg
            CadenceCardTone.LINEN -> palette.sunk
            CadenceCardTone.FOREST -> palette.forestBg
            CadenceCardTone.SAND -> palette.sand3
        }
    val shape = RoundedCornerShape(radius)

    Column(
        modifier =
            modifier
                // A static card starts no interaction collector at all: a list of
                // them would otherwise run one per item for a value nobody reads.
                .then(if (onClick != null) Modifier.cardPress(onClick) else Modifier)
                .shadow(elevation, shape, clip = false)
                .background(background, shape)
                .then(
                    if (tone == CadenceCardTone.OUTLINE) {
                        Modifier.border(1.dp, palette.bone, shape)
                    } else {
                        Modifier
                    },
                ).padding(padding),
        content = content,
    )
}

/** The grab handle at the top of a sheet. */
@Composable
fun CadenceGrabber(modifier: Modifier = Modifier) {
    Box(
        modifier =
            modifier
                .size(width = 38.dp, height = 4.dp)
                .background(Cadence.palette.border, RoundedCornerShape(CadenceRadius.pill)),
    )
}

/**
 * A bottom sheet: forest-tinted scrim, panel rounded 28 at the top.
 *
 * Composes nothing at all while closed, so a closed sheet costs neither layout
 * nor state. Tapping the scrim dismisses it — that behaviour is the contract,
 * and it has a test.
 */
@Suppress("DEPRECATION")
@OptIn(ExperimentalComposeUiApi::class)
@Composable
fun CadenceSheet(
    open: Boolean,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    if (!open) return

    // Mirrors the prototype's Modal `onRequestClose`: on Android, back closes
    // the sheet rather than finishing the activity and dropping the user out.
    // BackHandler is deprecated in favour of NavigationEventHandler, which
    // Compose Multiplatform 1.11.1 doesn't ship yet — accepted until it does.
    BackHandler(enabled = true, onBack = onDismiss)

    val palette = Cadence.palette

    Box(modifier = modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
        Box(
            Modifier
                .fillMaxSize()
                .background(palette.glassDark)
                .testTag(CADENCE_SHEET_SCRIM_TAG)
                .clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null,
                    onClick = onDismiss,
                ),
        )

        Column(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .shadow(
                        CadenceElevation.lg,
                        RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
                        clip = false,
                    ).background(
                        palette.bg,
                        RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
                    ).windowInsetsPadding(WindowInsets.navigationBars)
                    // max(inset, 20) + 12 in the prototype; windowInsetsPadding
                    // has already applied the inset, so this is the remainder.
                    .padding(
                        start = CadenceSpacing.xl,
                        end = CadenceSpacing.xl,
                        top = CadenceSpacing.md,
                        bottom = CadenceSpacing.md,
                    ),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            CadenceGrabber(Modifier.padding(bottom = CadenceSpacing.lg))
            Column(Modifier.fillMaxWidth(), content = content)
        }
    }
}

/** A hairline divider, the prototype's 8%-ink rule. */
@Composable
fun CadenceHairline(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().height(1.dp).background(Cadence.palette.hairline))
}

/** Press feedback for a tappable card, sharing the controls' 0.98 scale. */
@Composable
private fun Modifier.cardPress(onClick: () -> Unit): Modifier {
    val interactionSource = remember { MutableInteractionSource() }
    return this.pressable(onClick, interactionSource)
}
