package app.cadence.shell

import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceTheme
import kotlin.test.Test
import kotlin.test.assertEquals

/** Entries that are real screens — the graph's own root entry is not one. */
private fun NavHostController.routeDepth(): Int = currentBackStack.value.count { it.destination !is NavGraph }

/**
 * The shell under test, with its controller handed back.
 *
 * The controller is the point: «did that tap navigate» is not answerable from
 * what is on screen when two routes render the same word.
 */
@OptIn(ExperimentalTestApi::class)
private fun ComposeUiTest.startShell(): NavHostController {
    lateinit var nav: NavHostController
    setContent {
        nav = rememberNavController()
        CadenceTheme { CadenceShell(navController = nav) }
    }
    return nav
}

@OptIn(ExperimentalTestApi::class)
class CadenceNavigationTest {
    @Test
    fun theShellStartsOnToday() =
        runComposeUiTest {
            val nav = startShell()

            onNodeWithText("Экран «Сегодня»").assertIsDisplayed()
            assertEquals(1, nav.routeDepth())
        }

    @Test
    fun aTapOnADestinationPushesIt() =
        runComposeUiTest {
            val nav = startShell()

            onNodeWithText("Тренды").performClick()
            waitForIdle()

            assertEquals(2, nav.routeDepth())
        }
}
