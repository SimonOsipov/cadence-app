package app.cadence.shared.session

import app.cadence.shared.auth.SessionState
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import kotlinx.datetime.TimeZone
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

    // The criterion's own sentence — «a timezone changed between launches gets through». It has no
    // mutation of its own: each launch is a fresh call that reads the zone once either way.
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

    // A refusal raised by [zoneReporter] and a server that is not there arrive here alike, and
    // neither ends the collection: the next entry into a session reports again.
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

    // The one exception the swallow must not take. `kotlinx.coroutines.CancellationException` is an
    // `IllegalStateException`, so deleting the rethrow leaves it caught by the clause below and the
    // collector outliving its scope — which nothing else here would notice.
    @Test
    fun cancellationIsNotSwallowedWithTheRest() =
        runTest {
            assertFailsWith<CancellationException> {
                reportZoneWhileSignedIn(A_LAUNCH_WITH_A_SESSION, zone = { "Europe/Moscow" }) {
                    throw CancellationException("the collector was cancelled")
                }
            }
        }

    // The default every other test passes over and the app passes nothing else: an offset id like
    // «+03:00» or an abbreviation is not in `pg_timezone_names`, and the server refuses it.
    @Test
    fun theDeviceZoneIsANameTheServerCouldKnow() {
        assertTrue(deviceZone() in TimeZone.availableZoneIds, "deviceZone() answered ${deviceZone()}")
    }
}
