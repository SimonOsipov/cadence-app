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
 * The Auth module, pointed at our own GoTrue and holding its secrets where we put them.
 *
 * `customUrl` is what makes the vendor's client talk to a self-hosted server: measured in the
 * 3.7.0 artifact, `MainPlugin.resolveUrl` appends `auth/v1` only where it is null, and ours
 * answers on its own root — every `/auth/v1` path is a 404 there, measured against the local
 * contour. ADR-008 took the vendor away and left the mechanism.
 *
 * Both storage seams are substituted, and that is the condition the library was taken on rather
 * than a refinement: `SettingsSessionManager` and `SettingsCodeVerifierCache` default to
 * plaintext on both platforms — `SharedPreferences` and `NSUserDefaults`. They are handed two
 * different secure stores, because the store is written whole and one shared between them would
 * lose the verifier to the session's next write, in the middle of accepting an invite.
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

/**
 * The transport's view of this session, and the whole of the coupling between them.
 *
 * Refresh belongs to the Auth module and the transport does not do it — `refreshCurrentSession`
 * is the owner, and Ktor's plugin is what serialises the callers. Two mechanisms rotating one
 * refresh token is a revoked session.
 */
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
