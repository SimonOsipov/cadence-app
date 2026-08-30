# KMP Secure Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `kmp/shared` a `com.russhwolf.settings.Settings` backed by platform secure storage — Keychain on Apple, a Keystore-encrypted file on Android — so the session and the PKCE verifier cache never sit in plaintext, plus the Android runtime test infrastructure that makes any of that checkable.

**Architecture:** Three layers with one seam each. `Vault` (expect/actual) owns bytes at rest and nothing else. `VaultSettings` (common) is the `Settings` implementation over a `Vault`: a string map, serialised to one blob, and unreadable bytes read as an empty map rather than a throw. `FreshInstallGuard` (common) wipes the persistent store when a marker held in volatile storage is missing, which is how an Apple keychain surviving app deletion stops handing the next installation somebody else's session. Everything decidable is decided in common code and tested there; the two `actual`s carry only the platform call.

**Tech Stack:** Kotlin Multiplatform 2.4.10 · `multiplatform-settings` 1.3.0 · AndroidKeyStore AES/GCM · Security.framework via cinterop · Robolectric for the Android runtime tests

**Spec:** `docs/specs/kmp-wiring.md` (vault master: `20-Projects/cadence/specs/kmp-wiring.md`), step 1. Steps 2 and 3 get their own plans.

## Global Constraints

- Session and PKCE cache in platform secure storage. iOS: `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false` — otherwise the session enters an encrypted backup and travels to a new device.
- Android: our own AES/GCM with a Keystore key over a file. `androidx.security:security-crypto` is deprecated with no direct replacement and may not be relied on.
- The Keychain survives app deletion: the first launch after a reinstall wipes the store via a marker.
- Unreadable storage — a changed lock screen, corruption, a reinstall — is «no session», never a crash.
- The gate enforces it: Android test infrastructure **with a runtime** exists and `scripts/gate/kmp.sh` runs it. New source sets are added to the detekt list in `kmp/build.gradle.kts`, which replaces detekt's defaults and therefore analyses only what it names.
- A spike answers whether the Keychain is reachable from `iosSimulatorArm64Test`. If it is not — `errSecMissingEntitlement` is the expected refusal — the test moves to an XCTest host in `iosApp/` and that is written down.
- Comments follow `~/.claude/CLAUDE.md`: one line, and only where the reader would otherwise get it wrong. Measurements and named weaknesses may run longer.
- **Measured 2026-08-29, and it shapes this plan:** `KeychainSettings(serviceName: String)` is the library's whole constructor — it accepts neither accessibility nor the synchronisation flag. The spec's own fallback therefore applies: the Apple `Settings` is ours, over `Security.framework`, so the two attributes are in the query dictionary we build. This is a deviation to record on step 1.

---

### Task 1: The storage policy, in common code

The half that needs no platform: a `Settings` over a byte vault, and the fresh-install wipe. Both are decisions, and decisions belong where they can be tested without a device.

**Files:**
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/Vault.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/VaultSettings.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/FreshInstallGuard.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/storage/VaultSettingsTest.kt`
- Test: `kmp/shared/src/commonTest/kotlin/app/cadence/shared/storage/FreshInstallGuardTest.kt`
- Modify: `kmp/gradle/libs.versions.toml` — add the settings library
- Modify: `kmp/shared/build.gradle.kts` — depend on it

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `interface Vault { fun read(): ByteArray?; fun write(bytes: ByteArray); fun wipe() }`
  - `class VaultSettings(private val vault: Vault) : Settings`
  - `fun guardFreshInstall(persistent: Settings, volatile: Settings)`
  - `const val INSTALL_MARKER_KEY: String`

- [ ] **Step 1: Add the dependency to the version catalog**

In `kmp/gradle/libs.versions.toml`, under `[versions]`:

```toml
# Not optional and not ours to choose: supabase-kt depends on it hard, and the
# session storage this project substitutes is its Settings parameter.
multiplatform-settings = "1.3.0"
```

Under `[libraries]`:

```toml
multiplatform-settings = { module = "com.russhwolf:multiplatform-settings", version.ref = "multiplatform-settings" }
multiplatform-settings-test = { module = "com.russhwolf:multiplatform-settings-test", version.ref = "multiplatform-settings" }
```

- [ ] **Step 2: Wire it into `:shared`**

In `kmp/shared/build.gradle.kts`, inside `sourceSets`:

```kotlin
commonMain.dependencies {
    api(libs.kotlinx.datetime)
    // api, not implementation: Settings is in this module's public signature —
    // secureSettings() answers one — so a consumer redeclaring the dependency
    // could drift to another version of the type it is handed.
    api(libs.multiplatform.settings)
}
commonTest.dependencies {
    implementation(libs.kotlin.test)
    implementation(libs.kotlinx.coroutines.test)
    implementation(libs.multiplatform.settings.test)
}
```

- [ ] **Step 3: Write the failing test for the vault-backed settings**

Create `kmp/shared/src/commonTest/kotlin/app/cadence/shared/storage/VaultSettingsTest.kt`:

```kotlin
package app.cadence.shared.storage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A vault that holds bytes in memory, standing in for Keychain and Keystore alike. */
private class FakeVault(
    var bytes: ByteArray? = null,
    private val readFails: Boolean = false,
) : Vault {
    var wiped = false

    override fun read(): ByteArray? = if (readFails) throw IllegalStateException("unreadable") else bytes

    override fun write(bytes: ByteArray) {
        this.bytes = bytes
    }

    override fun wipe() {
        bytes = null
        wiped = true
    }
}

