import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.android.multiplatform.library)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.openapi.generator)
}

// The client is generated from the contract the API commits, and only three directories of
// what the generator emits are kept: it writes a whole standalone Gradle project — wrapper,
// docs, a `src/test/kotlin` source set invalid for KMP, and a build with no androidTarget() —
// so it cannot be included as it stands.
val generatedClient = layout.buildDirectory.dir("generated-openapi")

// The rest of what the generator writes — its own build files, docs, and a `src/test/kotlin`
// source set KMP cannot compile — is left in the build directory.
//
// `auth` is here and the spec's list of three is short by it, measured: every `*Api` extends
// `ApiClient`, and `ApiClient` holds a map of the generator's own credential helpers. It comes
// along because it will not compile otherwise, and it stays unused — the token is attached by
// the transport, and `theGeneratedCredentialHelpersAreNotUsed` is what keeps that true.
val keptFromTheGenerator = listOf("apis", "models", "infrastructure", "auth")

val generatedInto = layout.projectDirectory.dir("src/commonMain/generated")

/**
 * Committed rather than generated at build time, so a clone builds without the generator and a
 * reader can see what the app is compiled against. `openApiDrift` is what keeps it honest.
 */
val copyGeneratedClient by tasks.registering(Sync::class) {
    dependsOn(tasks.named("openApiGenerate"))
    from(generatedClient.map { it.dir("src/commonMain/kotlin/app/cadence/shared/api") }) {
        include(keptFromTheGenerator.map { "$it/**" })
    }
    into(generatedInto.dir("app/cadence/shared/api"))
}

/**
 * Fails where regenerating changes what is committed.
 *
 * The contract is generated from the API's own code and the client from the contract, so a
 * drift here means one of the two moved without the other — and the compiler would not say so,
 * because both halves compile against whichever version is in the tree.
 *
 * It catches a contract that moved, not a file somebody edited: the copy runs first, so a hand
 * edit is reverted rather than reported. Measured both ways — adding a property to the spec
 * fails this task and names the file; appending a line to a generated file does not.
 */
val openApiDrift by tasks.registering {
    dependsOn(copyGeneratedClient)
    doLast {
        val changed =
            providers
                .exec {
                    commandLine("git", "status", "--porcelain", "--", generatedInto.asFile.path)
                }.standardOutput
                .asText
                .get()
                .trim()
        check(changed.isEmpty()) {
            "the generated client is out of date — regenerate it and commit the result:\n$changed"
        }
    }
}

openApiGenerate {
    generatorName.set("kotlin")
    library.set("multiplatform")
    inputSpec.set(rootProject.file("../api/openapi.json").absolutePath)
    outputDir.set(generatedClient.get().asFile.absolutePath)
    packageName.set("app.cadence.shared.api")
    configOptions.set(
        mapOf(
            "dateLibrary" to "kotlinx-datetime",
            "omitGradleWrapper" to "true",
        ),
    )
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

        withHostTestBuilder {}.configure {
            // Robolectric needs the merged manifest and resources; without this it fails at
            // startup rather than at the first Android call, which reads as a broken harness.
            isIncludeAndroidResources = true
        }
    }

    sourceSets {
        commonMain {
            kotlin.srcDir(generatedInto)
        }
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
            api(libs.ktor.client.core)
            implementation(libs.ktor.client.content.negotiation)
            implementation(libs.ktor.serialization.kotlinx.json)
        }
        getByName("androidHostTest").dependencies {
            implementation(libs.robolectric)
        }
        commonTest.dependencies {
            implementation(libs.kotlin.test)
            implementation(libs.ktor.client.mock)
            implementation(libs.kotlinx.coroutines.test)
            implementation(libs.multiplatform.settings.test)
        }
    }
}
