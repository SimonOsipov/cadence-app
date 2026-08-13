package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** The bar row, so a test can measure a column's height against it. */
const val CADENCE_WEEK_ROW_TAG = "cadence-week-row"

/** One day's column, by its position in the week. */
fun weekBarTag(index: Int) = "cadence-week-bar-$index"

/** The dashed target line, so a test can measure where it sits against a bar. */
const val CADENCE_WEEK_GOAL_LINE_TAG = "cadence-week-goal-line"

/** The goal caption, so a test can measure that it stays inside the chart's own bounds. */
const val CADENCE_WEEK_GOAL_CAPTION_TAG = "cadence-week-goal-caption"

/** The prototype's `Math.max(...values, target) * 1.05` (`NutritionScreen.tsx:318`). */
private const val HEADROOM = 1.05

/**
 * The bar band's own height. Not a port of the prototype's `H = 90`: that's
 * the *whole* SVG — `padT = 4` and `padB = 18` (`NutritionScreen.tsx:322-324`)
 * leave only 68 of those 90 units for bars, and the day labels share that
 * viewBox at `y = H - 4` rather than a second row like this port draws. The
 * prototype also scales by `aspectRatio: W/H` — the screen's own width, not a
 * fixed dp figure — so there's no exact number to carry over; 84.dp is a
 * chosen band, not a measurement.
 */
private val WEEK_CHART_HEIGHT = 84.dp

/** The goal line's own stroke, the prototype's `strokeWidth={1}`. */
private val GOAL_LINE_STROKE = 1.dp

/** The goal line's dash pattern, the prototype's `strokeDasharray="2 3"`. */
private val GOAL_DASH_ON = 2.dp
private val GOAL_DASH_OFF = 3.dp

/** The prototype's `y = yFor(target) - 4` — the caption sits this far above the line, not on it. */
private val GOAL_CAPTION_GAP = 4.dp

/**
 * The height the tallest thing on the chart is drawn against.
 *
 * The goal participates: a week entirely under it would otherwise scale to its
 * own maximum and draw the dashed goal line off the top.
 */
fun weekScale(
    values: List<Double>,
    goal: Double,
): Double = (values.maxOrNull() ?: 0.0).coerceAtLeast(goal) * HEADROOM

/**
 * A column's height, as a fraction of the scale. Public and unit-tested
 * directly, same as [CadenceMacroBar]'s `macroFraction` and [CadenceGauge]'s
 * `gaugeFraction`: a `Box`'s fraction is only as trustworthy as the function a
 * test can call without going through a composable. Guards the zero-scale
 * week the same way every gauge here does.
 */
fun weekBarFraction(
    value: Double,
    scale: Double,
): Float = if (scale <= 0.0) 0f else (value / scale).toFloat().coerceIn(0f, 1f)

/**
 * Where the dashed goal line sits, measured down from the top of the chart.
 * Public for the same reason [weekBarFraction] is: a test needs to reach the
 * clamp and the zero-scale guard directly, not only through the composable.
 */
fun weekGoalFraction(
    goal: Double,
    scale: Double,
): Float = if (scale <= 0.0) 0f else (goal / scale).toFloat().coerceIn(0f, 1f)

