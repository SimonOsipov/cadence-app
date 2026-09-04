package app.cadence.shared.session

import app.cadence.shared.api.apis.IdentityApi
import app.cadence.shared.api.models.SessionBody
import app.cadence.shared.auth.SessionState
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.filter
import kotlinx.datetime.TimeZone

/**
 * Reports the device's zone for as long as [states] runs — signing in and launching with a session
 * are one transition into [SessionState.SignedIn], so a single collector serves both.
 *
 * [zone] is read inside the collection, and the mutation that names it is a second entry into a
 * session within one process: hoisted out, it re-reports the first one's zone. Across launches the
 * hoist is invisible — each launch is a fresh call that reads once either way.
 */
@Suppress("TooGenericExceptionCaught", "SwallowedException")
suspend fun reportZoneWhileSignedIn(
    states: Flow<SessionState>,
    zone: () -> String = ::deviceZone,
    report: suspend (String) -> Unit,
) {
    states.filter { it == SessionState.SignedIn }.collect {
        // Swallowed so one failed report does not end the collection; [zoneReporter] says what throws.
        try {
            report(zone())
        } catch (cancelled: CancellationException) {
            throw cancelled
        } catch (expected: Exception) {
            Unit
        }
    }
}

/** The device zone's IANA name, as `pg_timezone_names` spells it — what the server validates against. */
fun deviceZone(): String = TimeZone.currentSystemDefault().id

/**
 * The endpoint as the reporter's seam, raising a refusal rather than returning it.
 *
 * `expectSuccess` is unset and the generated `wrap()` never *fails* on the status, so a refusal
 * arrives as normally as a 200 and is lost unless it is read here.
 *
 * **Named gap.** A 400 is a zone the server will not take — `pg_timezone_names` does not carry it —
 * and unlike a 503 it is not answered by asking again: the same zone goes out on every launch, the
 * patient's schedule stays in the zone they left, and nothing on the device records it. The server
 * logs the refusal without naming the account. Closing this needs somewhere to put a failure a
 * patient cannot act on, which this step does not build. `deviceZone` can itself produce such an
 * id — a device set to a bare offset answers `GMT+03:00`, which is no zone name — and nothing
 * normalizes it.
 */
fun IdentityApi.zoneReporter(): suspend (String) -> Unit =
    {
        val answered = recordSession(SessionBody(timezone = it))

        check(answered.success) { "the zone report was refused with ${answered.status}" }
    }
