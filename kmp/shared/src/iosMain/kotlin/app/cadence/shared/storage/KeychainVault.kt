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
 * Ours rather than the library's `KeychainSettings`, and that is a measured constraint
 * rather than a preference: its entire constructor is `KeychainSettings(serviceName)`, so
 * neither the accessibility class nor the synchronisation flag reaches it. Both matter here.
 * `AfterFirstUnlockThisDeviceOnly` keeps the item readable to a background token refresh
 * while refusing to leave the device; synchronizable = false keeps it out of iCloud Keychain,
 * where the patient's session would otherwise land on their other devices.
 *
 * Written as delete-then-add rather than update: `SecItemAdd` over an existing account
 * answers `errSecDuplicateItem`, and one branch is cheaper to keep right than two.
 */
@OptIn(ExperimentalForeignApi::class)
class KeychainVault(
    private val service: String,
) : Vault {
    override fun read(): ByteArray? =
        memScoped {
            val query = attributes()
            CFDictionaryAddValue(query, kSecReturnData, kCFBooleanTrue)
            val found = alloc<CFTypeRefVar>()
            val status = SecItemCopyMatching(query, found.ptr)
            CFRelease(query)

            if (status != errSecSuccess) return@memScoped null

            (CFBridgingRelease(found.value) as? NSData)?.toByteArray()
        }

    override fun write(bytes: ByteArray) {
        wipe()
        val item = attributes()
        val data = CFBridgingRetain(bytes.toNSData())
        CFDictionaryAddValue(item, kSecValueData, data)
        CFRelease(data)
        SecItemAdd(item, null)
        CFRelease(item)
    }

    override fun wipe() {
        val query = attributes()
        SecItemDelete(query)
        CFRelease(query)
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
