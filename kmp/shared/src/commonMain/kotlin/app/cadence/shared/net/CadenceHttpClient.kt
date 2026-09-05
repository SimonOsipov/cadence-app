package app.cadence.shared.net

import io.ktor.client.HttpClient
import io.ktor.client.HttpClientConfig
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
 * Consumer-owned, and both halves are somebody else's work: the session lives in secure storage
 * and the refresh belongs to the auth module. Nothing here refreshes — see [cadenceHttpClient].
 */
interface SessionTokens {
    suspend fun current(): Session?

    /** Null where the session could not be renewed, which is «signed out» to the caller. */
    suspend fun refreshed(): Session?
}

/**
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
): HttpClient = HttpClient(engine) { cadence(tokens) }

/** The same client on whichever engine the platform ships — what the app builds; tests pass one. */
fun cadenceHttpClient(tokens: SessionTokens): HttpClient = HttpClient { cadence(tokens) }

// One transport per address among the app's roots, and the reason is sharper than the auth
// client's: built without an engine the client owns the engine it makes, and nothing here closes
// one. Held in a composition instead, every Android activity recreation — a font-scale, density or
// locale change, none of them in `configChanges` — would leak a connection pool.
//
// Not «one per process», and the exception is the same one `theClient` names: `:debugTools` builds
// its own client against this very address, so opening the debug screen beside the app does put two
// bearer providers on one refresh token. It ships in neither release build.
//
// Same named gap as `theClient` too: `getOrPut` is not atomic.
private val theTransport = mutableMapOf<String, HttpClient>()

/**
 * The API transport for [url] — see `theTransport` for why it is not built per caller.
 *
 * [tokens] is used only to build the first one for an address; a later caller's is discarded, which
 * the types cannot say and which is invisible at the call site.
 */
fun cadenceHttpClientFor(
    url: String,
    tokens: SessionTokens,
): HttpClient = theTransport.getOrPut(url) { cadenceHttpClient(tokens) }

private fun HttpClientConfig<*>.cadence(tokens: SessionTokens) {
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
