package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * CadenceIcon strokes a Heroicons outline path.
 *
 * One renderer for the whole set: the icons are path data (see [CadenceIcons]),
 * so adding one costs a string rather than a file. The canvas is scaled from
 * the 24-unit source viewport, which keeps the stroke proportional — a 16dp
 * icon does not come out visually heavier than a 24dp one.
 */
@Composable
fun CadenceIcon(
    paths: List<String>,
    modifier: Modifier = Modifier,
    size: Dp = 20.dp,
    tint: Color = Cadence.palette.ink2,
    // The prototype varies this per icon — 1 to 2.5 — so it is a parameter.
    strokeWidth: Float = CadenceIcons.STROKE_WIDTH,
) {
    // Parsing is pure string work; keep it off every recomposition.
    val outlines =
        remember(paths) {
            paths.map { PathParser().parsePathString(it).toPath() }
        }

    Canvas(modifier = modifier.size(size)) {
        val factor = this.size.minDimension / CadenceIcons.VIEWPORT
        scale(factor, factor, pivot = Offset.Zero) {
            outlines.forEach { outline ->
                drawPath(
                    path = outline,
                    color = tint,
                    style =
                        Stroke(
                            width = strokeWidth,
                            cap = StrokeCap.Round,
                            join = StrokeJoin.Round,
                        ),
                )
            }
        }
    }
}

// There was a `CadenceIcon(name: String)` overload here that resolved through
// `CadenceIcons.byName` and drew nothing on a miss. Six of its seven call sites
// passed a compile-time literal, and two of those literals were «chevrn-up» and
// «chevrn-dwn» — so the collapsible «В запасе» section shipped with no chevron
// in either state, silently, past both gates. No test in the tree ever passed an
// icon name as a string, and `DesignSystemTest` checks the size of `byName`
// rather than its keys, so nothing could have caught it.
//
// The typed vals on [CadenceIcons] are the whole API now, and a typo is a
// compile error. The one genuinely runtime name — a compound's `icon` field
// from the server — reads `byName` directly at `ProtocolStrip`, with an explicit
// `?: CadenceIcons.beaker` fallback where the miss is expected and handled.
