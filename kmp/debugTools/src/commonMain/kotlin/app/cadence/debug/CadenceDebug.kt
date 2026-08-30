package app.cadence.debug

import app.cadence.shared.api.apis.IdentityApi
import app.cadence.shared.auth.cadenceAuth
import app.cadence.shared.auth.sessionTokens
import app.cadence.shared.net.API_BASE
import app.cadence.shared.net.AUTH_BASE
import app.cadence.shared.net.cadenceHttpClient
import app.cadence.shared.storage.secureSettings
import com.russhwolf.settings.Settings
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.auth.providers.builtin.Email
import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import kotlinx.coroutines.CancellationException

/**
 * The whole stack assembled once, which is the only place in the tree where it is.
 *
 * The debug screen exists to answer whether the parts fit together, and a screen handed ready
 * answers would prove nothing. Every seam is a default rather than a fixed value so the assembly
 * itself is measurable off a device — the engine, both addresses, and the secure stores.
 *
 * Two clients and not one: [health] must go out with no credential attached, and there is no way
 * to un-attach the `Auth` plugin for one call. The engine is shared because neither client owns
 * it — `HttpClient(engine)` does not close an engine it was handed.
 */
class CadenceDebug(
    private val api: String = API_BASE,
    gotrue: String = AUTH_BASE,
    engine: HttpClientEngine = debugEngine(),
    stores: (String) -> Settings = ::secureSettings,
) {
    private val supabase = cadenceAuth(url = gotrue, stores = stores)
    private val identity = IdentityApi(api, cadenceHttpClient(engine, supabase.sessionTokens()))
    private val raw = HttpClient(engine)

    /** Null where the sign-in went through; otherwise what to put on the screen. */
    @Suppress("TooGenericExceptionCaught")
    suspend fun signIn(
        email: String,
        password: String,
    ): String? =
        try {
            supabase.auth.signInWith(Email) {
                this.email = email
                this.password = password
            }
            null
        } catch (cancelled: CancellationException) {
            throw cancelled
        } catch (refused: Exception) {
            refused.message ?: "the identity server refused the sign-in"
        }

    suspend fun me(): ProbeState = probeMe(identity)

    suspend fun health(): Boolean = probeHealth(raw, api)
}
