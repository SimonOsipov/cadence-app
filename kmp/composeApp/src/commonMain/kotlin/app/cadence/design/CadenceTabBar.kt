package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.max
import androidx.compose.ui.unit.sp

/** The prototype's floor for the bottom inset — `Math.max(insets.bottom, 16)`. */
private val MIN_BOTTOM_INSET = 16.dp

/** The current destination is stroked heavier, as in the prototype. */
private const val ACTIVE_STROKE = 1.8f
private const val INACTIVE_STROKE = 1.5f
private const val ACTION_STROKE = 2f

/**
 * The bottom bar.
 *
 * Presentational only: it reports what was tapped and knows nothing about what
 * a destination is. The shell owns that, and wires it in step 2 of the port.
 *
 * Switching destination and logging a dose are separate callbacks because they
 * are separate things — one navigates, the other pushes a wizard. One callback
 * over a set containing both would hand the shell a `when` to unpick, which is
 * the work a type is meant to save.
 */
@Composable
fun CadenceTabBar(
    active: CadenceDestination,
    onSelect: (CadenceDestination) -> Unit,
    onLog: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val inset = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
    val destinations = CadenceDestination.entries

    Row(
        modifier =
            modifier
                .fillMaxWidth()
                // The prototype fades cream upward over the scrolling content so
                // rows do not collide with the bar. Three stops, its own numbers.
                .background(
                    Brush.verticalGradient(
                        0.0f to palette.bg.copy(alpha = 0f),
                        0.4f to palette.bg.copy(alpha = 0.85f),
                        1.0f to palette.bg,
                    ),
                ).padding(
                    start = CadenceSpacing.sm,
                    end = CadenceSpacing.sm,
                    top = CadenceSpacing.sm,
                    // max(inset, 16), not inset + 8: on a device with no gesture
                    // bar the labels would otherwise sit 8dp from the edge where
                    // the prototype puts them 16.
                    bottom = max(inset, MIN_BOTTOM_INSET),
                ),
        verticalAlignment = Alignment.Bottom,
    ) {
        // The action sits in the middle of the destinations, as in the prototype.
        val half = destinations.size / 2
        destinations.take(half).forEach {
            Destination(it, active, onSelect, Modifier.weight(1f))
        }
        LogAction(onLog, Modifier.weight(1f))
        destinations.drop(half).forEach {
            Destination(it, active, onSelect, Modifier.weight(1f))
        }
    }
}

/** The raised centre action: a circle with no label, named for a screen reader. */
@Composable
private fun LogAction(
    onLog: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }

    Box(modifier = modifier, contentAlignment = Alignment.Center) {
        Box(
            modifier =
                Modifier
                    .pressable(onLog, interactionSource)
                    .size(52.dp)
                    .shadow(CadenceElevation.md, RoundedCornerShape(CadenceRadius.pill))
                    .background(CadenceColors.forest700, RoundedCornerShape(CadenceRadius.pill))
                    .semantics {
                        contentDescription = CadenceLogAction.ACCESSIBILITY_LABEL
                        role = Role.Button
                    },
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(
                paths = CadenceLogAction.icon,
                size = 24.dp,
                tint = CadenceColors.cream,
                strokeWidth = ACTION_STROKE,
            )
        }
    }
}

/**
 * An ordinary destination: icon over label.
 *
 * The current one is tinted, stroked heavier, and says so in its semantics. The
 * tint alone is not enough: a colour is invisible to a screen reader, and it
 * was invisible to the tests too, which is how a bar highlighting nothing
 * passed a suite that claimed to check exactly that.
 */
@Composable
private fun Destination(
    destination: CadenceDestination,
    active: CadenceDestination,
    onSelect: (CadenceDestination) -> Unit,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val current = destination == active
    val tint = if (current) CadenceColors.forest700 else Cadence.palette.subtle

    Column(
        modifier =
            modifier
                .pressable({ onSelect(destination) }, interactionSource)
                .semantics {
                    selected = current
                    role = Role.Tab
                }.padding(vertical = CadenceSpacing.xs),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        CadenceIcon(
            paths = destination.icon,
            size = 22.dp,
            tint = tint,
            strokeWidth = if (current) ACTIVE_STROKE else INACTIVE_STROKE,
        )
        BasicText(
            text = destination.label,
            style = Cadence.typography.label.copy(color = tint, fontSize = 10.sp),
            maxLines = 1,
        )
    }
}
