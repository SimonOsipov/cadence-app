package app.cadence.shell

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.max
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceElevation
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.format.formatInteger

/** How long the card stays up — `setTimeout(…, 1700)` in the prototype. */
const val CADENCE_CONFIRM_TOAST_MS: Long = 1700

/**
 * `dayKcal` is the day's running total including this meal, not the meal's own — matches
 * `showConfirm({ kcal: nextTotals.kcal, … })` in mobile/src/state/AppState.tsx. Renamed from
 * `kcal`, which invited wiring the meal's own figure and rendering a plausible wrong number.
 */
@Immutable
data class ConfirmToastState(
    val mealName: String,
    val dayKcal: Int,
)

/** The prototype's floor for the bottom inset, as in CadenceTabBar. */
private val MIN_BOTTOM_INSET = 16.dp

private val TOAST_RADIUS = 22.dp
private val TOAST_PADDING = 18.dp
private val TOAST_GAP = 14.dp
private val TOAST_TICK_BOX = 44.dp
private val TOAST_TITLE_SIZE = 18.sp
private val TOAST_META_SIZE = 12.sp

/**
 * Ported from mobile/src/navigation/ConfirmToast.tsx. Presentational and stateless — it
 * doesn't know it disappears; the shell owns the timer.
 */
@Composable
fun ConfirmToast(
    state: ConfirmToastState?,
    targetKcal: Int,
    modifier: Modifier = Modifier,
) {
    if (state == null) return
    val palette = Cadence.palette
    val bottomInset = WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding()
    val shape = RoundedCornerShape(TOAST_RADIUS)

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(palette.glassSoft)
                // Matches the prototype's pointerEvents="auto": swallows every touch while up,
                // or a tap on "+" underneath would open the action sheet through this layer.
                .pointerInput(Unit) { awaitPointerEventScope { while (true) awaitPointerEvent() } }
                .padding(
                    start = CadenceSpacing.lg,
                    end = CadenceSpacing.lg,
                    top = CadenceSpacing.sm,
                    // `Math.max(insets.bottom, 16) + 8` in the prototype — CadenceTabBar's
                    // shape, not windowInsetsPadding's; the inset alone gives 8dp not 24dp.
                    bottom = max(bottomInset, MIN_BOTTOM_INSET) + CadenceSpacing.sm,
                ),
        contentAlignment = Alignment.BottomCenter,
    ) {
        Row(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .shadow(CadenceElevation.lg, shape, clip = false)
                    .background(palette.bg, shape)
                    .border(1.dp, palette.hairline, shape)
                    .padding(TOAST_PADDING),
            horizontalArrangement = Arrangement.spacedBy(TOAST_GAP),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                modifier =
                    Modifier
                        .size(TOAST_TICK_BOX)
                        .background(CadenceColors.forest700, RoundedCornerShape(CadenceRadius.pill)),
                contentAlignment = Alignment.Center,
            ) {
                CadenceIcon(paths = CadenceIcons.check, size = 20.dp, tint = CadenceColors.cream)
            }

            Column(Modifier.fillMaxWidth()) {
                BasicText(
                    text = "${state.mealName} · записано",
                    style = Cadence.typography.title.copy(color = palette.ink, fontSize = TOAST_TITLE_SIZE),
                    maxLines = 1,
                )
                ToastTally(reached = state.dayKcal, target = targetKcal)
            }
        }
    }
}

/**
 * One BasicText with spans, not two Row siblings, so it wraps and reads as one sentence —
 * matching the prototype's nested Text.
 */
@Composable
private fun ToastTally(
    reached: Int,
    target: Int,
) {
    val palette = Cadence.palette

    BasicText(
        text =
            buildAnnotatedString {
                withStyle(
                    Cadence.typography.number
                        .toSpanStyle()
                        .copy(color = palette.ink2, fontSize = TOAST_META_SIZE),
                ) { append(formatInteger(reached)) }
                withStyle(
                    Cadence.typography.meta
                        .toSpanStyle()
                        .copy(color = palette.subtle, fontSize = TOAST_META_SIZE),
                ) { append(" / ${formatInteger(target)} ккал сегодня") }
            },
        modifier = Modifier.padding(top = 2.dp),
        style = Cadence.typography.meta.copy(color = palette.muted, fontSize = TOAST_META_SIZE),
        maxLines = 1,
    )
}