class VaultSettingsTest {
    @Test
    fun aValueSurvivesANewInstanceOverTheSameVault() {
        val vault = FakeVault()
        VaultSettings(vault).putString("refresh_token", "rt-1")

        assertEquals("rt-1", VaultSettings(vault).getStringOrNull("refresh_token"))
    }

    // The whole reason this type exists rather than a plain map: the session is
    // the thing being stored, and a corrupted blob must read as «not signed in»
    // rather than take the app down on launch.
    @Test
    fun unreadableStorageIsNoSessionRatherThanACrash() {
        val settings = VaultSettings(FakeVault(readFails = true))

        assertNull(settings.getStringOrNull("refresh_token"))
        assertEquals(0, settings.size)
    }

    @Test
    fun bytesThatAreNotAStoreAreAlsoNoSession() {
        val settings = VaultSettings(FakeVault(bytes = byteArrayOf(7, 7, 7)))

        assertNull(settings.getStringOrNull("refresh_token"))
    }

    @Test
    fun everyTypeTheSessionManagerStoresSurvivesTheRoundTrip() {
        val vault = FakeVault()
        val written = VaultSettings(vault)
        written.putString("s", "с кириллицей")
        written.putLong("expires_at", 1_924_000_000L)
        written.putBoolean("b", true)

        val read = VaultSettings(vault)
        assertEquals("с кириллицей", read.getStringOrNull("s"))
        assertEquals(1_924_000_000L, read.getLongOrNull("expires_at"))
        assertEquals(true, read.getBooleanOrNull("b"))
    }

    @Test
    fun removingAndClearingReachTheVault() {
        val vault = FakeVault()
        val settings = VaultSettings(vault)
        settings.putString("a", "1")
        settings.putString("b", "2")

        settings.remove("a")
        assertFalse(settings.hasKey("a"))
        assertTrue(settings.hasKey("b"))

        settings.clear()
        assertEquals(0, VaultSettings(vault).size)
    }
}
```

- [ ] **Step 4: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*VaultSettingsTest*'`
Expected: FAIL — `Unresolved reference: Vault`.

- [ ] **Step 5: Write the vault seam**

Create `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/Vault.kt`:

```kotlin
package app.cadence.shared.storage

/**
 * Bytes at rest, and nothing else: the platform decides where they live and how they are
 * protected, and everything above this line is the same on both.
 */
interface Vault {
    /** Null where nothing was ever written. Throwing is allowed and read as «no session». */
    fun read(): ByteArray?

    fun write(bytes: ByteArray)

    fun wipe()
}
```

- [ ] **Step 6: Write the settings over it**

Create `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/VaultSettings.kt`:

