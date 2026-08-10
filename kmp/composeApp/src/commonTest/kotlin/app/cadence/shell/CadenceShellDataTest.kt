package app.cadence.shell

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.v2.runComposeUiTest
import androidx.navigation.NavGraph
import androidx.navigation.NavHostController
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import app.cadence.design.CadenceTheme
import app.cadence.screens.inventory.ADD_VIAL_SAVE_TAG
import app.cadence.screens.inventory.addVialFieldTag
import app.cadence.screens.trends.CADENCE_TRENDS_HERO_TAG
import app.cadence.screens.trends.CADENCE_TREND_DETAIL_STATS_TAG
import app.cadence.screens.trends.cadenceTrendCardTag
import app.cadence.screens.trends.cadenceTrendWindowTag
import app.cadence.shared.domain.FixedCadenceClock
import app.cadence.shared.domain.Metric
import app.cadence.shared.domain.ProtocolCadence
import app.cadence.shared.domain.ProtocolItemId
import app.cadence.shared.domain.ProtocolItemKind
import app.cadence.shared.domain.ProtocolRow
import app.cadence.shared.domain.TrendWindow
import app.cadence.shared.mock.CadenceMocks
import app.cadence.shared.mock.MockSeed
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.LocalTime
import kotlinx.datetime.TimeZone
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

private val ZONE = TimeZone.of("Europe/Moscow")

/** The app on a day the test chose, reading a repository the test wound. */
@OptIn(ExperimentalTestApi::class)
private fun mocks(iso: String = "2026-05-31T09:00:00Z") = CadenceMocks(FixedCadenceClock.at(iso), ZONE)

/** A slot time for a hand-built row; nothing reads the hour. */
private val MORNING = LocalTime.parse("08:00")

@OptIn(ExperimentalTestApi::class)
class CadenceShellDataTest {
    /** A row with the cadence and slot count a case needs, and nothing else real. */
    private fun row(
        cadence: ProtocolCadence,
        times: Int,
    ) = ProtocolRow(
        itemId = ProtocolItemId("item"),
        kind = ProtocolItemKind.SUPPLEMENT,
        compound = MockSeed.glycine,
        dose = null,
        times = List(times) { MORNING },
        cadence = cadence,
        todayStatus = null,
        loggable = true,
    )

    /** The zone «Предложено: X — следующее в ротации» names, on the site step. */
    private fun ComposeUiTest.suggestedZone(): String =
        onNodeWithText("Предложено:", substring = true)
            .fetchSemanticsNode()
            .config
            .getOrNull(SemanticsProperties.Text)
            ?.firstOrNull()
            ?.text
            .orEmpty()
            .substringAfter("Предложено: ")
            .substringBefore(" —")

