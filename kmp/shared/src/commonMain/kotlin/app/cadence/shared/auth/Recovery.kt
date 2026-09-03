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
 * here?» to anyone who types an address. The per-address gap is folded into [Sent] for that same
 * reason and it cost a review round to see why — `GOTRUE_SMTP_MAX_FREQUENCY` is enforced against
 * `users.recovery_sent_at`, a row a stranger's address does not have, so a sentence of its own for
 * the gap is a sentence only a real patient can provoke. Two asks and sixty seconds would have
 * been the oracle this whole answer exists to close.
 */
sealed interface Recovery {
    data object Sent : Recovery

    data object Unreachable : Recovery
}

/**
 * Asks for a recovery mail.
 *
 * Every refusal not worth retrying is answered [Sent], the gap among them: there is nothing in one
 * a patient could act on, and saying anything else about it is saying something about the address.
 * The gap is `GOTRUE_SMTP_MAX_FREQUENCY` — a minute per person, measured, and the name matters:
 * `MAILER_MAX_FREQUENCY` is silently ignored.
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
            // Ahead of the retryable check, which counts 429 as «ask again». Here it is neither
            // that nor a sentence of its own: the last mail is already on its way, which is what
            // Sent says, and only a real patient can reach this arm at all.
            refused.statusCode == TOO_MANY_REQUESTS -> Recovery.Sent

            refused.isWorthAnotherTry() -> Recovery.Unreachable

            else -> Recovery.Sent
        }
    } catch (unreachable: IOException) {
        Recovery.Unreachable
    }
