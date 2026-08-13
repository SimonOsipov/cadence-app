package app.cadence.shell

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTabBar
import app.cadence.design.CadenceTitle
import app.cadence.shared.currentPlatform

/**
 * Scaffolding, not a screen — each route's replacement is one line in [CadenceShell].
 * Shows the platform name because `AppTest` proves `:shared` is linked into the UI via this
 * label; whoever deletes the last placeholder owes that assertion a new home.
 */
@Composable
fun PlaceholderScreen(
    title: String,
    modifier: Modifier = Modifier,
    destination: CadenceDestination? = null,
    onBack: (() -> Unit)? = null,
    onSelectTab: (CadenceDestination) -> Unit = { },
    onLog: () -> Unit = { },
    action: Pair<String, () -> Unit>? = null,
) {
    Column(modifier = modifier.fillMaxSize(), verticalArrangement = Arrangement.SpaceBetween) {
        Column(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .windowInsetsPadding(WindowInsets.statusBars)
                    .padding(CadenceSpacing.xl),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        ) {
            if (onBack != null) {
                CadenceIconButton(
                    icon = CadenceIcons.chevronLeft,
                    contentDescription = "Назад",
                    onClick = onBack,
                )
            }
            // Not the bare label: it's also the tab's own text, and two nodes with the same
            // word make every assertion about which screen is showing ambiguous.
            CadenceTitle("Экран «$title»")
            CadenceMeta("заглушка · ${currentPlatform().name}")
            if (action != null) {
                CadenceButton(label = action.first, onClick = action.second)
            }
        }

        if (destination != null) {
            // Kept per-screen like the prototype, not hoisted to the shell: Schedule and
            // Journal have no bar, and hoisting would put one there too.
            CadenceTabBar(active = destination, onSelect = onSelectTab, onLog = onLog)
        }
    }
}