    @Test
    fun theSheetShowsTheDayTheRepositoryReportsAndNotAConstant() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            // Two seeded meals of 320 and 520 kcal. Both numbers come from the
            // repository and are formatted by app.cadence.format on the way to
            // the screen — «840» is not a string anything upstream holds.
            onNodeWithText("2 приёма сегодня · 840 ккал").assertIsDisplayed()
            onNodeWithText("Семаглутид · 0,25 мг ждёт").assertIsDisplayed()
        }

    @Test
    fun theAppOpensOnTheRealTodayScreen() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            // Not «Экран «Сегодня»» any more: the placeholder is gone and the
            // ported screen is what the tab bar lands on.
            onNodeWithText("Воскресенье, день · 4-я неделя").assertIsDisplayed()
            assertTrue(onAllNodesWithText("Экран «Сегодня»").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theScheduleIsReachedFromTodayAndAgreesWithIt() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Весь график").performScrollTo().performClick()
            waitForIdle()

            // The same week, from the same generator, on the other screen.
            onNodeWithText("неделя 4 из 12").assertIsDisplayed()
            assertTrue(onAllNodesWithText("Экран «Расписание»").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun theWizardIsOfferedTheProtocolsLoggableItemsAndNothingElse() =
        runTest {
            // The mapping on its own, because the seed has no loggable item
            // that is not an injection — so through the screen a wizard that
            // stamped INJECTION on everything is invisible.
            val options = mocks().today.today().doseOptions()

            assertEquals(
                listOf("Семаглутид", "BPC-157"),
                options.map { it.nameRu },
                "the wizard offers something the protocol does not mark loggable",
            )
            // No kind assertion here: both seeded loggable items are
            // injections, so one would guard nothing. `DoseWizardTest
            // .anItemThatIsNotAnInjectionNeedsNoZoneToGetPast` covers the
            // non-injection path where it is observable.
            assertEquals(listOf(true, true), options.map { it.dueToday })
            assertEquals("п/к · еженедельно", options.first().modeRu)
            assertEquals("п/к · 2× в день", options.last().modeRu)
        }

    @Test
    fun onlyTheItemsDueTodayCarryTheBadge() =
        runTest {
            // A Tuesday: the daily injection is due, the weekly one is not.
            // Every earlier assertion ran on the seeded Sunday, where both are
            // — so a `dueToday` hardcoded to true was invisible, and a patient
            // on a Tuesday would be told their Sunday injection is due now.
            val options = mocks("2026-06-02T09:00:00Z").today.today().doseOptions()

            assertEquals(listOf("Семаглутид", "BPC-157"), options.map { it.nameRu })
            assertEquals(listOf(false, true), options.map { it.dueToday })
        }

    @Test
    fun theModeNamesEveryCadenceTheProtocolCanHave() =
        runTest {
            // Two of the three branches are unreachable from the seed: its only
            // daily item has two times, and nothing is N-per-week at all.
            assertEquals("внутрь · ежедневно", modeRu(row(ProtocolCadence.DAILY, times = 1)))
            assertEquals("внутрь · 3× в день", modeRu(row(ProtocolCadence.DAILY, times = 3)))
            assertEquals("внутрь · 2× в неделю", modeRu(row(ProtocolCadence.N_PER_WEEK, times = 2)))
        }

    @Test
    fun theWizardOpensOnAZoneTheRotationHasNotJustUsed() =
        runComposeUiTest {
            // The suggestion has to *reach* the site step. Read from the screen
            // rather than computed, because computing it here would only be the
            // production rule asked twice — and injecting into the zone the
            // caption names is the one case where the answer must move.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            val suggested = suggestedZone()

            // The suggestion may be on the back of the body — with a real
            // history the rotation has moved off the front.
            if (onAllNodesWithContentDescription(suggested).fetchSemanticsNodes().isEmpty()) {
                onNodeWithText("Сзади").performClick()
                waitForIdle()
            }
            onNodeWithContentDescription(suggested).performScrollTo().performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }

            assertNotEquals(suggested, suggestedZone(), "the wizard suggests the zone just injected")
        }

    @Test
    fun theCabinetTabDrawsTheVialsRatherThanAPlaceholder() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Аптечка").performClick()
            waitForIdle()

            onNodeWithText("Ваша аптечка").assertExists()
            onNodeWithText("4 флакона в холодильнике").assertExists()
        }

    // The three below walk the bar *on the ported screens*, which is the axis
    // `CadenceNavigationTest.onlyTheFourBarDestinationsCarryTheBar` never
    // measured: it starts a shell with no data, so all four tabs are
    // placeholders and `PlaceholderScreen` draws a bar of its own. With live
    // data the cabinet and trends replace the placeholder — and shipped
    // without one, which on iOS (no system back) is a tab a patient cannot
    // leave. The mutation each must fail against is named on the test.

    @Test
    fun theCabinetTabKeepsTheBarThatReachesTheOtherTabs() =
        runComposeUiTest {
            // Mutation: drop `CadenceTabBar` from `VialsScreen`.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Аптечка").performClick()
            waitForIdle()
            // The live cabinet, not the placeholder that carries a bar anyway.
            onNodeWithText("Ваша аптечка").assertExists()

            onNodeWithContentDescription("Тренды").performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_TRENDS_HERO_TAG).assertExists()
        }

    @Test
    fun theTrendsTabKeepsTheBarThatReachesTheOtherTabs() =
        runComposeUiTest {
            // Mutation: drop `CadenceTabBar` from `TrendsScreen`.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Тренды").performClick()
            waitForIdle()
            // The live trends screen, not the placeholder.
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG).assertExists()

            onNodeWithContentDescription("Сегодня").performClick()
            waitForIdle()

            onNodeWithText("Воскресенье, день · 4-я неделя").assertExists()
        }

    @Test
    fun theBarOnAPortedScreenMarksTheTabThatScreenIs() =
        runComposeUiTest {
            // Mutation: pass a constant — `active = CadenceDestination.TODAY` —
            // to either screen's bar. Reachability alone would not notice.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Аптечка").performClick()
            waitForIdle()
            onNodeWithContentDescription("Аптечка").assertIsSelected()
            onNodeWithContentDescription("Сегодня").assertIsNotSelected()

            onNodeWithContentDescription("Тренды").performClick()
            waitForIdle()
            onNodeWithContentDescription("Тренды").assertIsSelected()
            onNodeWithContentDescription("Аптечка").assertIsNotSelected()
        }

    @Test
    fun theWizardOpensOnTheVialTheSheetWasAbout() =
        runComposeUiTest {
            // `VialDetailSheet` has always reported which vial its «Записать
            // дозу» belongs to; the shell dropped the id — `onLogDose = {
            // onLogDose() }` — so the wizard opened with an empty draft and the
            // picker defaulted to the *fullest* open vial of the compound. A
            // patient who opened B-2510 and stepped through without noticing
            // the picker wrote the dose against B-2601, leaving one count a
            // dose short and the other a dose long with nothing to reconcile
            // them against.
            //
            // B-2510 deliberately: it is the emptier of the two open BPC vials,
            // so it is the one the default would *not* pick. Opening the
            // fullest would pass either way.
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            // Driven by the route rather than by walking the cabinet: the route
            // is what carries the id, and what the vial sheet hands it is a
            // one-line wiring the sheet's own suite already covers.
            runOnIdle { nav.navigate(CadenceRoute.LogDose("vial-bpc-2")) }
            waitForIdle()

            onNodeWithText("BPC-157").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            // B-2510 is `vial-bpc-2`, the *emptier* of the two open BPC vials —
            // so it is the one the picker's «fullest open vial» default would
            // not choose. Opening on the fullest would pass either way.
            onNodeWithText("B-2510", substring = true).performScrollTo().assertIsSelected()
            onNodeWithText("B-2601", substring = true).performScrollTo().assertIsNotSelected()
        }

    @Test
    fun theVialSheetHandsItsOwnIdToTheWizardRoute() =
        runComposeUiTest {
            // The other half of the seam. `theWizardOpensOnTheVialTheSheetWasAbout`
            // drives the route directly, so it cannot see the shell dropping the
            // id on the way *to* the route — which is precisely what it did:
            // `onLogDose = { onLogDose() }`, the argument discarded.
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Аптечка").performClick()
            waitForIdle()
            onNodeWithText("B-2510", substring = true).performScrollTo().performClick()
            waitForIdle()
            assertEquals(
                1,
                onAllNodesWithText("Записать дозу").fetchSemanticsNodes().size,
                "the sheet has to be open, and be the only thing offering this",
            )
            onNodeWithText("Записать дозу").performScrollTo().performClick()
            waitForIdle()

            // The route's own argument, read off the entry rather than inferred
            // from the screen: «which vial» is not drawn anywhere on step 1.
            val entry = nav.currentBackStackEntry
            assertEquals(
                "LogDose",
                entry
                    ?.destination
                    ?.route
                    ?.substringBefore('/')
                    ?.substringBefore('?')
                    ?.substringAfterLast('.'),
            )
            assertEquals("vial-bpc-2", entry?.toRoute<CadenceRoute.LogDose>()?.vialId)
        }

    @Test
    fun theActionSheetNamesTheDoseThatIsActuallyDue() =
        runComposeUiTest {
            // Week 5, where the course titrates to 0,5 мг. This row carried
            // «Семаглутид · 0,25 мг ждёт» as a literal for three blocks after
            // the repositories landed, with the live summary already in the
            // caller's scope — and every test that touched it ran on the seeded
            // Sunday of week 4, the one week where the literal and the truth
            // agree. A patient past the first band was told the wrong dose on
            // the sheet they open to record one.
            setContent { CadenceTheme { CadenceApp(mocks = mocks("2026-06-07T09:00:00Z")) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Семаглутид · 0,5 мг ждёт").assertIsDisplayed()
            assertEquals(
                0,
                onAllNodesWithText("Семаглутид · 0,25 мг ждёт").fetchSemanticsNodes().size,
                "the literal this row used to carry",
            )
        }

    @Test
    fun addingAVialFromTheCabinetPutsItInTheCabinet() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Аптечка").performClick()
            waitForIdle()
            onNodeWithContentDescription("Добавить флакон").performClick()
            waitForIdle()

            onNodeWithText("Семаглутид").performScrollTo().performClick()
            waitForIdle()
            onNodeWithTag(addVialFieldTag("total"), useUnmergedTree = true).performTextReplacement("20")
            onNodeWithTag(addVialFieldTag("expires"), useUnmergedTree = true).performTextReplacement("2027-01-31")
            waitForIdle()
            onNodeWithTag(ADD_VIAL_SAVE_TAG, useUnmergedTree = true).performClick()
            waitForIdle()

            // Back in the cabinet, and the count moved — the write went through
            // the repository and the next read reflected it.
            onNodeWithText("5 флаконов в холодильнике").assertExists()
        }

    @Test
    fun theWizardIsOfferedTheCabinetsOwnOpenVials() =
        runComposeUiTest {
            // Through the shell, not against a hand-built option list: the
            // wiring from the cabinet to the picker is what six mutants lived
            // in, and a wizard test that builds its own vials cannot see it.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()
            onNodeWithText("BPC-157").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            // The two open BPC vials, fullest first, and no sealed spare.
            onNodeWithText("B-2601").performScrollTo().assertExists()
            onNodeWithText("B-2510").performScrollTo().assertExists()
            assertEquals(0, onAllNodesWithText("B-2610").fetchSemanticsNodes().size, "sealed stock is offered")
            // And no semaglutide vial: the picker is for the chosen compound.
            assertEquals(
                0,
                onAllNodesWithText("A-2261").fetchSemanticsNodes().size,
                "another compound's vial is offered",
            )

            // Fullest first: B-2601 has four doses left, B-2510 has two. Read
            // off the rows' own lots, because each row merges its lot and its
            // count into one node and the lot is what it leads with.
            val order =
                onAllNodesWithText("доз", substring = true)
                    .fetchSemanticsNodes()
                    .mapNotNull {
                        it.config
                            .getOrNull(SemanticsProperties.Text)
                            ?.firstOrNull()
                            ?.text
                    }
            assertEquals(listOf("B-2601", "B-2510"), order)
        }

    @Test
    fun aSingleOpenVialIsNotAChoiceAndIsNotDrawn() =
        runComposeUiTest {
            // Semaglutide has one. A picker with one option is not a picker,
            // and the write makes the same choice anyway.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            assertEquals(0, onAllNodesWithText("Из вашей аптечки", ignoreCase = true).fetchSemanticsNodes().size)
        }

    @Test
    fun theHeroOpensTheDoseWizard() =
        runComposeUiTest {
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Записать →").performClick()
            waitForIdle()

            // The wizard itself now, not the placeholder that stood in for it.
            onNodeWithText("Шаг 1 · Препарат", ignoreCase = true).assertIsDisplayed()
            onNodeWithText("Шаг 1 из 5").assertIsDisplayed()
        }

    @Test
    fun theWizardOffersTheProtocolsOwnItemsWithTodaysDose() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()

            // The seeded protocol's two injections, and the daily one is not
            // due «сегодня» in the badge sense on this Sunday — the weekly one
            // is. A wizard built from a literal list would name whatever the
            // prototype's `COMPOUNDS` held.
            onNodeWithText("Семаглутид").assertIsDisplayed()
            onNodeWithText("BPC-157").assertIsDisplayed()
            onNodeWithText("0,25 мг", substring = true).assertIsDisplayed()
        }

    @Test
    fun theWizardOpensOnTheDoseTheDayIsWaitingFor() =
        runComposeUiTest {
            // Mutation: build the draft without an item — `DoseDraft(vialId =
            // openedVial)`, which is what shipped. Step 1 says «Сегодняшняя
            // доза уже выбрана» unconditionally, so a wizard that preselects
            // nothing leaves the patient reading that sentence above two empty
            // radios and a «Дальше» that does nothing, with no reason given.
            //
            // Asserted through step 2's subtitle rather than a checked radio:
            // the radio is a picture of the draft, the subtitle is built from
            // the option the draft actually holds.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()

            // Deliberately nothing tapped on step 1.
            onNodeWithText("Дальше").performClick()
            waitForIdle()

            onNodeWithText("По умолчанию для Семаглутид", substring = true).assertIsDisplayed()
        }

    @Test
    fun onADayTheHeroHasNoDoseTheWizardStillPreselectsNothing() =
        runComposeUiTest {
            // Not the behaviour anyone wants — it is the behaviour there is,
            // pinned so the gap is measured rather than assumed away.
            //
            // `TodaySummary.nextDose` is not «the dose that is due»: the mock
            // builds it as `todays.firstOrNull { it.itemId == semaItemId }`
            // (CadenceMocks.kt:133), so it is the weekly injection's occurrence
            // and nothing else. On this Tuesday BPC-157 is due twice and
            // semaglutide is not due at all, so there is no occurrence to open
            // on — and step 1 goes on claiming the choice was made.
            //
            // Deciding what should be preselected on such a day is a product
            // rule this port does not have; it is recorded in
            // docs/prototype-divergences.md instead of invented here.
            setContent { CadenceTheme { CadenceApp(mocks = mocks("2026-06-02T09:00:00Z")) } }

            // Through the sheet, not the hero: the hero card is about the
            // weekly injection and draws no «Записать →» on a day it is not
            // due, which is the same fact this test turns on.
            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            onNodeWithText("Дальше").performClick()
            waitForIdle()

            assertTrue(
                onAllNodesWithText("Шаг 2", substring = true, ignoreCase = true)
                    .fetchSemanticsNodes()
                    .isEmpty(),
                "the wizard advanced, so something was preselected after all — " +
                    "update this test and the divergence note together",
            )
        }

    @Test
    fun finishingTheWizardLogsTheDoseAndReturnsToToday() =
        runComposeUiTest {
            // Task 6's whole assertion: the hero opens the wizard, the wizard
            // writes through the repository, and Today reads the write back.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Записать →").performClick()
            waitForIdle()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithContentDescription("Правое плечо").performScrollTo().performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Дальше").performClick()
            waitForIdle()
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            // Both places the day reads as logged. The strip's word is
            // lowercase and the hero's is not, and `onAllNodesWithText` is
            // case-sensitive — one query looked like it covered both and
            // matched only the hero.
            assertTrue(
                onAllNodesWithText("Записано", substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the hero does not say the dose was recorded",
            )
            assertTrue(
                onAllNodesWithText("записано", substring = true).fetchSemanticsNodes().isNotEmpty(),
                "the protocol strip does not say the dose was recorded",
            )
            assertTrue(
                onAllNodesWithText("Записать →").fetchSemanticsNodes().isEmpty(),
                "the hero still offers a dose it already has",
            )
            // The vial's decrement is asserted through the repository, in
            // `MockRepositoryTest.aLoggedDoseComesOutOfTheVialItWasDrawnFrom`.
            // Asserting the rendered count here would pin a copy string that
            // says nothing more about the wiring.
        }

    @Test
    fun anotherDayGivesTheSheetAnotherAnswer() =
        runComposeUiTest {
            // Week 5 of the seeded protocol, and a day with no meals on it.
            // Without this the shell could hold the seeded day's numbers as
            // literals and every assertion above would still pass — a mutation
            // doing exactly that survived until this test was added.
            setContent { CadenceTheme { CadenceApp(mocks = mocks("2026-06-07T09:00:00Z")) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Пока ничего сегодня · начнём ритм").assertIsDisplayed()
        }

    @Test
    fun loggingADoseChangesWhatTheSheetSaysNextTime() =
        runComposeUiTest {
            // The acceptance criterion of the whole block, on screen: a write
            // goes through the repository and the next read reflects it, with
            // ActionChooserSheet unchanged — it already took its four values as
            // parameters, which is the point.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()
            onNodeWithText("Записать дозу").performClick()
            waitForIdle()

            onNodeWithText("Шаг 1 · Препарат", ignoreCase = true).assertIsDisplayed()
            onNodeWithText("Семаглутид").performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            onNodeWithContentDescription("Правое плечо").performScrollTo().performClick()
            waitForIdle()
            repeat(2) {
                onNodeWithText("Дальше").performClick()
                waitForIdle()
            }
            onNodeWithText("Сохранить дозу").performClick()
            waitForIdle()

            onNodeWithContentDescription("Записать").performClick()
            waitForIdle()

            onNodeWithText("Уже записано сегодня · открыть или поправить").assertIsDisplayed()
            assertTrue(
                onAllNodesWithText("Семаглутид · 0,25 мг ждёт").fetchSemanticsNodes().isEmpty(),
                "the sheet is still reading its old answer",
            )
        }

    @Test
    fun theTrendsTabDrawsTheListItReadRatherThanAPlaceholder() =
        runComposeUiTest {
            // The step's headline acceptance, and the only place it is
            // measured against the *shipped* wiring: `CadenceNavigationTest`
            // hands `CadenceShell` an overview of its own, so it proves the
            // test's replica rather than `CadenceApp`'s read.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Тренды").performClick()
            waitForIdle()

            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertExists()
            assertTrue(onAllNodesWithText("Экран «Тренды»").fetchSemanticsNodes().isEmpty())
        }

    @Test
    fun aChipOnTheTrendsTabChangesWhatTheListReads() =
        runComposeUiTest {
            // The window is shell state and the read is keyed on it. An effect
            // keyed on the reload counter alone leaves the chips changing
            // nothing — the comment beside that effect says so, and until this
            // test nothing measured it.
            setContent { CadenceTheme { CadenceApp(mocks = mocks()) } }

            onNodeWithText("Тренды").performClick()
            waitForIdle()

            // «3 месяца» is the default, and on it the weight has eight
            // readings and therefore a delta. A week has one and none.
            val quarter = onAllNodesWithText("↓", substring = true).fetchSemanticsNodes().size
            assertTrue(quarter > 0, "the three-month list carries deltas")

            onNodeWithTag(cadenceTrendWindowTag(TrendWindow.WEEK)).performClick()
            waitForIdle()

            assertTrue(
                onAllNodesWithText("↓", substring = true).fetchSemanticsNodes().size < quarter,
                "a week holds fewer readings, so fewer metrics have moved",
            )
            onNodeWithTag(CADENCE_TRENDS_HERO_TAG, useUnmergedTree = true).assertExists()
        }

    @Test
    fun tappingAMetricOpensItWithTheDataTheAppRead() =
        runComposeUiTest {
            // The other half of the shipped wiring: `onLoadMetric` reaching the
            // repository rather than answering null forever.
            lateinit var nav: NavHostController
            setContent {
                nav = rememberNavController()
                CadenceTheme { CadenceApp(navController = nav, mocks = mocks()) }
            }

            onNodeWithText("Тренды").performClick()
            waitForIdle()
            onNodeWithTag(cadenceTrendCardTag(Metric.WEIGHT)).performClick()
            waitForIdle()

            assertEquals(
                listOf("Today", "Trends", "TrendDetail"),
                nav.currentBackStack.value
                    .filter { it.destination !is NavGraph }
                    .mapNotNull { it.destination.route }
                    .map { it.substringBefore('/').substringBefore('?').substringAfterLast('.') },
            )
            // The aggregates only exist once the read landed — a loader stuck
            // on null leaves the placeholder here.
            onNodeWithTag(CADENCE_TREND_DETAIL_STATS_TAG, useUnmergedTree = true).assertExists()
        }
}
