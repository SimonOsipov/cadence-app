package app.cadence.shared.storage

/**
 * What a [Vault] answered, and the distinction the whole design rests on.
 *
 * [Absent] and [Unavailable] both mean «no session to hand back», and treating them as one
 * is what turns «unreadable right now» into «erased for good»: the store is written back
 * whole, so a caller that read nothing and then writes serialises that nothing over a
 * session that was only ever unreadable. A locked device between boot and first unlock is
 * exactly that case, and it is the case a background token refresh runs in.
 */
sealed interface Stored {
    /** Nothing was ever written here. Writing is safe. */
    data object Absent : Stored

    class Present(
        val bytes: ByteArray,
    ) : Stored

    /** Something may be here and could not be read. [reason] carries the platform's own word. */
    data class Unavailable(
        val reason: String,
    ) : Stored
}

/**
 * Bytes at rest, and nothing else: the platform decides where they live and how they are
 * protected, and everything above this line is the same on both.
 */
interface Vault {
    fun read(): Stored

    /** False where the store was not written, so a caller cannot mistake it for kept. */
    fun write(bytes: ByteArray): Boolean

    /** False where the store may still be there — sign-out has to be able to tell. */
    fun wipe(): Boolean
}
