package app.cadence

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.LocalSaveableStateRegistry
import androidx.compose.runtime.saveable.SaveableStateRegistry
import androidx.compose.runtime.setValue
import app.cadence.shared.auth.Acceptance

/**
 * What the platform does to a screen it recreates.
 *
 * Driven through the registry rather than `StateRestorationTester`, which answers a
 * `NotImplementedError` outside Android — measured on Compose 1.11.1, the version this module
 * builds against, and composeApp's tests run on the iOS target only. The subtree is disposed and
 * composed again in its own place rather than under a second `setContent`: `rememberSaveable`'s
 * own deprecation of the `key` parameter calls the alternative «positional scoping», and two
 * roots do not hold one position.
 */
internal class Recreation {
    private var registry by mutableStateOf(bundleLike(null))
    private var alive by mutableStateOf(true)

    @Composable
    fun around(content: @Composable () -> Unit) =
        CompositionLocalProvider(LocalSaveableStateRegistry provides registry) {
            if (alive) content()
        }

    fun happen(settle: () -> Unit) {
        val kept = registry.performSave()

        alive = false
        settle()

        registry = bundleLike(kept)
        alive = true
        settle()
    }

    // Refuses the one answer a saver has to convert, asked of the value inside a state wrapper
    // because that is what `rememberSaveable { mutableStateOf(…) }` hands over — measured in the
    // runtime-saveable bytecode, which calls `canBeSaved` on the wrapper. Not a model of a Bundle;
    // the rest of the tree brings saved state of its own. Without it the saver is unmeasured:
    // under a registry that accepts everything, dropping it leaves every recreation test green.
    private fun bundleLike(kept: Map<String, List<Any?>>?) =
        SaveableStateRegistry(kept) { saved ->
            val value = if (saved is MutableState<*>) saved.value else saved

            value !is Acceptance
        }
}
