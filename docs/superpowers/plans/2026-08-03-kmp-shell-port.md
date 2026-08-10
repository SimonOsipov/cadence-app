# KMP Shell Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the ported screens somewhere to land — the prototype's 18-route screen graph, its bottom action sheet and its confirmation toast — so the app is clickable end to end before a single section has been ported.

**Architecture:** `mobile/src/navigation/` is three files: `AppNavigator.tsx` (the React Navigation native stack), `ActionChooserSheet.tsx` (the sheet the `+` tab opens) and `ConfirmToast.tsx` (the 1,7 s card after a meal is logged). They port to a new `app.cadence.shell` package: a `CadenceRoute` sealed hierarchy, a `CadenceShell` composable holding a `NavHost` plus the two overlays, and disposable placeholder screens keyed by route. Screens never see the `NavHostController` — the shell hands each one plain lambdas, exactly as the prototype hands each `Stack.Screen` its `onBack` / `onOpen…`. Steps 3–9 of the block replace one placeholder at a time; the graph does not move again.

**Tech Stack:** Kotlin Multiplatform 2.4.10, Compose Multiplatform 1.11.1, `org.jetbrains.androidx.navigation:navigation-compose:2.9.2` (stable), `kotlinx-serialization` 1.11.0 for type-safe routes, `kotlin.test` + `compose.ui.test`, ktlint + detekt through `scripts/gate/kmp.sh`.

## Global Constraints

- **The prototype is the specification.** `mobile/src/navigation/{AppNavigator,ActionChooserSheet,ConfirmToast}.tsx` and `mobile/src/components/shared.tsx` are read, never edited. Divergence is a deliberate decision with a comment on the spot and an entry in `docs/prototype-divergences.md` — invariant 1 of the `kmp-app` note.
- **No new colours or fonts.** Everything comes from `CadenceColors` / `CadencePalette` / `CadenceTypography`.
- **No Material.** `BasicText`, `Box`, `Row`, `Column` and the project's own primitives only. No ripple — press feedback is the 0.98 scale in `Modifier.pressable`.
- **RU is the product language.** Every user-visible string is Russian. Code, comments and commit messages are English.
- **Numbers are data, formatting is presentation.** `1 234` and «2 приёма» are assembled by `app.cadence.format`, never baked into a call site.
- **Nothing derived is stored.** The shell holds exactly two pieces of state: is the action sheet open, and what the toast is showing.
- **Compose UI tests run on the iOS simulator target only** — `./gradlew :composeApp:iosSimulatorArm64Test`. `composeApp` has no Android host-test builder by design (`kmp-wiring` spec). Anything provable without a runtime is a plain `kotlin.test`.
- **The gate is `./scripts/gate/kmp.sh`** — ktlint, detekt, `testAndroidHostTest`, `:androidApp:assembleDebug`. Green at the end of every task. The gate does **not** run the Compose UI tests; run them separately, every task.
- **detekt's `MagicNumber` exemption covers `**/app/cadence/design/**` and anything `@Composable`.** `app.cadence.shell` and `app.cadence.format` get neither pass for non-composable code — 1700 and 380 are named constants there.
- **detekt's `LongMethod` default is 60 lines.** An 18-entry `NavHost` will not fit in one function; the graph is split by responsibility, not by adding an exclusion.
- **This is the after-sign-in area only.** Block 7 («Вход пациента», step 1) splits the graph into before/after sign-in. Nothing here is built to be moved by that; `CadenceShell` is the after-sign-in host and stays one.

---

## File Structure

**Created:**

- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceRoute.kt` — the sealed route hierarchy, all 18, plus the `CadenceDestination ↔ CadenceRoute` mapping
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceNavigation.kt` — the four back-stack operations with React Navigation's semantics, and the push/modal transitions
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt` — the host: `NavHost`, the two overlays, and the shell's two pieces of state
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/PlaceholderScreen.kt` — disposable scaffolding, one per unported route
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ActionChooserSheet.kt` — port of `ActionChooserSheet.tsx`
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ConfirmToast.kt` — port of `ConfirmToast.tsx`
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/format/CadenceFormat.kt` — the one formatting module for this surface
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/format/CadenceFormatTest.kt`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceNavigationTest.kt`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/ActionChooserSheetTest.kt`
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/ConfirmToastTest.kt`

**Modified:**

- `kmp/gradle/libs.versions.toml` — `navigation-compose`, `kotlinx-serialization-core`, the serialization plugin
- `kmp/composeApp/build.gradle.kts` — the serialization plugin and the two new dependencies
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceColors.kt` — the toast scrim: the prototype's second alpha on the ink it already carries
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTheme.kt` — `glassSoft` on the palette
- `kmp/composeApp/src/commonMain/kotlin/app/cadence/App.kt` — the showcase becomes the shell
- `kmp/composeApp/src/commonTest/kotlin/app/cadence/AppTest.kt` — rewritten against the shell
- `docs/prototype-divergences.md` — the entries this step makes

**Read, never edited:** everything under `mobile/`.

**Deliberately not built:** a `CadenceNavigator` object handed down to screens. The prototype gives each screen named lambdas (`onBack`, `onOpenTrends`, `onLogDose`) and no navigator, which is why its screens are testable in isolation. Copying that is the port; introducing a navigator would be a redesign with no call site asking for it.

**Deliberately not extracted:** the sheet's «Отмена» pill. `CadenceButtonKind` has no shape matching it (transparent ground, `pal.border` outline, muted label) and `borderRadius: 999` appears 153 times across the prototype in shapes that are mostly *not* this one. One call site is not a pattern — it stays inline in the sheet until a second one proves otherwise (the lesson recorded as `read-the-call-sites-first`).

---

### Task 1: The dependency, the routes, and proof the harness survives it

Retires the one risk that could invalidate every later task: `rememberNavController()` needs a `ViewModelStoreOwner` and a `SavedStateRegistryOwner`, and it is not given that `runComposeUiTest` on `iosSimulatorArm64` provides them. Find out with two routes, before eighteen are written.

**Files:**
- Modify: `kmp/gradle/libs.versions.toml`
- Modify: `kmp/composeApp/build.gradle.kts`
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceRoute.kt`
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceNavigationTest.kt`

**Interfaces:**
- Consumes: `CadenceDestination` and `CadenceTheme` from `app.cadence.design`.
- Produces: `CadenceRoute` (sealed interface, 18 `@Serializable` members); `CadenceDestination.route: CadenceRoute`; `CadenceShell(navController: NavHostController = rememberNavController(), modifier: Modifier = Modifier)`.

- [ ] **Step 1: Add the versions to the catalog**

In `kmp/gradle/libs.versions.toml`, under `[versions]`:

```toml
# Stable, not the 2.10.0-alpha line: the alpha pulls lifecycle 2.11.0-beta01
# and compose 1.10.0 transitively, and nothing here needs what it adds.
navigation-compose = "2.9.2"
kotlinx-serialization = "1.11.0"
```

Under `[libraries]`:

```toml
navigation-compose = { module = "org.jetbrains.androidx.navigation:navigation-compose", version.ref = "navigation-compose" }
kotlinx-serialization-core = { module = "org.jetbrains.kotlinx:kotlinx-serialization-core", version.ref = "kotlinx-serialization" }
```

Under `[plugins]`:

```toml
# Versioned with the compiler, not independently: the serialization plugin is
# a Kotlin compiler plugin and only ever matches its own compiler.
kotlin-serialization = { id = "org.jetbrains.kotlin.plugin.serialization", version.ref = "kotlin" }
```

Also extend the header comment so the next reader knows why a navigation library is here at all when material3 was refused:

```toml
# navigation-compose is the one library taken rather than ported. Unlike the
# Material widgets, what it carries is not a look — it is back-stack ownership,
# saved state across process death and the platform back gesture, and a
# hand-rolled stack would have to reimplement all three before the first screen
# lands. Type-safe routes are why kotlinx-serialization comes with it.
```

- [ ] **Step 2: Wire the dependency**

In `kmp/composeApp/build.gradle.kts`, add to `plugins {}`:

```kotlin
    alias(libs.plugins.kotlin.serialization)
```

and to `commonMain.dependencies`:

```kotlin
            implementation(libs.navigation.compose)
            implementation(libs.kotlinx.serialization.core)
```

- [ ] **Step 3: Write the failing test**

Create `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceNavigationTest.kt`:

```kotlin
package app.cadence.shell

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import kotlin.test.Test
import kotlin.test.assertEquals

/** Entries that are real screens — the graph's own root entry is not one. */
private fun NavHostController.routeDepth(): Int =
    currentBackStack.value.count { it.destination !is NavGraph }

