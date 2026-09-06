package app.cadence.design

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.SemanticsNodeInteractionsProvider
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.v2.runComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull

private const val A_SECRET = "correct-horse-battery"

private const val FIELD = "поле"

/** What the field draws, which is not what it holds — see the masked case. */
private fun SemanticsNodeInteractionsProvider.drawnIn(field: String) =
    onNodeWithContentDescription(field)
        .fetchSemanticsNode()
        .config
        .getOrNull(SemanticsProperties.EditableText)
        ?.text

private fun SemanticsNodeInteractionsProvider.declaredAPassword(field: String) =
    onNodeWithContentDescription(field).fetchSemanticsNode().config.getOrNull(SemanticsProperties.Password)

@OptIn(ExperimentalTestApi::class)
class CadenceMaskedFieldTest {
    // A password readable over a shoulder is the threat the sign-out button is justified by, in the
    // same clinic room. Measured on the semantics rather than the pixels, and the two properties
    // are not interchangeable: `EditableText` is what the field draws, `InputText` still holds the
    // typed value for the accessibility and autofill channels, so asserting on the wrong one would
    // pass for either setting.
    @Test
    fun aMaskedFieldDoesNotDrawWhatWasTyped() =
        runComposeUiTest {
            setContent { CadenceTheme { AField(masked = true) } }

            onNodeWithContentDescription(FIELD).performTextInput(A_SECRET)
            waitForIdle()

            assertNotEquals(A_SECRET, drawnIn(FIELD), "the password was drawn in clear text")
            assertNotNull(declaredAPassword(FIELD), "the field does not tell the platform it is a password")
            // The keyboard half, and it needs its own assertion: masking is a visual transform, so
            // every property above survives dropping the keyboard type — measured, that mutation
            // lived through the whole suite until this line.
            assertNotNull(
                onNodeWithContentDescription(FIELD)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.ContentType),
                "the platform IME was not told this is a password, so it learns what is typed",
            )
        }

    // The half that makes the one above mean anything: unmasked, the same field does draw what was
    // typed — measured, and the first version of the test above passed against a field that drew
    // nothing at all, because the state it held was not remembered.
    @Test
    fun anUnmaskedFieldDrawsWhatWasTyped() =
        runComposeUiTest {
            setContent { CadenceTheme { AField(masked = false) } }

            onNodeWithContentDescription(FIELD).performTextInput(A_SECRET)
            waitForIdle()

            assertEquals(A_SECRET, drawnIn(FIELD))
            assertNull(declaredAPassword(FIELD))
            assertNull(
                onNodeWithContentDescription(FIELD)
                    .fetchSemanticsNode()
                    .config
                    .getOrNull(SemanticsProperties.ContentType),
            )
        }
}

@Composable
private fun AField(masked: Boolean) {
    var typed by remember { mutableStateOf("") }

    CadenceTextField(
        value = typed,
        onValueChange = { typed = it },
        placeholder = FIELD,
        fieldModifier = Modifier.semantics { contentDescription = FIELD },
        masked = masked,
    )
}
