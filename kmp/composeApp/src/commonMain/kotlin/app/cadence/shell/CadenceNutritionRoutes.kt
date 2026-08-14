package app.cadence.shell

import androidx.compose.runtime.Composable
import androidx.compose.runtime.rememberCoroutineScope
import app.cadence.screens.nutrition.LogMealScreen
import app.cadence.shared.domain.MealDraft
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.repository.MealLogResult
import app.cadence.shared.repository.TodaySummary
import kotlinx.coroutines.launch
import kotlinx.datetime.LocalDateTime

// The nutrition port's own routes, split out of `CadenceShell.kt` — that file had reached
// detekt's function ceiling, and these five screens (the wizard, «Питание», «Рецепты», the
// recipe card and the builder) are one feature's worth of wiring.

/**
 * Gated on the read like its siblings — targets come from the day, and the header's clock
 * reading from [CadenceShellData.now], never from a literal (the prototype's own `08:42`).
 *
 * The write is awaited, and only a [MealLogResult.Written] closes the modal: a rejected
 * draft leaves the patient on the screen with what they typed, rather than dismissing it
 * as if it had been recorded.
 */
@Composable
internal fun LogMealModal(
    summary: TodaySummary?,
    now: LocalDateTime?,
    actions: CadenceShellActions,
    back: () -> Unit,
) {
    val scope = rememberCoroutineScope()

    if (summary == null || now == null) {
        PlaceholderScreen(title = "Записать приём пищи", onBack = back)
        return
    }

    LogMealScreen(
        now = now,
        targets = summary.targets,
        parse = actions.parseMeal,
        onSave = { draft ->
            scope.launch {
                if (actions.onMealLogged(draft) is MealLogResult.Written) back()
            }
        },
        onCancel = back,
    )
}

/**
 * Writes the meal and confirms it with the day total **from the write's own answer**.
 *
 * Not from the `summary` the shell is holding: that is still the pre-write snapshot when
 * this returns, because `LaunchedEffect(reloads)` re-reads only afterwards, and a toast
 * built from it names the day without the meal it is confirming.
 */
internal suspend fun logMeal(
    mocks: CadenceMocks,
    toast: ToastUiState,
    draft: MealDraft,
    onWritten: () -> Unit,
): MealLogResult {
    val result = mocks.nutrition.log(draft)
    if (result is MealLogResult.Written) {
        toast.raise(draft.name.orEmpty(), result.dayTotals.kcal)
        onWritten()
    }
    return result
}
