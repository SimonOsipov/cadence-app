package app.cadence.shared.session

import app.cadence.shared.auth.SessionState
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

private val A_LAUNCH_WITH_A_SESSION = flowOf(SessionState.Deciding, SessionState.SignedIn)

private val A_SIGN_IN = flowOf(SessionState.Deciding, SessionState.SignedOut, SessionState.SignedIn)

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

    // The criterion's own sentence: two launches, a zone that changed between them, and both
    // values arriving. A patient who flew keeps a schedule computed where they left until this
    // holds.
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

    // The same property where a launch cannot carry it, and this is the one that fails against a
    // zone read once: hoisted out of the collection both reports say Moscow, and the test above
    // stays green because each of its launches reads once anyway.
    @Test
    fun theZoneIsAskedForAtEachReportRatherThanOnce() =
        runTest {
            val zones = mutableListOf("Europe/Moscow", "Asia/Tbilisi")
            val reported = mutableListOf<String>()

            reportZoneWhileSignedIn(
                flowOf(
                    SessionState.SignedIn,
                    SessionState.SignedOut,
                    SessionState.SignedIn,
                ),
                zone = { zones.removeAt(0) },
            ) { reported += it }

            assertEquals(listOf("Europe/Moscow", "Asia/Tbilisi"), reported)
        }

    // A zone the server refuses, or a server that is not there, is not the patient's problem and
    // not the end of the reporting: the next transition into a session asks again.
    @Test
    fun aRefusedReportDoesNotStopTheNextOne() =
        runTest {
            val reported = mutableListOf<String>()
            var first = true

            reportZoneWhileSignedIn(
                flowOf(SessionState.SignedIn, SessionState.SignedOut, SessionState.SignedIn),
                zone = { "Europe/Moscow" },
            ) {
                if (first) {
                    first = false
                    error("the server said no")
                }
                reported += it
            }

            assertEquals(listOf("Europe/Moscow"), reported)
        }
}