@OptIn(ExperimentalTestApi::class)
class CadenceNavigationTest {
    @Test
    fun theShellStartsOnToday() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            onNodeWithText("Сегодня").assertIsDisplayed()
            assertEquals(1, nav.routeDepth())
        }

    @Test
    fun atapOnADestinationPushesIt() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            onNodeWithText("Тренды").performClick()
            waitForIdle()

            assertEquals(2, nav.routeDepth())
        }
}
```

- [ ] **Step 4: Run it and watch it fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.CadenceNavigationTest"`
Expected: FAIL — `CadenceShell` and `CadenceRoute` are unresolved.

- [ ] **Step 5: Write the route hierarchy**

Create `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceRoute.kt`:

```kotlin
package app.cadence.shell

import app.cadence.design.CadenceDestination
import kotlinx.serialization.Serializable

/**
 * The screen graph, ported one for one from `RootStackParamList` in
 * mobile/src/navigation/AppNavigator.tsx.
 *
 * Names are the prototype's, in English, because a route is not product copy —
 * nothing here is ever shown to a patient. The four routes carrying a
 * parameter carry exactly the parameter the prototype passes, and no more:
 * `TrendDetail` gets a biomarker id, not a biomarker.
 *
 * This is the area *after* sign-in. Block 7 adds the area before it; that
 * boundary lands above this graph rather than inside it.
 */
sealed interface CadenceRoute {
    @Serializable
    data object Today : CadenceRoute

    @Serializable
    data object Trends : CadenceRoute

    @Serializable
    data class TrendDetail(
        val biomarkerId: String,
    ) : CadenceRoute

    @Serializable
    data object Nutrition : CadenceRoute

    @Serializable
    data object Vials : CadenceRoute

    @Serializable
    data object Schedule : CadenceRoute

    @Serializable
    data object Learn : CadenceRoute

    @Serializable
    data class Article(
        val articleId: String,
    ) : CadenceRoute

    @Serializable
    data object Journal : CadenceRoute

    @Serializable
    data object Body : CadenceRoute

    @Serializable
    data object Recipes : CadenceRoute

    @Serializable
    data class RecipeDetail(
        val recipeId: String,
    ) : CadenceRoute

    @Serializable
    data object Profile : CadenceRoute

    @Serializable
    data object ChatList : CadenceRoute

    @Serializable
    data class ChatThread(
        val threadId: String,
    ) : CadenceRoute

    // The four the prototype presents as full-screen modals rather than
    // pushing — `Stack.Group` with `presentation: 'fullScreenModal'`. They
    // slide up, not in from the right, and that is the only thing that makes
    // them a group.
    @Serializable
    data object LogDose : CadenceRoute

    @Serializable
    data object LogMeal : CadenceRoute

    @Serializable
    data object AddVial : CadenceRoute

    @Serializable
    data object RecipeBuilder : CadenceRoute
}

/**
 * Where a bottom-bar destination lands.
 *
 * The prototype writes this as a ternary chain inside `changeTab`; here it is
 * a `when` over a closed enum, so a fifth destination fails to compile instead
 * of quietly falling through to Nutrition — which is what the chain's trailing
 * `: 'Nutrition'` does today.
 */
val CadenceDestination.route: CadenceRoute
    get() =
        when (this) {
            CadenceDestination.TODAY -> CadenceRoute.Today
            CadenceDestination.INVENTORY -> CadenceRoute.Vials
            CadenceDestination.TRENDS -> CadenceRoute.Trends
            CadenceDestination.NUTRITION -> CadenceRoute.Nutrition
        }
```

- [ ] **Step 6: Write the smallest shell that can pass**

Create `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt`. Two routes only — the full graph is Task 2:

```kotlin
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
 * Takes its controller as a parameter so a test can assert on the back stack
 * — «did that tap navigate» is not answerable from what is on screen when two
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
        CadenceTitle(destination.label)
        CadenceTabBar(
            active = destination,
            onSelect = { navController.navigate(it.route) },
            onLog = { },
        )
    }
}
```

- [ ] **Step 7: Run the test**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.CadenceNavigationTest"`
Expected: PASS, 2 tests.

**If `rememberNavController()` throws for a missing `ViewModelStoreOwner` or `SavedStateRegistryOwner`**, that is the risk this task exists to find. Wrap the test body's content in the owners Compose supplies for exactly this — `CompositionLocalProvider(LocalViewModelStoreOwner provides …, LocalSavedStateRegistryOwner provides …)` — put the wrapper in the test file as a `private fun ShellUnderTest()`, and record what was needed in `docs/prototype-divergences.md` so the next reader is not surprised. Do **not** work around it by asserting on text instead of the back stack.

- [ ] **Step 8: Run the gate**

Run: `cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 9: Commit**

```bash
git add kmp/gradle/libs.versions.toml kmp/composeApp/build.gradle.kts \
        kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/
git commit -m "feat(kmp): the screen graph gets a host, and the tests can read its back stack"
```

---

### Task 2: The whole graph, its transitions, and the four stack operations

The prototype's navigator uses exactly six calls: `navigate`, `push`, `goBack`, `replace`, `popToTop`, `canGoBack`. Two of them do not mean in Compose what they mean in React Navigation, and the difference is observable — so they are ported as named operations with their own tests, not spelled inline at each call site.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceNavigation.kt`
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/PlaceholderScreen.kt`
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceNavigationTest.kt`

**Interfaces:**
- Consumes: `CadenceRoute`, `CadenceDestination.route` (Task 1).
- Produces: `NavHostController.openRoute(route)`, `.pushRoute(route)`, `.replaceRoute(route)`, `.popToTop()`, `.selectDestination(destination)`; `PlaceholderScreen(title, modifier, destination, onBack, onSelectTab, onLog, action)`; `CADENCE_PUSH_DURATION_MS`.

- [ ] **Step 1: Write the failing tests**

Replace the body of `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/CadenceNavigationTest.kt` — keep the `routeDepth` helper and the two tests from Task 1, and add:

```kotlin
    @Test
    fun openingARouteAlreadyInTheStackReturnsToItInsteadOfStackingASecond() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            // Today → Nutrition → Recipes → RecipeDetail, then the prototype's
            // «добавить в день» hands back to Nutrition with `navigate`. React
            // Navigation walks back to the existing Nutrition; a plain push
            // would leave two of them and a back button that visits the recipe
            // again.
            runOnUiThread { nav.selectDestination(CadenceDestination.NUTRITION) }
            waitForIdle()
            runOnUiThread { nav.openRoute(CadenceRoute.Recipes) }
            waitForIdle()
            runOnUiThread { nav.openRoute(CadenceRoute.RecipeDetail("r-1")) }
            waitForIdle()
            assertEquals(4, nav.routeDepth())

            runOnUiThread { nav.openRoute(CadenceRoute.Nutrition) }
            waitForIdle()

            assertEquals(2, nav.routeDepth())
            assertTrue(nav.currentBackStackEntry?.destination?.hasRoute<CadenceRoute.Nutrition>() == true)
        }

    @Test
    fun pushingARouteAlreadyInTheStackAddsASecondCopy() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            // An article linking to an article is the one place the prototype
            // asks for `push` by name. Reusing the instance would leave the
            // reader on the article they just left.
            runOnUiThread { nav.openRoute(CadenceRoute.Article("a-1")) }
            waitForIdle()
            runOnUiThread { nav.pushRoute(CadenceRoute.Article("a-2")) }
            waitForIdle()

            assertEquals(3, nav.routeDepth())
        }

    @Test
    fun theTodayTabReturnsToTheRootFromAnyDepth() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            runOnUiThread { nav.openRoute(CadenceRoute.Trends) }
            waitForIdle()
            runOnUiThread { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            waitForIdle()
            assertEquals(3, nav.routeDepth())

            runOnUiThread { nav.selectDestination(CadenceDestination.TODAY) }
            waitForIdle()

            assertEquals(1, nav.routeDepth())
            assertTrue(nav.currentBackStackEntry?.destination?.hasRoute<CadenceRoute.Today>() == true)
        }

    @Test
    fun aTabFromDeepInsideAnotherTabDoesNotStackOnTopOfIt() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            // `changeTab` pops to the root *before* it navigates, so the bar
            // never builds a stack of tabs. Without the pop, four taps on the
            // bar would leave four entries and a back button that walks the
            // user's tab history.
            runOnUiThread { nav.openRoute(CadenceRoute.Trends) }
            waitForIdle()
            runOnUiThread { nav.openRoute(CadenceRoute.TrendDetail("hrv")) }
            waitForIdle()
            runOnUiThread { nav.selectDestination(CadenceDestination.INVENTORY) }
            waitForIdle()

            assertEquals(2, nav.routeDepth())
            assertTrue(nav.currentBackStackEntry?.destination?.hasRoute<CadenceRoute.Vials>() == true)
        }

    @Test
    fun replacingSwapsTheTopEntryRatherThanCoveringIt() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            // The prototype's chat thread replaces itself with the list, and
            // the builder replaces itself with the saved recipe: in both, going
            // back must not return to the screen that just finished.
            runOnUiThread { nav.openRoute(CadenceRoute.ChatThread("ksenia")) }
            waitForIdle()
            runOnUiThread { nav.replaceRoute(CadenceRoute.ChatList) }
            waitForIdle()

            assertEquals(2, nav.routeDepth())
            assertTrue(nav.currentBackStackEntry?.destination?.hasRoute<CadenceRoute.ChatList>() == true)
        }

    @Test
    fun everyRouteInTheGraphRenders() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceShell(navController = nav)
            }

            // A route declared in CadenceRoute but never added to the NavHost
            // throws only when something navigates to it — which, until the
            // screens land, is never. This walks all of them so a missing
            // `composable<…>` fails now rather than in step 9 of the block.
            CadenceRouteSamples.all.forEach { route ->
                runOnUiThread { nav.pushRoute(route) }
                waitForIdle()
                assertTrue(
                    nav.currentBackStackEntry?.destination != null,
                    "no destination registered for $route",
                )
            }
        }
```

