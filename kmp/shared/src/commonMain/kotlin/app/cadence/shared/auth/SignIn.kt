package app.cadence.shared.auth

import io.github.jan.supabase.SupabaseClient
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.auth.providers.builtin.Email
import io.github.jan.supabase.exceptions.RestException
import io.ktor.utils.io.errors.IOException
import kotlinx.coroutines.CancellationException

/**
 * The answer to signing in, and it is three rather than four on purpose.
 *
 * [Refused] carries no reason, and that is the point: an address the clinic never invited and a
 * password typed wrong have to answer the same sentence, or the form tells anyone who asks which
 * of the clinic's patients has an account here.
 */
sealed interface SignIn {
    data object Accepted : SignIn

    data object Refused : SignIn

    data object Unreachable : SignIn
}

/**
 * Signs in with an address and a password, which the vendor then stores.
 *
 * The retryable answers are picked out by status for the reason [acceptInvitation] records, and
 * everything else is a refusal — carrying nothing, because there is nothing here it would be safe
 * to say.
 */
@Suppress("SwallowedException")
suspend fun SupabaseClient.signIn(
    email: String,
    password: String,
): SignIn =
    try {
        auth.signInWith(Email) {
            this.email = email
            this.password = password
        }

        SignIn.Accepted
    } catch (cancelled: CancellationException) {
        throw cancelled
    } catch (refused: RestException) {
        if (refused.isWorthAnotherTry()) SignIn.Unreachable else SignIn.Refused
    } catch (unreachable: IOException) {
        SignIn.Unreachable
    }

/**
 * Ends the session, whatever the server says.
 *
 * A server that cannot be reached does not get a veto here: the patient has decided, and the harm
 * the button exists to prevent — a device handed on while still signed in — is entirely local.
 * The stored session goes either way; what the server loses is the chance to revoke the refresh
 * token now rather than at its expiry.
 */
@Suppress("SwallowedException")
suspend fun SupabaseClient.signOut() {
    try {
        auth.signOut()
    } catch (cancelled: CancellationException) {
        throw cancelled
    } catch (refused: RestException) {
        auth.clearSession()
    } catch (unreachable: IOException) {
        auth.clearSession()
    }
}
