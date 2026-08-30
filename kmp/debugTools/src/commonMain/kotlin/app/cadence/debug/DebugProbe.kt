package app.cadence.debug

import io.ktor.client.HttpClient
import io.ktor.client.request.get
import io.ktor.client.statement.bodyAsText
import io.ktor.http.HttpStatusCode
import io.ktor.http.isSuccess

/**
 * `GET /v1/me` through the client that carries the session, which is the whole pipeline in one
 * call: generation, the `Authorization` header, deserialisation and the error shape.
 *
 * One endpoint is enough and that is a recorded decision — a second says something about the
 * endpoint rather than about the pipeline.
 *
 * The catch is broad on purpose and reported rather than swallowed: this screen exists to say
 * what went wrong, and narrowing it would turn an unforeseen failure into a crash on the one
 * surface whose job is to describe failures.
 */
@Suppress("TooGenericExceptionCaught")
suspend fun probeMe(
    client: HttpClient,
    base: String,
): ProbeState =
    try {
        val answer = client.get("$base/v1/me")
        when {
            answer.status.isSuccess() -> ProbeState.SignedIn(answer.bodyAsText())
            answer.status == HttpStatusCode.Unauthorized -> ProbeState.SignedOut
            else -> ProbeState.Unavailable("the API answered ${answer.status}")
        }
    } catch (expected: Exception) {
        // No answer at all: a host that is down, a name that does not resolve, a socket that
        // closed. «Signed out» would be the wrong word — it sends a developer to re-authenticate
        // against something that cannot authenticate anybody.
        ProbeState.Unavailable(expected.message ?: "the API did not answer")
    }

/**
 * `/healthz`, on a client with no auth plugin.
 *
 * It is outside the OpenAPI contract deliberately, so it is absent from the generated client and
 * there is nothing to attach a Bearer to. Asked without a credential, which is what separates
 * «the API is up» from «my token works» — the two questions the screen exists to tell apart.
 *
 * The exception is dropped rather than carried because the answer is a boolean: what went wrong
 * belongs to the line above, which runs against the same host.
 */
@Suppress("TooGenericExceptionCaught", "SwallowedException")
suspend fun probeHealth(
    raw: HttpClient,
    base: String,
): Boolean =
    try {
        raw.get("$base/healthz").status.isSuccess()
    } catch (expected: Exception) {
        false
    }
