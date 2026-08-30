import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.android.multiplatform.library)
    alias(libs.plugins.compose.multiplatform)
    alias(libs.plugins.compose.compiler)
}

@Suppress("UNCHECKED_CAST")
val swiftRuntimeFor = rootProject.extra["swiftRuntimeFor"] as (String) -> List<String>

kotlin {
    listOf(
        iosArm64() to "iphoneos",
        iosSimulatorArm64() to "iphonesimulator",
    ).forEach { (target, sdk) ->
        target.binaries.all {
            linkerOpts(swiftRuntimeFor(sdk))
        }
    }

    android {
        namespace = "app.cadence.debug"
        compileSdk = libs.versions.android.compileSdk.get().toInt()
        minSdk = libs.versions.android.minSdk.get().toInt()

        compilerOptions {
            jvmTarget = JvmTarget.JVM_17
        }

        withHostTestBuilder {}.configure {
            // Robolectric needs the merged manifest and resources; without this it fails at
            // startup rather than at the first Android call, which reads as a broken harness.
            isIncludeAndroidResources = true
        }
    }

    sourceSets {
        commonMain.dependencies {
            implementation(project(":shared"))
            implementation(libs.compose.runtime)
            implementation(libs.compose.foundation)
            implementation(libs.compose.ui)
        }
        androidMain.dependencies {
            implementation(libs.ktor.client.okhttp)
        }
        iosMain.dependencies {
            implementation(libs.ktor.client.darwin)
        }
        getByName("androidHostTest").dependencies {
            implementation(libs.robolectric)
        }
        commonTest.dependencies {
            implementation(libs.kotlin.test)
            implementation(libs.kotlinx.coroutines.test)
            implementation(libs.ktor.client.mock)
        }
    }
}