/**
 * Seven days of a nutrient (kcal, most often) against a goal, with today's
 * column picked out — `WeekBars` in the prototype (`NutritionScreen.tsx:316-387`).
 *
 * Not an extension of [CadenceUsageRow]: that row has neither a goal nor a
 * dashed target, so it scales to the week's own maximum with no headroom —
 * correct with nothing to compare against, wrong here, where a week that
 * never reaches its goal would draw the dashed line off the top of that
 * scale. Threading a goal, a dashed line and a highlighted column through as
 * three optional parameters would leave that row carrying two shapes for two
 * call sites needing different scales; a second, smaller component is
 * cheaper than a first one bent to fit.
 *
 * `goalLabel` is the caller's finished caption — the prototype's own
 * `цель ${target.toLocaleString()}` — not `goal` re-rendered in here: numbers
 * are data, formatting is presentation, and this primitive isn't the
 * nutrition screen's formatter module.
 *
 * The fill is a laid-out `Box` per day, not a `Canvas` bar chart: a canvas's
 * pixels are asserted nowhere in this codebase, and [weekBarTag] lets a test
 * measure a column's height against [CADENCE_WEEK_ROW_TAG] instead of
 * trusting a draw call. The dashed goal line stays on a `Canvas` — it carries
 * no value a test needs beyond [weekGoalFraction], public and unit-tested
 * directly — but [CADENCE_WEEK_GOAL_LINE_TAG] still measures where the line
 * lands, since *where a fraction is drawn* is a layout question a pure
 * function can't answer alone.
 */
@Composable
fun CadenceWeekBars(
    values: List<Double>,
    labels: List<String>,
    goal: Double,
    goalLabel: String,
    todayIndex: Int,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val scale = weekScale(values, goal)
    val goalFraction = weekGoalFraction(goal, scale)

    Column(modifier.fillMaxWidth()) {
        Box(Modifier.fillMaxWidth().height(WEEK_CHART_HEIGHT)) {
            GoalMarker(goalLabel, goalFraction)

            Row(
                Modifier.fillMaxSize().testTag(CADENCE_WEEK_ROW_TAG),
                horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
                verticalAlignment = Alignment.Bottom,
            ) {
                values.forEachIndexed { index, value ->
                    val isToday = index == todayIndex

                    Box(
                        Modifier
                            .weight(1f)
                            .fillMaxHeight(weekBarFraction(value, scale))
                            .clip(RoundedCornerShape(CadenceRadius.xs))
                            .background(if (isToday) CadenceColors.forest700 else CadenceColors.sand500.copy(0.7f))
                            .testTag(weekBarTag(index))
                            .semantics { selected = isToday },
                    )
                }
            }
        }

        Row(
            Modifier.fillMaxWidth().padding(top = CadenceSpacing.xxs),
            // Same gap as the bar row above: without it, the two rows'
            // weighted columns disagree (the bar row reserves
            // CadenceSpacing.xxs between columns, this row didn't), and a
            // label drifts up to ±1.7dp from its bar at seven columns over
            // ~350dp.
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
        ) {
            labels.forEachIndexed { index, label ->
                Box(Modifier.weight(1f), contentAlignment = Alignment.Center) {
                    val isToday = index == todayIndex
                    CadenceMeta(label, color = if (isToday) palette.ink else palette.subtle)
                }
            }
        }
    }
}

/**
 * The dashed target line and its caption, placed at [goalFraction] down from
 * the top of the chart.
 *
 * A custom [Layout], not `BoxWithConstraints` with an `offset` per child: the
 * caption's placement depends on its own *measured* height, not an assumed
 * one. An assumed 16dp constant here once clamped the caption's top correctly
 * but let its bottom escape the chart whenever the real text was taller
 * (larger font scale, or a wrapping `goalLabel`), because the clamp used the
 * assumption while the drawn box used whatever Compose measured. [Layout]
 * measures both children once, so [weekGoalCaptionOffset]'s placement math
 * and the box Compose actually draws agree by construction; no other
 * technique in this file (`BoxWithConstraints`, the weighted-`Spacer` trick
 * [CadenceSplitBar] uses) reads a *sibling's* measured size, only the
 * parent's own constraints.
 *
 * The caption is not the line's row partner: the two are measured and placed
 * independently so the caption's height can never perturb the line's
 * position — the defect the weighted-`Spacer` layout this replaced had,
 * where the caption's ~13dp intrinsic height ate space two weighted spacers
 * should have divided, and the line landed off by
 * `captionHeight × (goalFraction − 0.5)`.
 *
 * The caption draws *below* the line whenever there isn't room above — not a
 * rare edge case. At *default* font scale that flip happens at roughly 79%
 * of the scale or higher (using the caption's real, varying height), which
 * includes every week whose goal sits at or above its own maximum. That 79%
 * is a default-scale measurement only: a taller caption needs more headroom,
 * so the flip point drops as font scale grows — roughly 61% at fontScale 2.0,
 * roughly 44% at fontScale 3.0. The prototype always draws the caption above
 * the line; this port doesn't attempt that everywhere.
 * [weekGoalCaptionOffset]'s final `coerceIn` pulls the below-the-line
 * fallback back toward the chart when it would otherwise escape the bottom,
 * and usually keeps the caption clear of the line too — but not always: at
 * fontScale 3.0, with a goal mid-range, the clamp pulls the caption up far
 * enough that the line lands strictly inside the caption's box. Nothing in
 * this suite measures caption/line overlap above default scale, so that case
 * is an accepted, previously-undocumented gap, not a placement being hidden.
 */
