package app.cadence.design

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.onAllNodesWithContentDescription
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.v2.runComposeUiTest
import app.cadence.shared.domain.InjectionSite
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/** The prototype's `viewBox="0 0 200 340"`, which every coordinate is in. */
private const val VIEW_WIDTH = 200f

/** Half a target's width, in fractions — a zone is a disc, not a point. */
private const val TOLERANCE = 0.02f

@OptIn(ExperimentalTestApi::class)
class BodyMapTest {
    @Test
    fun everyZoneOfTheSetHasALabelAndASide() {
        // The set, not the count. Ten zones that no diagram draws is the defect
        // this codebase already shipped once — two invented codes passed a test
        // that counted entries.
        assertEquals(InjectionSite.entries.toSet(), CadenceBodyZone.entries.map { it.site }.toSet())
        assertEquals(InjectionSite.entries.size, CadenceBodyZone.entries.size, "a zone is drawn twice")

        CadenceBodyZone.entries.forEach {
            assertTrue(it.labelRu.isNotBlank(), "${it.site} has no Russian label")
        }
    }

    @Test
    fun theCoordinateTableIsTheProtototypesOwn() {
        // Literals, against `ZONES_FRONT` and `ZONES_BACK` in
        // `mobile/src/features/log-dose/data.ts`. Without them the table is ten
        // compiled constants nothing checks: swapping the two deltoids mirrors
        // the whole body and every other test in this file still passes,
        // because a tap is routed by label and not by where the node sits.
        assertEquals(54f to 88f, CadenceBodyZone.RIGHT_DELTOID.x to CadenceBodyZone.RIGHT_DELTOID.y)
        assertEquals(146f to 88f, CadenceBodyZone.LEFT_DELTOID.x to CadenceBodyZone.LEFT_DELTOID.y)
        assertEquals(82f to 148f, CadenceBodyZone.RIGHT_ABDOMEN.x to CadenceBodyZone.RIGHT_ABDOMEN.y)
        assertEquals(118f to 148f, CadenceBodyZone.LEFT_ABDOMEN.x to CadenceBodyZone.LEFT_ABDOMEN.y)
        assertEquals(82f to 230f, CadenceBodyZone.RIGHT_THIGH.x to CadenceBodyZone.RIGHT_THIGH.y)
        assertEquals(118f to 230f, CadenceBodyZone.LEFT_THIGH.x to CadenceBodyZone.LEFT_THIGH.y)
        assertEquals(82f to 175f, CadenceBodyZone.LEFT_LOWER_BACK.x to CadenceBodyZone.LEFT_LOWER_BACK.y)
        assertEquals(118f to 175f, CadenceBodyZone.RIGHT_LOWER_BACK.x to CadenceBodyZone.RIGHT_LOWER_BACK.y)
        assertEquals(82f to 210f, CadenceBodyZone.LEFT_GLUTE.x to CadenceBodyZone.LEFT_GLUTE.y)
        assertEquals(118f to 210f, CadenceBodyZone.RIGHT_GLUTE.x to CadenceBodyZone.RIGHT_GLUTE.y)
    }

    @Test
    fun eachSideHoldsTheZonesTheProtototypePutsThere() {
        // The set, not the count. Moving a zone from the front to the back in
        // pairs keeps six and four, and the render tests derive their
        // expectation from the same field they are checking.
        assertEquals(
            setOf(
                InjectionSite.RIGHT_DELTOID,
                InjectionSite.LEFT_DELTOID,
                InjectionSite.RIGHT_ABDOMEN,
                InjectionSite.LEFT_ABDOMEN,
                InjectionSite.RIGHT_THIGH,
                InjectionSite.LEFT_THIGH,
            ),
            CadenceBodyZone.entries
                .filter { it.view == CadenceBodyView.FRONT }
                .map { it.site }
                .toSet(),
        )
        assertEquals(
            setOf(
                InjectionSite.LEFT_LOWER_BACK,
                InjectionSite.RIGHT_LOWER_BACK,
                InjectionSite.LEFT_GLUTE,
                InjectionSite.RIGHT_GLUTE,
            ),
            CadenceBodyZone.entries
                .filter { it.view == CadenceBodyView.BACK }
                .map { it.site }
                .toSet(),
        )
    }

    @Test
    fun aZoneAlreadyUsedSaysSoAndTheRestDoNot() =
        runComposeUiTest {
            // The prototype's `state.lastUsed` dots. Without them the rotation
            // suggestion reads as arbitrary — nothing on screen says why this
            // zone is next rather than any other.
            val used = InjectionSite.RIGHT_ABDOMEN

            setContent {
                CadenceTheme {
                    CadenceBodyMap(selected = null, suggested = null, lastUsed = listOf(used))
                }
            }

            onNodeWithContentDescription("Правый живот · использовано").assertExists()
            CadenceBodyZone.entries
                .filter { it.view == CadenceBodyView.FRONT && it.site != used }
                .forEach { onNodeWithContentDescription(it.labelRu).assertExists() }
        }

    @Test
    fun aZoneIsDrawnWhereItsCoordinateSays() =
        runComposeUiTest {
            // The only assertion in this file with positional content, and the
            // reason it exists: a target positioned in scaled viewBox units but
            // *sized* in fixed dp drifts by `TARGET_R × (1dp − unit)`, which on
            // a phone-width diagram is 14dp up and to the left of the shoulder
            // the silhouette draws. Every label-driven test stays green through
            // it, because a tap is routed by semantics and not by geometry.
            setContent { CadenceTheme { CadenceBodyMap(selected = null, suggested = null) } }

            val diagram = onNodeWithTag(CADENCE_BODY_MAP_TAG).fetchSemanticsNode().boundsInRoot
            val zone = CadenceBodyZone.RIGHT_DELTOID
            val node = onNodeWithContentDescription(zone.labelRu).fetchSemanticsNode().boundsInRoot

            val centre = (node.left + node.right) / 2
            val fraction = (centre - diagram.left) / diagram.width

            assertTrue(
                kotlin.math.abs(fraction - zone.x / VIEW_WIDTH) < TOLERANCE,
                "${zone.labelRu} sits at $fraction of the width, not ${zone.x / VIEW_WIDTH}",
            )
        }

