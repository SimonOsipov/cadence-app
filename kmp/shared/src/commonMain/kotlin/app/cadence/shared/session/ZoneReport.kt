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
 * [zone] is asked at every report and deliberately never kept: hoisting it out of the collection
 * is the change that silently stops a zone changed between launches from getting through.
 */
@Suppress("TooGenericExceptionCaught", "SwallowedException")
suspend fun reportZoneWhileSignedIn(
    states: Flow<SessionState>,
    zone: () -> String = ::deviceZone,
    report: suspend (String) -> Unit,
) {
    states.filter { it == SessionState.SignedIn }.collect {
        // Swallowed: a zone the server refuses and a server that is not there are both answered by
        // the next sign-in asking again, and neither is anything to show a patient.
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

/** The endpoint as the reporter's seam; the status goes unread, for the reason the swallow above gives. */
fun IdentityApi.zoneReporter(): suspend (String) -> Unit = { recordSession(SessionBody(timezone = it)) }
