package app.cadence.screens.schedule

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.pressable
import app.cadence.format.cycleWeekLabel
import app.cadence.format.dayAndMonth
import app.cadence.format.formatDose
import app.cadence.format.leadingBlanks
import app.cadence.format.weekdayHeadings
import app.cadence.shared.repository.ScheduleDay
import kotlinx.datetime.LocalDate

private val CARD_RADIUS = 18.dp
private val DOT = 5.dp

/** The injection dot has no text, so this is the only handle a test has on it. */
const val INJECTION_DOT_TAG: String = "schedule-injection-dot"
private const val WEEK_COLUMNS = 7

/**
 * «График», ported from mobile/src/features/schedule/ScheduleScreen.tsx. Every
 * mark on the grid comes from `ScheduleDay`, which the repository builds from the
 * same `occurrencesFor` the Today strip reads, so the two screens cannot disagree
 * about a day — what §03's seventh correction asks for.
 */
@Composable
fun ScheduleScreen(
    state: ScheduleState,
    modifier: Modifier = Modifier,
    onBack: () -> Unit = { },
    onOpenDay: (LocalDate) -> Unit = { },
) {
    Column(modifier.fillMaxSize()) {
        Row(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .windowInsetsPadding(WindowInsets.statusBars)
                    .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            CadenceIconButton(CadenceIcons.chevronLeft, "Назад", onBack)
            CadenceTitle("График")
        }

        Column(
            modifier = Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        ) {
            CycleBand(state.cycleWeek, state.cycleWeeks)
            state.titration?.let { TitrationCallout(it) }
            MonthGrid(state.days, state.today, onOpenDay)
        }
    }
}

/** «Текущий курс · неделя 4 из 12». */
@Composable
private fun CycleBand(
    week: Int?,
    weeks: Int,
) {
    val palette = Cadence.palette

    Column(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .background(palette.forestBg, RoundedCornerShape(CARD_RADIUS))
                .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xs),
    ) {
        CadenceEyebrow("Текущий курс", color = CadenceColors.sand300)
        BasicText(
            text = if (week == null) "вне курса" else "неделя $week из $weeks",
            style = Cadence.typography.title.copy(color = CadenceColors.cream, fontSize = 24.sp),
        )
    }
}

/** «Доза растёт: 0,25 мг → 0,5 мг» and the day it happens. */
@Composable
private fun TitrationCallout(step: app.cadence.shared.domain.TitrationStep) {
    val (fromValue, fromUnit) = formatDose(step.from)
    val (toValue, toUnit) = formatDose(step.to)

    Column(
        modifier =
            Modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg)
                .background(CadenceColors.sand100, RoundedCornerShape(CARD_RADIUS))
                .padding(CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
    ) {
        CadenceEyebrow("Шаг титрации", color = CadenceColors.sand900)
        BasicText(
            text = "Доза растёт: $fromValue $fromUnit → $toValue $toUnit",
            style = Cadence.typography.label.copy(color = CadenceColors.ink900, fontSize = 14.sp),
        )
        BasicText(
            text = "${dayAndMonth(step.date)} · ${cycleWeekLabel(step.week)}",
            style = Cadence.typography.meta.copy(color = CadenceColors.sand900, fontSize = 12.sp),
        )
    }
}

/**
 * One month, Monday first. The leading blanks align the first of the month to its
 * weekday — skip them and every dot shifts: 1 May 2026 is a Friday, so four cells
 * of nothing come first.
 */
@Composable
private fun MonthGrid(
    days: List<ScheduleDay>,
    today: LocalDate,
    onOpenDay: (LocalDate) -> Unit,
) {
    if (days.isEmpty()) return
    val palette = Cadence.palette

    Column(
        modifier = Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.lg),
        verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        Row(Modifier.fillMaxWidth()) {
            weekdayHeadings().forEach { heading ->
                BasicText(
                    text = heading,
                    modifier = Modifier.weight(1f),
                    style = Cadence.typography.meta.copy(color = palette.subtle, fontSize = 11.sp),
                )
            }
        }

        val cells: List<ScheduleDay?> = List(leadingBlanks(days.first().date)) { null } + days
        cells.chunked(WEEK_COLUMNS).forEach { week ->
            Row(Modifier.fillMaxWidth()) {
                week.forEach { day ->
                    if (day == null) {
                        Box(Modifier.weight(1f).aspectRatio(1f))
                    } else {
                        DayCell(day, isToday = day.date == today, onOpenDay = onOpenDay, modifier = Modifier.weight(1f))
                    }
                }
                repeat(WEEK_COLUMNS - week.size) { Box(Modifier.weight(1f).aspectRatio(1f)) }
            }
        }
    }
}

@Composable
private fun DayCell(
    day: ScheduleDay,
    isToday: Boolean,
    onOpenDay: (LocalDate) -> Unit,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }

    Column(
        modifier =
            modifier
                .aspectRatio(1f)
                .pressable({ onOpenDay(day.date) }, interactionSource)
                .then(
                    if (isToday) {
                        Modifier.border(1.dp, CadenceColors.forest700, RoundedCornerShape(CadenceRadius.md))
                    } else {
                        Modifier
                    },
                ).semantics { contentDescription = describe(day, isToday) },
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        BasicText(
            text = day.date.day.toString(),
            style =
                Cadence.typography.number.copy(
                    color = if (day.cycleWeek == null) palette.placeholder else palette.ink,
                    fontSize = 13.sp,
                ),
        )
        if (day.hasInjection) {
            // Tagged because a dot carries no text: `describe` is built from
            // the same field, so it cannot witness the dot's own presence —
            // deleting the Box left every assertion green.
            Box(
                Modifier
                    .padding(top = 2.dp)
                    .testTag(INJECTION_DOT_TAG)
                    .size(DOT)
                    .background(CadenceColors.forest700, RoundedCornerShape(CadenceRadius.pill)),
            )
        }
    }
}

/**
 * What a screen reader says about a cell — and what a test can grab hold of,
 * since a dot has no text.
 */
private fun describe(
    day: ScheduleDay,
    isToday: Boolean,
): String {
    val today = if (isToday) " · сегодня" else ""
    val suffix =
        when {
            day.cycleWeek == null -> " · вне курса"
            day.hasInjection -> " · инъекция"
            else -> ""
        }
    return dayAndMonth(day.date) + today + suffix
}