Add the imports the new tests need:

```kotlin
import androidx.compose.ui.test.waitForIdle
import androidx.navigation.hasRoute
import app.cadence.design.CadenceDestination
import kotlin.test.assertTrue
```

and, in the same test file, the sample set the walk needs:

```kotlin
/**
 * One instance of every route. Parameterised routes cannot be enumerated from
 * the sealed hierarchy, so they are listed by hand — and the list being wrong
 * is what `everyRouteInTheGraphRenders` is for.
 */
private object CadenceRouteSamples {
    val all: List<CadenceRoute> =
        listOf(
            CadenceRoute.Today,
            CadenceRoute.Trends,
            CadenceRoute.TrendDetail("hrv"),
            CadenceRoute.Nutrition,
            CadenceRoute.Vials,
            CadenceRoute.Schedule,
            CadenceRoute.Learn,
            CadenceRoute.Article("a-1"),
            CadenceRoute.Journal,
            CadenceRoute.Body,
            CadenceRoute.Recipes,
            CadenceRoute.RecipeDetail("r-1"),
            CadenceRoute.Profile,
            CadenceRoute.ChatList,
            CadenceRoute.ChatThread("ksenia"),
            CadenceRoute.LogDose,
            CadenceRoute.LogMeal,
            CadenceRoute.AddVial,
            CadenceRoute.RecipeBuilder,
        )
}
```

- [ ] **Step 2: Run them and watch them fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.CadenceNavigationTest"`
Expected: FAIL — `openRoute`, `pushRoute`, `replaceRoute`, `selectDestination` unresolved.

- [ ] **Step 3: Write the stack operations**

Create `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceNavigation.kt`:

```kotlin
package app.cadence.shell

import androidx.compose.animation.core.tween
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.slideOutVertically
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.compose.animation.AnimatedContentTransitionScope
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import app.cadence.design.CadenceDestination

/** `animationDuration: 380` on the prototype's stack — both push and modal. */
const val CADENCE_PUSH_DURATION_MS: Int = 380

/** How far the outgoing screen trails the incoming one. iOS's own parallax. */
private const val PARALLAX_DIVISOR = 3

/**
 * React Navigation's `navigate`: return to the route if it is already on the
 * stack, otherwise push it.
 *
 * This is not what `NavController.navigate` does, and not what `launchSingleTop`
 * does either — `launchSingleTop` de-duplicates only when the route is already
 * on top. The difference shows the moment a recipe hands back to Nutrition
 * from three screens deep: the faithful behaviour shrinks the stack, a plain
 * navigate grows it, and back then walks a path the user never took.
 */
fun NavHostController.openRoute(route: CadenceRoute) {
    if (currentBackStackEntry?.destination?.hasRoute(route::class) == true) return
    if (!popBackStack(route, inclusive = false)) navigate(route)
}

/** React Navigation's `push`: always a new entry, even for the same route. */
fun NavHostController.pushRoute(route: CadenceRoute) {
    navigate(route)
}

/** React Navigation's `replace`: the finished screen must not be behind you. */
fun NavHostController.replaceRoute(route: CadenceRoute) {
    navigate(route) {
        currentBackStackEntry?.destination?.route?.let { popUpTo(it) { inclusive = true } }
    }
}

/** React Navigation's `popToTop`. */
fun NavHostController.popToTop() {
    popBackStack(CadenceRoute.Today, inclusive = false)
}

/**
 * The bottom bar's `changeTab`, minus the `'log'` branch — logging is not a
 * destination, so it never reaches here (see [CadenceShell]).
 *
 * Pops to the root before navigating, exactly as the prototype does. Tapping
 * a tab is «go there», not «go there on top of everything I was doing».
 */
fun NavHostController.selectDestination(destination: CadenceDestination) {
    popToTop()
    if (destination != CadenceDestination.TODAY) openRoute(destination.route)
}

// The prototype's `animation: 'slide_from_right'` for pushes and
// `'slide_from_bottom'` for the modal group. Compose has no native-stack
// preset, so both are spelled out — the durations and directions are the
// prototype's, the parallax is what a native iOS push does under them.

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.pushEnter(): EnterTransition =
    slideInHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { it }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.pushExit(): ExitTransition =
    slideOutHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { -it / PARALLAX_DIVISOR }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.popEnter(): EnterTransition =
    slideInHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { -it / PARALLAX_DIVISOR }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.popExit(): ExitTransition =
    slideOutHorizontally(tween(CADENCE_PUSH_DURATION_MS)) { it }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.modalEnter(): EnterTransition =
    slideInVertically(tween(CADENCE_PUSH_DURATION_MS)) { it }

internal fun AnimatedContentTransitionScope<NavBackStackEntry>.modalExit(): ExitTransition =
    slideOutVertically(tween(CADENCE_PUSH_DURATION_MS)) { it }
```

Note `hasRoute(route::class)` needs `import androidx.navigation.hasRoute`; add it if the compiler asks.

- [ ] **Step 4: Write the placeholder screen**

Create `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/PlaceholderScreen.kt`:

```kotlin
package app.cadence.shell

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import app.cadence.design.Cadence
import app.cadence.design.CadenceButton
import app.cadence.design.CadenceButtonKind
import app.cadence.design.CadenceDestination
import app.cadence.design.CadenceIconButton
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceMeta
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTabBar
import app.cadence.design.CadenceTitle
import app.cadence.shared.currentPlatform

/**
 * Scaffolding, not a screen. Every route renders one until steps 3–9 of the
 * block replace it with the ported article, and each replacement is one line
 * in [CadenceShell].
 *
 * It shows the platform name because `AppTest` used to prove `:shared` is
 * linked into the UI and not merely into the module graph, and the showcase
 * that carried that proof is gone. **Whoever deletes the last placeholder owes
 * that assertion a new home** — a real screen must not grow a platform label
 * to keep a test green.
 */
@Composable
fun PlaceholderScreen(
    title: String,
    modifier: Modifier = Modifier,
    destination: CadenceDestination? = null,
    onBack: (() -> Unit)? = null,
    onSelectTab: (CadenceDestination) -> Unit = { },
    onLog: () -> Unit = { },
    action: Pair<String, () -> Unit>? = null,
) {
    Column(modifier = modifier.fillMaxSize(), verticalArrangement = Arrangement.SpaceBetween) {
        Column(
            modifier = Modifier.fillMaxWidth().padding(CadenceSpacing.xl),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.md),
        ) {
            if (onBack != null) {
                CadenceIconButton(
                    icon = CadenceIcons.chevronLeft,
                    contentDescription = "Назад",
                    onClick = onBack,
                )
            }
            CadenceTitle(title)
            CadenceMeta("заглушка · ${currentPlatform().name}", color = Cadence.palette.subtle)
            if (action != null) {
                CadenceButton(
                    label = action.first,
                    onClick = action.second,
                    kind = CadenceButtonKind.PRIMARY,
                )
            }
        }

        if (destination != null) {
            // In the prototype the bar lives inside the four screens that have
            // one, not in the navigator — Schedule and Journal have none. The
            // port keeps it there rather than hoisting it into the shell,
            // because hoisting it would put a bar on the screens without one.
            CadenceTabBar(active = destination, onSelect = onSelectTab, onLog = onLog)
        }
    }
}
```

