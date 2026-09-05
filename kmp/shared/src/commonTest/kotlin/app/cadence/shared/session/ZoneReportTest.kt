package app.cadence.shared.session

import app.cadence.shared.auth.SessionState
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

private val A_LAUNCH_WITH_A_SESSION = flowOf(SessionState.Deciding, SessionState.SignedIn)

private val A_SIGN_IN = flowOf(SessionState.Deciding, SessionState.SignedOut, SessionState.SignedIn)

private val TWO_ENTRIES_IN_ONE_PROCESS =
    flowOf(SessionState.SignedIn, SessionState.SignedOut, SessionState.SignedIn)

class ZoneReportTest {
    @Test
    fun aLaunchThatAlreadyHasASessionReportsTheZone() =
        runTest {
            val reported = mutableListOf<String>()

            reportZoneWhileSignedIn(A_LAUNCH_WITH_A_SESSION, zone = { "Europe/Moscow" }) { reported += it }

            assertEquals(listOf("Europe/Moscow"), reported)
        }

    @Test
    fun signingInReportsTheZone() =
        runTest {
            val reported = mutableListOf<String>()

            reportZoneWhileSignedIn(A_SIGN_IN, zone = { "Europe/Moscow" }) { reported += it }

            assertEquals(listOf("Europe/Moscow"), reported)
        }

    // The states before a session are not a report: a zone belongs to an account, and there is
    // nobody to record it against until the vendor has said who is here.
    @Test
    fun nothingIsReportedWithoutASession() =
        runTest {
            val reported = mutableListOf<String>()

            reportZoneWhileSignedIn(
                flowOf(SessionState.Deciding, SessionState.SignedOut),
                zone = { "Europe/Moscow" },
            ) { reported += it }

            assertTrue(reported.isEmpty(), "reported $reported without a session")
        }

    // No mutation of its own: each launch is a fresh call that reads the zone once either way.
    @Test
    fun aZoneChangedBetweenLaunchesGetsThrough() =
        runTest {
            val reported = mutableListOf<String>()
            var zone = "Europe/Moscow"

            reportZoneWhileSignedIn(A_LAUNCH_WITH_A_SESSION, zone = { zone }) { reported += it }
            zone = "Asia/Tbilisi"
            reportZoneWhileSignedIn(A_LAUNCH_WITH_A_SESSION, zone = { zone }) { reported += it }

            assertEquals(listOf("Europe/Moscow", "Asia/Tbilisi"), reported)
        }

    // The mutation the read inside the collection is for: hoisted out, both entries report Moscow.
    // The third answer is what makes dropping the SignedIn filter fail here as well — a fixture
    // that ran out would throw into the collector's own swallow and leave this green.
    @Test
    fun theZoneIsAskedForAtEachReportRatherThanOnce() =
        runTest {
            val zones = listOf("Europe/Moscow", "Asia/Tbilisi")
            var asked = 0
            val reported = mutableListOf<String>()

            reportZoneWhileSignedIn(
                TWO_ENTRIES_IN_ONE_PROCESS,
                zone = { zones.getOrElse(asked++) { "a-third-ask" } },
            ) { reported += it }

            assertEquals(listOf("Europe/Moscow", "Asia/Tbilisi"), reported)
        }

    @Test
    fun aRefusedReportDoesNotStopTheNextOne() =
        runTest {
            val reported = mutableListOf<String>()
            var first = true

            reportZoneWhileSignedIn(TWO_ENTRIES_IN_ONE_PROCESS, zone = { "Europe/Moscow" }) {
                if (first) {
                    first = false
                    error("the server said no")
                }
                reported += it
            }

            assertEquals(listOf("Europe/Moscow"), reported)
        }

    // The clause below the rethrow catches `Exception`, so deleting the rethrow swallows a
    // cancellation and the collector outlives the scope that cancelled it.
    @Test
    fun cancellationIsNotSwallowedWithTheRest() =
        runTest {
            assertFailsWith<CancellationException> {
                reportZoneWhileSignedIn(A_LAUNCH_WITH_A_SESSION, zone = { "Europe/Moscow" }) {
                    throw CancellationException("the collector was cancelled")
                }
            }
        }
}
