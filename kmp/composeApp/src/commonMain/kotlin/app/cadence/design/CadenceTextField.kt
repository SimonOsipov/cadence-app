package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp

/** The field's own hairline border, matching the private `HAIRLINE` on the two screens it replaces. */
private val TEXT_FIELD_BORDER = 1.dp

/**
 * A bordered text input: `paper` background, `border` outline, [CadenceRadius.md]
 * corners — the shape `DoseSteps.kt` drew before this task folded it in here
 * (`aaf74de:DoseSteps.kt:380-382`), so the two raw `BasicTextField` call sites
 * this task converts (the note field, `AddVialScreen.kt`'s five form fields)
 * stop each repeating it.
 *
 * Controlled, not stateful: the field always renders [value], never a
 * character it was typed but the parent hasn't echoed back through
 * [onValueChange] — a parent that rejects an edit isn't silently overridden
 * by the field's own buffer.
 *
 * **Two modifiers, deliberately.** [modifier] lands on the outer box, matching
 * every other primitive in this package (`CadenceMacroBar.kt`,
 * `CadenceWeekBars.kt`, `CadenceControls.kt`, `CadenceSegmented.kt` all apply
 * `modifier.then(ownChain)` to their outer node). [fieldModifier] lands on the
 * editable [BasicTextField] beneath it, for the few things that must target
 * that exact node — chiefly a `testTag` a test calls `performTextReplacement` on.
 *
 * This widens the brief's four-parameter modifier surface deliberately, not by
 * oversight: routing the *whole* `modifier` to the inner field (this
 * component's first shape) put a caller's `Modifier.weight(1f)` on the
 * `BasicTextField`, not the box — and the box's unconditional `fillMaxWidth()`
 * still ate the entire row, leaving a sibling at width zero (the same
 * absorption this component's `singleLine` test already hit once with a
 * `.width(...)` modifier). That breaks the two shapes this component exists
 * for — the chat composer's `flex: 1` beside a send button
 * (`ChatThreadScreen.tsx:290-312`) and the ingredient search's `flex: 1`
 * beside an icon (`RecipeBuilderScreen.tsx:187-199`) — both needing `weight`
 * to reach the box. Overridden by the plan owner on the same grounds as the
 * stepper's signature in Task 1: the package convention wins.
 */
@Composable
fun CadenceTextField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier,
    fieldModifier: Modifier = Modifier,
    singleLine: Boolean = false,
    minLines: Int = 1,
    masked: Boolean = false,
) {
    Box(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(CadenceRadius.md))
            .background(Cadence.palette.paper)
            .border(TEXT_FIELD_BORDER, Cadence.palette.border, RoundedCornerShape(CadenceRadius.md))
            .padding(CadenceSpacing.md),
    ) {
        if (value.isEmpty()) CadenceMeta(placeholder)
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            textStyle = Cadence.typography.body.copy(color = Cadence.palette.ink),
            singleLine = singleLine,
            minLines = minLines,
            // Both halves, and the second is the one a screenshot does not show: without the
            // password keyboard type the platform treats the field as ordinary text and learns
            // what is typed into it.
            visualTransformation = if (masked) PasswordVisualTransformation() else VisualTransformation.None,
            keyboardOptions =
                if (masked) KeyboardOptions(keyboardType = KeyboardType.Password) else KeyboardOptions.Default,
            modifier = Modifier.fillMaxWidth().then(fieldModifier),
        )
    }
}