- [ ] **Step 5: Write the full graph**

Replace `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt` with the whole graph. The four `NavGraphBuilder` extensions exist because detekt caps a function at 60 lines and because the prototype's own grouping is the same one:

```kotlin
package app.cadence.shell

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import app.cadence.design.CadenceDestination

/**
 * The after-sign-in host: the screen graph and the overlays above it.
 *
 * Screens get named lambdas and never the controller, which is how the
 * prototype wires its `Stack.Screen` children and what keeps a screen
 * testable without a navigator. `onOpenActions` is the one callback that is
 * not navigation: the `+` in the bar opens a sheet, and a sheet is not a
 * place.
 *
 * Takes its controller as a parameter so a test can assert on the back stack —
 * «did that tap navigate» is not answerable from what is on screen when two
 * routes render the same word.
 */
@Composable
fun CadenceShell(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
    onOpenActions: () -> Unit = { },
    onMealLogged: (String, Int) -> Unit = { _, _ -> },
) {
    NavHost(
        navController = navController,
        startDestination = CadenceRoute.Today,
        modifier = modifier.fillMaxSize(),
        enterTransition = { pushEnter() },
        exitTransition = { pushExit() },
        popEnterTransition = { popEnter() },
        popExitTransition = { popExit() },
    ) {
        tabRoutes(navController, onOpenActions)
        pushedRoutes(navController)
        modalRoutes(navController, onMealLogged)
    }
}

/** The four the bottom bar reaches. Only these carry the bar. */
private fun NavGraphBuilder.tabRoutes(
    nav: NavHostController,
    onOpenActions: () -> Unit,
) {
    CadenceDestination.entries.forEach { destination ->
        val body: @Composable () -> Unit = {
            PlaceholderScreen(
                title = destination.label,
                destination = destination,
                onBack = if (destination == CadenceDestination.TODAY) null else nav::popBackStack,
                onSelectTab = nav::selectDestination,
                onLog = onOpenActions,
            )
        }
        when (destination) {
            CadenceDestination.TODAY -> composable<CadenceRoute.Today> { body() }
            CadenceDestination.INVENTORY -> composable<CadenceRoute.Vials> { body() }
            CadenceDestination.TRENDS -> composable<CadenceRoute.Trends> { body() }
            CadenceDestination.NUTRITION -> composable<CadenceRoute.Nutrition> { body() }
        }
    }
}

/** Everything reached by a push: slides in from the right, backed out of. */
private fun NavGraphBuilder.pushedRoutes(nav: NavHostController) {
    composable<CadenceRoute.TrendDetail> { entry ->
        val route = entry.toRoute<CadenceRoute.TrendDetail>()
        PlaceholderScreen("Биомаркер · ${route.biomarkerId}", onBack = nav::popBackStack)
    }
    composable<CadenceRoute.Schedule> { PlaceholderScreen("Расписание", onBack = nav::popBackStack) }
    composable<CadenceRoute.Learn> { PlaceholderScreen("Обучение", onBack = nav::popBackStack) }
    composable<CadenceRoute.Article> { entry ->
        val route = entry.toRoute<CadenceRoute.Article>()
        PlaceholderScreen("Статья · ${route.articleId}", onBack = nav::popBackStack)
    }
    composable<CadenceRoute.Journal> { PlaceholderScreen("Дневник", onBack = nav::popBackStack) }
    composable<CadenceRoute.Body> { PlaceholderScreen("Тело", onBack = nav::popBackStack) }
    composable<CadenceRoute.Recipes> { PlaceholderScreen("Рецепты", onBack = nav::popBackStack) }
    composable<CadenceRoute.RecipeDetail> { entry ->
        val route = entry.toRoute<CadenceRoute.RecipeDetail>()
        PlaceholderScreen("Рецепт · ${route.recipeId}", onBack = nav::popBackStack)
    }
    composable<CadenceRoute.Profile> { PlaceholderScreen("Профиль", onBack = nav::popBackStack) }
    composable<CadenceRoute.ChatList> { PlaceholderScreen("Чаты", onBack = nav::popBackStack) }
    composable<CadenceRoute.ChatThread> { entry ->
        val route = entry.toRoute<CadenceRoute.ChatThread>()
        PlaceholderScreen("Переписка · ${route.threadId}", onBack = nav::popBackStack)
    }
}

/**
 * The prototype's `Stack.Group` with `presentation: 'fullScreenModal'`: these
 * slide up rather than in, and every one of them ends by dismissing itself.
 */
private fun NavGraphBuilder.modalRoutes(
    nav: NavHostController,
    onMealLogged: (String, Int) -> Unit,
) {
    composable<CadenceRoute.LogDose>(
        enterTransition = { modalEnter() },
        popExitTransition = { modalExit() },
    ) {
        PlaceholderScreen(
            title = "Записать дозу",
            onBack = nav::popBackStack,
            action = "Записать" to { nav.popBackStack(); Unit },
        )
    }
    composable<CadenceRoute.LogMeal>(
        enterTransition = { modalEnter() },
        popExitTransition = { modalExit() },
    ) {
        PlaceholderScreen(
            title = "Записать приём пищи",
            onBack = nav::popBackStack,
            // The prototype's LogMeal hands the meal to the app, which raises
            // the toast and dismisses. The placeholder hands over a fixed one
            // so the toast is reachable before the wizard is ported.
            action = "Записать" to { onMealLogged(PLACEHOLDER_MEAL_NAME, PLACEHOLDER_MEAL_KCAL); nav.popBackStack(); Unit },
        )
    }
    composable<CadenceRoute.AddVial>(
        enterTransition = { modalEnter() },
        popExitTransition = { modalExit() },
    ) {
        PlaceholderScreen("Добавить флакон", onBack = nav::popBackStack)
    }
    composable<CadenceRoute.RecipeBuilder>(
        enterTransition = { modalEnter() },
        popExitTransition = { modalExit() },
    ) {
        PlaceholderScreen("Новый рецепт", onBack = nav::popBackStack)
    }
}

/** Stand-ins until the meal wizard lands in step 8 of the block. */
private const val PLACEHOLDER_MEAL_NAME = "Обед"
private const val PLACEHOLDER_MEAL_KCAL = 520
```

- [ ] **Step 6: Run the navigation tests**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.CadenceNavigationTest"`
Expected: PASS, 8 tests.

- [ ] **Step 7: Mutate the stack operations and confirm the tests catch it**

Do all four, one at a time, reverting each before the next:

1. In `openRoute`, delete the `popBackStack` line so it always navigates → `openingARouteAlreadyInTheStackReturnsToItInsteadOfStackingASecond` must fail.
2. In `selectDestination`, delete the `popToTop()` call → `aTabFromDeepInsideAnotherTabDoesNotStackOnTopOfIt` must fail.
3. In `pushRoute`, call `openRoute` instead of `navigate` → `pushingARouteAlreadyInTheStackAddsASecondCopy` must fail.
4. In `replaceRoute`, drop the `popUpTo` block → `replacingSwapsTheTopEntryRatherThanCoveringIt` must fail.

Any mutation that leaves the suite green means the test asserts nothing; fix the test before moving on. Record the result in the commit message.

- [ ] **Step 8: Run the gate**

Run: `cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 9: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/
git commit -m "feat(kmp): eighteen routes, and the two stack operations Compose spells differently"
```

---

### Task 3: The formatters the sheet needs

The action sheet's second line is «2 приёма сегодня · 1 240 ккал». Both halves are formatting decisions with rules, both are pure functions, and both belong in one module rather than at the call site — the project rule, and the only way step 4's copy has a test that does not need a screen.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/format/CadenceFormat.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/format/CadenceFormatTest.kt`

**Interfaces:**
- Produces: `formatInteger(value: Int): String`, `pluralMeals(count: Int): String`.

- [ ] **Step 1: Write the failing test**

Create `kmp/composeApp/src/commonTest/kotlin/app/cadence/format/CadenceFormatTest.kt`:

```kotlin
package app.cadence.format

import kotlin.test.Test
import kotlin.test.assertEquals

class CadenceFormatTest {
    @Test
    fun integersGroupInThreesWithARussianNonBreakingSpace() {
        // `toLocaleString('ru-RU')` in the prototype. The separator is U+00A0,
        // not a plain space: a kcal count must not wrap between its thousands
        // and its hundreds.
        assertEquals("0", formatInteger(0))
        assertEquals("999", formatInteger(999))
        assertEquals("1 000", formatInteger(1000))
        assertEquals("1 240", formatInteger(1240))
        assertEquals("12 345", formatInteger(12345))
        assertEquals("1 234 567", formatInteger(1234567))
    }

