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

/**
 * The prototype's `MEAL_TARGETS.kcal`, and the day total the placeholder emits.
 *
 * `DAY_KCAL` is the running total *after* the meal, not the meal's own figure —
 * `showConfirm({ kcal: nextTotals.kcal, … })` in AppState.tsx. The two are
 * easily confused and the toast would render a plausible wrong number.
 */
private const val TARGET_KCAL = 2100
private const val DAY_KCAL = 1240

/**
 * The mocks wound to the day the seed has meals on.
 *
 * Not the default `CadenceMocks()`: that reads the system clock, and once the
 * repository began filtering meals by date, a test running on any other day saw
 * an empty one. A test whose result depends on when it runs is not a test.
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
            // Reached and target are two runs of one line, as in the prototype:
            // the count in mono and inked, what it is measured against not.
            // Escaped rather than typed: the group separator is U+00A0, and
            // a plain space here fails against a correct implementation
            // while looking identical in the diff. It did, twice, before
            // every expectation in these suites was escaped.
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

            // Everything after the tap is measured from the tap. The modal's
            // own dismissal takes 380 ms, and letting it run first would spend
            // a fifth of the toast's life before the first assertion.
            onNodeWithText("Записать").performClick()
            mainClock.advanceTimeBy(FRAME_MS)
            waitForIdle()
            onNodeWithText("Обед · записано").assertIsDisplayed()

            // The deadline is bracketed with literals, deliberately, rather
            // than measured with CADENCE_CONFIRM_TOAST_MS: an assertion that
            // advances by the same constant it is checking moves with it, and
            // passes against 17 000 ms as happily as against 1 700. A mutation
            // proved that before these two lines replaced the one.
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

            // The unit tests above supply their own numbers, so nothing pinned
            // what the shell actually passes: a target of 0 went unnoticed.
            // 840 is the seeded day's two meals, 1 800 the seeded target — both
            // from the repository now, where the shell used to hold constants.
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

            // The prototype's overlay is `pointerEvents="auto"` — for 1 700 ms
            // it is the topmost hit target and nothing beneath it is live.
            // Without that the screen is merely dimmed: a tap on «+» opens the
            // action sheet, which composes *under* this layer and would be read
            // through a scrim.
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
 * The two sides of the prototype's 1 700 ms, written out rather than derived.
 *
 * With the 100 ms already spent above, the card is asserted alive at 1 600 and
 * gone at 1 800 — a bracket a wrong constant cannot slip through in either
 * direction.
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
    // autoAdvance is off so the toast's own timer can be driven by hand; the
    // clock therefore has to be walked past each transition explicitly.
    mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS.toLong() + FRAME_MS)
    waitForIdle()
}