@Composable
private fun GoalMarker(
    goalLabel: String,
    goalFraction: Float,
) {
    Layout(
        modifier = Modifier.fillMaxSize(),
        content = {
            Canvas(
                Modifier
                    .fillMaxWidth()
                    .height(GOAL_LINE_STROKE)
                    .testTag(CADENCE_WEEK_GOAL_LINE_TAG),
            ) {
                drawLine(
                    color = CadenceColors.sand700.copy(alpha = 0.4f),
                    start = Offset(0f, size.height / 2f),
                    end = Offset(size.width, size.height / 2f),
                    strokeWidth = GOAL_LINE_STROKE.toPx(),
                    pathEffect = PathEffect.dashPathEffect(floatArrayOf(GOAL_DASH_ON.toPx(), GOAL_DASH_OFF.toPx())),
                )
            }

            CadenceMeta(
                goalLabel,
                modifier = Modifier.padding(end = CadenceSpacing.xs).testTag(CADENCE_WEEK_GOAL_CAPTION_TAG),
                color = CadenceColors.sand700.copy(alpha = 0.75f),
            )
        },
    ) { measurables, constraints ->
        val loose = constraints.copy(minWidth = 0, minHeight = 0)
        val linePlaceable = measurables[0].measure(loose)
        val captionPlaceable = measurables[1].measure(loose)

        val chartHeight = constraints.maxHeight.toDp()
        val lineY = chartHeight * (1f - goalFraction)
        val lineTop = lineY - linePlaceable.height.toDp() * 0.5f
        val captionTop = weekGoalCaptionOffset(lineY, chartHeight, captionPlaceable.height.toDp())

        layout(constraints.maxWidth, constraints.maxHeight) {
            linePlaceable.placeRelative(0, lineTop.roundToPx())
            captionPlaceable.placeRelative(constraints.maxWidth - captionPlaceable.width, captionTop.roundToPx())
        }
    }
}

/**
 * Where [GoalMarker]'s caption's top edge lands: above the line with
 * [GOAL_CAPTION_GAP] to spare, below it when there isn't, never outside
 * `[0, chartHeight - captionHeight]` either way. `captionHeight` is the
 * caption's own *measured* height, from [GoalMarker]'s [Layout] — not an
 * assumed constant, which is what once let the caption's bottom escape the
 * chart when the real text was taller (larger font scale, or a wrapped
 * `goalLabel`), even though this function's clamp was applied correctly to
 * the number it was given.
 */
fun weekGoalCaptionOffset(
    lineY: Dp,
    chartHeight: Dp,
    captionHeight: Dp,
): Dp {
    val above = lineY - GOAL_CAPTION_GAP - captionHeight
    val fallback = lineY + GOAL_LINE_STROKE + GOAL_CAPTION_GAP
    val floor = (chartHeight - captionHeight).coerceAtLeast(0.dp)

    return (if (above >= 0.dp) above else fallback).coerceIn(0.dp, floor)
}