    @Test
    fun negativeIntegersKeepTheirSignOutsideTheGrouping() {
        // No call site passes one today; the guard exists because the obvious
        // implementation groups the minus sign into the first triple and
        // renders «-1 234» as «-1 234» only by luck of digit count.
        assertEquals("-1 234", formatInteger(-1234))
        assertEquals("-999", formatInteger(-999))
    }

    @Test
    fun mealsTakeTheRussianPluralAndNotTheProtypesApproximationOfIt() {
        assertEquals("приём", pluralMeals(1))
        assertEquals("приёма", pluralMeals(2))
        assertEquals("приёма", pluralMeals(4))
        assertEquals("приёмов", pluralMeals(5))
        assertEquals("приёмов", pluralMeals(11))
        assertEquals("приёмов", pluralMeals(14))
        // Where the prototype's `count < 5 ? 'приёма' : 'приёмов'` is wrong.
        assertEquals("приём", pluralMeals(21))
        assertEquals("приёма", pluralMeals(22))
        assertEquals("приёмов", pluralMeals(25))
        assertEquals("приёмов", pluralMeals(111))
        assertEquals("приём", pluralMeals(101))
        // Never rendered — the sheet takes its zero-state branch — but a
        // plural function that is wrong at zero is wrong.
        assertEquals("приёмов", pluralMeals(0))
    }
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.format.CadenceFormatTest"`
Expected: FAIL — unresolved references.

- [ ] **Step 3: Write the module**

Create `kmp/composeApp/src/commonMain/kotlin/app/cadence/format/CadenceFormat.kt`:

```kotlin
package app.cadence.format

/**
 * The one place this surface turns numbers into Russian text.
 *
 * Numbers are data and formatting is presentation — the project rule — so
 * nothing upstream of here ever holds «1 240» as a string, and nothing
 * downstream builds one of its own.
 *
 * Hand-rolled rather than taken from a locale API: `kotlinx-datetime` has no
 * number formatting, Kotlin/Native has no ICU, and the two rules the product
 * needs fit in thirty lines that a test can pin exactly.
 */

/** Russian groups thousands with U+00A0, so the number never wraps mid-value. */
private const val GROUP_SEPARATOR = ' '
private const val GROUP_SIZE = 3

/** `toLocaleString('ru-RU')` for a whole number: «1240» → «1 240». */
fun formatInteger(value: Int): String {
    val negative = value < 0
    // Not -value: Int.MIN_VALUE has no positive counterpart, and the obvious
    // negation returns it unchanged and silently formats a negative number as
    // a positive one.
    val digits = value.toLong().let { if (negative) -it else it }.toString()

    val grouped =
        buildString {
            digits.forEachIndexed { index, digit ->
                if (index > 0 && (digits.length - index) % GROUP_SIZE == 0) append(GROUP_SEPARATOR)
                append(digit)
            }
        }

    return if (negative) "-$grouped" else grouped
}

private const val TEEN_FLOOR = 11
private const val TEEN_CEILING = 14
private const val FEW_CEILING = 4

/**
 * «приём» / «приёма» / «приёмов».
 *
 * The prototype writes `count === 1 ? 'приём' : count < 5 ? 'приёма' :
 * 'приёмов'`, which is right for 1–20 and wrong from 21 up. Every count it can
 * actually reach is inside the range where the two agree, so this is a
 * divergence with no visible effect today — and a correct rule costs six lines
 * where carrying the approximation forward would cost an explanation on every
 * later screen that counts something.
 */
fun pluralMeals(count: Int): String {
    val n = if (count < 0) -count else count
    val lastTwo = n % 100
    val last = n % 10

    return when {
        lastTwo in TEEN_FLOOR..TEEN_CEILING -> "приёмов"
        last == 1 -> "приём"
        last in 2..FEW_CEILING -> "приёма"
        else -> "приёмов"
    }
}
```

- [ ] **Step 4: Run the test**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.format.CadenceFormatTest"`
Expected: PASS, 3 tests.

- [ ] **Step 5: Mutate and confirm**

One at a time, reverting each:

1. Change `GROUP_SEPARATOR` to a plain `' '` → the grouping test must fail (it asserts the code point, not «looks spaced»).
2. Delete the `lastTwo in TEEN_FLOOR..TEEN_CEILING` branch → `pluralMeals(11)` must fail.
3. Replace the whole plural body with the prototype's ternary → the 21/22 assertions must fail.

- [ ] **Step 6: Record the divergence**

Append to `docs/prototype-divergences.md`:

```markdown
## Russian plurals: the correct rule, not the prototype's approximation

**What the prototype does:** `ActionChooserSheet.tsx` picks the meal noun with
`mealCount === 1 ? 'приём' : mealCount < 5 ? 'приёма' : 'приёмов'`.

**What we do:** `pluralMeals` in `app.cadence.format` applies the actual rule —
11–14 take «приёмов», then the last digit decides.

**Why:** the ternary is right for 1–20 and wrong from 21 up («21 приёмов»). No
count the sheet can reach today leaves that range, so nothing on screen
changes; what changes is that the next screen counting something copies a rule
that holds instead of one that happens to. Pinned by
`CadenceFormatTest.mealsTakeTheRussianPluralAndNotTheProtypesApproximationOfIt`,
which asserts exactly the counts where the two disagree.
```

- [ ] **Step 7: Run the gate**

Run: `cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 8: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/format/ \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/format/ \
        docs/prototype-divergences.md
git commit -m "feat(kmp): one module turns numbers into Russian, and it counts properly"
```

---

### Task 4: The action chooser sheet

`ActionChooserSheet.tsx`, 129 lines. It reads four values off the app state; the app state arrives with the next subtask, so it takes them as parameters and the shell passes today's zero-state.

**Files:**
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ActionChooserSheet.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/ActionChooserSheetTest.kt`

**Interfaces:**
- Consumes: `CadenceSheet`, `cadenceEmphasisedTitle`, `CadenceIcons`, `Modifier.pressable` from `app.cadence.design`; `formatInteger`, `pluralMeals` from `app.cadence.format`.
- Produces: `ActionChooserSheet(open, doseLogged, mealCount, mealKcal, onDismiss, onPickDose, onPickMeal)`.

- [ ] **Step 1: Write the failing test**

Create `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/ActionChooserSheetTest.kt`:

```kotlin
package app.cadence.shell

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class ActionChooserSheetTest {
    @Test
    fun theSheetOffersBothWaysToRecordAndAWayOut() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    ActionChooserSheet(
                        open = true,
                        doseLogged = false,
                        mealCount = 0,
                        mealKcal = 0,
                        onDismiss = { },
                        onPickDose = { },
                        onPickMeal = { },
                    )
                }
            }

            onNodeWithText("Что записываем?".uppercase()).assertIsDisplayed()
            onNodeWithText("Выберите ритм.").assertIsDisplayed()
            onNodeWithText("Записать дозу").assertIsDisplayed()
            onNodeWithText("Записать приём пищи").assertIsDisplayed()
            onNodeWithText("Отмена").assertIsDisplayed()
        }

    @Test
    fun eachRowReportsWhichOneWasTapped() =
        runComposeUiTest {
            var picked = ""
            setContent {
                CadenceTheme {
                    ActionChooserSheet(
                        open = true,
                        doseLogged = false,
                        mealCount = 0,
                        mealKcal = 0,
                        onDismiss = { picked = "dismiss" },
                        onPickDose = { picked = "dose" },
                        onPickMeal = { picked = "meal" },
                    )
                }
            }

            // The two rows sit one above the other and carry the same shape;
            // a port that wires both to the same lambda looks identical.
            onNodeWithText("Записать приём пищи").performClick()
            assertEquals("meal", picked)

            onNodeWithText("Записать дозу").performClick()
            assertEquals("dose", picked)

            onNodeWithText("Отмена").performClick()
            assertEquals("dismiss", picked)
        }

    @Test
    fun theSubtitlesSayWhatTheDayLooksLike() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    ActionChooserSheet(
                        open = true,
                        doseLogged = true,
                        mealCount = 2,
                        mealKcal = 1240,
                        onDismiss = { },
                        onPickDose = { },
                        onPickMeal = { },
                    )
                }
            }

            onNodeWithText("Уже записано сегодня · открыть или поправить").assertIsDisplayed()
            onNodeWithText("2 приёма сегодня · 1 240 ккал").assertIsDisplayed()
        }

    @Test
    fun aClosedSheetComposesNothing() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    ActionChooserSheet(
                        open = false,
                        doseLogged = false,
                        mealCount = 0,
                        mealKcal = 0,
                        onDismiss = { },
                        onPickDose = { },
                        onPickMeal = { },
                    )
                }
            }

            assertTrue(onAllNodesWithText("Записать дозу").fetchSemanticsNodes().isEmpty())
        }
}
```

Add `import androidx.compose.ui.test.onAllNodesWithText`.

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.ActionChooserSheetTest"`
Expected: FAIL — `ActionChooserSheet` unresolved.

