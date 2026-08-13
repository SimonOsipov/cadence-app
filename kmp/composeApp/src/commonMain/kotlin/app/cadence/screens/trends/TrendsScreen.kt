package app.cadence.screens.trends

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceSpark
import app.cadence.design.CadenceTabBar
import app.cadence.design.CadenceTitle
import app.cadence.format.formatDecimal
import app.cadence.format.formatSignedDelta
import app.cadence.format.unitRu
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.MetricAccent
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.domain.TrendsOverview
import app.cadence.shared.domain.notableShifts

/** The whole scroll, so a test can reach the screen without knowing its shape. */
const val CADENCE_TRENDS_TAG = "cadence-trends"

/** The featured card. Tagged apart from the grid because it renders differently. */
const val CADENCE_TRENDS_HERO_TAG = "cadence-trends-hero"

const val CADENCE_TRENDS_SHIFTS_TAG = "cadence-trends-shifts"

/** One tag per metric, so «tapping this card opens this metric» is one check each. */
fun cadenceTrendCardTag(metric: Metric): String = "cadence-trend-card-${metric.code}"

const val CADENCE_TRENDS_JOURNAL_TAG = "cadence-trends-journal"
const val CADENCE_TRENDS_BODY_TAG = "cadence-trends-body"

private const val PERCENT = 100.0

/** «13 %» — how far a metric moved, as the share the shifts are ranked by. */
private fun formatShare(movement: Double): String = "${formatDecimal(movement * PERCENT, 0)} %"

private val HERO_SPARK_HEIGHT = 56.dp
private val GRID_SPARK_HEIGHT = 32.dp
private val SHIFT_ICON = 40.dp

/**
 * Every metric of the protocol, for one window. [window] is a parameter and
 * [onWindowChange] an event because the detail screen shares the choice: the
 * prototype keeps it in app state for that reason, and a window remembered here
 * would reset every time a patient came back from a metric.
 */
@Composable
fun TrendsScreen(
    overview: TrendsOverview,
    onOpenMetric: (Metric) -> Unit,
    onWindowChange: (TrendWindow) -> Unit,
    onOpenJournal: () -> Unit,
    onOpenBody: () -> Unit,
    modifier: Modifier = Modifier,
    onSelectTab: (CadenceDestination) -> Unit = { },
    onOpenActions: () -> Unit = { },
) {
    val palette = Cadence.palette

    Column(
        modifier
            .fillMaxSize()
            .background(palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars),
    ) {
        Column(
            // The scrolling body is now a sibling of the bar rather than the
            // whole screen, so it takes the height the bar leaves. The tag
            // stays on it: what the suite scrolls is this, not the frame.
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .testTag(CADENCE_TRENDS_TAG),
        ) {
            CadenceTitle(
                text = "Тренды",
                modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
            )

            TrendWindowChips(overview.window, onWindowChange)

            overview.hero?.let { hero ->
                HeroCard(hero, onOpen = { onOpenMetric(hero.meta.metric) })
            }

            TrendsMetricGrid(overview.rest, onOpenMetric)

            NotableShifts(overview.notableShifts())

            LinkRow(
                eyebrow = "Самочувствие",
                title = "Дневник самочувствия",
                subtitle = "Настроение, побочные, заметки по курсу",
                icon = CadenceIcons.heart,
                tag = CADENCE_TRENDS_JOURNAL_TAG,
                onOpen = onOpenJournal,
            )
            LinkRow(
                eyebrow = "Состав тела",
                title = "Тело · замеры и фото",
                subtitle = "Состав, талия, бёдра, снимки",
                icon = CadenceIcons.scale,
                tag = CADENCE_TRENDS_BODY_TAG,
                onOpen = onOpenBody,
            )
        }

        // As on Today and the cabinet: the prototype keeps the bar inside the
        // screen. Trends is a bar destination, so it carries one.
        CadenceTabBar(
            active = CadenceDestination.TRENDS,
            onSelect = onSelectTab,
            onLog = onOpenActions,
        )
    }
}

/**
 * The featured metric: its latest reading, how far it moved, and a sparkline. The
 * prototype's hero reads its *value* from the cycle series and its *delta* from the
 * selected one, so switching to «7 дней» changes the pill under a number that never
 * moves. Both come from the same series here.
 */
@Composable
private fun HeroCard(
    trend: MetricTrend,
    onOpen: () -> Unit,
) {
    val palette = Cadence.palette

    // Two tags, two nodes: `Modifier.testTag(a).testTag(b)` keeps only the
    // last, so the hero would answer to one name and silently lose the other.
    Box(Modifier.fillMaxWidth().padding(horizontal = CadenceSpacing.lg).testTag(CADENCE_TRENDS_HERO_TAG)) {
        Column(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(CadenceRadius.lg))
                .background(palette.paper)
                .clickable(onClick = onOpen)
                .testTag(cadenceTrendCardTag(trend.meta.metric))
                .padding(CadenceSpacing.md),
        ) {
            CadenceEyebrow(text = trend.meta.eyebrow, color = palette.subtle)
            CadenceTitle(text = trend.meta.label)
            MetricReading(trend)
            Spark(trend, HERO_SPARK_HEIGHT)
        }
    }
}

/**
 * The two-up grid, on its own. `internal` and separate so a test can compose it
 * alone: inside the whole screen these cards sit below the test window, where a
 * card cannot be clicked at all and one scrolled into view is clicked at the
 * bounds it had before the scroll — every sweep across them reported the
 * neighbouring metric. Composed alone with three cards, «this card opens this
 * metric» is one exact check each, against a list deliberately not in enum order.
 */
