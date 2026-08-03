package app.cadence.shell

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.windowInsetsPadding
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
 * What a logged meal has to say for itself.
 *
 * `dayKcal` is **the day's running total including this meal**, not the meal's
 * own — `showConfirm({ kcal: nextTotals.kcal, … })` in
 * mobile/src/state/AppState.tsx, where `nextTotals = dayTotals(nextMeals)`. The
 * field was called `kcal` first, which invited step 8 to wire the meal's own
 * figure and render «300 / 2 100 ккал сегодня» after a 300-kcal breakfast: a
 * plausible wrong number, in a health product, that nothing would have caught.
 */
@Immutable
data class ConfirmToastState(
    val mealName: String,
    val dayKcal: Int,
)

private val TOAST_RADIUS = 22.dp
private val TOAST_PADDING = 18.dp
private val TOAST_GAP = 14.dp
private val TOAST_TICK_BOX = 44.dp
private val TOAST_TITLE_SIZE = 18.sp
private val TOAST_META_SIZE = 12.sp

/**
 * The card that confirms a logged meal, ported from
 * mobile/src/navigation/ConfirmToast.tsx.
 *
 * Presentational and stateless: it does not know that it disappears. The shell
 * owns the timer, because how long something stays on screen is a property of
 * the screen and not of the card.
 */
@Composable
fun ConfirmToast(
    state: ConfirmToastState?,
    targetKcal: Int,
    modifier: Modifier = Modifier,
) {
    if (state == null) return
    val palette = Cadence.palette
    val shape = RoundedCornerShape(TOAST_RADIUS)

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(palette.glassSoft)
                // The prototype's overlay is `pointerEvents="auto"`: for the
                // 1 700 ms it is up it is the topmost hit target and swallows
                // every touch. Without this the screen is dimmed but fully
                // live — a tap on «+» underneath opens the action sheet, which
                // composes *below* this layer and would be read through it.
                .pointerInput(Unit) { awaitPointerEventScope { while (true) awaitPointerEvent() } }
                .windowInsetsPadding(WindowInsets.navigationBars)
                // max(inset, 16) + 8 in the prototype; windowInsetsPadding has
                // already applied the inset, so this is the remainder — the
                // same shape CadenceTabBar and CadenceSheet both use.
                .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
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
 * «1 240 / 2 100 ккал сегодня» — two runs of one line.
 *
 * One BasicText with spans rather than two siblings in a Row, because in the
 * prototype the second is a `Text` nested in the first: it flows and wraps as
 * one sentence, and a screen reader reads it as one.
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