- [ ] **Step 3: Write the sheet**

Create `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ActionChooserSheet.kt`:

```kotlin
package app.cadence.shell

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceEyebrow
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSheet
import app.cadence.design.CadenceSpacing
import app.cadence.design.CadenceTitle
import app.cadence.design.cadenceEmphasisedTitle
import app.cadence.design.pressable
import app.cadence.format.formatInteger
import app.cadence.format.pluralMeals

/**
 * The sheet the `+` in the bottom bar opens, ported from
 * mobile/src/navigation/ActionChooserSheet.tsx.
 *
 * The prototype reads `doseLogged`, `meals` and `mealTotals` straight off the
 * app state. Here they are parameters: the repositories arrive with the next
 * subtask, and a component that pulls its own data cannot be told to render
 * yesterday. The shell passes what it knows.
 */
@Composable
fun ActionChooserSheet(
    open: Boolean,
    doseLogged: Boolean,
    mealCount: Int,
    mealKcal: Int,
    onDismiss: () -> Unit,
    onPickDose: () -> Unit,
    onPickMeal: () -> Unit,
) {
    CadenceSheet(open = open, onDismiss = onDismiss) {
        Column(Modifier.padding(bottom = 18.dp)) {
            CadenceEyebrow("Что записываем?", Modifier.padding(bottom = CadenceSpacing.xs))
            CadenceTitle(
                cadenceEmphasisedTitle(prefix = "Выберите ", emphasis = "ритм", suffix = "."),
                size = 28.sp,
            )
        }

        Column(
            modifier = Modifier.fillMaxWidth().padding(bottom = 14.dp),
            verticalArrangement = Arrangement.spacedBy(CadenceSpacing.sm + 2.dp),
        ) {
            ActionOption(
                iconBackground = CadenceColors.forest700,
                iconTint = CadenceColors.cream,
                icon = CadenceIcons.beaker,
                title = "Записать дозу",
                subtitle =
                    if (doseLogged) {
                        "Уже записано сегодня · открыть или поправить"
                    } else {
                        // Fixed copy in the prototype too — the real protocol
                        // arrives with the repositories.
                        "Семаглутид · 0,25 мг ждёт"
                    },
                onClick = onPickDose,
            )
            ActionOption(
                iconBackground = CadenceColors.sand500,
                iconTint = CadenceColors.ink900,
                icon = CadenceIcons.cake,
                title = "Записать приём пищи",
                subtitle =
                    if (mealCount == 0) {
                        "Пока ничего сегодня · начнём ритм"
                    } else {
                        "$mealCount ${pluralMeals(mealCount)} сегодня · ${formatInteger(mealKcal)} ккал"
                    },
                onClick = onPickMeal,
            )
        }

        CancelPill(onDismiss)
    }
}

private val OPTION_RADIUS = 18.dp
private val OPTION_ICON_BOX = 52.dp
private val OPTION_ICON_RADIUS = 14.dp

@Composable
private fun ActionOption(
    iconBackground: Color,
    iconTint: Color,
    icon: List<String>,
    title: String,
    subtitle: String,
    onClick: () -> Unit,
) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(OPTION_RADIUS)

    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .pressable(onClick, interactionSource)
                .background(palette.paper, shape)
                .border(1.dp, palette.hairline, shape)
                .padding(14.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier =
                Modifier
                    .size(OPTION_ICON_BOX)
                    .background(iconBackground, RoundedCornerShape(OPTION_ICON_RADIUS)),
            contentAlignment = Alignment.Center,
        ) {
            CadenceIcon(paths = icon, size = 24.dp, tint = iconTint)
        }

        Column(Modifier.weight(1f)) {
            CadenceTitle(title, size = 22.sp, maxLines = 1)
            BasicText(
                text = subtitle,
                modifier = Modifier.padding(top = 3.dp),
                style = Cadence.typography.meta.copy(color = palette.muted, fontSize = 12.sp),
            )
        }

        CadenceIcon(paths = CadenceIcons.chevronRight, size = 18.dp, tint = palette.subtle)
    }
}

/**
 * Kept local rather than added to [app.cadence.design.CadenceButton].
 *
 * No `CadenceButtonKind` matches it — transparent ground, `border` outline,
 * muted label — and `borderRadius: 999` appears 153 times across the prototype
 * in shapes that are mostly not this one. One call site is not a pattern; the
 * second one moves it into the design system.
 */
@Composable
private fun CancelPill(onDismiss: () -> Unit) {
    val palette = Cadence.palette
    val interactionSource = remember { MutableInteractionSource() }
    val shape = RoundedCornerShape(CadenceRadius.pill)

    Box(
        modifier =
            Modifier
                .fillMaxWidth()
                .pressable(onDismiss, interactionSource)
                .border(1.dp, palette.border, shape)
                .padding(vertical = 13.dp),
        contentAlignment = Alignment.Center,
    ) {
        BasicText(
            text = "Отмена",
            style = Cadence.typography.label.copy(color = palette.muted, fontSize = 13.sp),
        )
    }
}
```

`pressable` is `internal` to the `design` package today; if it is not visible from `app.cadence.shell`, widen it to `internal` at module scope (it already is) — `internal` in Kotlin is module-wide, so no change should be needed. If the compiler disagrees, the fix is to make it `public` with a KDoc saying why, not to duplicate the scale logic.

- [ ] **Step 4: Run the test**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.ActionChooserSheetTest"`
Expected: PASS, 4 tests.

- [ ] **Step 5: Mutate and confirm**

1. Wire `onPickMeal` to `onPickDose` → `eachRowReportsWhichOneWasTapped` must fail.
2. Replace `formatInteger(mealKcal)` with `mealKcal.toString()` → `theSubtitlesSayWhatTheDayLooksLike` must fail.
3. Invert the `doseLogged` branch → the same test must fail.

- [ ] **Step 6: Run the gate**

Run: `cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 7: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ActionChooserSheet.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/ActionChooserSheetTest.kt
git commit -m "feat(kmp): the plus opens a sheet that says what today already holds"
```

---

### Task 5: The confirmation toast, and the shell state that raises it

`ConfirmToast.tsx`, 80 lines, plus the 1 700 ms timer that lives in `AppState.showConfirm`. The toast is presentational; the timer belongs to the shell, because the shell is what owns «what is on screen right now».

**Files:**
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceColors.kt`
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/design/CadenceTheme.kt`
- Create: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ConfirmToast.kt`
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt`
- Test: `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/ConfirmToastTest.kt`

**Interfaces:**
- Consumes: `formatInteger` (Task 3); `CadenceShell`'s `onMealLogged` hook (Task 2).
- Produces: `ConfirmToastState(mealName: String, kcal: Int)`; `ConfirmToast(state: ConfirmToastState?, targetKcal: Int)`; `CADENCE_CONFIRM_TOAST_MS`; `CadencePalette.glassSoft`.

- [ ] **Step 1: Give the palette the toast's scrim**

In `CadenceColors.kt`, replace the single `glassDark` line with the ink named once and both alphas the prototype uses:

```kotlin
    // The prototype names only `glassDark: 'rgba(20,44,31,.35)'` in its theme
    // and writes the toast's `rgba(20,44,31,.25)` inline. Same ink, two
    // weights — named here so neither is a literal at a call site.
    private val forestScrim = Color(0xFF142C1F)
    val glassDark = forestScrim.copy(alpha = 0.35f)
    val glassSoft = forestScrim.copy(alpha = 0.25f)
```

In `CadenceTheme.kt`, add `val glassSoft: Color,` to `CadencePalette` beside `glassDark`, and `glassSoft = CadenceColors.glassSoft,` to `CadenceLightPalette`.

- [ ] **Step 2: Write the failing test**

Create `kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/ConfirmToastTest.kt`:

