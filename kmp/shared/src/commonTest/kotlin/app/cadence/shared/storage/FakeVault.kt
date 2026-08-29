package app.cadence.shared.storage

/**
 * A vault holding bytes in memory, standing in for Keychain and Keystore alike — and able to
 * refuse, because refusing is the axis this package's decisions turn on.
 */
internal class FakeVault(
    var bytes: ByteArray? = null,
    private val unavailable: Boolean = false,
    private val writes: Boolean = true,
    private val wipes: Boolean = true,
) : Vault {
    override fun read(): Stored =
        when {
            unavailable -> Stored.Unavailable("the fixture says so")
            else -> bytes?.let { Stored.Present(it) } ?: Stored.Absent
        }

    override fun write(bytes: ByteArray): Boolean {
        if (!writes) return false
        this.bytes = bytes

        return true
    }

    override fun wipe(): Boolean {
        if (!wipes) return false
        bytes = null

        return true
    }
}

internal fun storeHolding(token: String) = FakeVault().also { VaultSettings(it).putString("refresh_token", token) }
