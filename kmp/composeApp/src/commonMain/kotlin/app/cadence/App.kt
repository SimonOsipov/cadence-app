package app.cadence

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.sp
import app.cadence.shared.currentPlatform

/**
 * App is the single Compose entry point both platforms render. It is a
 * placeholder until the design system is ported from the frozen Expo prototype;
 * what it proves today is that one composable tree draws on Android and iOS and
 * that :shared is linked into both.
 *
 * The three colours are read from the prototype's palette (paper, forest900,
 * ink600) rather than invented — the real tokens arrive as a whole in BST-05.
 */
@Composable
fun App() {
    Box(
        modifier = Modifier.fillMaxSize().background(Color(0xFFFBF8F3)),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            BasicText(
                text = "Cadence",
                style = TextStyle(fontSize = 28.sp, color = Color(0xFF142C1F)),
            )
            BasicText(
                text = currentPlatform().name,
                style = TextStyle(fontSize = 14.sp, color = Color(0xFF5C5852)),
            )
        }
    }
}