```kotlin
package app.cadence.shared.storage

import com.russhwolf.settings.Settings

private const val RECORD_SEPARATOR = ''
private const val UNIT_SEPARATOR = ''

/**
 * A [Settings] whose whole store is one blob in a [Vault], so the platform protects one
 * object rather than a key at a time.
 *
 * Every read failure is «no session»: a lock screen the patient changed, a restore onto
 * another device, a half-written file. The alternative is a medical app that cannot open
 * because of a value it could simply have asked for again.
 */
class VaultSettings(private val vault: Vault) : Settings {
    private val entries: MutableMap<String, String> = load()

    private fun load(): MutableMap<String, String> {
        val bytes =
            try {
                vault.read()
            } catch (_: Throwable) {
                null
            } ?: return mutableMapOf()

        return try {
            bytes
                .decodeToString(throwOnInvalidSequence = true)
                .split(RECORD_SEPARATOR)
                .filter { it.isNotEmpty() }
                .associateTo(mutableMapOf()) { record ->
                    val at = record.indexOf(UNIT_SEPARATOR)
                    require(at > 0)
                    record.substring(0, at) to record.substring(at + 1)
                }
        } catch (_: Throwable) {
            mutableMapOf()
        }
    }

    private fun flush() {
        val blob = entries.entries.joinToString(RECORD_SEPARATOR.toString()) { "${it.key}$UNIT_SEPARATOR${it.value}" }
        vault.write(blob.encodeToByteArray())
    }

    override val keys: Set<String> get() = entries.keys
    override val size: Int get() = entries.size

    override fun clear() {
        entries.clear()
        vault.wipe()
    }

    override fun remove(key: String) {
        entries.remove(key)
        flush()
    }

    override fun hasKey(key: String): Boolean = entries.containsKey(key)

    private fun put(key: String, value: String) {
        entries[key] = value
        flush()
    }

    override fun putInt(key: String, value: Int) = put(key, value.toString())

    override fun getInt(key: String, defaultValue: Int): Int = getIntOrNull(key) ?: defaultValue

    override fun getIntOrNull(key: String): Int? = entries[key]?.toIntOrNull()

    override fun putLong(key: String, value: Long) = put(key, value.toString())

    override fun getLong(key: String, defaultValue: Long): Long = getLongOrNull(key) ?: defaultValue

    override fun getLongOrNull(key: String): Long? = entries[key]?.toLongOrNull()

    override fun putString(key: String, value: String) = put(key, value)

    override fun getString(key: String, defaultValue: String): String = getStringOrNull(key) ?: defaultValue

    override fun getStringOrNull(key: String): String? = entries[key]

    override fun putFloat(key: String, value: Float) = put(key, value.toString())

    override fun getFloat(key: String, defaultValue: Float): Float = getFloatOrNull(key) ?: defaultValue

    override fun getFloatOrNull(key: String): Float? = entries[key]?.toFloatOrNull()

    override fun putDouble(key: String, value: Double) = put(key, value.toString())

    override fun getDouble(key: String, defaultValue: Double): Double = getDoubleOrNull(key) ?: defaultValue

    override fun getDoubleOrNull(key: String): Double? = entries[key]?.toDoubleOrNull()

    override fun putBoolean(key: String, value: Boolean) = put(key, value.toString())

    override fun getBoolean(key: String, defaultValue: Boolean): Boolean = getBooleanOrNull(key) ?: defaultValue

    override fun getBooleanOrNull(key: String): Boolean? = entries[key]?.toBooleanStrictOrNull()
}
```

- [ ] **Step 7: Run the test and watch it pass**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*VaultSettingsTest*'`
Expected: PASS, five tests.

- [ ] **Step 8: Write the failing test for the fresh-install guard**

Create `kmp/shared/src/commonTest/kotlin/app/cadence/shared/storage/FreshInstallGuardTest.kt`:

```kotlin
package app.cadence.shared.storage

import com.russhwolf.settings.MapSettings
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class FreshInstallGuardTest {
    // The case the guard exists for: Apple's keychain outlives the app, so the next
    // installation — a different person on a shared device, or the same one after a
    // restore — would be handed a session it never signed into.
    @Test
    fun aStoreOutlivingItsInstallationIsWiped() {
        val persistent = MapSettings("refresh_token" to "rt-of-whoever-had-this-phone")
        val volatileStore = MapSettings()

        guardFreshInstall(persistent, volatileStore)

        assertNull(persistent.getStringOrNull("refresh_token"))
        assertTrue(volatileStore.getBoolean(INSTALL_MARKER_KEY, false))
    }

    @Test
    fun anOrdinaryLaunchKeepsTheSession() {
        val persistent = MapSettings("refresh_token" to "rt-1")
        val volatileStore = MapSettings(INSTALL_MARKER_KEY to true)

        guardFreshInstall(persistent, volatileStore)

        assertEquals("rt-1", persistent.getStringOrNull("refresh_token"))
    }

    // Idempotent, because it runs on every launch and the second run must not be a
    // sign-out: the marker written by the first is what the second reads.
    @Test
    fun runningTwiceSignsNobodyOut() {
        val persistent = MapSettings()
        val volatileStore = MapSettings()
        guardFreshInstall(persistent, volatileStore)
        persistent.putString("refresh_token", "rt-after-sign-in")

        guardFreshInstall(persistent, volatileStore)

        assertEquals("rt-after-sign-in", persistent.getStringOrNull("refresh_token"))
    }
}
```

