package app.cadence.screens.trends

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import app.cadence.design.Cadence
import app.cadence.design.CadenceBody
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceScrubChart
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.format.dayAndMonth
import app.cadence.format.formatDecimal
import app.cadence.format.formatSignedDelta
import app.cadence.format.unitRu
import app.cadence.shared.domain.Measurement
import app.cadence.shared.domain.MetricTrend
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.repository.MetricDetail

const val CADENCE_TREND_DETAIL_TAG = "cadence-trend-detail"
const val CADENCE_TREND_DETAIL_BACK_TAG = "cadence-trend-detail-back"
const val CADENCE_TREND_DETAIL_STATS_TAG = "cadence-trend-detail-stats"
const val CADENCE_TREND_DETAIL_RECENT_TAG = "cadence-trend-detail-recent"

/** What the screen shows instead of a metric when the route named none. */
const val CADENCE_TREND_DETAIL_UNKNOWN_TAG = "cadence-trend-detail-unknown"

/** One tag per aggregate, so «this box holds this number» is one check each. */
fun cadenceTrendStatTag(stat: String): String = "cadence-trend-stat-$stat"

/** «Сред.» / «Мин» / «Макс» / «Δ», as the prototype writes them. */
private const val STAT_AVERAGE = "avg"
private const val STAT_MINIMUM = "min"
private const val STAT_MAXIMUM = "max"
private const val STAT_DELTA = "delta"

private val CHART_HEIGHT = 200.dp

/** «7-pt series per metric», §11 — the same tail the biomarker sheet lists. */
private const val RECENT_ROWS = 7

/**
 * One metric, with the protocol drawn beside it.
 *
 * [detail] is null when the route's code named no metric of §03. The prototype
 * has a `thigh` and a `bmi` that the data model does not, and a deep link can
 * carry anything at all — so «нет такой метрики» is a state this screen renders
 * rather than a crash it avoids by hoping.
 *
 * [scrubIndex] and [window] are hoisted: the window is shared with the list
 * screen, and the scrub position belongs to whoever can also reset it when the
 * metric changes.
 */
@Composable
fun TrendDetailScreen(
    detail: MetricDetail?,
    window: TrendWindow,
    onWindowChange: (TrendWindow) -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    scrubIndex: Int? = null,
    onScrub: (Int, Measurement) -> Unit = { _, _ -> },
) {
    val palette = Cadence.palette

    Column(
        modifier
            .fillMaxSize()
            .background(palette.bg)
            .windowInsetsPadding(WindowInsets.statusBars)
            .verticalScroll(rememberScrollState())
            .testTag(CADENCE_TREND_DETAIL_TAG),
    ) {
        DetailHeader(detail?.trend?.meta?.label, onBack)

        if (detail == null) {
            UnknownMetric()
        } else {
            MetricBody(detail, window, onWindowChange, scrubIndex, onScrub)
        }
    }
}

/**
 * Everything below the header, once there is a metric to show.
 *
 * Split out rather than guarded with `return@Column`: an early exit inside a
 * `Column` means anything appended below it later silently does not render in
 * the states that took the exit, and `BiomarkerSheet` expresses the same choice
 * as a branch.
 */
@Composable
private fun ColumnScope.MetricBody(
    detail: MetricDetail,
    window: TrendWindow,
    onWindowChange: (TrendWindow) -> Unit,
    scrubIndex: Int?,
    onScrub: (Int, Measurement) -> Unit,
) {
    val palette = Cadence.palette

    CadenceEyebrow(
        text = detail.trend.meta.eyebrow,
        color = palette.subtle,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg),
    )

    TrendWindowChips(window, onWindowChange)

    // Said before the chart is drawn, not after it. The criterion is
    // «метрика без измерений говорит об этом, **а не рисует пустую ось**»,
    // and `BiomarkerSheet` set the precedent: an empty axis under an
    // invitation to drag it is worse than a sentence.
    if (detail.trend.series.points
            .isEmpty()
    ) {
        CadenceBody(
            text = NO_READINGS,
            color = palette.subtle,
            modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.md),
        )
        return
    }

    CadenceScrubChart(
        series = detail.trend.series,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg),
        bands = detail.bands,
        marks = detail.marks,
        accent = detail.trend.meta.accent,
        scrubIndex = scrubIndex,
        onScrub = onScrub,
        height = CHART_HEIGHT,
    )

    CadenceMeta(
        text = "Пунктир — изменения протокола. Тяните по графику, чтобы посмотреть любой день.",
        color = palette.subtle,
        modifier = Modifier.padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
    )

    StatStrip(detail.trend)
    RecentReadings(detail.trend)
}

