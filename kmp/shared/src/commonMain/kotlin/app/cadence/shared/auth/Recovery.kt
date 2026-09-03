package app.cadence.shared.auth

import io.github.jan.supabase.SupabaseClient
import io.github.jan.supabase.auth.auth
import io.github.jan.supabase.exceptions.RestException
import io.ktor.utils.io.errors.IOException
import kotlinx.coroutines.CancellationException

/**
 * The answer to asking for a recovery mail, and [Sent] is what an address the clinic has never
 * seen gets too.
 *
 * That is the point rather than a shortcut: told apart, the form answers «is this person a patient
 * here?» to anyone who types an address. [TooSoon] is the one refusal a patient can act on, and it
 * is separated from [Unreachable] because the two ask for opposite things — one to wait, the other
 * to try again now.
 */
sealed interface Recovery {
    data object Sent : Recovery

    data object TooSoon : Recovery

    data object Unreachable : Recovery
}

/**
 * Asks for a recovery mail.
 *
 * Every refusal that is not the gap and not worth retrying is answered [Sent]: there is nothing in
 * one a patient could act on, and saying anything else about it is saying something about the
 * address. The gap is `GOTRUE_SMTP_MAX_FREQUENCY` — a minute per person, measured, and the name
 * matters: `MAILER_MAX_FREQUENCY` is silently ignored.
 */
@Suppress("SwallowedException")
suspend fun SupabaseClient.recover(email: String): Recovery =
    try {
        auth.resetPasswordForEmail(email)

        Recovery.Sent
    } catch (cancelled: CancellationException) {
        throw cancelled
    } catch (refused: RestException) {
        when {
            // Ahead of the retryable check, which counts 429 as one: here it is not «ask again»
            // but «the last mail is already on its way».
            refused.statusCode == TOO_MANY_REQUESTS -> Recovery.TooSoon

            refused.isWorthAnotherTry() -> Recovery.Unreachable

            else -> Recovery.Sent
        }
    } catch (unreachable: IOException) {
        Recovery.Unreachable
    }
