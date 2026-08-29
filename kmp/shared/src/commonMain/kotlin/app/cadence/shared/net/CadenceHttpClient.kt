package app.cadence.shared.net

import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.plugins.auth.Auth
import io.ktor.client.plugins.auth.providers.BearerTokens
import io.ktor.client.plugins.auth.providers.bearer
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.json.Json

/**
 * The session as this app carries it, which is deliberately not Ktor's `BearerTokens`.
 *
 * Measured against the 3.5.1 artifact rather than assumed: `BearerTokens` has no `toString`,
 * so it prints an identity — but it carries both tokens in fields, and a session reaches a log
 * or a crash report the moment the app's own code holds one. This is the type the app holds,
 * and it redacts; Ktor's is built at the plugin boundary, where nothing prints.
 */
class Session(
    val access: String,
    val refresh: String,
) {
    override fun toString(): String = "Session(access=…, refresh=…)"
}

/**
 * Where the transport gets a token and where it asks for a new one.
 *
 * Consumer-owned, and both halves are somebody else's work: the session lives in secure storage
 * and the refresh belongs to the auth module. Nothing here refreshes — see [cadenceHttpClient].
 */
interface SessionTokens {
    suspend fun current(): Session?

    /** Null where the session could not be renewed, which is «signed out» to the caller. */
    suspend fun refreshed(): Session?
}

/**
 * The client every call goes through.
 *
 * **There is one owner of token refresh and it is not this.** Ktor's `Auth` plugin serialises
 * `refreshTokens` on its own — several requests meeting a 401 at once wake one refresh and all
 * of them retry with what it produced. That is named here rather than reimplemented, because a
 * second mechanism racing the first is what revokes a session: the provider rotates the refresh
 * token, and the loser of the race spends one that is already spent.
 *
 * The token is attached by this layer and by no call site, and only as a header. In a query
 * string it would be written to every access log and proxy between here and the API, and it is
 * the token that opens the patient's record.
 *
 * No logging plugin is installed, so the acceptance criterion about `sanitizeHeader` on
 * `Authorization` has nothing to attach to yet — it becomes due with the first one, and the
 * half that can be checked without it is [Session], which does not print what it holds.
 */
fun cadenceHttpClient(
    engine: HttpClientEngine,
    tokens: SessionTokens,
): HttpClient =
    HttpClient(engine) {
        install(ContentNegotiation) {
            json(
                Json {
                    ignoreUnknownKeys = true
                    explicitNulls = false
                },
            )
        }
        install(Auth) {
            bearer {
                loadTokens { tokens.current()?.asBearer() }
                refreshTokens { tokens.refreshed()?.asBearer() }
            }
        }
    }

private fun Session.asBearer() = BearerTokens(accessToken = access, refreshToken = refresh)