- [ ] **Step 9: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*FreshInstallGuardTest*'`
Expected: FAIL — `Unresolved reference: guardFreshInstall`.

- [ ] **Step 10: Write the guard**

Create `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/FreshInstallGuard.kt`:

```kotlin
package app.cadence.shared.storage

import com.russhwolf.settings.Settings

/** Written into storage the platform clears on delete, which is what dates the installation. */
const val INSTALL_MARKER_KEY: String = "app.cadence.installed"

/**
 * Wipes the persistent store when the installation that filled it is gone.
 *
 * Apple's keychain survives app deletion by design, so without this the next installation
 * inherits the previous one's session. [volatileStore] is storage the platform does clear —
 * `NSUserDefaults`, `SharedPreferences` — and its marker is the only thing separating «a
 * fresh install» from «an ordinary launch».
 */
fun guardFreshInstall(persistent: Settings, volatileStore: Settings) {
    if (volatileStore.getBoolean(INSTALL_MARKER_KEY, false)) return

    persistent.clear()
    volatileStore.putBoolean(INSTALL_MARKER_KEY, true)
}
```

- [ ] **Step 11: Run both suites and watch them pass**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*storage*'`
Expected: PASS, eight tests.

- [ ] **Step 12: Run the style gate**

Run: `cd kmp && ./gradlew ktlintCheck detekt`
Expected: green. If detekt reports the new package unanalysed, that is Task 2's allow-list work — note it and continue.

- [ ] **Step 13: Commit**

```bash
git add kmp/gradle/libs.versions.toml kmp/shared/build.gradle.kts \
        kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage \
        kmp/shared/src/commonTest/kotlin/app/cadence/shared/storage
git commit -m "feat(kmp): the session store is a blob in a vault, and unreadable means signed out

The half of secure storage that needs no platform: a Settings over a byte
vault, and the wipe that stops an Apple keychain outliving its installation
from handing the next one somebody else's session. Both are decisions rather
than platform calls, so both are tested without a device."
```

---

### Task 2: The Android vault, and the runtime tests that can see it

**Files:**
- Create: `kmp/shared/src/androidMain/kotlin/app/cadence/shared/storage/AndroidVault.kt`
- Create: `kmp/shared/src/androidMain/kotlin/app/cadence/shared/storage/SecureSettings.android.kt`
- Create: `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/SecureSettings.kt`
- Test: `kmp/shared/src/androidHostTest/kotlin/app/cadence/shared/storage/AndroidVaultTest.kt`
- Modify: `kmp/gradle/libs.versions.toml` — Robolectric
- Modify: `kmp/shared/build.gradle.kts` — the Robolectric dependency and `isIncludeAndroidResources`
- Modify: `kmp/build.gradle.kts:45-53` — the detekt source list, if a new source set appears
- Modify: `scripts/gate/kmp.sh:26-28` — say what now runs there

**Interfaces:**
- Consumes: `Vault`, `VaultSettings` from Task 1.
- Produces: `expect fun secureSettings(context: PlatformContext): Settings`, `class AndroidVault(file: File) : Vault`.

- [ ] **Step 1: Measure whether Robolectric reaches AndroidKeyStore**

This decides the task's shape and is cheaper to run than to argue about. Add Robolectric to the catalog:

```toml
robolectric = "4.16"
```

```toml
robolectric = { module = "org.robolectric:robolectric", version.ref = "robolectric" }
```

and to `kmp/shared/build.gradle.kts`:

```kotlin
androidHostTest.dependencies {
    implementation(libs.robolectric)
}
```

with, inside the `android { }` block:

```kotlin
withHostTestBuilder {}.configure {
    // Robolectric needs the merged manifest and resources; without this it fails at
    // startup rather than at the first Android call, which reads as a broken harness.
    isIncludeAndroidResources = true
}
```

Then write the probe as a test — a spike that stays in the tree is a measurement, one that is deleted is a memory:

