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

// The release cannot be assembled against the dev address. Hung off the outputs rather than
// checked in a script; composeApp carries the same dependency on the iOS release links, and
// between the two there is no release output that goes around it.
tasks.matching { it.name == "assembleRelease" || it.name == "bundleRelease" }.configureEach {
    dependsOn(":shared:refuseDevAddressInRelease")
}
