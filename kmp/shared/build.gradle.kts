import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.android.multiplatform.library)
}

kotlin {
    iosArm64()
    iosSimulatorArm64()

    android {
        namespace = "app.cadence.shared"
        compileSdk = libs.versions.android.compileSdk.get().toInt()
        minSdk = libs.versions.android.minSdk.get().toInt()

        compilerOptions {
            jvmTarget = JvmTarget.JVM_17
        }

        withHostTestBuilder {}.configure {}
    }

    sourceSets {
        commonMain.dependencies {
            // api, not implementation: LocalDate and TimeZone are in this
            // module's public signature — Occurrence.date, Vial.expiresOn,
            // ScheduleRepository.month, CadenceClock.today — so a consumer that
            // had to redeclare the dependency could drift to another version of
            // the types it is already being handed. (Instant is not among them:
            // kotlinx-datetime 0.7 moved it to the standard library.)
            api(libs.kotlinx.datetime)
            // api for the same reason: Settings is in this module's public signature —
            // secureSettings() answers one — so a consumer redeclaring the dependency
            // could drift to another version of the type it is handed.
            api(libs.multiplatform.settings)
        }
        commonTest.dependencies {
            implementation(libs.kotlin.test)
            implementation(libs.kotlinx.coroutines.test)
            implementation(libs.multiplatform.settings.test)
        }
    }
}
