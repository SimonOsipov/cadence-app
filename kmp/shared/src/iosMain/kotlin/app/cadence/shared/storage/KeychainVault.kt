package app.cadence.shared.storage

import kotlinx.cinterop.BetaInteropApi
import kotlinx.cinterop.CValuesRef
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.addressOf
import kotlinx.cinterop.alloc
import kotlinx.cinterop.convert
import kotlinx.cinterop.memScoped
import kotlinx.cinterop.ptr
import kotlinx.cinterop.usePinned
import kotlinx.cinterop.value
import platform.CoreFoundation.CFDictionaryAddValue
import platform.CoreFoundation.CFDictionaryCreateMutable
import platform.CoreFoundation.CFMutableDictionaryRef
import platform.CoreFoundation.CFRelease
import platform.CoreFoundation.CFTypeRefVar
import platform.CoreFoundation.kCFAllocatorDefault
import platform.CoreFoundation.kCFBooleanFalse
import platform.CoreFoundation.kCFBooleanTrue
import platform.CoreFoundation.kCFTypeDictionaryKeyCallBacks
import platform.CoreFoundation.kCFTypeDictionaryValueCallBacks
import platform.Foundation.CFBridgingRelease
import platform.Foundation.CFBridgingRetain
import platform.Foundation.NSData
import platform.Foundation.create
import platform.Security.SecItemAdd
import platform.Security.SecItemCopyMatching
import platform.Security.SecItemDelete
import platform.Security.errSecItemNotFound
import platform.Security.errSecSuccess
import platform.Security.kSecAttrAccessible
import platform.Security.kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
import platform.Security.kSecAttrAccount
import platform.Security.kSecAttrService
import platform.Security.kSecAttrSynchronizable
import platform.Security.kSecClass
import platform.Security.kSecClassGenericPassword
import platform.Security.kSecReturnData
import platform.Security.kSecValueData
import platform.posix.memcpy

private const val ACCOUNT = "store"
private const val ATTRIBUTE_COUNT = 6L

/**
 * The whole store in one generic-password item.
 *
 * Ours rather than the library's `KeychainSettings`, and the reason recorded here first was
 * wrong: that class does take the attributes, through a second `vararg defaultProperties`
 * constructor — measured in the 1.3.0 klib after review said so. Two reasons stand. It
 * carries `@ExperimentalSettingsApi` on top of the class's own
 * `@ExperimentalSettingsImplementation`, which is two experimental opt-ins on the
 * authentication path of a medical app; and it stores one keychain item per key, while the
 * store above is one blob — and the blob is what carries «read it or do not overwrite it».
 *
 * The attributes are the point of the item either way. `AfterFirstUnlockThisDeviceOnly` keeps
 * it readable to a background token refresh while refusing to leave the device;
 * synchronizable = false keeps it out of iCloud Keychain, where the patient's session would
 * otherwise land on their other devices.
 *
 * Written as delete-then-add rather than update: `SecItemAdd` over an existing account
 * answers `errSecDuplicateItem`, and one branch is cheaper to keep right than two.
 */
@OptIn(ExperimentalForeignApi::class)
class KeychainVault(
    private val service: String,
) : Vault {
    override fun read(): Stored =
        memScoped {
            val query = attributes()
            CFDictionaryAddValue(query, kSecReturnData, kCFBooleanTrue)
            val found = alloc<CFTypeRefVar>()
            val status = SecItemCopyMatching(query, found.ptr)
            CFRelease(query)

            when (status) {
                errSecSuccess -> {
                    (CFBridgingRelease(found.value) as? NSData)
                        ?.toByteArray()
                        ?.let { Stored.Present(it) }
                        ?: Stored.Unavailable("the keychain answered success without data")
                }

                errSecItemNotFound -> {
                    Stored.Absent
                }

                // Everything else is «there may be a session and it cannot be read now», and
                // the one that matters is -25308, errSecInteractionNotAllowed: the device has
                // not been unlocked since boot. That is precisely the window
                // AfterFirstUnlockThisDeviceOnly was chosen to survive, and the window a
                // background token refresh runs in.
                else -> {
                    Stored.Unavailable("the keychain answered $status")
                }
            }
        }

    override fun write(bytes: ByteArray): Boolean {
        if (!wipe()) return false

        val item = attributes()
        val data = CFBridgingRetain(bytes.toNSData())
        CFDictionaryAddValue(item, kSecValueData, data)
        CFRelease(data)
        val status = SecItemAdd(item, null)
        CFRelease(item)

        // Answered rather than discarded: this deletes before it adds, so a dropped failure
        // leaves the keychain empty while the caller's own copy still holds the session — the
        // app works until it is restarted and then signs the patient out for no visible
        // reason. errSecMissingEntitlement in a mis-signed release does exactly that on every
        // write.
        return status == errSecSuccess
    }

    override fun wipe(): Boolean {
        val query = attributes()
        val status = SecItemDelete(query)
        CFRelease(query)

        // Nothing to delete is a wipe that happened. Anything else is a store that may still
        // be there, and sign-out has to be able to tell: taken for done, it hands the next
        // person on the device this patient's session.
        return status == errSecSuccess || status == errSecItemNotFound
    }

    /**
     * What names this store and what keeps it on one device, on every call: the accessibility
     * class is part of the item's identity when it is added and part of the match when it is
     * read, so a query missing it would look for a different item than the one written.
     *
     * The caller releases what comes back.
     */
    private fun attributes(): CFMutableDictionaryRef {
        val attributes =
            checkNotNull(
                CFDictionaryCreateMutable(
                    kCFAllocatorDefault,
                    ATTRIBUTE_COUNT,
                    kCFTypeDictionaryKeyCallBacks.ptr,
                    kCFTypeDictionaryValueCallBacks.ptr,
                ),
            ) { "the allocator refused a dictionary of $ATTRIBUTE_COUNT" }
        CFDictionaryAddValue(attributes, kSecClass, kSecClassGenericPassword)
        attributes.addBridged(kSecAttrService, service)
        attributes.addBridged(kSecAttrAccount, ACCOUNT)
        CFDictionaryAddValue(attributes, kSecAttrAccessible, kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly)
        CFDictionaryAddValue(attributes, kSecAttrSynchronizable, kCFBooleanFalse)

        return attributes
    }
}

/** Bridges a Kotlin value in and hands the dictionary's own retain back, balancing ours. */
@OptIn(ExperimentalForeignApi::class)
private fun CFMutableDictionaryRef?.addBridged(
    key: CValuesRef<*>?,
    value: Any,
) {
    val bridged = CFBridgingRetain(value)
    CFDictionaryAddValue(this, key, bridged)
    CFRelease(bridged)
}

@OptIn(ExperimentalForeignApi::class, BetaInteropApi::class)
private fun ByteArray.toNSData(): NSData =
    if (isEmpty()) {
        NSData()
    } else {
        usePinned { NSData.create(bytes = it.addressOf(0), length = size.convert()) }
    }

@OptIn(ExperimentalForeignApi::class)
private fun NSData.toByteArray(): ByteArray =
    ByteArray(length.toInt()).also { out ->
        if (out.isNotEmpty()) out.usePinned { memcpy(it.addressOf(0), bytes, length) }
    }
