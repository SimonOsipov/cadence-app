package app.cadence.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp

/** The field's own hairline border, matching the private `HAIRLINE` on the two screens it replaces. */
private val TEXT_FIELD_BORDER = 1.dp

/**
 * A bordered text input: `paper` background, `border` outline, [CadenceRadius.md]
 * corners — the shape `DoseSteps.kt` drew before this task folded it in here
 * (`aaf74de:DoseSteps.kt:380-382`, the commit immediately before this task):
 * ```
 * .background(Cadence.palette.paper)
 * .border(HAIRLINE, Cadence.palette.border, RoundedCornerShape(CadenceRadius.md))
 * .padding(CadenceSpacing.md),
 * ```
 * folded into one primitive so the two raw `BasicTextField` call sites this task
 * converts — that note field, and `AddVialScreen.kt`'s five form fields — stop
 * each repeating it.
 *
 * Controlled, not stateful: the field always renders [value], never a character
 * it was typed but the parent has not echoed back through [onValueChange] — a
 * parent that rejects an edit is not silently overridden by the field's own
 * buffer.
 *
 * **Two modifiers, deliberately.** [modifier] lands on the outer box — the whole
 * field, background and border included — matching every other primitive in
 * this package (`CadenceMacroBar.kt`, `CadenceWeekBars.kt`, `CadenceControls.kt`,
 * `CadenceSegmented.kt` all apply `modifier.then(ownChain)` to their outer node).
 * [fieldModifier] lands on the editable [BasicTextField] beneath it, for the
 * handful of things that have to target that exact node rather than the box
 * around it — a `testTag` a test then calls `performTextReplacement` on, chiefly.
 *
 * This is a deliberate widening of the brief's four-parameter modifier surface,
 * not an oversight: routing the *whole* `modifier` to the inner field (this
 * component's first shape) meant a caller's `Modifier.weight(1f)` landed on the
 * `BasicTextField`, not the box — and the box always calls `fillMaxWidth()`
 * unconditionally, so it still ate the entire row and left a sibling at width
 * zero, the same absorption this component's own `singleLine` test already hit
 * once with a `.width(...)` modifier. That breaks the two shapes this component
 * exists for: the chat composer sits `flex: 1` beside a send button
 * (`mobile/src/features/chat/ChatThreadScreen.tsx:290-312`) and the ingredient
 * search sits `flex: 1` beside an icon
 * (`mobile/src/features/recipe/RecipeBuilderScreen.tsx:187-199`). Overridden by
 * the plan owner on the same grounds as the stepper's signature in Task 1: the
 * package convention wins, and both prototype shapes above need `weight` to
 * actually reach the box.
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
            modifier = Modifier.fillMaxWidth().then(fieldModifier),
        )
    }
}
