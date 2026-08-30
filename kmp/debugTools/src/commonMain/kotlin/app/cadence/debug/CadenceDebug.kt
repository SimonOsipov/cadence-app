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
 * The debug screen exists to answer whether the parts fit together, and a screen handed ready
 * answers would prove nothing. Every seam is a default rather than a fixed value so the assembly
 * itself is measurable off a device — the engine, both addresses, and the secure stores.
 *
 * The engine is shared by all three — the auth module included, which is what makes the address
 * it uses observable — because none of them owns it: `HttpClient(engine)` does not close an
 * engine it was handed.
 */
class CadenceDebug(
    private val api: String = API_BASE,
    gotrue: String = AUTH_BASE,
    engine: HttpClientEngine = debugEngine(),
    stores: (String) -> Settings = ::secureSettings,
) {
    private val supabase = cadenceAuth(url = gotrue, stores = stores, engine = engine)
    private val identity = IdentityApi(api, cadenceHttpClient(engine, supabase.sessionTokens()))
    private val raw = HttpClient(engine)

    @Suppress("TooGenericExceptionCaught")
    suspend fun signIn(
        email: String,
        password: String,
    ): SignIn =
        try {
            supabase.auth.signInWith(Email) {
                this.email = email
                this.password = password
            }
            SignIn.Accepted
        } catch (cancelled: CancellationException) {
            throw cancelled
        } catch (refused: Exception) {
            // The exception's class and not its message. kotlinx.serialization appends the input
            // it could not decode, and on this path that input is GoTrue's token response — so
            // the one arbitrary string on the screen is the one that could print a token. Session
            // redacts its own fields for the same reason.
            SignIn.Refused(refused::class.simpleName ?: "the identity server refused the sign-in")
        }

    suspend fun me(): ProbeState = probeMe(identity)

    suspend fun health(): Boolean = probeHealth(raw, api)
}
