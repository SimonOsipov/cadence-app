import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.android.multiplatform.library)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.openapi.generator)
}

// The client is generated from the contract the API commits, and four of the directories it
// emits are kept. It writes a whole standalone Gradle project — its own build files, docs, and
// a `src/test/kotlin` source set invalid for KMP — so it cannot be included as it stands.
val generatedClient = layout.buildDirectory.dir("generated-openapi")

// `auth` is here and the spec's list of three is short by it, measured: every `*Api` extends
// `ApiClient`, and `ApiClient` holds a map of the generator's own credential helpers. It comes
// along because it will not compile otherwise, and it stays unused — the token is attached by
// the transport, and `theGeneratedCredentialHelpersAreNotUsed` is what keeps that true.
val keptFromTheGenerator = listOf("apis", "models", "infrastructure", "auth")

// Pointed at the package rather than at `generated/`, so that ktlint's exemption and «written
// by the generator» name the same set: a file dropped beside this path compiles into commonMain
// and would otherwise be exempt from formatting too.
//
// detekt is a different story and the spec asks for it: its `source` list names seven `kotlin`
// directories and `generated/` is under none of them, so nothing here is analysed by it at any
// depth — measured. The residual is named rather than closed: a hand-written file under
// `src/commonMain/generated/` is formatted by ktlint and unseen by detekt.
val generatedInto = layout.projectDirectory.dir("src/commonMain/generated")
val generatedPackage = "app/cadence/shared/api"

/**
 * Committed rather than generated at build time, so a clone builds without the generator and a
 * reader can see what the app is compiled against. `openApiDrift` is what keeps it honest.
 */
val copyGeneratedClient by tasks.registering(Sync::class) {
    dependsOn(tasks.named("openApiGenerate"))
    from(generatedClient.map { it.dir("src/commonMain/kotlin/app/cadence/shared/api") }) {
        include(keptFromTheGenerator.map { "$it/**" })
    }
    into(generatedInto.dir(generatedPackage))
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

// The address the app talks to, and where it comes from. Modules on
// `com.android.kotlin.multiplatform.library` have neither `buildConfig` nor build variants, so
// there is nothing to read a constant out of — this is the named mechanism the spec asks for: a
// task writing one Kotlin file into a registered source directory.
val devApiBase = "http://localhost:8080"
val apiBase = providers.gradleProperty("cadence.apiBase").orElse(devApiBase)

val generateApiConfig by tasks.registering {
    val into = layout.buildDirectory.dir("generated-config")
    val base = apiBase
    outputs.dir(into)
    inputs.property("base", base)
    doLast {
        val file = into.get().file("app/cadence/shared/net/ApiConfig.kt").asFile
        file.parentFile.mkdirs()
        file.writeText(
            """
            package app.cadence.shared.net

            /** Written by the build. Set with `-Pcadence.apiBase=…`; the default is the dev contour. */
            const val API_BASE: String = "${base.get()}"

            """.trimIndent(),
        )
    }
}

/**
 * Refuses a release built against the dev address.
 *
 * An enforced rule rather than an intention, which is what the spec asks: it hangs off the
 * release outputs themselves, so there is no path to one that skips it.
 */
val refuseDevAddressInRelease by tasks.registering {
    val base = apiBase
    doLast {
        // The shape and not the literal: an exact comparison against one URL is defeated by a
        // trailing slash. What is refused is https missing, and a host naming this machine, a
        // container host or a private network rather than the product.
        //
        // Every address below was tried against this task rather than read off the regex, and
        // twice the reading was wrong: the case-sensitive first version admitted LOCALHOST, and
        // the second omitted 172.16/12 — Docker's own bridge — along with host.docker.internal
        // and api.localhost. 10.0.2.2 is in it because that is how an Android emulator reaches
        // its host.
        val address = base.get()
        val notTheProducts =
            Regex(
                """//([^@/]*@)?(""" +
                    """localhost|[^/:]*\.localhost|[^/:]*\.local|host\.docker\.internal|""" +
                    """127\.\d+\.\d+\.\d+|0\.0\.0\.0|\[::1]|169\.254\.\d+\.\d+|""" +
                    """10\.\d+\.\d+\.\d+|192\.168\.\d+\.\d+|""" +
                    """172\.(1[6-9]|2\d|3[01])\.\d+\.\d+""" +
                    """)(:|/|$)""",
                RegexOption.IGNORE_CASE,
            )
        check(address.startsWith("https://") && !notTheProducts.containsMatchIn(address)) {
            "a release cannot be built against $address — pass -Pcadence.apiBase=https://…"
        }
    }
}

/**
 * Fails where the generator's Ktor and ours are not the same version.
 *
 * They are both 3.5.1 today and that is a coincidence, not a guarantee: the generator pins Ktor
 * in the build file it writes, and the catalog pins ours, and neither knows about the other. Two
 * versions in one link are what the message from that day would not say.
 */
val ktorAlignment by tasks.registering {
    dependsOn(tasks.named("openApiGenerate"))
    val generatedBuildFile = generatedClient.map { it.file("build.gradle.kts") }
    val ours = libs.versions.ktor.get()
    doLast {
        val theirs =
            Regex("""val ktor_version = "([^"]+)"""")
                .find(generatedBuildFile.get().asFile.readText())
                ?.groupValues
                ?.get(1)
        check(theirs == ours) {
            "the generator builds against Ktor $theirs and the catalog pins $ours"
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
            kotlin.srcDir(generateApiConfig)
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
            implementation(libs.ktor.client.auth)
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
