import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.compose.compiler)
}

dependencies {
    // Declared rather than inherited: this module calls installSecureStorage directly, and it
    // compiles today only because :composeApp exports :shared for the iOS framework's header.
    // Undoing that export is an iOS decision, and it would break this build with nothing about
    // Android in the diff.
    implementation(project(":shared"))
    implementation(project(":composeApp"))
    implementation(libs.androidx.activity.compose)
    implementation(libs.compose.runtime)
}

android {
    namespace = "app.cadence.android"
    compileSdk = libs.versions.android.compileSdk.get().toInt()

    defaultConfig {
        applicationId = "app.cadence"
        minSdk = libs.versions.android.minSdk.get().toInt()
        targetSdk = libs.versions.android.targetSdk.get().toInt()
        versionCode = 1
        versionName = "0.1.0"
    }

    buildTypes {
        getByName("release") {
            isMinifyEnabled = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    compilerOptions {
        jvmTarget = JvmTarget.JVM_17
    }
}

// The release cannot be built against the dev address, on the artifact producers rather than
// on the lifecycle aliases: `assembleRelease` carried the dependency and `packageRelease` —
// the task that actually writes build/outputs/apk/release — did not, so the rule had a bypass
// one command wide. Measured with --dry-run on both.
tasks
    .matching {
        it.name in
            setOf(
                "assembleRelease",
                "bundleRelease",
                "packageRelease",
                "packageReleaseBundle",
                "signReleaseBundle",
            )
    }.configureEach {
        dependsOn(":shared:refuseDevAddressInRelease")
    }
