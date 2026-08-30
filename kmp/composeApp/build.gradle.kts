import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.android.multiplatform.library)
    alias(libs.plugins.compose.multiplatform)
    alias(libs.plugins.compose.compiler)
    alias(libs.plugins.kotlin.serialization)
}

kotlin {
    // The iOS app links this framework; iosApp/project.yml points its search
    // paths at the output of embedAndSignAppleFrameworkForXcode.
    listOf(
        iosArm64() to "iphoneos",
        iosSimulatorArm64() to "iphonesimulator",
    ).forEach { (iosTarget, sdk) ->
        @Suppress("UNCHECKED_CAST")
        val swiftRuntimeFor = rootProject.extra["swiftRuntimeFor"] as (String) -> List<String>
        iosTarget.binaries.all {
            linkerOpts(swiftRuntimeFor(sdk))
        }
        iosTarget.binaries.framework {
            baseName = "ComposeApp"
            isStatic = true
            // :shared is exported so its types reach the framework's Objective-C header,
            // which is the only way an XCTest bundle can call into them. The cost is real
            // and named: every public declaration in :shared becomes visible to Swift, not
            // just the one under test. It is exported rather than a second framework built
            // because two frameworks carrying one Kotlin runtime is the worse trade.
            export(project(":shared"))
        }
    }

    android {
        namespace = "app.cadence.ui"
        compileSdk = libs.versions.android.compileSdk.get().toInt()
        minSdk = libs.versions.android.minSdk.get().toInt()

        compilerOptions {
            jvmTarget = JvmTarget.JVM_17
        }
    }

    sourceSets {
        // The Apple half of what :androidApp does with debugImplementation. Apple has no
        // variants here, so the switch is a build property: absent, neither the module nor its
        // call site is on the compile path and the framework cannot carry the screen.
        //
        // Hung off iosMain and not commonMain, which is the whole point. A Gradle property is
        // global to the invocation, so on commonMain `-Pcadence.debugTools` put the screen —
        // and the sign-in wiring and the dev addresses behind it — into a **release** Android
        // APK: measured at five occurrences in the dex before this moved. Android's switch is
        // the variant, and this one now cannot reach it.
        //
        // The call site is the acceptance rather than a convenience: Kotlin/Native links what is
        // reachable, so a screen nothing calls is a screen no `strings` on the binary can find.
        if (providers.gradleProperty("cadence.debugTools").isPresent) {
            iosMain.get().kotlin.srcDir("src/debugToolsIosMain/kotlin")
            iosMain.dependencies {
                implementation(project(":debugTools"))
            }
        }
        commonMain.dependencies {
            // api rather than implementation: export above requires it.
            api(project(":shared"))
            implementation(libs.compose.runtime)
            implementation(libs.compose.foundation)
            implementation(libs.compose.ui)
            implementation(libs.compose.ui.backhandler)
            implementation(libs.compose.resources)
            implementation(libs.navigation.compose)
            implementation(libs.kotlinx.serialization.core)
        }
        // No host-test builder on the Android target on purpose: a Compose UI
        // test needs a real (or Robolectric) Android runtime, so these run on
        // the iOS simulator target only. The Android side gets its own Compose
        // UI tests on a device once there are screens to test.
        commonTest.dependencies {
            implementation(libs.kotlin.test)
            implementation(libs.compose.ui.test)
        }
    }
}

compose.resources {
    // Internal — the generated accessor is how the design system reaches its
    // own files, not part of the module's surface.
    publicResClass = false
    packageOfResClass = "app.cadence.design.generated"
}

// The iOS half of the same refusal. `API_BASE` lives in `:shared`, which this module exports
// into the framework, so a release framework would otherwise link with the dev address
// compiled in — measured, it did. Android's release tasks carry the same dependency in
// androidApp/build.gradle.kts; between them there is no release output that skips it.
tasks.matching { it.name.startsWith("link") && it.name.contains("Release") }.configureEach {
    dependsOn(":shared:refuseDevAddressInRelease")
}
