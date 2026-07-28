package app.cadence

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import app.cadence.design.Cadence
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceCard
import app.cadence.design.CadenceChip
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceNumber
import app.cadence.design.CadencePill
import app.cadence.design.CadenceSheet
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTheme
import app.cadence.design.CadenceTitle
import app.cadence.shared.currentPlatform

/**
 * App is the single Compose entry point both platforms render.
 *
 * It is still a placeholder — the 24 real screens are ported one milestone at a
 * time out of the frozen Expo prototype. What it shows today is the design
 * system that came over ahead of them: the palette, the type scale, the
 * primitives and the icon set, all rendering identically on Android and iOS.
 */
@Composable
fun App() {
    CadenceTheme {
        var sheetOpen by remember { mutableStateOf(false) }
        var weekly by remember { mutableStateOf(true) }

        Column(
            modifier = Modifier.fillMaxSize().padding(CadenceSpacing.xl),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.lg, Alignment.CenterVertically),
        ) {
            CadenceEyebrow("сегодня")
            CadenceTitle("Cadence")
            CadenceMeta(currentPlatform().name)

            CadenceCard(modifier = Modifier.fillMaxWidth()) {
                CadenceEyebrow("следующая доза")
                // The value and its unit stay two fields all the way to the
                // screen; «0,25» is this locale's rendering of 0.25.
                CadenceNumber(value = "0,25", unit = "мг")
                CadencePill("по расписанию")
            }

            Row(horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.sm)) {
                CadenceChip("Неделя", onClick = { weekly = true }, active = weekly)
                CadenceChip("Месяц", onClick = { weekly = false }, active = !weekly)
            }

            CadenceButton(
                label = "Открыть лист",
                onClick = { sheetOpen = true },
                icon = CadenceIcons.plus,
                fillWidth = true,
            )
        }

        CadenceSheet(open = sheetOpen, onDismiss = { sheetOpen = false }) {
            CadenceTitle("Лист")
            CadenceMeta("Нажмите на затемнение, чтобы закрыть")
        }
    }
}