@Composable
internal fun TrendsMetricGrid(
    metrics: List<MetricTrend>,
    onOpen: (Metric) -> Unit,
) {
    val palette = Cadence.palette

    // Two up, laid out as rows of pairs rather than a lazy grid: eight cards
    // inside a scrolling column is a list that fits on one screen, and nesting
    // a lazy grid in a scroll is the shape that measures forever.
    metrics.chunked(2).forEach { pair ->
        Row(
            Modifier
                .fillMaxWidth()
                .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.xs),
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        ) {
            pair.forEach { trend ->
                Column(
                    Modifier
                        .weight(1f)
                        .clip(RoundedCornerShape(CadenceRadius.lg))
                        .background(palette.paper)
                        .clickable { onOpen(trend.meta.metric) }
                        .testTag(cadenceTrendCardTag(trend.meta.metric))
                        .padding(CadenceSpacing.sm),
                ) {
                    CadenceMeta(text = trend.meta.label, color = palette.subtle)
                    MetricReading(trend)
                    Spark(trend, GRID_SPARK_HEIGHT)
                }
            }
            // An odd count leaves the last row half full rather than stretching
            // one card across two columns.
            if (pair.size == 1) Box(Modifier.weight(1f))
        }
    }
}

/** The number, its unit and the window's delta — or that there is no number. */
@Composable
private fun MetricReading(trend: MetricTrend) {
    val palette = Cadence.palette
    val latest = trend.series.latest

    if (latest == null) {
        CadenceBody(text = NO_READINGS, color = palette.subtle)
        return
    }

    Row(verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xs)) {
        CadenceTitle(text = formatDecimal(latest, trend.meta.decimals))
        CadenceMeta(text = unitRu(trend.meta.unit), color = palette.muted)
        trend.series.delta?.let { delta ->
            CadenceMeta(
                text = formatSignedDelta(delta, unitRu(trend.meta.unit), trend.meta.decimals),
                color = palette.muted,
            )
        }
    }
}

@Composable
private fun Spark(
    trend: MetricTrend,
    height: Dp,
) {
    CadenceSpark(
        data = trend.series.points.map { it.value.toFloat() },
        color = sparkStroke(trend.meta.accent),
        // The neighbours' fill, not the stroke's own colour: `CadenceSpark`
        // multiplies by 0,4, and forest700 under that reads far darker than
        // the prototype's `rgba(45,95,63,0.35)`.
        fill = sparkFill(trend.meta.accent),
        height = height,
        modifier = Modifier.fillMaxWidth().padding(top = CadenceSpacing.xs),
    )
}

/**
 * The line's colour for a metric's family.
 *
 * A `when` rather than an `if`, for `Metric.meta`'s reason one layer down: a
 * third family should stop the build rather than quietly render forest.
 */
internal fun sparkStroke(accent: MetricAccent): Color =
    when (accent) {
        MetricAccent.FOREST -> CadenceColors.forest700
        MetricAccent.SAND -> CadenceColors.sand700
    }

internal fun sparkFill(accent: MetricAccent): Color =
    when (accent) {
        MetricAccent.FOREST -> CadenceColors.forest50
        MetricAccent.SAND -> CadenceColors.sand100
    }

/** «Заметные сдвиги» — the metrics that moved most, and by how much. */
@Composable
private fun NotableShifts(shifts: List<MetricTrend>) {
    if (shifts.isEmpty()) return
    val palette = Cadence.palette

    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md)
            .testTag(CADENCE_TRENDS_SHIFTS_TAG),
    ) {
        CadenceEyebrow(text = "Заметные сдвиги", color = palette.subtle)

        shifts.forEach { shift ->
            Row(
                Modifier.fillMaxWidth().padding(vertical = CadenceSpacing.sm),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
            ) {
                Box(
                    Modifier
                        .size(SHIFT_ICON)
                        .clip(RoundedCornerShape(CadenceRadius.md))
                        .background(palette.sunk),
                    contentAlignment = Alignment.Center,
                ) {
                    // Which way the number went, not whether that is progress.
                    // The icon set has only one trending arrow, and an upward
                    // one beside «↓ 2,2 кг» would contradict the text under it.
                    CadenceIcon(
                        paths =
                            if ((shift.series.delta ?: 0.0) <
                                0
                            ) {
                                CadenceIcons.chevronDown
                            } else {
                                CadenceIcons.chevronUp
                            },
                        tint = palette.ink2,
                    )
                }
                Column(Modifier.weight(1f)) {
                    CadenceBody(text = shift.meta.label)
                    shift.series.delta?.let {
                        CadenceMeta(
                            text = formatSignedDelta(it, unitRu(shift.meta.unit), shift.meta.decimals),
                            color = palette.muted,
                        )
                    }
                }
                // The share the list is ordered by. Ranked on one number and
                // showing another is a list whose order nothing on screen
                // explains — and the prototype showed exactly this one («+31%
                // к старту»).
                shift.movement?.let {
                    CadenceMeta(text = formatShare(it), color = palette.subtle)
                }
            }
        }
    }
}

@Composable
private fun LinkRow(
    eyebrow: String,
    title: String,
    subtitle: String,
    icon: List<String>,
    tag: String,
    onOpen: () -> Unit,
) {
    val palette = Cadence.palette

    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
    ) {
        CadenceEyebrow(text = eyebrow, color = palette.subtle)
        Row(
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(CadenceRadius.lg))
                .background(palette.paper)
                .clickable(onClick = onOpen)
                .testTag(tag)
                .padding(CadenceSpacing.sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
        ) {
            CadenceIcon(paths = icon, tint = palette.ink2)
            Column(Modifier.weight(1f)) {
                CadenceBody(text = title)
                CadenceMeta(text = subtitle, color = palette.muted)
            }
        }
    }
}
