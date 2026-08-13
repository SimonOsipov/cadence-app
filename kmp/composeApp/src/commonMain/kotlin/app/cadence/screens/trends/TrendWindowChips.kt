package app.cadence.screens.trends

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import app.cadence.design.CadenceChip
import app.cadence.design.CadenceSpacing
import app.cadence.shared.domain.TrendWindow

/**
 * What a metric with nothing in the window says. One copy, beside the switcher,
 * for the switcher's reason: the list and the detail must not disagree about the
 * wording.
 */
internal const val NO_READINGS = "нет данных"

/** One tag per window, so «this chip is the chosen one» is one check each. */
fun cadenceTrendWindowTag(window: TrendWindow): String = "cadence-trend-window-${window.name.lowercase()}"

/** «7 дней» / «4 недели» / «3 месяца» / «Весь цикл», as the prototype writes them. */
private fun windowLabel(window: TrendWindow): String =
    when (window) {
        TrendWindow.WEEK -> "7 дней"
        TrendWindow.FOUR_WEEKS -> "4 недели"
        TrendWindow.THREE_MONTHS -> "3 месяца"
        TrendWindow.CYCLE -> "Весь цикл"
    }

/**
 * The window switcher, shared by the list and the detail. One component because
 * the prototype shares the *choice* between the two screens — it keeps the
 * timeframe in app state — and two copies of the labels would be two places to
 * disagree about what «весь цикл» is called.
 */
@Composable
internal fun TrendWindowChips(
    selectedWindow: TrendWindow,
    onChange: (TrendWindow) -> Unit,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.sm),
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm),
    ) {
        TrendWindow.entries.forEach { window ->
            CadenceChip(
                label = windowLabel(window),
                onClick = { onChange(window) },
                modifier =
                    Modifier
                        .testTag(cadenceTrendWindowTag(window))
                        // `CadenceChip` says «chosen» in colour alone, and a
                        // colour lands in no semantics: a screen reader hears
                        // four identical chips, and a test cannot tell which is
                        // active.
                        .semantics { selected = window == selectedWindow },
                active = window == selectedWindow,
            )
        }
    }
}
