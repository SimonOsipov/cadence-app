package app.cadence.design

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlin.math.round

/** `size: 120` on the prototype's ring (`NutritionScreen.tsx:31`). */
private val RING_SIZE = 120.dp

/** `stroke: 10` on the prototype's outer, kcal ring (`NutritionScreen.tsx:32,55`). */
private val OUTER_STROKE = 10.dp

/** `stroke={6}` on the prototype's inner, protein ring (`NutritionScreen.tsx:61,69`). */
private val INNER_STROKE = 6.dp

/**
 * The prototype's `innerR = r - (stroke + 4)` (`NutritionScreen.tsx:41`) — the
 * gap between the two rings, on top of the outer ring's own stroke.
 */
private val RING_GAP = 4.dp

/** `fontSize={22}` on the prototype's percent readout (`NutritionScreen.tsx:90`). */
private val RING_PERCENT_SIZE = 22.sp

/**
 * The rings, so a test can reach both fractions at once. A `Canvas` draws
 * nothing a test can read, and the centre text names only the calorie ring —
 * without this, the protein ring's own wiring is unreachable by anything:
 * wiring it to read calories instead of protein would leave every test in
 * this suite green.
 */
const val CADENCE_RINGS_TAG = "cadence-rings"

private const val PERCENT = 100f
private const val FULL_SWEEP_DEGREES = 360f

/**
 * `transform="rotate(-90 ...)"` on both arcs (`NutritionScreen.tsx:58,72`): the
 * sweep starts at 12 o'clock rather than at 3 o'clock, which is where
 * [androidx.compose.ui.graphics.drawscope.DrawScope.drawArc] starts a 0°
 * angle.
 */
private const val START_ANGLE_DEGREES = -90f

/**
 * Value against goal, as a fraction of a ring's sweep.
 *
 * Not [macroFraction], even though the formula is the same shape (`Math.min(1,
 * v / goal)` in the prototype, same as the macro bar's clamp): sharing it would
 * couple the nutrition screen's rings to its bars, so an edit to one — say, a
 * new rounding rule for a dosing bar — would silently move the other. Clamped
 * at the top for the reason every gauge in this design system is: a patient
 * over goal reads a full ring, not an arc that overshoots its own track. A
 * zero goal draws nothing rather than dividing — a patient without a target
 * set is a real state, not an error.
 */
fun ringSweep(
    value: Double,
    goal: Double,
): Float = if (goal <= 0.0) 0f else (value / goal).toFloat().coerceIn(0f, 1f)

/**
 * Two concentric rings — outer for calories, inner for protein — with the
 * calorie percentage read out at the centre. `NutritionRing` in the prototype
 * (`NutritionScreen.tsx:24-98`).
 *
 * The arcs are drawn on a `Canvas`: a circle has no `Box`/`weight` expression
 * the way every other bar in this design system does, which is why this is
 * the one primitive here that uses one. But the fraction each arc sweeps
 * comes from [ringSweep], not from arithmetic inside the drawing lambda — a
 * canvas's pixels are asserted nowhere in this codebase, so the number a test
 * can reach has to live beside the draw call, not inside it.
 *
 * The centre text is real nodes laid over the canvas, not more drawing: a
 * `Canvas` has no text and no children, the same reason [CadenceBodyMap]
 * overlays its zone targets rather than drawing them.
 *
 * The container also carries both rings' fractions as a `contentDescription`
 * — the same hook [CadenceGauge] and the syringe bar in `CadenceStepper.kt`
 * use for a canvas-invisible fraction. The centre text alone names only the
 * calorie ring, so without this the protein ring's own sweep would be
 * unobservable by any test or screen reader.
 */
@Composable
fun CadenceRings(
    kcal: Double,
    kcalGoal: Double,
    proteinG: Double,
    proteinGoal: Double,
    modifier: Modifier = Modifier,
) {
    val palette = Cadence.palette
    val kcalSweep = ringSweep(kcal, kcalGoal)
    val proteinSweep = ringSweep(proteinG, proteinGoal)
    val kcalPercent = round(kcalSweep * PERCENT).toInt()
    val proteinPercent = round(proteinSweep * PERCENT).toInt()

    Box(
        modifier
            .size(RING_SIZE)
            .testTag(CADENCE_RINGS_TAG)
            .semantics {
                contentDescription =
                    "Калории ${round(kcal).toInt()} из ${round(kcalGoal).toInt()} ккал · $kcalPercent% · " +
                    "Белок ${round(proteinG).toInt()} из ${round(proteinGoal).toInt()} г · $proteinPercent%"
            },
        contentAlignment = Alignment.Center,
    ) {
        Canvas(Modifier.size(RING_SIZE)) {
            val outerStrokePx = OUTER_STROKE.toPx()
            val outerRadius = (size.minDimension - outerStrokePx) / 2f
            val innerRadius = outerRadius - (outerStrokePx + RING_GAP.toPx())

            drawRing(
                radius = outerRadius,
                strokeWidth = outerStrokePx,
                trackColor = palette.sunk,
                fillColor = CadenceColors.sand500,
                sweepFraction = kcalSweep,
            )
            drawRing(
                radius = innerRadius,
                strokeWidth = INNER_STROKE.toPx(),
                trackColor = palette.sunk,
                fillColor = CadenceColors.forest700,
                sweepFraction = proteinSweep,
            )
        }

        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            CadenceEyebrow("Ккал")
            // The calorie ring's own fraction, not the protein one's — on the
            // brief's lopsided fixture (900/1800 kcal, 35/140 g protein) the
            // two disagree, «50%» against «25%», and this is the number the
            // test measures. `%` is a unit, the same as «мг» or «г» — passed
            // to `unit`, not folded into `value`: "numbers are data,
            // formatting is presentation".
            CadenceNumber(value = kcalPercent.toString(), unit = "%", size = RING_PERCENT_SIZE)
        }
    }
}

/** One ring: a full track plus an arc swept to [sweepFraction] of it. */
private fun DrawScope.drawRing(
    radius: Float,
    strokeWidth: Float,
    trackColor: Color,
    fillColor: Color,
    sweepFraction: Float,
) {
    val diameter = radius * 2f
    val topLeft = Offset(center.x - radius, center.y - radius)
    val arcSize = Size(diameter, diameter)
    val stroke = Stroke(width = strokeWidth, cap = StrokeCap.Round)

    drawArc(
        color = trackColor,
        startAngle = 0f,
        sweepAngle = FULL_SWEEP_DEGREES,
        useCenter = false,
        topLeft = topLeft,
        size = arcSize,
        style = stroke,
    )

    if (sweepFraction > 0f) {
        drawArc(
            color = fillColor,
            startAngle = START_ANGLE_DEGREES,
            sweepAngle = FULL_SWEEP_DEGREES * sweepFraction,
            useCenter = false,
            topLeft = topLeft,
            size = arcSize,
            style = stroke,
        )
    }
}
