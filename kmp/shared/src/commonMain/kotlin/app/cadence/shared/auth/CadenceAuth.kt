package app.cadence.shared.auth

import app.cadence.shared.net.Session
import app.cadence.shared.net.SessionTokens
import app.cadence.shared.storage.PKCE_STORE
import app.cadence.shared.storage.SESSION_STORE
import app.cadence.shared.storage.secureSettings
import com.russhwolf.settings.Settings
import io.github.jan.supabase.SupabaseClient
import io.github.jan.supabase.auth.Auth
import io.github.jan.supabase.auth.SettingsCodeVerifierCache
import io.github.jan.supabase.auth.SettingsSessionManager
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.createSupabaseClient
import io.ktor.client.engine.HttpClientEngine
import kotlinx.coroutines.CancellationException

/**
 * `customUrl` is what makes the vendor's client talk to a self-hosted server: measured in the
 * 3.7.0 artifact, `MainPlugin.resolveUrl` appends `auth/v1` only where it is null, and ours
 * answers on its own root — every `/auth/v1` path is a 404 there, measured against the local
 * contour. ADR-008 took the vendor away and left the mechanism.
 *
 * Both storage seams are substituted, which is the condition the library was taken on rather
 * than a refinement — see [secureSettings] for why they are two stores and not one.
 *
 * The verifier cache is groundwork and has no consumer today: accepting an invitation was to be a
 * PKCE flow until it was measured not to be one — GoTrue v2.194.0 accepts a `code_challenge` on
 * the admin route and ignores it — and the invitation now leads into the app with a token the app
 * exchanges itself. It stays because a provider sign-in would want it back, and an empty store
 * costs nothing; the reasoning is in the proposal «Приём приглашения: PKCE недостижим».
 *
 * [stores] is a parameter so the seam can be measured without a device; production passes
 * [secureSettings]. [engine] is one for the same reason and a sharper one: the module builds its
 * own client, so left to itself nothing outside this file can observe which address it talks to —
 * a test can only restate the argument it just passed.
 */
fun cadenceAuth(
    url: String,
    stores: (String) -> Settings = ::secureSettings,
    engine: HttpClientEngine? = null,
): SupabaseClient =
    createSupabaseClient(supabaseUrl = url, supabaseKey = "") {
        engine?.let { httpEngine = it }
        install(Auth) {
            customUrl = url
            sessionManager = SettingsSessionManager(stores(SESSION_STORE))
            codeVerifierCache = SettingsCodeVerifierCache(stores(PKCE_STORE))
        }
    }

/** The whole of the coupling between the Auth module and the transport. */
fun SupabaseClient.sessionTokens(): SessionTokens =
    object : SessionTokens {
        override suspend fun current(): Session? = auth.currentSessionOrNull()?.asSession()

        /**
         * Null rather than a throw, which is what the caller's contract says and what the module
         * does not do.
         *
         * Measured in the 3.7.0 artifact: `refreshCurrentSession` answers by throwing —
         * `IllegalStateException("No refresh token found in current session")` where nothing is
         * stored, and out of the HTTP call where the token is refused. Left to propagate, the
         * first request of a signed-out app throws instead of routing to sign-in, and the
         * screen's «signed out» state becomes unreachable: every refusal would arrive as «the
         * server is unavailable», which is the one confusion three states exist to prevent.
         *
         * The session is **not** cleared here, and that is a divergence from the spec's wording.
         * A refusal and a network blip arrive alike, and clearing on both would sign a patient
         * out because their train went into a tunnel — the same rule the vault keeps: do not
         * erase on a failure you cannot name. What clears a session is signing out.
         *
         * The swallow is the contract and not an oversight: what the caller is owed is «could
         * not be renewed», and carrying the exception further would put a refresh token's
         * failure into whatever the transport logs.
         */
        @Suppress("TooGenericExceptionCaught", "SwallowedException")
        override suspend fun refreshed(): Session? =
            try {
                auth.refreshCurrentSession()
                auth.currentSessionOrNull()?.asSession()
            } catch (cancelled: CancellationException) {
                throw cancelled
            } catch (expected: Exception) {
                null
            }
    }

private fun io.github.jan.supabase.auth.user.UserSession.asSession() =
    Session(access = accessToken, refresh = refreshToken)

// The process's one client, and the reason it is one is the invariant the transport is built
// around. Every SupabaseClient loads the stored session and starts its own auto-refresh on a
// scope of its own, which nothing outside the client cancels — so two of them are two owners
// rotating one refresh token, and under rotation the loser spends a token already spent. The
// patient is then signed out by their own app having been recreated.
//
// A named gap, not a solved one: `getOrPut` is not atomic, so two callers racing on different
// threads can both build a client and one is discarded — with its auto-refresh already
// started, which is the very thing this exists to prevent. Reachable only if a second entry
// point appears; today both platform roots call it from the main thread at launch. The same
// gap is named on `secureSettings`, and it is the same fix when either needs one.
private val theClient = mutableMapOf<String, SupabaseClient>()

/**
 * The auth client for [url], built once among the app's roots — see `theClient` for why one.
 *
 * Not «one per process», and the exception is named rather than closed: `:debugTools` builds its
 * own client on the same URL and the same session store, so opening the debug screen beside the
 * app does put two refresh owners in one process and can sign a developer out of the dev contour.
 * It is absent from both release builds — `debugImplementation` on Android, a source directory
 * added only under `-Pcadence.debugTools` on iOS — so the cost is the developer's, not a patient's.
 */
fun cadenceAuthFor(url: String): SupabaseClient = theClient.getOrPut(url) { cadenceAuth(url) }