```kotlin
package app.cadence.shell

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import app.cadence.design.CadenceTheme
import kotlin.test.Test
import kotlin.test.assertTrue

@OptIn(ExperimentalTestApi::class)
class ConfirmToastTest {
    @Test
    fun theToastNamesTheMealAndBothSidesOfTheTarget() =
        runComposeUiTest {
            setContent {
                CadenceTheme {
                    ConfirmToast(
                        state = ConfirmToastState(mealName = "Обед", kcal = 1240),
                        targetKcal = 2100,
                    )
                }
            }

            onNodeWithText("Обед · записано").assertIsDisplayed()
            onNodeWithText("1 240").assertIsDisplayed()
            onNodeWithText(" / 2 100 ккал сегодня").assertIsDisplayed()
        }

    @Test
    fun noStateMeansNoToast() =
        runComposeUiTest {
            setContent { CadenceTheme { ConfirmToast(state = null, targetKcal = 2100) } }

            assertTrue(onAllNodesWithText("записано", substring = true).fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun loggingAMealRaisesTheToastAndItLeavesOnItsOwn() =
        runComposeUiTest {
            mainClock.autoAdvance = false
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav) }
            }

            runOnUiThread { nav.openRoute(CadenceRoute.LogMeal) }
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS.toLong() * 2)

            onNodeWithText("Записать").performClick()
            mainClock.advanceTimeBy(CADENCE_PUSH_DURATION_MS.toLong() * 2)
            onNodeWithText("Обед · записано").assertIsDisplayed()

            // 1700 ms, and then it is gone without anyone dismissing it. A
            // toast that stays is a modal nobody asked for.
            mainClock.advanceTimeBy(CADENCE_CONFIRM_TOAST_MS + 100L)
            assertTrue(onAllNodesWithText("Обед · записано").fetchSemanticsNodes().isEmpty())
        }
}
```

- [ ] **Step 3: Run it and watch it fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.ConfirmToastTest"`
Expected: FAIL — `ConfirmToast`, `ConfirmToastState`, `CadenceApp` unresolved.

- [ ] **Step 4: Write the toast**

Create `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/ConfirmToast.kt`:

```kotlin
package app.cadence.shell

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import app.cadence.design.Cadence
import app.cadence.design.CadenceColors
import app.cadence.design.CadenceElevation
import app.cadence.design.CadenceIcon
import app.cadence.design.CadenceIcons
import app.cadence.design.CadenceRadius
import app.cadence.design.CadenceSpacing
import app.cadence.format.formatInteger

/** How long the card stays up — `setTimeout(…, 1700)` in the prototype. */
const val CADENCE_CONFIRM_TOAST_MS: Long = 1700

/** What a logged meal has to say for itself. Nothing derived is kept here. */
@Immutable
data class ConfirmToastState(
    val mealName: String,
    val kcal: Int,
)

private val TOAST_RADIUS = 22.dp
private val TOAST_TICK_BOX = 44.dp

/**
 * The card that confirms a logged meal, ported from
 * mobile/src/navigation/ConfirmToast.tsx.
 *
 * Presentational and stateless: it does not know it disappears. The shell owns
 * the timer, because how long something stays on screen is a property of the
 * screen, not of the card.
 */
@Composable
fun ConfirmToast(
    state: ConfirmToastState?,
    targetKcal: Int,
    modifier: Modifier = Modifier,
) {
    if (state == null) return
    val palette = Cadence.palette
    val shape = RoundedCornerShape(TOAST_RADIUS)

    Box(
        modifier =
            modifier
                .fillMaxSize()
                .background(palette.glassSoft)
                .windowInsetsPadding(WindowInsets.navigationBars)
                .padding(horizontal = CadenceSpacing.lg, vertical = CadenceSpacing.xxl),
        contentAlignment = Alignment.BottomCenter,
    ) {
        Row(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .shadow(CadenceElevation.lg, shape, clip = false)
                    .background(palette.bg, shape)
                    .border(1.dp, palette.hairline, shape)
                    .padding(18.dp),
            horizontalArrangement = Arrangement.spacedBy(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                modifier =
                    Modifier
                        .size(TOAST_TICK_BOX)
                        .background(CadenceColors.forest700, RoundedCornerShape(CadenceRadius.pill)),
                contentAlignment = Alignment.Center,
            ) {
                CadenceIcon(
                    paths = CadenceIcons.check,
                    size = 20.dp,
                    tint = CadenceColors.cream,
                )
            }

            Column(Modifier.weight(1f)) {
                BasicText(
                    text = "${state.mealName} · записано",
                    style = Cadence.typography.title.copy(color = palette.ink, fontSize = 18.sp),
                    maxLines = 1,
                )
                // Two runs in one line, as in the prototype: the count reached
                // is mono and inked, the target it is measured against is not.
                BasicText(
                    text =
                        buildAnnotatedString {
                            withStyle(
                                Cadence.typography.number
                                    .toSpanStyle()
                                    .copy(color = palette.ink2, fontSize = 12.sp),
                            ) { append(formatInteger(state.kcal)) }
                            withStyle(
                                Cadence.typography.meta
                                    .toSpanStyle()
                                    .copy(color = palette.subtle, fontSize = 12.sp),
                            ) { append(" / ${formatInteger(targetKcal)} ккал сегодня") }
                        },
                    modifier = Modifier.padding(top = 2.dp),
                    style = Cadence.typography.meta.copy(color = palette.muted, fontSize = 12.sp),
                )
            }
        }
    }
}
```

- [ ] **Step 5: Wire the shell**

Add to `kmp/composeApp/src/commonMain/kotlin/app/cadence/shell/CadenceShell.kt` — a `CadenceApp` that owns the shell's two pieces of state and stacks the overlays over the graph:

```kotlin
/** The day's kcal target — `MEAL_TARGETS.kcal` in the prototype's meal data. */
private const val PLACEHOLDER_KCAL_TARGET = 2100

/**
 * The whole after-sign-in surface: the graph, the sheet the `+` opens, and the
 * card a logged meal raises.
 *
 * Two pieces of state, both about what is on screen right now and neither
 * derived from anything: is the sheet open, and what the toast is showing.
 * Both are the prototype's — `actionSheetOpen` in the navigator and
 * `confirmSheet` in the app state.
 */
@Composable
fun CadenceApp(
    navController: NavHostController = rememberNavController(),
    modifier: Modifier = Modifier,
) {
    var actionsOpen by remember { mutableStateOf(false) }
    var toast by remember { mutableStateOf<ConfirmToastState?>(null) }

    LaunchedEffect(toast) {
        if (toast != null) {
            delay(CADENCE_CONFIRM_TOAST_MS)
            toast = null
        }
    }

    Box(modifier.fillMaxSize()) {
        CadenceShell(
            navController = navController,
            onOpenActions = { actionsOpen = true },
            onMealLogged = { name, kcal -> toast = ConfirmToastState(name, kcal) },
        )

        ActionChooserSheet(
            open = actionsOpen,
            // The zero-state, until the repositories land with the next
            // subtask. Wire these three to them and nothing here changes.
            doseLogged = false,
            mealCount = 0,
            mealKcal = 0,
            onDismiss = { actionsOpen = false },
            onPickDose = {
                actionsOpen = false
                navController.openRoute(CadenceRoute.LogDose)
            },
            onPickMeal = {
                actionsOpen = false
                navController.openRoute(CadenceRoute.LogMeal)
            },
        )

        ConfirmToast(state = toast, targetKcal = PLACEHOLDER_KCAL_TARGET)
    }
}
```

with the imports `androidx.compose.foundation.layout.Box`, `androidx.compose.runtime.LaunchedEffect`, `getValue`, `mutableStateOf`, `remember`, `setValue`, `kotlinx.coroutines.delay`.

**If `kotlinx.coroutines.delay` does not resolve**, compose-runtime is exposing coroutines as `implementation` rather than `api`. Add to the catalog `kotlinx-coroutines-core = { module = "org.jetbrains.kotlinx:kotlinx-coroutines-core", version.ref = "kotlinx-coroutines" }` with the newest stable version, and depend on it from `commonMain`. Do not replace the `delay` with a hand-rolled clock.

- [ ] **Step 6: Run the tests**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.shell.*"`
Expected: PASS — the navigation, sheet and toast suites together.

- [ ] **Step 7: Mutate and confirm**

1. Delete the `delay`/`toast = null` body of the `LaunchedEffect` → `loggingAMealRaisesTheToastAndItLeavesOnItsOwn` must fail on the disappearance, not on the appearance.
2. Change `CADENCE_CONFIRM_TOAST_MS` to 17000 → the same test must fail.
3. Drop the `if (state == null) return` guard and render an empty card → `noStateMeansNoToast` must fail.

- [ ] **Step 8: Run the gate**

Run: `cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 9: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/ \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/shell/
git commit -m "feat(kmp): a logged meal says so for 1700ms and then gets out of the way"
```

---

### Task 6: The showcase gives way to the app