```kotlin
package app.cadence.shared.storage

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.security.KeyStore
import kotlin.test.assertNotNull

@RunWith(RobolectricTestRunner::class)
class AndroidKeyStoreReachabilityTest {
    // Measured rather than assumed: Robolectric ships no AndroidKeyStore provider on
    // every version, and the whole shape of this task depends on which is true here.
    // If this fails, the vault's binding moves to androidDeviceTest and this test is
    // rewritten to say so.
    @Test
    fun theAndroidKeyStoreProviderExists() {
        val keyStore = KeyStore.getInstance("AndroidKeyStore")
        keyStore.load(null)

        assertNotNull(keyStore)
    }
}
```

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*AndroidKeyStoreReachabilityTest*'`

**Branch on the result and write it down in the step's deviation either way.**
- PASS → continue with steps 2–8 as written.
- FAIL → the binding cannot be host-tested. Keep `AndroidVault` as designed, move `AndroidVaultTest` to a new `androidDeviceTest` source set, add that source set to the detekt list in `kmp/build.gradle.kts`, and have `scripts/gate/kmp.sh` run it only where an emulator exists — the gate must stay green on a machine without one, so guard it and say so in the echo. The rest of the steps are unchanged.

- [ ] **Step 2: Write the failing test for the Android vault**

Create `kmp/shared/src/androidHostTest/kotlin/app/cadence/shared/storage/AndroidVaultTest.kt`:

```kotlin
package app.cadence.shared.storage

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import java.io.File
import kotlin.test.assertContentEquals
import kotlin.test.assertNotEquals
import kotlin.test.assertNull

@RunWith(RobolectricTestRunner::class)
class AndroidVaultTest {
    private fun aFile(): File = File.createTempFile("vault", ".bin").apply { delete() }

    @Test
    fun whatIsWrittenComesBack() {
        val file = aFile()
        AndroidVault(file).write("rt-1".encodeToByteArray())

        assertContentEquals("rt-1".encodeToByteArray(), AndroidVault(file).read())
    }

    // The point of the whole class: the bytes on disk are not the bytes handed in.
    // Without this the file could be a plaintext store and every other test would pass.
    @Test
    fun whatIsOnDiskIsNotWhatWasHandedIn() {
        val file = aFile()
        val secret = "refresh-token-in-the-clear".encodeToByteArray()

        AndroidVault(file).write(secret)

        assertNotEquals(secret.toList(), file.readBytes().toList())
    }

    @Test
    fun nothingWrittenReadsAsNothing() {
        assertNull(AndroidVault(aFile()).read())
    }

    @Test
    fun aTamperedFileReadsAsNothingRatherThanThrowing() {
        val file = aFile()
        AndroidVault(file).write("rt-1".encodeToByteArray())
        file.writeBytes(file.readBytes().also { it[it.size - 1] = (it[it.size - 1] + 1).toByte() })

        assertNull(AndroidVault(file).read())
    }

    @Test
    fun wipingLeavesNothingBehind() {
        val file = aFile()
        val vault = AndroidVault(file)
        vault.write("rt-1".encodeToByteArray())

        vault.wipe()

        assertNull(vault.read())
    }
}
```

- [ ] **Step 3: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*AndroidVaultTest*'`
Expected: FAIL — `Unresolved reference: AndroidVault`.

- [ ] **Step 4: Write the Android vault**

Create `kmp/shared/src/androidMain/kotlin/app/cadence/shared/storage/AndroidVault.kt`:

```kotlin
package app.cadence.shared.storage

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val KEY_ALIAS = "app.cadence.vault"
private const val PROVIDER = "AndroidKeyStore"
private const val TRANSFORMATION = "AES/GCM/NoPadding"
private const val IV_BYTES = 12
private const val TAG_BITS = 128

/**
 * AES/GCM over a file, with the key in the Keystore and never in the process.
 *
 * Ours rather than `androidx.security:security-crypto`, which is deprecated with no direct
 * replacement — depending on it would mean depending on something with no upgrade path on
 * the authentication path of a medical app.
 *
 * The IV is generated by the cipher and stored ahead of the ciphertext: GCM with a reused
 * IV under one key is a key recovery, and letting the provider choose is what guarantees
 * it is not reused.
 */
class AndroidVault(private val file: File) : Vault {
    override fun read(): ByteArray? {
        if (!file.exists()) return null

        return try {
            val stored = file.readBytes()
            if (stored.size <= IV_BYTES) return null
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(
                Cipher.DECRYPT_MODE,
                key(),
                GCMParameterSpec(TAG_BITS, stored, 0, IV_BYTES),
            )
            cipher.doFinal(stored, IV_BYTES, stored.size - IV_BYTES)
        } catch (_: Exception) {
            // A changed lock screen invalidates the key, a restore brings a file no key
            // here can open, and a half-written file fails the tag. All three are «no
            // session», which is what the caller can act on.
            null
        }
    }

    override fun write(bytes: ByteArray) {
        val cipher = Cipher.getInstance(TRANSFORMATION).apply { init(Cipher.ENCRYPT_MODE, key()) }
        file.parentFile?.mkdirs()
        file.writeBytes(cipher.iv + cipher.doFinal(bytes))
    }

    override fun wipe() {
        file.delete()
    }

    private fun key(): SecretKey {
        val keyStore = KeyStore.getInstance(PROVIDER).apply { load(null) }
        (keyStore.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }

        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, PROVIDER).apply {
            init(
                KeyGenParameterSpec
                    .Builder(
                        KEY_ALIAS,
                        KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
                    ).setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .build(),
            )
        }.generateKey()
    }
}
```

