package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.format.weekdayHeadings
import app.cadence.shared.domain.MoodLevel
import app.cadence.shared.domain.TrendRange
import kotlinx.datetime.LocalDate

const val CADENCE_HEATMAP_TAG = "cadence-heatmap"

/** One node per drawn day, so «the grid reaches day 83» is a query and not a reading of pixels. */
fun cadenceHeatmapCellTag(date: LocalDate): String = "cadence-heatmap-cell-$date"

/** The dot a titration day carries, tagged separately: it sits on a cell that is otherwise ordinary. */
fun cadenceHeatmapTitrationTag(date: LocalDate): String = "cadence-heatmap-titration-$date"

private val WEEK_COLUMN_WIDTH = 30.dp
private val CELL_GAP = 4.dp
private val CELL_CORNER = 5.dp
private val WEEK_NUMBER_SIZE = 9.5.sp
private val TITRATION_DOT = 6.dp
private val TITRATION_OFFSET = 2.dp

/**
 * The course's days, Monday first, one row per calendar week — [heatmapRows] carries
 * why the row count is counted rather than fixed at the prototype's twelve.
 *
 * Every cell is a laid-out node rather than a painted square, on
 * [CadenceScrubChart]'s precedent: painted, «the last day of the course is on the
 * grid» could only be asserted about a bitmap.
 */
@Composable
fun CadenceHeatmap(
    course: TrendRange,
    today: LocalDate,
    moodByDate: Map<LocalDate, MoodLevel>,
    titrations: Set<LocalDate>,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val weeks = heatmapWeeks(course, today, moodByDate, titrations)

    Column(
        modifier = modifier.testTag(CADENCE_HEATMAP_TAG),
        verticalArrangement = Arrangement.spacedBy(CELL_GAP),
    ) {
        HeatmapHeadings()

        weeks.forEach { week ->
            Row(
                horizontalArrangement = Arrangement.spacedBy(CELL_GAP),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                BasicText(
                    text =
                        week
                            .filterNotNull()
                            .firstOrNull()
                            ?.week
                            ?.toString()
                            .orEmpty(),
                    // The mono face, so 1 and 13 line up down the column.
                    style = Cadence.typography.number.copy(color = palette.subtle, fontSize = WEEK_NUMBER_SIZE),
                    modifier = Modifier.width(WEEK_COLUMN_WIDTH),
                )

                week.forEach { cell ->
                    if (cell == null) {
                        Box(Modifier.weight(1f).aspectRatio(1f))
                    } else {
                        HeatmapCellBox(cell, Modifier.weight(1f))
                    }
                }
            }
        }
    }
}

@Composable
private fun HeatmapHeadings() {
    val palette = Cadence.palette

    Row(horizontalArrangement = Arrangement.spacedBy(CELL_GAP)) {
        Box(Modifier.width(WEEK_COLUMN_WIDTH))

        weekdayHeadings().forEach { heading ->
            Box(Modifier.weight(1f), contentAlignment = Alignment.Center) {
                CadenceMeta(heading.uppercase(), color = palette.placeholder)
            }
        }
    }
}

@Composable
private fun HeatmapCellBox(
    cell: HeatmapCell,
    modifier: Modifier,
) {
    val palette = Cadence.palette
    val fill = heatmapFill(cell, palette)
    val border = heatmapBorder(cell.standing, palette)

    Box(
        modifier
            .aspectRatio(1f)
            .background(fill, RoundedCornerShape(CELL_CORNER))
            .then(
                border?.let {
                    Modifier.border(it.width, it.color, RoundedCornerShape(CELL_CORNER))
                } ?: Modifier,
            ).testTag(cadenceHeatmapCellTag(cell.date)),
    ) {
        if (cell.titration) {
            Box(
                Modifier
                    .align(Alignment.TopEnd)
                    .offset(x = TITRATION_OFFSET, y = -TITRATION_OFFSET)
                    .size(TITRATION_DOT)
                    .background(CadenceColors.sand500, CircleShape)
                    .testTag(cadenceHeatmapTitrationTag(cell.date)),
            )
        }
    }
}
