package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import app.cadence.format.formatDose
import app.cadence.shared.domain.Dose
import app.cadence.shared.domain.DoseUnit
import kotlin.math.round

/** The prototype's two rates: `step={state.unit === 'мкг' ? 25 : 0.05}`. */
private const val STEP_MG = 0.05
private const val STEP_MCG = 25.0

/**
 * Rounding, not tidiness: `0.25 + 0.05` is `0.30000000000000004` in binary
 * floating point, and a dose is two significant figures. The prototype does the
 * same thing with `+(num + delta).toFixed(3)`.
 *
 * Three decimal places, and it has to *round* rather than truncate. Truncating
 * turns `0.35 + 0.05` into `0.399` — a value `formatDose` then rounds back to
 * «0,4 мг» on screen, so the number the patient reads and the number written to
 * `dose_events` stop being the same one.
 */
private const val DOSE_SCALE = 1000.0

private val STEPPER_BUTTON = 52.dp

/**
 * The rate a unit steps at — the prototype's own two.
 *
 * A default rather than a rule buried in the control: a compound whose
 * titration moves in something other than 0,05 мг is a fact about the protocol,
 * and the caller overrides `step` without touching the design system.
 */
fun stepFor(unit: DoseUnit): Double =
    when (unit) {
        DoseUnit.MG -> STEP_MG
        DoseUnit.MCG -> STEP_MCG
    }

/**
 * «Сколько взять?» — the big mono number with a minus and a plus.
 *
 * It reports a [Dose] rather than a delta, and never a string. The prototype
 * holds `dose` as a `string` and does `parseFloat` on every tap, which is the
 * subtask's one explicit prohibition; the arithmetic is here, once, and the
 * comma in «0,25» is [formatDose]'s business.
 */
@Composable
fun CadenceDoseStepper(
    dose: Dose,
    onChange: (Dose) -> Unit,
    modifier: Modifier = Modifier,
    step: Double = stepFor(dose.unit),
) {
    Row(
        modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // A bar rather than an icon: there is no «minus» in `CadenceIcons`,
        // and `CadenceIcon` draws nothing for a name it does not know — so the
        // decrement button on the product's critical path would have rendered
        // as an empty circle with every test still green. The prototype draws
        // its own `<Line strokeWidth={2} strokeLinecap="round">` for the same
        // reason.
        StepperButton("Уменьшить дозу") { onChange(dose.bumped(-step)) }

        val (number, unit) = formatDose(dose)

        CadenceNumber(
            value = number,
            unit = unit,
            modifier = Modifier.padding(horizontal = CadenceSpacing.xl),
        )

        StepperButton("Увеличить дозу", icon = CadenceIcons.plus) { onChange(dose.bumped(step)) }
    }
}

/** Never below zero, and the unit travels with the number. */
private fun Dose.bumped(delta: Double): Dose {
    val next = round((value + delta) * DOSE_SCALE) / DOSE_SCALE

    return copy(value = next.coerceAtLeast(0.0))
}

@Composable
private fun StepperButton(
    label: String,
    icon: List<String>? = null,
    onClick: () -> Unit,
) {
    Box(
        Modifier
            .size(STEPPER_BUTTON)
            .clip(CircleShape)
            .background(Cadence.palette.paper)
            .clickable(onClick = onClick)
            .clearAndSetSemantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        if (icon == null) {
            Box(
                Modifier
                    .size(width = CadenceSpacing.xxl, height = MINUS_BAR)
                    .clip(RoundedCornerShape(CadenceRadius.pill))
                    .background(Cadence.palette.ink2),
            )
        } else {
            CadenceIcon(paths = icon, size = CadenceSpacing.xxl, tint = Cadence.palette.ink2)
        }
    }
}

/** `strokeWidth={2}` on the prototype's minus. */
private val MINUS_BAR = 2.dp

/** The handle a test needs, because a `Canvas` has no text. */
const val CADENCE_SYRINGE_TAG = "cadence-syringe"

/** The fill itself, so a test can measure how wide it actually came out. */
const val CADENCE_SYRINGE_FILL_TAG = "cadence-syringe-fill"

