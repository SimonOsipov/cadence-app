package app.cadence.debug

import app.cadence.shared.api.apis.IdentityApi
import io.ktor.client.HttpClient
import io.ktor.client.request.get
import io.ktor.http.HttpStatusCode
import io.ktor.http.isSuccess
import kotlinx.coroutines.CancellationException

/**
 * The whole pipeline in one call: generation, the `Authorization` header the transport attaches,
 * deserialisation into the contract's own type, and the error shape.
 *
 * One endpoint is enough and that is a recorded decision — a second says something about the
 * endpoint rather than about the pipeline.
 *
 * [identity] must be built on a client that already parses JSON: `ApiClient(baseUrl, httpClient)`
 * assigns the client it is given and configures nothing on it, so the generated class's own
 * serializer never runs. `cadenceHttpClient` installs it; a bare `HttpClient` would fail on the
 * body rather than on the call.
 *
 * The catch is broad on purpose and reported rather than swallowed: this screen exists to say
 * what went wrong, and narrowing it would turn an unforeseen failure into a crash on the one
 * surface whose job is to describe failures.
 */
@Suppress("TooGenericExceptionCaught")
suspend fun probeMe(identity: IdentityApi): ProbeState =
    try {
        val answer = identity.getMe()
        when {
            // `full_name` is optional in the contract — an account the clinic holds no profile
            // for is signed in, and saying so is the point on a contour where GoTrue accounts
            // are made by hand.
            answer.success -> ProbeState.SignedIn(answer.body().fullName ?: "no profile for this account")

            answer.status == HttpStatusCode.Unauthorized.value -> ProbeState.SignedOut

            else -> ProbeState.Unavailable("the API answered ${answer.status}")
        }
    } catch (cancelled: CancellationException) {
        // The screen leaves composition mid-request. Cancellation is not a server failure, and
        // reported as one it would have the effect body carry on instead of unwinding.
        throw cancelled
    } catch (expected: Exception) {
        // No answer at all: a host that is down, a name that does not resolve, a socket that
        // closed, a body the contract cannot parse. «Signed out» would be the wrong word — it
        // sends a developer to re-authenticate against something that cannot authenticate.
        ProbeState.Unavailable(expected.message ?: "the API did not answer")
    }

/**
 * Outside the OpenAPI contract deliberately, so it is absent from the generated client and there
 * is nothing to attach a Bearer to. Asked without a credential, which is what separates «the API
 * is up» from «my token works» — the two questions the screen exists to tell apart.
 */
@Suppress("TooGenericExceptionCaught", "SwallowedException")
suspend fun probeHealth(
    raw: HttpClient,
    base: String,
): Boolean =
    try {
        raw.get("$base/healthz").status.isSuccess()
    } catch (cancelled: CancellationException) {
        throw cancelled
    } catch (expected: Exception) {
        false
    }
