package app.cadence.shared.storage

import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.ptr
import platform.CoreFoundation.CFDictionaryAddValue
import platform.CoreFoundation.CFDictionaryCreateMutable
import platform.CoreFoundation.CFRelease
import platform.CoreFoundation.kCFAllocatorDefault
import platform.CoreFoundation.kCFTypeDictionaryKeyCallBacks
import platform.CoreFoundation.kCFTypeDictionaryValueCallBacks
import platform.Foundation.CFBridgingRetain
import platform.Foundation.NSData
import platform.Security.SecItemAdd
import platform.Security.kSecAttrAccount
import platform.Security.kSecAttrService
import platform.Security.kSecClass
import platform.Security.kSecClassGenericPassword
import platform.Security.kSecValueData
import kotlin.test.Test
import kotlin.test.assertEquals

private const val ATTRIBUTE_COUNT = 4L

// errSecNotAvailable, written as a number because the constant is not in the Kotlin bindings.
private const val KEYCHAIN_UNAVAILABLE = -25291

/**
 * The spike the spec asked for, kept as its answer rather than written up as a note.
 *
 * **Measured 2026-08-29.** `SecItemAdd` from `iosSimulatorArm64Test` answers -25291,
 * `errSecNotAvailable`: a Kotlin/Native test binary is not an app bundle and has no keychain
 * to write to. The spec predicted -34018, `errSecMissingEntitlement` — the outcome it named
 * was right and the code it guessed was not.
 *
 * So [KeychainVault]'s behaviour cannot be measured from this source set at all, and the
 * suite that measures it belongs in an XCTest host under `iosApp/`. This test stands in the
 * meantime: it pins the reason, and it goes red the day this runtime gains a keychain —
 * which is the day those tests should move back.
 */
@OptIn(ExperimentalForeignApi::class)
class KeychainReachabilityTest {
    @Test
    fun theKeychainIsUnreachableFromThisTestTarget() {
        val item =
            checkNotNull(
                CFDictionaryCreateMutable(
                    kCFAllocatorDefault,
                    ATTRIBUTE_COUNT,
                    kCFTypeDictionaryKeyCallBacks.ptr,
                    kCFTypeDictionaryValueCallBacks.ptr,
                ),
            )
        CFDictionaryAddValue(item, kSecClass, kSecClassGenericPassword)
        val service = CFBridgingRetain("app.cadence.reachability")
        CFDictionaryAddValue(item, kSecAttrService, service)
        CFRelease(service)
        val account = CFBridgingRetain("probe")
        CFDictionaryAddValue(item, kSecAttrAccount, account)
        CFRelease(account)
        val data = CFBridgingRetain(NSData())
        CFDictionaryAddValue(item, kSecValueData, data)
        CFRelease(data)

        val status = SecItemAdd(item, null)
        CFRelease(item)

        assertEquals(KEYCHAIN_UNAVAILABLE, status, "the keychain answered $status rather than refusing")
    }
}