/**
 * How much of the barrel is filled, as a fraction.
 *
 * A function rather than arithmetic inside the drawing, so the clamping is
 * testable: `Canvas` renders and asserts nothing. Clamped at both ends, as the
 * prototype's `Math.min(100, Math.max(0, …))` is — a fill wider than the barrel
 * draws outside it. A barrel of zero draws nothing rather than dividing.
 */
fun syringeFillFraction(
    units: Float,
    max: Float,
): Float = if (max <= 0f) 0f else (units / max).coerceIn(0f, 1f)

private val SYRINGE_HEIGHT = 22.dp

private const val PERCENT = 100f

/** «На шприце 100 ед.» — the horizontal pen under the stepper. */
@Composable
fun CadenceSyringeBar(
    units: Float,
    max: Float,
    modifier: Modifier = Modifier,
) {
    val fraction = syringeFillFraction(units, max)

    Column(modifier.fillMaxWidth()) {
        Box(
            Modifier
                .fillMaxWidth()
                .height(SYRINGE_HEIGHT)
                .clip(RoundedCornerShape(CadenceRadius.pill))
                .background(Cadence.palette.sunk)
                .border(HAIRLINE, Cadence.palette.border, RoundedCornerShape(CadenceRadius.pill))
                .testTag(CADENCE_SYRINGE_TAG)
                // `semantics`, not `clearAndSetSemantics`: clearing would take
                // the fill's own tag with it, and the fill's width is the one
                // thing a test can measure about a bar.
                .semantics {
                    contentDescription =
                        "${round(units.toDouble()).toInt()} из ${round(max.toDouble()).toInt()} ед. · " +
                        "${round(fraction * PERCENT).toInt()}%"
                },
        ) {
            // A laid-out box rather than a `Canvas`: a canvas's pixels are
            // asserted nowhere in this codebase, so a barrel painted full
            // regardless of the dose passed every test. A `fillMaxWidth(fraction)`
            // child has bounds, and bounds are measurable.
            Box(
                Modifier
                    .fillMaxWidth(fraction)
                    .fillMaxHeight()
                    .background(CadenceColors.sand500)
                    .testTag(CADENCE_SYRINGE_FILL_TAG),
            )

            SyringeTicks()

            // The needle, at the far end of the barrel.
            Box(
                Modifier
                    .align(Alignment.CenterEnd)
                    .size(width = NEEDLE_LENGTH, height = NEEDLE_THICKNESS)
                    .background(Cadence.palette.border),
            )
        }

        Row(
            Modifier
                .fillMaxWidth()
                .padding(top = CadenceSpacing.xs),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            CadenceMeta("0u")
            CadenceMeta("${round(units.toDouble()).toInt()} ед. в шприце", color = Cadence.palette.ink2)
            CadenceMeta("${round(max.toDouble()).toInt()}u")
        }
    }
}

/** Every tenth of the barrel, with the halves and the middle drawn stronger. */
@Composable
private fun BoxScope.SyringeTicks() {
    for (tenth in 1 until TICKS) {
        Box(
            Modifier
                .align(Alignment.CenterStart)
                .fillMaxWidth(tenth / TICKS.toFloat())
                .fillMaxHeight()
                .padding(vertical = CadenceSpacing.xxs)
                .wrapContentWidth(Alignment.End)
                .width(HAIRLINE)
                .fillMaxHeight()
                .background(
                    Cadence.palette.border.copy(
                        alpha = if (tenth % HALF_TICK == 0) TICK_STRONG else TICK_FAINT,
                    ),
                ),
        )
    }
}

/** The prototype draws nine of them, at every tenth. */
private const val TICKS = 10
private const val HALF_TICK = 5
private const val TICK_STRONG = 0.6f
private const val TICK_FAINT = 0.25f

private val NEEDLE_LENGTH = 18.dp

/** `borderWidth: 1` and the needle's `height: 2` in the prototype. */
private val HAIRLINE = 1.dp
private val NEEDLE_THICKNESS = 2.dp