- [ ] **Step 5: Run the test and watch it pass**

Run: `cd kmp && ./gradlew :shared:testAndroidHostTest --tests '*AndroidVaultTest*'`
Expected: PASS, five tests.

- [ ] **Step 6: Declare the platform seam**

Create `kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/SecureSettings.kt`:

```kotlin
package app.cadence.shared.storage

import com.russhwolf.settings.Settings

/**
 * Where the session and the PKCE verifier live. The stock managers of `supabase-kt` keep
 * both in plaintext on both platforms — `SharedPreferences` and `NSUserDefaults` — so this
 * is the Settings they are handed instead.
 */
expect fun secureSettings(): Settings
```

Create `kmp/shared/src/androidMain/kotlin/app/cadence/shared/storage/SecureSettings.android.kt`:

```kotlin
package app.cadence.shared.storage

import com.russhwolf.settings.Settings

/**
 * The file sits in the app's own storage, which the system removes on uninstall — so
 * Android needs no fresh-install guard, and the one in common code is for Apple's keychain.
 */
actual fun secureSettings(): Settings = VaultSettings(AndroidVault(vaultFile()))
```

Add `vaultFile()` beside it, taking the directory from whatever context holder the module already has; if none exists, declare `lateinit var vaultDirectory: File` initialised from `:androidApp`'s `Application.onCreate`, and say in one line why a context holder rather than a parameter: the `expect` has no place to carry one.

- [ ] **Step 7: Bring the new source sets under detekt and the gate**

In `kmp/build.gradle.kts`, the `source.setFrom(...)` list replaces detekt's defaults, so anything unnamed is silently unanalysed. Confirm `src/androidMain/kotlin` and `src/androidHostTest/kotlin` are present — they are — and add `src/androidDeviceTest/kotlin` only if step 1 sent the binding there.

In `scripts/gate/kmp.sh`, the echo above `testAndroidHostTest` counts the tests it runs. Update the count and add one line naming what now runs with a runtime.

- [ ] **Step 8: Run the whole gate**

Run: `scripts/gate/kmp.sh`
Expected: green, with the new tests in the count.

- [ ] **Step 9: Commit**

```bash
git add kmp/gradle/libs.versions.toml kmp/shared/build.gradle.kts kmp/build.gradle.kts \
        kmp/shared/src/androidMain kmp/shared/src/androidHostTest \
        kmp/shared/src/commonMain/kotlin/app/cadence/shared/storage/SecureSettings.kt \
        scripts/gate/kmp.sh
git commit -m "feat(kmp): the Android vault is AES/GCM under a Keystore key

Ours rather than androidx.security:security-crypto, which is deprecated with
no direct replacement. The IV comes from the provider and is stored ahead of
the ciphertext, so it cannot be reused under one key.

The runtime the tests needed arrives with them: Robolectric under
:shared:testAndroidHostTest, and the gate runs it."
```

---

### Task 3: The Apple vault, its two attributes, and the simulator spike

**Files:**
- Create: `kmp/shared/src/iosMain/kotlin/app/cadence/shared/storage/KeychainVault.kt`
- Create: `kmp/shared/src/iosMain/kotlin/app/cadence/shared/storage/SecureSettings.ios.kt`
- Test: `kmp/shared/src/iosTest/kotlin/app/cadence/shared/storage/KeychainVaultTest.kt`