`App()` is still the design-system showcase. Replacing it is named in `mobile-sign-in` step 1 and in the subtask, so it is expected — but the showcase carries four assertions, and dropping one that nothing else makes would be a silent loss of coverage.

**Files:**
- Modify: `kmp/composeApp/src/commonMain/kotlin/app/cadence/App.kt`
- Modify: `kmp/composeApp/src/commonTest/kotlin/app/cadence/AppTest.kt`
- Modify: `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt` (only if the audit finds a gap)
- Modify: `docs/prototype-divergences.md`

- [ ] **Step 1: Audit what the showcase was the only proof of**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test` and note the full test list. Then, for each assertion in `AppTest.theShowcaseRendersEveryComponentItIsSupposedTo` — the eyebrow, the emphasised title, `CadenceNumber`'s value and unit, the pill, the chip, the button, the tab bar, and `SHOWCASE_SPARK_TAG` — find the test elsewhere that would fail if that component stopped rendering:

```bash
cd /Users/dmitriiporollo/programming/projects/cadence-app/kmp/composeApp/src/commonTest
grep -rn "CadenceEyebrow\|cadenceEmphasisedTitle\|CadenceNumber\|CadencePill\|CadenceChip\|CadenceButton\|CadenceSpark" .
```

Write the mapping down — component → the test that now covers it. Any component with no cover gets a test added to `DesignSystemTest.kt` in Step 2. Do not delete the showcase before this list is complete.

- [ ] **Step 2: Close any gap the audit found**

For each uncovered component, add to `kmp/composeApp/src/commonTest/kotlin/app/cadence/design/DesignSystemTest.kt` a test in the file's existing style that renders it inside `CadenceTheme` and asserts its text or tag is displayed. If the audit found no gap, say so in the commit message and skip this step.

- [ ] **Step 3: Rewrite the test**

Replace `kmp/composeApp/src/commonTest/kotlin/app/cadence/AppTest.kt`:

```kotlin
package app.cadence

import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.currentPlatform
import kotlin.test.Test

@OptIn(ExperimentalTestApi::class)
class AppTest {
    @Test
    fun theAppOpensOnToday() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithText("Сегодня").assertIsDisplayed()
            onNodeWithText("Сегодня").assertIsSelected()
        }

    @Test
    fun theAppShowsThePlatformItRunsOn() =
        runComposeUiTest {
            setContent { App() }

            // Proves :shared is linked into the UI and not merely into the
            // module graph. It rides on the placeholder's footer today; see the
            // note on PlaceholderScreen about where it goes when the last
            // placeholder is deleted.
            onNodeWithText("заглушка · ${currentPlatform().name}").assertIsDisplayed()
        }

    @Test
    fun thePlusOpensTheSheetRatherThanChangingDestination() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithContentDescription("Записать").performClick()

            onNodeWithText("Записать дозу").assertIsDisplayed()
            // And the bar did not treat it as a place.
            onNodeWithText("Сегодня").assertIsSelected()
        }

    @Test
    fun theSheetSendsTheUserIntoTheDoseWizard() =
        runComposeUiTest {
            setContent { App() }

            onNodeWithContentDescription("Записать").performClick()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            // The wizard's own screen, not the sheet row that opened it: the
            // row's title and the modal's title are the same words, so the
            // back affordance is what tells them apart.
            onNodeWithContentDescription("Назад").assertIsDisplayed()
        }
}
```

Add `import androidx.compose.ui.test.waitForIdle`.

- [ ] **Step 4: Run it and watch it fail**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test --tests "app.cadence.AppTest"`
Expected: FAIL — `App()` still renders the showcase.

- [ ] **Step 5: Replace App.kt**

```kotlin
package app.cadence

import androidx.compose.runtime.Composable
import app.cadence.design.CadenceTheme
import app.cadence.shell.CadenceApp

/**
 * App is the single Compose entry point both platforms render.
 *
 * It was a design-system showcase until the shell landed; the components it
 * displayed are covered by their own tests, and the app itself is now the
 * place a regression shows. The 24 screens arrive one at a time behind
 * [app.cadence.shell.CadenceShell]'s placeholders.
 */
@Composable
fun App() {
    CadenceTheme {
        CadenceApp()
    }
}
```

Delete `SHOWCASE_SPARK_TAG` — nothing references it once the showcase is gone. Confirm with `grep -rn "SHOWCASE_SPARK_TAG" kmp/`.

- [ ] **Step 6: Run the whole suite**

Run: `cd kmp && ./gradlew :composeApp:iosSimulatorArm64Test`
Expected: PASS, no failures. Compare the count against Step 1's list — a drop that is not accounted for by the four showcase tests being replaced is a lost assertion.

- [ ] **Step 7: Record the shell's divergences**

Append to `docs/prototype-divergences.md` the entries this step actually made. At minimum:

```markdown
## Navigation: one library taken rather than ported

**What the prototype does:** `@react-navigation/native-stack`, with
`animation: 'slide_from_right'`, `animationDuration: 380`, and a
`fullScreenModal` group for the four logging flows.

**What we do:** `org.jetbrains.androidx.navigation:navigation-compose` 2.9.2
with the same durations and directions spelled out as Compose transitions.

**Why:** unlike the Material widgets this project refused, what a navigation
library carries is not a look — it is back-stack ownership, saved state across
process death, and the platform back gesture. A hand-rolled stack would have to
reimplement all three before the first screen lands. `slide_from_right` has no
Compose preset, so the push is written out: full-width slide in, the outgoing
screen trailing at a third of the width, which is what a native iOS push does.

## `navigate` is not `navigate`

**What the prototype does:** React Navigation's `navigate` returns to an
existing instance of a route if the stack already holds one, and its `push`
always adds a new entry. `AppNavigator.tsx` uses both, deliberately — `push`
only for article-to-article.

**What we do:** `NavHostController.openRoute` reproduces the first,
`pushRoute` the second. Compose's own `navigate` always pushes, and
`launchSingleTop` de-duplicates only against the top of the stack, so neither
is the prototype's behaviour.

**Why it is visible:** «добавить в день» on a recipe hands back to Nutrition
from three screens deep. Faithful, the stack shrinks to two; with a plain
navigate it grows to five and back walks a path the user never took. Pinned by
`CadenceNavigationTest.openingARouteAlreadyInTheStackReturnsToItInsteadOfStackingASecond`.
```

Add an entry for anything Task 1 Step 7's fallback required, if it required any.

- [ ] **Step 8: Run the gate**

Run: `cd /Users/dmitriiporollo/programming/projects/cadence-app && ./scripts/gate/kmp.sh`
Expected: `kmp gate: green`.

- [ ] **Step 9: Commit**

```bash
git add kmp/composeApp/src/commonMain/kotlin/app/cadence/App.kt \
        kmp/composeApp/src/commonTest/kotlin/app/cadence/ \
        docs/prototype-divergences.md
git commit -m "feat(kmp): the showcase gives way to the app it was standing in for"
```

---

## Self-Review

**Spec coverage.** The subtask names three prototype files and one boundary
condition. `AppNavigator.tsx` → Tasks 1, 2 and 5 (its `actionSheetOpen` state).
`ActionChooserSheet.tsx` → Task 4, with its number formatting split into Task 3
because the project keeps formatters in one module. `ConfirmToast.tsx` → Task 5,
with `AppState.showConfirm`'s 1 700 ms timer, which lives in the prototype's
state file rather than in the toast. The «only the area after sign-in» boundary
is a Global Constraint and is honoured by `CadenceShell` having no notion of a
session at all. The parent task's «моки за интерфейсами» acceptance criterion is
the *next* subtask, which is why the sheet takes its four values as parameters
rather than reading them.

**Known gaps, deliberately.** The prototype's screens each render their own tab
bar; the placeholders do the same, so no bar is hoisted into the shell that
would then need removing from Schedule and Journal. Nothing here proves the
transitions look right — a 380 ms slide is not assertable from a semantics tree;
that is step 11's side-by-side run, and it is what `[!deviation]` exists for if
it disagrees.

**Type consistency.** `openRoute` / `pushRoute` / `replaceRoute` / `popToTop` /
`selectDestination` are named identically in Tasks 2, 4, 5 and 6.
`ConfirmToastState(mealName, kcal)` is constructed in Task 5's shell exactly as
declared in Task 5's toast. `CadenceShell`'s `onMealLogged: (String, Int) -> Unit`
in Task 2 matches the `{ name, kcal -> }` lambda in Task 5. `PlaceholderScreen`'s
`action: Pair<String, () -> Unit>?` is used with the same arity in both modal
routes. `formatInteger` and `pluralMeals` are declared in Task 3 and called
under those names in Tasks 4 and 5.
