package app.cadence.shared.storage

/**
 * Bytes at rest, and nothing else: the platform decides where they live and how they are
 * protected, and everything above this line is the same on both.
 */
interface Vault {
    /** Null where nothing was written. Throwing is allowed, and is read as «no session». */
    fun read(): ByteArray?

    fun write(bytes: ByteArray)

    fun wipe()
}