**Interfaces:**
- Consumes: `Vault`, `VaultSettings`, `guardFreshInstall`, `secureSettings` from Tasks 1–2.
- Produces: `class KeychainVault(service: String) : Vault`.

- [ ] **Step 1: Write the failing spike-and-behaviour test**

The spike and the behaviour are one test file on purpose: whether the Keychain answers at all from `iosSimulatorArm64Test` is the first assertion, and everything after it is the behaviour that assertion makes measurable.

Create `kmp/shared/src/iosTest/kotlin/app/cadence/shared/storage/KeychainVaultTest.kt`:

```kotlin
package app.cadence.shared.storage

import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.assertContentEquals
import kotlin.test.assertNull

private const val SERVICE = "app.cadence.test-vault"

class KeychainVaultTest {
    private val vault = KeychainVault(SERVICE)

    @AfterTest
    fun clean() = vault.wipe()

    // The spike, kept as a test: if the Keychain is unreachable from the simulator test
    // target the refusal is errSecMissingEntitlement, and this fails first and loudest —
    // at which point the suite moves to an XCTest host in iosApp/ and this comment says so.
    @Test
    fun theKeychainAnswersFromTheSimulatorTestTarget() {
        vault.write("rt-1".encodeToByteArray())

        assertContentEquals("rt-1".encodeToByteArray(), vault.read())
    }

    @Test
    fun nothingWrittenReadsAsNothing() {
        assertNull(KeychainVault("$SERVICE.empty").read())
    }

    @Test
    fun writingTwiceUpdatesRatherThanDuplicates() {
        vault.write("rt-1".encodeToByteArray())
        vault.write("rt-2".encodeToByteArray())

        assertContentEquals("rt-2".encodeToByteArray(), vault.read())
    }

    @Test
    fun wipingLeavesNothingBehind() {
        vault.write("rt-1".encodeToByteArray())

        vault.wipe()

        assertNull(vault.read())
    }
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd kmp && ./gradlew :shared:iosSimulatorArm64Test --tests '*KeychainVaultTest*'`
Expected: FAIL — `Unresolved reference: KeychainVault`.

- [ ] **Step 3: Write the Keychain vault**

Create `kmp/shared/src/iosMain/kotlin/app/cadence/shared/storage/KeychainVault.kt`:

```kotlin
package app.cadence.shared.storage

import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.alloc
import kotlinx.cinterop.memScoped
import kotlinx.cinterop.ptr
import kotlinx.cinterop.value
import platform.CoreFoundation.CFDictionaryCreateMutable
import platform.CoreFoundation.CFDictionarySetValue
import platform.CoreFoundation.CFTypeRefVar
import platform.CoreFoundation.kCFBooleanFalse
import platform.CoreFoundation.kCFBooleanTrue
import platform.Foundation.CFBridgingRelease
import platform.Foundation.CFBridgingRetain
import platform.Foundation.NSData
import platform.Security.SecItemAdd
import platform.Security.SecItemCopyMatching
import platform.Security.SecItemDelete
import platform.Security.errSecSuccess
import platform.Security.kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
import platform.Security.kSecAttrAccessible
import platform.Security.kSecAttrAccount
import platform.Security.kSecAttrService
import platform.Security.kSecAttrSynchronizable
import platform.Security.kSecClass
import platform.Security.kSecClassGenericPassword
import platform.Security.kSecReturnData
import platform.Security.kSecValueData

private const val ACCOUNT = "session"

/**
 * The store, in one generic-password item.
 *
 * Ours rather than the library's `KeychainSettings`, and that is a measured constraint
 * rather than a preference: its whole constructor is `KeychainSettings(serviceName)` — the
 * accessibility class and the synchronisation flag do not reach it. Both matter here.
 * `AfterFirstUnlockThisDeviceOnly` keeps the item readable to a background refresh while
 * refusing to leave the device; `kSecAttrSynchronizable = false` keeps it out of iCloud
 * Keychain, where the patient's session would land on their other devices.
 */
@OptIn(ExperimentalForeignApi::class)
class KeychainVault(private val service: String) : Vault {
    override fun read(): ByteArray? =
        memScoped {
            val query = query()
            CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue)
            val found = alloc<CFTypeRefVar>()
            val status = SecItemCopyMatching(query, found.ptr)
            if (status != errSecSuccess) return null

            (CFBridgingRelease(found.value) as? NSData)?.toByteArray()
        }

    override fun write(bytes: ByteArray) {
        wipe()
        val item = query()
        CFDictionarySetValue(item, kSecValueData, CFBridgingRetain(bytes.toNSData()))
        SecItemAdd(item, null)
    }

    override fun wipe() {
        SecItemDelete(query())
    }

    private fun query() =
        CFDictionaryCreateMutable(null, 0, null, null).also {
            CFDictionarySetValue(it, kSecClass, kSecClassGenericPassword)
            CFDictionarySetValue(it, kSecAttrService, CFBridgingRetain(service))
            CFDictionarySetValue(it, kSecAttrAccount, CFBridgingRetain(ACCOUNT))
            CFDictionarySetValue(it, kSecAttrAccessible, kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly)
            CFDictionarySetValue(it, kSecAttrSynchronizable, kCFBooleanFalse)
        }
}
```

