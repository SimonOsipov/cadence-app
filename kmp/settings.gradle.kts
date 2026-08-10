pluginManagement {
    repositories {
        google {
            content {
                includeGroupByRegex("com\\.android.*")
                includeGroupByRegex("com\\.google.*")
                includeGroupByRegex("androidx.*")
            }
        }
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositories {
        google {
            content {
                includeGroupByRegex("com\\.android.*")
                includeGroupByRegex("com\\.google.*")
                includeGroupByRegex("androidx.*")
            }
        }
        mavenCentral()
    }
}

rootProject.name = "cadence"

// shared     — models, network, logic; no UI
// composeApp — the Compose Multiplatform UI, shared by both platforms
// androidApp — thin Android host, the mirror image of iosApp/
//
// The Android application lives in its own module because since AGP 9 the
// `com.android.application` plugin cannot be combined with
// `org.jetbrains.kotlin.multiplatform`; a Compose Multiplatform module has to
// use `com.android.kotlin.multiplatform.library` instead. So the Android build
// entry point is `:androidApp:assembleDebug`, not `:composeApp:assembleDebug`.
include(":shared")
include(":composeApp")
include(":androidApp")
