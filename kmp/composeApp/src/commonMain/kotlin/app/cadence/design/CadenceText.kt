package app.cadence.design

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
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
    color: Color = Cadence.palette.ink,
    maxLines: Int = Int.MAX_VALUE,
) {
    BasicText(
        text = text,
        modifier = modifier,
        // Leading and tracking are stored as ratios, so they follow the size.
        style = Cadence.typography.title.copy(color = color, fontSize = size),
        maxLines = maxLines,
        overflow = TextOverflow.Ellipsis,
    )
}

/**
 * A title carrying styled runs, for [cadenceEmphasisedTitle].
 *
 * Separate overload rather than one function taking AnnotatedString: a plain
 * title is what almost every call site wants, and making them all wrap a
 * String would be noise for the sake of the rarer case.
 */
@Composable
fun CadenceTitle(
    text: AnnotatedString,
    modifier: Modifier = Modifier,
    size: TextUnit = CadenceTitleSize,
    color: Color = Cadence.palette.ink,
    maxLines: Int = Int.MAX_VALUE,
) {
    BasicText(
        text = text,
        modifier = modifier,
        style = Cadence.typography.title.copy(color = color, fontSize = size),
        maxLines = maxLines,
        overflow = TextOverflow.Ellipsis,
    )
}

/**
 * A title with one word set in the italic display face — «Кому *напишем*?»,
 * «Ваша *аптечка*», «Что *прибыло?*».
 *
 * One string with a span, not three composables in a row: in the prototype the
 * emphasis is a `Text` nested inside a `Text`, which flows inline and wraps as
 * one paragraph. Three siblings in a Row would neither wrap together nor be
 * reachable by the whole sentence.
 *
 * Emphasis first, prefix and suffix optional: of the 12 uses across 6 files in
 * the frozen prototype, 11 are prefix + emphasis (+ optional suffix) and one —
 * the dose in VialDetailSheet — is the emphasis alone. An earlier count put it
 * at five, which is why the first shape made the prefix mandatory.
 *
 * Not a general builder, because a second emphasised run has no call site to
 * design against. What breaks first is nesting, and that replaces this
 * signature rather than extending it — fixed arity does not grow a fourth part.
 */
@Composable
fun cadenceEmphasisedTitle(
    emphasis: String,
    prefix: String = "",
    suffix: String = "",
    emphasisColor: Color = CadenceColors.forest700,
): AnnotatedString {
    val style =
        Cadence.typography.titleEmphasis
            .toSpanStyle()
            // Everything the emphasis style carries, not a hand-picked three:
            // naming fontFamily, weight and slant by hand happened to be enough
            // only because title and titleEmphasis share their metrics today,
            // and the first time the italic wants its own tracking the emphasis
            // would silently keep the upright's.
            //
            // The size is the exception, and it has to be: the style carries the
            // scale's default 28sp, while the call site chooses the size on
            // CadenceTitle. Leaving it set would put a 28sp word inside a 22sp
            // line.
            .copy(fontSize = TextUnit.Unspecified, color = emphasisColor)

    return buildAnnotatedString {
        append(prefix)
        withStyle(style) { append(emphasis) }
        append(suffix)
    }
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
