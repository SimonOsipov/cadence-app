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
 * Scaffolding, not a screen. Every route renders one until steps 3–9 of the
 * block replace it with the ported article, and each replacement is one line in
 * [CadenceShell].
 *
 * It shows the platform name because `AppTest` used to prove `:shared` is linked
 * into the UI and not merely into the module graph, and the showcase that
 * carried that proof is gone. **Whoever deletes the last placeholder owes that
 * assertion a new home** — a real screen must not grow a platform label to keep
 * a test green.
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
            // «Экран «Сегодня»», not «Сегодня»: the bare label is also the tab's
            // own text, and two nodes reading the same word make every
            // assertion about which screen is showing ambiguous.
            CadenceTitle("Экран «$title»")
            CadenceMeta("заглушка · ${currentPlatform().name}")
            if (action != null) {
                CadenceButton(label = action.first, onClick = action.second)
            }
        }

        if (destination != null) {
            // In the prototype the bar lives inside the four screens that have
            // one, not in the navigator — Schedule and Journal have none. The
            // port keeps it there rather than hoisting it into the shell,
            // because hoisting it would put a bar on the screens without one.
            CadenceTabBar(active = destination, onSelect = onSelectTab, onLog = onLog)
        }
    }
}
