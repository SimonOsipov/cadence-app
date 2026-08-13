package app.cadence.screens.inventory

import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ComposeUiTest
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onChildren
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.design.CadenceTheme
import app.cadence.shared.domain.VialId
import app.cadence.shared.domain.inventorySummary
import app.cadence.shared.mock.MockSeed
import kotlinx.datetime.LocalDate
import kotlin.test.Test
import kotlin.test.assertEquals

private val TODAY = LocalDate(2026, 5, 31)

/** The seeded cabinet, resolved the way the repository will hand it over. */
private fun cabinet() = inventorySummary(MockSeed.plan, MockSeed.vials, MockSeed.history, TODAY, MockSeed.compounds)

@OptIn(ExperimentalTestApi::class)
class VialsScreenTest {
    /** How many cards show this lot number. */
    private fun ComposeUiTest.lots(lot: String) = onAllNodesWithText(lot, substring = true).fetchSemanticsNodes().size

    /** The number on one of the summary card's four stats. */
    private fun ComposeUiTest.statFor(group: String): String? =
        onNodeWithTag(cabinetStatTag(group), useUnmergedTree = true)
            .onChildren()
            .fetchSemanticsNodes()
            .firstNotNullOfOrNull {
                it.config
                    .getOrNull(SemanticsProperties.Text)
                    ?.firstOrNull()
                    ?.text
            }

    @Test
    fun theHeaderCountsTheVialsInRussianAndNamesTheScreen() =
        runComposeUiTest {
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet()) } }

            // Four live vials — the disposed one is not in the fridge any more. «флакона», not
            // «флаконов»: the rule, not the prototype's ternary, which gets 21 wrong.
            onNodeWithText("4 флакона в холодильнике").assertExists()
            onNodeWithText("Ваша аптечка").assertExists()
        }

    @Test
    fun theSummaryCardCountsEachGroup() =
        runComposeUiTest {
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet()) } }

            // The seeded shape, each read off its own stat rather than a word that is also a
            // chip's label and a pill's.
            assertEquals("3", statFor("ACTIVE"))
            assertEquals("1", statFor("EXPIRING"))
            assertEquals("1", statFor("SEALED"))
            assertEquals("1", statFor("LOW"))
        }

    @Test
    fun eachChipFiltersTheListToItsOwnGroup() =
        runComposeUiTest {
            // By which vials are on screen, not by which chip looks selected: a filter wired to
            // the wrong group still highlights.
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet()) } }

            onNodeWithText("Истекают · 1").performScrollTo().performClick()
            waitForIdle()

            assertEquals(1, lots("B-2601"), "the expiring vial is not shown")
            assertEquals(0, lots("A-2261"), "semaglutide survived the filter")
        }

    @Test
    fun theAllChipBringsEveryLiveVialBack() =
        runComposeUiTest {
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet()) } }

            onNodeWithText("Истекают · 1").performScrollTo().performClick()
            waitForIdle()
            onNodeWithText("Все · 4").performScrollTo().performClick()
            waitForIdle()

            listOf("A-2261", "B-2510", "B-2601").forEach {
                assertEquals(1, lots(it), "$it is missing from «Все»")
            }
            assertEquals(0, lots("B-2204"), "the disposed vial is listed")
        }

    @Test
    fun theSealedSectionStartsClosedAndOpens() =
        runComposeUiTest {
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet()) } }

            assertEquals(0, lots("B-2610"), "the spare is shown unasked")

            // Uppercased on screen: the section heading is an eyebrow.
            onNodeWithText("В запасе · 1", ignoreCase = true).performScrollTo().performClick()
            waitForIdle()

            assertEquals(1, lots("B-2610"))
        }

    @Test
    fun eachCardNamesItsCompoundItsDoseAndWhatIsLeft() =
        runComposeUiTest {
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet()) } }

            onNodeWithText("Семаглутид").assertExists()
            // Through the formatter — the RU comma — beside the lot that tells two vials of one
            // compound apart.
            onNodeWithText("0,25 мг · A-2261").assertExists()
            // The remaining count is a subtraction, not a field: the seed stores none.
            onNodeWithText("1 / 4").assertExists()
        }

    @Test
    fun everyCardReportsItsOwnVialAndNotItsNeighbours() =
        runComposeUiTest {
            // One tap per card: a loop that captured the wrong variable would report the last
            // vial for every tap, invisible to a single-card test.
            var opened: VialId? = null
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet(), onOpenVial = { opened = it }) } }

            mapOf("A-2261" to "vial-sema-1", "B-2510" to "vial-bpc-2", "B-2601" to "vial-bpc-3").forEach { (lot, id) ->
                opened = null
                onNodeWithText(lot, substring = true).performScrollTo().performClick()
                waitForIdle()

                assertEquals(VialId(id), opened, "tapping the $lot card reported the wrong vial")
            }
        }

    @Test
    fun aStatusPillSaysWhichStateEachVialIsIn() =
        runComposeUiTest {
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet()) } }

            // Three different words for three different vials: a pill fixed to one label
            // renders on every card and says nothing.
            assertEquals(1, onAllNodesWithText("Активный").fetchSemanticsNodes().size)
            assertEquals(1, onAllNodesWithText("Истекает").fetchSemanticsNodes().size)
            // Twice: the pill on the low card, and the summary card's own
            // count of the same group.
            assertEquals(2, onAllNodesWithText("На исходе").fetchSemanticsNodes().size)
        }

    @Test
    fun anEmptyCabinetOffersToFillItRatherThanShowingNothing() =
        runComposeUiTest {
            var adding = 0
            val empty = inventorySummary(MockSeed.plan, emptyList(), emptyList(), TODAY, MockSeed.compounds)

            setContent { CadenceTheme { VialsScreen(cabinet = empty, onAddVial = { adding++ }) } }

            onNodeWithText("Аптечка пуста").assertExists()
            onNodeWithText("Добавить флакон").performScrollTo().performClick()

            assertEquals(1, adding)
        }

    @Test
    fun theAddButtonReportsItsTapOnAFullCabinetToo() =
        runComposeUiTest {
            var adding = 0
            setContent { CadenceTheme { VialsScreen(cabinet = cabinet(), onAddVial = { adding++ }) } }

            onNodeWithContentDescription("Добавить флакон").performClick()

            assertEquals(1, adding)
        }
}