/** Back, and whichever metric this is — «Метрика» when the route named none. */
@Composable
private fun DetailHeader(
    label: String?,
    onBack: () -> Unit,
) {
    val palette = Cadence.palette

    Row(
        Modifier.fillMaxWidth().padding(CadenceSpacing.lg),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        CadenceIcon(
            paths = CadenceIcons.chevronLeft,
            tint = palette.ink2,
            modifier =
                Modifier
                    .clip(RoundedCornerShape(CadenceRadius.sm))
                    .clickable(onClick = onBack)
                    .testTag(CADENCE_TREND_DETAIL_BACK_TAG),
        )
        CadenceTitle(text = label ?: "Метрика")
    }
}

/**
 * The four aggregates.
 *
 * All of them come off the same window as the chart above. The prototype's
 * `seriesStats` does too — its bug is on the *list* screen's hero, not here —
 * but they are recomputed there from the series rather than stored, which is
 * why nothing on this screen holds a baseline of its own.
 */
@Composable
private fun StatStrip(trend: MetricTrend) {
    val palette = Cadence.palette
    val digits = trend.meta.decimals

    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg)
            .clip(RoundedCornerShape(CadenceRadius.lg))
            .background(palette.paper)
            .testTag(CADENCE_TREND_DETAIL_STATS_TAG)
            .padding(CadenceSpacing.md),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
    ) {
        Stat(STAT_AVERAGE, "Сред.", trend.series.average?.let { formatDecimal(it, digits) })
        Stat(STAT_MINIMUM, "Мин", trend.series.minimum?.let { formatDecimal(it, digits) })
        Stat(STAT_MAXIMUM, "Макс", trend.series.maximum?.let { formatDecimal(it, digits) })
        // The signed one, because «Δ» without a direction is half an answer.
        Stat(STAT_DELTA, "Δ", trend.series.delta?.let { formatSignedDelta(it, unitRu(trend.meta.unit), digits) })
    }
}

@Composable
private fun RowScope.Stat(
    tag: String,
    label: String,
    value: String?,
) {
    val palette = Cadence.palette

    // An even share each. A `Row` hands width out in child order, and «Δ» — the
    // only box carrying an arrow and a unit — is last and widest, so it is the
    // one a narrow screen clips.
    Column(Modifier.weight(1f).testTag(cadenceTrendStatTag(tag))) {
        CadenceEyebrow(text = label, color = palette.subtle)
        // A dash, not a blank: an empty box under a heading reads as a number
        // that failed to arrive.
        CadenceBody(text = value ?: "—")
    }
}

/**
 * «Последние записи» — the tail of the window, newest first.
 *
 * Newest first because that is the order a patient reads a log in, and the
 * chart above already runs the other way. The prototype's rows carry a note
 * («Утром · натощак»); `Measurement.note` is empty in the seed, so nothing is
 * drawn for it rather than a blank line being reserved — recorded as a
 * divergence.
 */
@Composable
private fun RecentReadings(trend: MetricTrend) {
    val palette = Cadence.palette

    Column(
        Modifier
            .fillMaxWidth()
            .padding(CadenceSpacing.lg)
            .testTag(CADENCE_TREND_DETAIL_RECENT_TAG),
    ) {
        CadenceEyebrow(text = "Последние записи", color = palette.subtle)

        trend.series.points
            .takeLast(RECENT_ROWS)
            .reversed()
            .forEach { point ->
                Row(
                    Modifier.fillMaxWidth().padding(vertical = CadenceSpacing.sm),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        CadenceBody(text = dayAndMonth(trend.series.dayOf(point)))
                        point.note?.takeIf { it.isNotBlank() }?.let {
                            CadenceMeta(text = it, color = palette.subtle)
                        }
                    }
                    CadenceBody(
                        text = "${formatDecimal(point.value, trend.meta.decimals)} ${unitRu(trend.meta.unit)}",
                    )
                }
            }
    }
}

@Composable
private fun UnknownMetric() {
    val palette = Cadence.palette

    Column(
        Modifier
            .fillMaxWidth()
            .padding(CadenceSpacing.lg)
            .testTag(CADENCE_TREND_DETAIL_UNKNOWN_TAG),
    ) {
        CadenceBody(text = "Такой метрики нет")
        CadenceMeta(
            text = "Вернитесь к списку и выберите одну из восьми.",
            color = palette.subtle,
        )
    }
}
