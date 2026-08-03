package app.cadence.shell

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceTabBar
import app.cadence.design.CadenceTitle

/**
 * The after-sign-in host: the screen graph plus the overlays that sit above
 * every screen in it.
 *
 * Takes its controller as a parameter so a test can assert on the back stack —
 * «did that tap navigate» is not answerable from what is on screen when two
 * routes render the same word.
 */
@Composable
fun CadenceShell(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
) {
    NavHost(
        navController = navController,
        startDestination = CadenceRoute.Today,
        modifier = modifier.fillMaxSize(),
    ) {
        composable<CadenceRoute.Today> {
            TabScaffold(CadenceDestination.TODAY, navController)
        }
        composable<CadenceRoute.Trends> {
            TabScaffold(CadenceDestination.TRENDS, navController)
        }
    }
}

@Composable
private fun TabScaffold(
    destination: CadenceDestination,
    navController: NavHostController,
) {
    Column(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.SpaceBetween,
    ) {
        // «Экран «Сегодня»», not «Сегодня»: the bare label is also the tab's
        // own text, and two nodes reading the same word make every assertion
        // about «which screen am I on» ambiguous.
        CadenceTitle("Экран «${destination.label}»")
        CadenceTabBar(
            active = destination,
            onSelect = { navController.navigate(it.route) },
            onLog = { },
        )
    }
}
