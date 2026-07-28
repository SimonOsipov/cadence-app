package app.cadence.design

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.TextUnit

// The text primitives, ported from mobile/src/components/primitives.tsx.
//
// BasicText rather than Material's Text: the product supplies its own type
// scale and there is no Material theme underneath to inherit from.

/** Uppercase micro-label above a section or a value. */
@Composable
fun CadenceEyebrow(
    text: String,
    modifier: Modifier = Modifier,
    color: Color = Cadence.palette.subtle,
) {
    BasicText(
        text = text.uppercase(),
        modifier = modifier,
        style = Cadence.typography.eyebrow.copy(color = color),
        maxLines = 1,
        overflow = TextOverflow.Ellipsis,
    )
}

@Composable
fun CadenceTitle(
    text: String,
    modifier: Modifier = Modifier,
    size: TextUnit = CadenceTitleSize,
    italic: Boolean = false,
    color: Color = Cadence.palette.ink,
    maxLines: Int = Int.MAX_VALUE,
) {
    val base = if (italic) Cadence.typography.titleEmphasis else Cadence.typography.title
    BasicText(
        text = text,
        modifier = modifier,
        // Leading and tracking are stored as ratios, so they follow the size.
        style = base.copy(color = color, fontSize = size),
        maxLines = maxLines,
        overflow = TextOverflow.Ellipsis,
    )
}

@Composable
fun CadenceBody(
    text: String,
    modifier: Modifier = Modifier,
    color: Color = Cadence.palette.ink2,
    maxLines: Int = Int.MAX_VALUE,
) {
    BasicText(
        text = text,
        modifier = modifier,
        style = Cadence.typography.body.copy(color = color),
        maxLines = maxLines,
        overflow = TextOverflow.Ellipsis,
    )
}

@Composable
fun CadenceMeta(
    text: String,
    modifier: Modifier = Modifier,
    color: Color = Cadence.palette.subtle,
    maxLines: Int = Int.MAX_VALUE,
) {
    BasicText(
        text = text,
        modifier = modifier,
        style = Cadence.typography.meta.copy(color = color),
        maxLines = maxLines,
        overflow = TextOverflow.Ellipsis,
    )
}

/**
 * A measured value and its unit, set as two runs: tabular mono for the number,
 * serif italic for the unit.
 *
 * The signature takes them apart on purpose. A dose is {value, unit} — «0,25»
 * and «мг» — and the decimal comma is a rendering decision made by a formatter,
 * never a number baked into a string upstream.
 */
@Composable
fun CadenceNumber(
    value: String,
    modifier: Modifier = Modifier,
    unit: String? = null,
    size: TextUnit = CadenceNumberSize,
    color: Color = Cadence.palette.ink,
    unitColor: Color = Cadence.palette.muted,
) {
    Row(
        modifier = modifier,
        verticalAlignment = Alignment.Bottom,
        horizontalArrangement = Arrangement.spacedBy(CadenceSpacing.xxs),
    ) {
        BasicText(
            text = value,
            style = Cadence.typography.number.copy(color = color, fontSize = size),
            maxLines = 1,
        )
        if (unit != null) {
            BasicText(
                text = unit,
                // The unit tracks the number rather than sitting at a fixed size.
                style =
                    Cadence.typography.numberUnit.copy(
                        color = unitColor,
                        fontSize = size * CADENCE_NUMBER_UNIT_RATIO,
                    ),
                maxLines = 1,
            )
        }
    }
}
