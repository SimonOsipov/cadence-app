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
import app.cadence.shared.domain.MoodLevel

/** §03: `mood smallint 1..5`, and the prototype's five points. */
const val CADENCE_MOOD_MAX = 5

/**
 * The product's one mood scale, in order. Read from [MoodLevel] rather than declared here:
 * the prototype keeps two lists for the same `mood 1..5` — «Никак / Слабо / …» in the dose
 * wizard, «Тяжело / Так себе / …» in the journal — and the journal's wording is the one the
 * number is read back in.
 */
val CADENCE_MOOD_LABELS: List<String> = MoodLevel.entries.map { it.labelRu }

private val MOOD_DOT = 34.dp

/** `borderWidth: 1.5` in the prototype. */
private val MOOD_BORDER = 1.5.dp

/**
 * «Самочувствие · сегодня» — five points, and the word for the one chosen.
 * [value] is nullable, the one difference from the prototype: its
 * `INITIAL_LOG_STATE` seeds `mood: 3`, so the slider always shows «Ровно» — a
 * word the patient didn't say. Step 4 is «всё по желанию», so nothing chosen
 * has to be a state this can be in.
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
            // `MoodLevel.of`: the parameter admits any Int, and a wizard passing 0 or 6 would
            // crash rather than show no word — 1..5 isn't the type's own invariant.
            MoodLevel.of(value)?.let { CadenceMeta(it.labelRu, color = Cadence.palette.ink2) }
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
