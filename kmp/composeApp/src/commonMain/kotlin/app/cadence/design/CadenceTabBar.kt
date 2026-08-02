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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * The bottom bar.
 *
 * Presentational only: it reports which destination was tapped and knows
 * nothing about what a destination is. The shell owns that, and wires it in
 * step 2 of the port.
 *
 * `active` is not nullable. The prototype's type allows it, but all four call
 * sites pass a value — the bar only appears on the four tab screens, and the
 * centre action is a push rather than a destination that can be current.
 */
@Composable
fun CadenceTabBar(
    active: CadenceTab,
    onSelect: (CadenceTab) -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette

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
                ).windowInsetsPadding(WindowInsets.navigationBars)
                .padding(horizontal = CadenceSpacing.sm, vertical = CadenceSpacing.sm),
        verticalAlignment = Alignment.Bottom,
    ) {
        CadenceTab.entries.forEach { tab ->
            if (tab == CadenceTab.LOG) {
                PrimaryAction(tab, onSelect, Modifier.weight(1f))
            } else {
                Destination(tab, active, onSelect, Modifier.weight(1f))
            }
        }
    }
}

/** The raised centre action: a circle, no label, announced by its name. */
@Composable
private fun PrimaryAction(
    tab: CadenceTab,
    onSelect: (CadenceTab) -> Unit,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }

    Box(modifier = modifier, contentAlignment = Alignment.Center) {
        Box(
            modifier =
                Modifier
                    .pressable({ onSelect(tab) }, interactionSource)
                    .size(52.dp)
                    .shadow(CadenceElevation.md, RoundedCornerShape(CadenceRadius.pill))
                    .background(CadenceColors.forest700, RoundedCornerShape(CadenceRadius.pill))
                    .semantics { contentDescription = tab.label },
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(
                paths = CadenceIcons.byName.getValue(tab.icon),
                size = 24.dp,
                tint = CadenceColors.cream,
            )
        }
    }
}

/** An ordinary destination: icon over label, tinted by whether it is current. */
@Composable
private fun Destination(
    tab: CadenceTab,
    active: CadenceTab,
    onSelect: (CadenceTab) -> Unit,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val tint = cadenceTabTint(tab, active, Cadence.palette.subtle)

    Column(
        modifier =
            modifier
                .pressable({ onSelect(tab) }, interactionSource)
                .padding(vertical = CadenceSpacing.xs),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        CadenceIcon(paths = CadenceIcons.byName.getValue(tab.icon), size = 22.dp, tint = tint)
        BasicText(
            text = tab.label,
            style = Cadence.typography.label.copy(color = tint, fontSize = 10.sp),
            maxLines = 1,
        )
    }
}
