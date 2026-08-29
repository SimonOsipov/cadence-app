package app.cadence.shared.storage

import com.russhwolf.settings.Settings

private const val LENGTH_END = ':'

/**
 * A [Settings] whose whole store is one blob in a [Vault], so the platform protects a single
 * object rather than a key at a time.
 *
 * A store that could not be read is empty here and **not writable**: see [isWritable]. That
 * asymmetry is the point of the class. «Unreadable storage is no session» is a statement
 * about reading, and without the second half it becomes one about erasing — the blob is
 * written back whole, so one write after one failed read replaces a live session with
 * nothing.
 *
 * Framing is length-prefixed rather than separated, because a separator is a character some
 * token is eventually allowed to contain — and the token that contains it would empty the
 * store rather than be stored.
 *
 * The function count detekt objects to is the [Settings] interface's, not a design here: six
 * types, four accessors each.
 */
@Suppress("TooManyFunctions")
class VaultSettings(
    private val vault: Vault,
) : Settings {
    private val entries: MutableMap<String, String>

    /**
     * False where the store could not be read, in which case nothing here reaches the vault.
     *
     * The running session still works — values put on this instance are answered by it — but
     * they are not persisted, so the patient signs in again next launch instead of losing
     * the session they had. The consumer is expected to surface this; there is no logging in
     * this module to carry it, which is named as a gap rather than fixed here.
     */
    val isWritable: Boolean

    init {
        when (val stored = vault.read()) {
            is Stored.Absent -> {
                entries = mutableMapOf()
                isWritable = true
            }

            is Stored.Present -> {
                // Bytes we wrote and cannot parse are our own corruption, and writing over
                // them is right: keeping them would leave the store unusable for good.
                entries = decode(stored.bytes)
                isWritable = true
            }

            is Stored.Unavailable -> {
                entries = mutableMapOf()
                isWritable = false
            }
        }
    }

    override val keys: Set<String> get() = entries.keys

    override val size: Int get() = entries.size

    override fun clear() {
        entries.clear()
        vault.wipe()
    }

    /**
     * Clearing, and whether it happened. Sign-out and the fresh-install guard both need the
     * answer: a wipe that failed and was taken for done leaves the next person on the device
     * holding this session.
     */
    fun clearAndConfirm(): Boolean {
        entries.clear()

        return vault.wipe()
    }

    override fun remove(key: String) {
        entries.remove(key)
        flush()
    }

    override fun hasKey(key: String): Boolean = entries.containsKey(key)

    override fun putInt(
        key: String,
        value: Int,
    ) = put(key, value.toString())

    override fun getInt(
        key: String,
        defaultValue: Int,
    ): Int = getIntOrNull(key) ?: defaultValue

    override fun getIntOrNull(key: String): Int? = entries[key]?.toIntOrNull()

    override fun putLong(
        key: String,
        value: Long,
    ) = put(key, value.toString())

    override fun getLong(
        key: String,
        defaultValue: Long,
    ): Long = getLongOrNull(key) ?: defaultValue

    override fun getLongOrNull(key: String): Long? = entries[key]?.toLongOrNull()

    override fun putString(
        key: String,
        value: String,
    ) = put(key, value)

    override fun getString(
        key: String,
        defaultValue: String,
    ): String = getStringOrNull(key) ?: defaultValue

    override fun getStringOrNull(key: String): String? = entries[key]

    override fun putFloat(
        key: String,
        value: Float,
    ) = put(key, value.toString())

    override fun getFloat(
        key: String,
        defaultValue: Float,
    ): Float = getFloatOrNull(key) ?: defaultValue

    override fun getFloatOrNull(key: String): Float? = entries[key]?.toFloatOrNull()

    override fun putDouble(
        key: String,
        value: Double,
    ) = put(key, value.toString())

    override fun getDouble(
        key: String,
        defaultValue: Double,
    ): Double = getDoubleOrNull(key) ?: defaultValue

    override fun getDoubleOrNull(key: String): Double? = entries[key]?.toDoubleOrNull()

    override fun putBoolean(
        key: String,
        value: Boolean,
    ) = put(key, value.toString())

    override fun getBoolean(
        key: String,
        defaultValue: Boolean,
    ): Boolean = getBooleanOrNull(key) ?: defaultValue

    override fun getBooleanOrNull(key: String): Boolean? = entries[key]?.toBooleanStrictOrNull()

    private fun put(
        key: String,
        value: String,
    ) {
        entries[key] = value
        flush()
    }

    private fun flush() {
        if (!isWritable) return

        val blob =
            buildString {
                for ((key, value) in entries) {
                    appendField(key)
                    appendField(value)
                }
            }
        vault.write(blob.encodeToByteArray())
    }
}

private fun decode(bytes: ByteArray): MutableMap<String, String> {
    val blob =
        try {
            bytes.decodeToString(throwOnInvalidSequence = true)
        } catch (expected: CharacterCodingException) {
            // Bytes that passed the platform's own integrity check and are still not text are
            // this module's own corruption. There is nowhere to report it from here.
            return mutableMapOf()
        }

    return decode(blob)
}

private fun StringBuilder.appendField(field: String) {
    append(field.length)
    append(LENGTH_END)
    append(field)
}

private fun decode(blob: String): MutableMap<String, String> {
    val entries = mutableMapOf<String, String>()
    var at = 0
    while (at < blob.length) {
        val key = readField(blob, at) ?: return mutableMapOf()
        val value = readField(blob, key.second) ?: return mutableMapOf()
        entries[key.first] = value.first
        at = value.second
    }

    return entries
}

/** The field and where the next one starts, or null where the blob is not one of ours. */
private fun readField(
    blob: String,
    from: Int,
): Pair<String, Int>? {
    val lengthEnd = blob.indexOf(LENGTH_END, from)
    val length = if (lengthEnd < 0) null else blob.substring(from, lengthEnd).toIntOrNull()
    if (length == null || length < 0) return null
    val start = lengthEnd + 1
    val end = start + length
    if (end > blob.length) return null

    return blob.substring(start, end) to end
}
