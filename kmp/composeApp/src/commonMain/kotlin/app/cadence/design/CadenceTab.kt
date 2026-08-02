package app.cadence.design

import androidx.compose.ui.graphics.Color

/**
 * The five destinations of the patient app, ported from `TABS` in
 * mobile/src/components/shared.tsx.
 *
 * The labels are product copy and stay Russian. The icon names index
 * [CadenceIcons].
 */
enum class CadenceTab(
    val icon: String,
    val label: String,
) {
    TODAY("home", "Сегодня"),
    INVENTORY("beaker", "Аптечка"),
    LOG("plus", "Записать"),
    TRENDS("chart-bar", "Тренды"),
    NUTRITION("cake", "Питание"),
}

/**
 * The tint a destination is drawn in.
 *
 * A colour cannot be queried from the composed tree, so the one rule worth
 * asserting — that the active destination looks different from the rest —
 * lives here where a test can reach it.
 */
internal fun cadenceTabTint(
    tab: CadenceTab,
    active: CadenceTab,
    subtle: Color,
): Color = if (tab == active) CadenceColors.forest700 else subtle
