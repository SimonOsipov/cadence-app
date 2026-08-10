package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.unit.dp

/** §03: `mood smallint 1..5`, and the prototype's five points. */
const val CADENCE_MOOD_MAX = 5

/** `labels` in the prototype's `MoodSlider`, in its order. */
val CADENCE_MOOD_LABELS = listOf("Никак", "Слабо", "Ровно", "Хорошо", "Светло")

private val MOOD_DOT = 34.dp

/** `borderWidth: 1.5` in the prototype. */
private val MOOD_BORDER = 1.5.dp

/**
 * «Энергия · сегодня» — five points, and the word for the one chosen.
 *
 * [value] is nullable, and that is the one difference from the prototype:
 * `INITIAL_LOG_STATE` seeds `mood: 3`, so its slider always shows «Ровно» — a
 * word the patient did not say. Step 4 of the wizard is «всё по желанию», so
 * nothing chosen has to be a state this can be in.
 */
@Composable
fun CadenceMoodSlider(
    value: Int?,
    onChange: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            for (point in 1..CADENCE_MOOD_MAX) {
                MoodDot(point = point, selected = value == point, onClick = { onChange(point) })
            }
        }

        Row(
            Modifier
                .fillMaxWidth()
                .padding(top = CadenceSpacing.sm, start = CadenceSpacing.xxs, end = CadenceSpacing.xxs),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            CadenceMeta(CADENCE_MOOD_LABELS.first())
            // Nothing at all until a point is chosen, rather than a default word.
            // `getOrNull`: the parameter admits any Int, and a wizard that
            // passed 0 or 6 would crash the composition rather than show no
            // word. The invariant is 1..5 and it is not the type's.
            value?.let { point ->
                CADENCE_MOOD_LABELS.getOrNull(point - 1)?.let { CadenceMeta(it, color = Cadence.palette.ink2) }
            }
            CadenceMeta(CADENCE_MOOD_LABELS.last())
        }
    }
}

@Composable
private fun MoodDot(
    point: Int,
    selected: Boolean,
    onClick: () -> Unit,
) {
    val on = CadenceColors.forest700

    Box(
        Modifier
            .size(MOOD_DOT)
            .clip(CircleShape)
            .background(if (selected) on else Color.Transparent)
            .border(
                width = MOOD_BORDER,
                color = if (selected) on else Cadence.palette.border,
                shape = CircleShape,
            ).clickable(onClick = onClick)
            .clearAndSetSemantics {
                contentDescription = "Самочувствие $point из $CADENCE_MOOD_MAX"
                this.selected = selected
            },
        contentAlignment = Alignment.Center,
    ) {
        CadenceMeta(
            point.toString(),
            color = if (selected) CadenceColors.cream else Cadence.palette.muted,
        )
    }
}