    @Test
    fun bothSidesOfTheBodyAreDrawnOn() {
        // The prototype splits six zones onto the front and four onto the back.
        // A map that put them all on one side would still «draw ten».
        assertEquals(6, CadenceBodyZone.entries.count { it.view == CadenceBodyView.FRONT })
        assertEquals(4, CadenceBodyZone.entries.count { it.view == CadenceBodyView.BACK })
    }

    @Test
    fun theFrontZonesAreNamedForAScreenReaderAndTheBackOnesAreBehindTheToggle() =
        runComposeUiTest {
            setContent { CadenceTheme { CadenceBodyMap(selected = null, suggested = null) } }

            // `assertExists`, not `assertIsDisplayed`: the diagram is taller
            // than the test window, and how much of it is on screen is the
            // containing screen's business — the site step scrolls.
            CadenceBodyZone.entries.filter { it.view == CadenceBodyView.FRONT }.forEach {
                onNodeWithContentDescription(it.labelRu).assertExists()
            }
            CadenceBodyZone.entries.filter { it.view == CadenceBodyView.BACK }.forEach {
                assertEquals(
                    0,
                    onAllNodesWithContentDescription(it.labelRu).fetchSemanticsNodes().size,
                    "${it.labelRu} is drawn on the front",
                )
            }
        }

    @Test
    fun theToggleTurnsTheBodyRound() =
        runComposeUiTest {
            var view by mutableStateOf(CadenceBodyView.FRONT)

            setContent {
                CadenceTheme {
                    CadenceBodyMap(selected = null, suggested = null, view = view, onView = { view = it })
                }
            }

            onNodeWithText("Сзади").performClick()
            waitForIdle()

            CadenceBodyZone.entries.filter { it.view == CadenceBodyView.BACK }.forEach {
                onNodeWithContentDescription(it.labelRu).assertExists()
            }
            assertEquals(
                0,
                onAllNodesWithContentDescription("Правое плечо").fetchSemanticsNodes().size,
                "a front zone survived the turn",
            )
        }

    @Test
    fun everyZoneReportsItsOwnIdAndNotItsNeighboursWhenTapped() =
        runComposeUiTest {
            // One tap per zone, each asserted separately. A map whose handlers
            // were built in a loop that captured the wrong variable reports the
            // last zone for every tap, and a single-zone test would not see it.
            var tapped by mutableStateOf<InjectionSite?>(null)
            var view by mutableStateOf(CadenceBodyView.FRONT)

            setContent {
                CadenceTheme {
                    CadenceBodyMap(
                        selected = null,
                        suggested = null,
                        view = view,
                        onView = { view = it },
                        onSelect = { tapped = it },
                    )
                }
            }

            CadenceBodyZone.entries.forEach { zone ->
                view = zone.view
                waitForIdle()
                tapped = null

                onNodeWithContentDescription(zone.labelRu).performClick()
                waitForIdle()

                assertEquals(zone.site, tapped, "tapping ${zone.labelRu} reported the wrong zone")
            }
        }

    @Test
    fun theSelectedZoneSaysItIsSelectedAndTheOthersDoNot() =
        runComposeUiTest {
            val chosen = CadenceBodyZone.entries.first { it.view == CadenceBodyView.FRONT }

            setContent { CadenceTheme { CadenceBodyMap(selected = chosen.site, suggested = null) } }

            // A `Canvas` has no text, so «selected» has to be a semantics
            // property or a test cannot see the one thing this control says.
            val node = onNodeWithContentDescription(chosen.labelRu).fetchSemanticsNode()
            assertEquals(true, node.config.getOrNull(SemanticsProperties.Selected))

            CadenceBodyZone.entries
                .filter { it.view == CadenceBodyView.FRONT && it != chosen }
                .forEach {
                    assertEquals(
                        false,
                        onNodeWithContentDescription(it.labelRu).fetchSemanticsNode().config.getOrNull(
                            SemanticsProperties.Selected,
                        ),
                        "${it.labelRu} claims to be selected too",
                    )
                }
        }

    @Test
    fun theSuggestedZoneIsNamedUntilOneIsChosen() =
        runComposeUiTest {
            // «Предложено: Левый живот — следующее в ротации», and the prototype
            // drops the whole line the moment the patient picks anything.
            val suggested = InjectionSite.LEFT_ABDOMEN
            var selected by mutableStateOf<InjectionSite?>(null)

            setContent { CadenceTheme { CadenceBodyMap(selected = selected, suggested = suggested) } }

            onNodeWithText("Левый живот", substring = true).assertExists()

            selected = InjectionSite.RIGHT_THIGH
            waitForIdle()

            assertEquals(
                0,
                onAllNodesWithText("Предложено:", substring = true).fetchSemanticsNodes().size,
                "the suggestion outlived the choice",
            )
            onNodeWithText("Правое бедро", substring = true).assertExists()
        }

    @Test
    fun aZoneLabelIsLookedUpByItsSiteAndNeverGuessed() {
        InjectionSite.entries.forEach {
            assertNotNull(CadenceBodyZone.of(it), "no zone drawn for $it")
            assertEquals(it, CadenceBodyZone.of(it)?.site)
        }
    }
}
