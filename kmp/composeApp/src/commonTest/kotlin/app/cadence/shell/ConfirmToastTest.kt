package app.cadence.shell

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.mock.CadenceMocks
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertTrue

private const val TARGET_KCAL = 2100

/**
 * The running total *after* the meal, not the meal's own figure — matches
 * `showConfirm({ kcal: nextTotals.kcal, … })` in AppState.tsx. Easily confused; the toast
 * would render a plausible wrong number.
 */
private const val DAY_KCAL = 1240

/**
 * Not the default `CadenceMocks()`: that reads the system clock, and once the repository
 * filtered meals by date a test on any other day saw an empty one.
 */
private fun seededDay() = CadenceMocks(FixedCadenceClock.at("2026-05-31T09:00:00Z"), TimeZone.of("Europe/Moscow"))

@OptIn(ExperimentalTestApi::class)
class ConfirmToastTest {
    @Test
    fun theToastNamesTheMealAndBothSidesOfTheTarget() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    ConfirmToast(
                        state = ConfirmToastState(mealName = "Обед", dayKcal = DAY_KCAL),
                        targetKcal = TARGET_KCAL,
                    )
                }
            }

            onNodeWithText("Обед · записано").assertIsDisplayed()
            // Escaped, not typed: a plain space here fails against a correct implementation
            // while looking identical in the diff — it did, twice.
            onNodeWithText("1\u00A0240 / 2\u00A0100 ккал сегодня").assertIsDisplayed()
        }

    @Test
    fun noStateMeansNoToast() =
        runComposeUiTest {
            setContent { CadenceTheme { ConfirmToast(state = null, targetKcal = TARGET_KCAL) } }

            assertTrue(onAllNodesWithText("записано", substring = true).fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun loggingAMealRaisesTheToastAndItLeavesOnItsOwn() =
        runComposeUiTest {
            mainClock.autoAdvance = false
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = seededDay()) }
            }

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            settle()

            // Measured from the tap: the modal's own dismissal takes 380ms, and letting it run
            // first would spend a fifth of the toast's life before the first assertion.
            onNodeWithText("Записать").performClick()
            mainClock.advanceTimeBy(FRAME_MS)
            waitForIdle()
            onNodeWithText("Обед · записано").assertIsDisplayed()

            // Bracketed with literals deliberately, not CADENCE_CONFIRM_TOAST_MS: an assertion
            // that advances by the constant it's checking passes at 17000ms as happily as 1700 —
            // a mutation proved that before these two lines replaced one.
            mainClock.advanceTimeBy(JUST_UNDER_THE_DEADLINE_MS)
            onNodeWithText("Обед · записано").assertIsDisplayed()

            mainClock.advanceTimeBy(PAST_THE_DEADLINE_MS)
            assertTrue(onAllNodesWithText("Обед · записано").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theToastRaisedByTheShellCarriesTheShellsOwnNumbers() =
        runComposeUiTest {
            mainClock.autoAdvance = false
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = seededDay()) }
            }

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            settle()
            onNodeWithText("Записать").performClick()
            mainClock.advanceTimeBy(FRAME_MS)
            waitForIdle()

            // The unit tests above supply their own numbers, so nothing pinned what the shell
            // actually passes — a target of 0 went unnoticed. 840/1800 are the repository's now.
            onNodeWithText("840 / 1\u00A0800 ккал сегодня").assertIsDisplayed()
        }

    @Test
    fun theToastSwallowsEveryTouchWhileItIsUp() =
        runComposeUiTest {
            mainClock.autoAdvance = false
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = seededDay()) }
            }

            logAMeal(nav)
            onNodeWithText("Обед · записано").assertIsDisplayed()

            // Matches the prototype's pointerEvents="auto": nothing beneath is live for 1700ms,
            // or a tap on "+" would open the action sheet through this layer.
            onNodeWithText("Тренды").performClick()
            waitForIdle()

            assertTrue(
                nav.currentBackStack.value.none { it.destination.hasRoute<CadenceRoute.Trends>() },
                "a tap went through the toast overlay",
            )
        }
}

/** One transition's worth of frames, plus slack. */
private const val FRAME_MS = 100L

/**
 * Written out rather than derived. With the 100ms already spent above, alive at 1600 and
 * gone at 1800 — a bracket a wrong constant can't slip through either side.
 */
private const val JUST_UNDER_THE_DEADLINE_MS = 1500L
private const val PAST_THE_DEADLINE_MS = 200L

/** Opens the meal wizard, taps «Записать», and lets the toast appear. */
@OptIn(ExperimentalTestApi::class)
private fun androidx.compose.ui.test.ComposeUiTest.logAMeal(nav: NavHostController) {
    runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
    settle()
    onNodeWithText("Записать").performClick()
    mainClock.advanceTimeBy(FRAME_MS)
    waitForIdle()
}

@OptIn(ExperimentalTestApi::class)
private fun androidx.compose.ui.test.ComposeUiTest.settle() {
    // autoAdvance is off (toast timer driven by hand), so each transition is walked explicitly.
    mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS.toLong() + FRAME_MS)
    waitForIdle()
}
