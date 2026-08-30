package app.cadence.shared.storage

import com.russhwolf.settings.Settings

private const val LENGTH_END = ':'

/**
 * The whole store is one blob in a [Vault], so the platform protects a single object rather
 * than a key at a time — and a store that could not be read is empty here and **not
 * writable**, which is the asymmetry [Stored] exists to carry.
 *
 * Framing is length-prefixed rather than separated, because a separator is a character some
 * token is eventually allowed to contain — and the token that contains it would empty the
 * store rather than be stored.
 *
 * The function count detekt objects to is the [Settings] interface's: six types, four
 * accessors each.
 */
@Suppress("TooManyFunctions")
class VaultSettings(
    private val vault: Vault,
) : Settings {
    private val entries: MutableMap<String, String>

    private var writable: Boolean

    /**
     * False where the store could not be read, or where a write to it was refused.
     *
     * The running session still works — values put on this instance are answered by it — but
     * they are not persisted, so the patient signs in again next launch instead of losing the
     * session they had. It does not clear itself: an instance is cached for the life of the
     * process, so a store first reached before the device was unlocked stays unwritable until
     * the app is restarted. The consumer is expected to surface this; there is no logging in
     * this module to carry it, which is named as a gap rather than fixed here.
     */
    val isWritable: Boolean get() = writable

    init {
        when (val stored = vault.read()) {
            is Stored.Absent -> {
                entries = mutableMapOf()
                writable = true
            }

            is Stored.Present -> {
                entries = decode(stored.bytes)
                writable = true
            }

            is Stored.Unavailable -> {
                entries = mutableMapOf()
                writable = false
            }
        }
    }

    override val keys: Set<String> get() = entries.keys

    override val size: Int get() = entries.size

    // Reaches the vault whatever the store's state: clear() is sign-out, and a session left
    // at rest because the store could not be read is the opposite of what was asked for.
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

    /**
     * Removal that could not be written selectively wipes the whole store.
     *
     * `supabase-kt` signs out with `remove(key)`, and the alternative is to do nothing at rest —
     * leaving the session on the device after the patient asked for it to go. An instance that
     * cannot write cannot rewrite the blob without one, so total erasure is the only erasure it
     * has; erasing more than was asked is the safe direction, and keeping a session that was
     * signed out of is not.
     *
     * The fallback is on the write not happening rather than on the flag alone: a store that
     * was readable and whose write is then refused reaches the same place, and on Apple a
     * refusal that came from the delete leaves the old blob — the very session — in the
     * keychain.
     */
    override fun remove(key: String) {
        entries.remove(key)
        flush()
        if (!writable) vault.wipe()
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
        if (!writable) return

        val blob =
            buildString {
                for ((key, value) in entries) {
                    appendField(key)
                    appendField(value)
                }
            }
        // The answer is consumed rather than discarded: the Apple vault deletes before it
        // adds, so a refused add leaves the keychain empty while this map still answers with
        // the value. Unnoticed, the caller writes, reads back its own copy, and the session is
        // gone at rest until the next launch says so.
        if (!vault.write(blob.encodeToByteArray())) writable = false
    }
}

@Suppress("SwallowedException")
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
