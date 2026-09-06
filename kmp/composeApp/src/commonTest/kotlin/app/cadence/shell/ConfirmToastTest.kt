package app.cadence.shell

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceTheme
import app.cadence.screens.nutrition.LOG_MEAL_CHAT_FIELD_TAG
import app.cadence.screens.nutrition.LOG_MEAL_SAVE_TAG
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
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = seededDay()) }
            }

            val raisedAt = logAMeal(nav)
            onNodeWithText("Обед · записано").assertIsDisplayed()

            // Absolute deadlines measured from the raise, so the walk's own virtual time
            // cannot widen the bracket. Literals deliberately, not CADENCE_CONFIRM_TOAST_MS:
            // an assertion that advances by the constant it checks passes at 17000ms as
            // happily as at 1700 — a mutation proved that before these lines replaced one.
            mainClock.autoAdvance = false
            mainClock.advanceTimeBy(raisedAt + JUST_UNDER_THE_DEADLINE_MS - mainClock.currentTime)
            onNodeWithText("Обед · записано").assertIsDisplayed()

            mainClock.advanceTimeBy(PAST_THE_DEADLINE_MS)
            assertTrue(onAllNodesWithText("Обед · записано").fetchSemanticsNodes().isEmpty())
        }

    /**
     * The step's own named test: the toast names the day **including** the meal it confirms.
     * The seeded day is 840 kcal and the canned «Обед» parse is 625, so the correct answer is
     * 1465 and the pre-write snapshot — what `summary` still holds when the toast is raised,
     * since `LaunchedEffect(reloads)` has not re-read yet — is 840. Two numbers a mutation
     * cannot confuse.
     */
    @Test
    fun theToastNamesTheDayTotalIncludingTheMealJustLogged() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = seededDay()) }
            }

            logAMeal(nav)

            // The target half also pins what the shell passes, not what a unit test supplies —
            // a target of 0 went unnoticed until this assertion named 1800.
            onNodeWithText("1\u00A0465 / 1\u00A0800 ккал сегодня").assertIsDisplayed()
        }

    @Test
    fun theToastSwallowsEveryTouchWhileItIsUp() =
        runComposeUiTest {
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

/**
 * Written out rather than derived from `CADENCE_CONFIRM_TOAST_MS`: an assertion that
 * advances by the constant it checks passes at 17000ms as happily as at 1700, and a
 * mutation proved exactly that. Both are absolute offsets from the raise — alive at 1600,
 * gone at 1800, a 200ms window neither a longer nor a shorter constant fits through.
 */
private const val JUST_UNDER_THE_DEADLINE_MS = 1600L
private const val PAST_THE_DEADLINE_MS = 200L

/**
 * Opens the meal wizard, parses «курица с рисом» through the shell's own parser and saves —
 * the whole path a patient walks, since step-13 replaced the placeholder that used to log a
 * constant. The canned «Обед» parse is 625 kcal (240+145+60+90+90).
 */
@OptIn(ExperimentalTestApi::class)
private fun androidx.compose.ui.test.ComposeUiTest.logAMeal(nav: NavHostController): Long {
    // Driven with the clock free: both the parse and the write are `launch`ed coroutines,
    // and neither is dispatched while `autoAdvance` is off. The toast's own timer survives
    // this — a pending `delay` is not what «idle» waits for — so a test that needs to
    // measure the toast's life turns `autoAdvance` off *after* calling this.
    runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
    waitForIdle()
    onNodeWithTag(LOG_MEAL_CHAT_FIELD_TAG, useUnmergedTree = true).performTextReplacement("курица с рисом")
    waitForIdle()
    onNodeWithText("Разобрать →").performClick()
    waitForIdle()
    onNodeWithTag(LOG_MEAL_SAVE_TAG).performClick()
    val clickedAt = mainClock.currentTime
    waitForIdle()
    // Returned, not assumed: the walk above spends virtual time of its own, and a bracket
    // measured from zero would pin the 1700ms constant only to «somewhere between one and
    // two seconds». The toast is raised no earlier than this — the write is launched by the
    // click and the timer starts with it — so the far edge below carries a few frames of
    // slack and the near edge none.
    return clickedAt
}