Add `toByteArray()` / `toNSData()` beside it as private helpers over `NSData.bytes` and `NSData.create(bytes:length:)`; they are two lines each and belong with their only caller.

- [ ] **Step 4: Run the test and watch it pass — or record the refusal**

Run: `cd kmp && ./gradlew :shared:iosSimulatorArm64Test --tests '*KeychainVaultTest*'`

- PASS → the spike's answer is «reachable», and the step's deviation says so with the date.
- FAIL with `errSecMissingEntitlement` → the spike's answer is «not reachable». Move this file to an XCTest host under `iosApp/`, keep the assertions identical, and record both the refusal and the move. This is the outcome the spec named in advance; it is not a defect.

- [ ] **Step 5: Write the Apple side of the seam**

Create `kmp/shared/src/iosMain/kotlin/app/cadence/shared/storage/SecureSettings.ios.kt`:

```kotlin
package app.cadence.shared.storage

import com.russhwolf.settings.NSUserDefaultsSettings
import com.russhwolf.settings.Settings
import platform.Foundation.NSUserDefaults

private const val SERVICE = "app.cadence.session"

/**
 * The keychain outlives the app, so the guard runs before the store is handed out: the
 * marker lives in NSUserDefaults, which deletion does clear.
 */
actual fun secureSettings(): Settings {
    val persistent = VaultSettings(KeychainVault(SERVICE))
    guardFreshInstall(persistent, NSUserDefaultsSettings(NSUserDefaults.standardUserDefaults))

    return persistent
}
```

- [ ] **Step 6: Run the iOS gate**

Run: `scripts/gate/ios.sh`
Expected: green.

- [ ] **Step 7: Run both gates**

Run: `scripts/gate/kmp.sh && scripts/gate/ios.sh`
Expected: green.

- [ ] **Step 8: Commit**

```bash
git add kmp/shared/src/iosMain kmp/shared/src/iosTest
git commit -m "feat(kmp): the Apple vault carries the two attributes that keep a session on one device

Ours rather than the library's KeychainSettings, measured rather than
preferred: its whole constructor is KeychainSettings(serviceName), so neither
the accessibility class nor the synchronisation flag reaches it. Both matter —
AfterFirstUnlockThisDeviceOnly keeps the item readable to a background refresh
while refusing to leave the device, and synchronizable=false keeps the
patient's session out of iCloud Keychain.

The spike the spec asked for is a test rather than a note: whether the Keychain
answers from iosSimulatorArm64Test is its first assertion."
```

---

## Self-Review

**Spec coverage for step 1.** Keychain with the two attributes — Task 3. Android encryption with a Keystore key — Task 2. Keychain survives deletion, marker wipe — Task 1 (policy) and Task 3 (wiring). Unreadable storage is «no session» — Task 1, and again per-platform in Tasks 2 and 3. Android test infrastructure with a runtime, run by the gate — Task 2. Detekt allow-list — Task 2, step 7. The Keychain spike with the XCTest fallback — Task 3, steps 1 and 4.

**Deviations this plan already owes the spec** (record them on the step, per `/implement`): the Apple `Settings` is ours rather than `KeychainSettings`, because the library's constructor takes no attributes — measured 2026-08-29 against the library's own README and reference; and the PKCE verifier cache is *storage* here and gains its consumer in step 3, so this step ships the store without the mechanism that reads it.

**Not covered here, deliberately:** `supabase-kt` itself. The spec names it as the single owner of token refresh, and ADR-008 replaced the vendor a day after the spec was written. That question belongs to step 2, is decided by a measurement against the local GoTrue contour, and does not touch a byte of this plan.
