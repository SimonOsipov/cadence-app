import io.gitlab.arturbosch.detekt.extensions.DetektExtension
import org.jlleitschuh.gradle.ktlint.KtlintExtension

plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.android.multiplatform.library) apply false
    alias(libs.plugins.kotlin.multiplatform) apply false
    alias(libs.plugins.compose.multiplatform) apply false
    alias(libs.plugins.compose.compiler) apply false
    alias(libs.plugins.ktlint)
    alias(libs.plugins.detekt)
}

// Read from the catalog here, where its type-safe accessors exist; inside
// subprojects {} they do not.
val ktlintVersion = libs.versions.ktlint.tool.get()
val ktlintPluginId = libs.plugins.ktlint.get().pluginId
val detektPluginId = libs.plugins.detekt.get().pluginId
val detektConfig = file("config/detekt.yml")

// Where this machine's Swift runtime is, and nothing at all where there is no such machine.
//
// supabase-kt's PKCE hashing reaches CryptoKit through a Swift-interop cinterop library, and
// linking it wants libswiftCompatibility*.a. Kotlin/Native emits a search path of
// `/Applications/Xcode.app/…`, which is a default rather than an answer: on a host whose Xcode
// is installed under any other name the libraries are there and the linker cannot see them —
// measured here, where Xcode is `Xcode-26.6.0.app` and the link failed on
// `__swift_FORCE_LOAD_$_swiftCompatibility56`. Derived from xcode-select so it is right on every
// Apple host rather than on this one.
//
// Guarded on the host, and that guard was measured too: the Apple targets are declared
// unconditionally, so `binaries.all {}` configures them everywhere — and asking a Linux runner
// for `xcode-select` failed the configuration of `:shared:compileAndroidMain`, which is to say
// it would have taken the whole Linux half of the gate down while this machine stayed green.
// Apple targets cannot be linked off an Apple host anyway, so there is nothing to point at.
val onAnAppleHost = System.getProperty("os.name").orEmpty().startsWith("Mac")

val swiftRuntimeFor: (String) -> List<String> = { sdk ->
    if (!onAnAppleHost) {
        emptyList()
    } else {
        val developer =
            providers
                .exec { commandLine("xcode-select", "-p") }
                .standardOutput
                .asText
                .get()
                .trim()
        listOf("-L$developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/swift/$sdk")
    }
}
extra["swiftRuntimeFor"] = swiftRuntimeFor

// Style and static analysis are configured once and applied everywhere — a
// per-module copy is how the rules start to diverge. allprojects, not
// subprojects: the root build script and settings.gradle.kts are Kotlin too,
// and under subprojects ktlint would check them with the plugin's bundled
// version instead of the pinned one. (Detekt at the root analyses nothing
// either way — none of the source paths below exist at kmp/ — but keeping both
// plugins configured in one place is what stops them diverging later.)
allprojects {
    apply(plugin = ktlintPluginId)
    apply(plugin = detektPluginId)

    extensions.configure<KtlintExtension> {
        version.set(ktlintVersion)
        filter {
            exclude { it.file.path.contains("/build/") }
        }
    }

    extensions.configure<DetektExtension> {
        parallel = true
        buildUponDefaultConfig = true
        config.setFrom(detektConfig)
        // setFrom replaces detekt's defaults, so every source set is listed
        // explicitly — including the test ones. Test code is in scope: a test
        // is code the product depends on.
        // `src/commonMain/generated` is absent at every depth, which is what the spec asks
        // for — «its path allow-list does not contain `generated/`» — and the residual is
        // named in shared/build.gradle.kts rather than closed. detekt's list replaces its own
        // defaults, so anything unnamed is unanalysed — and this is the one place where that
        // silence is chosen rather than inherited: the file is written by a generator, this
        // project cannot act on a finding in it, and a red gate nobody can clear is worse than
        // a scope that says where it ends. What the client does is measured by the tests that
        // use it, not by static analysis of it.
        source.setFrom(
            "src/commonMain/kotlin",
            "src/androidMain/kotlin",
            "src/iosMain/kotlin",
            "src/main/kotlin",
            "src/commonTest/kotlin",
            "src/androidHostTest/kotlin",
            "src/iosTest/kotlin",
            // The two composition roots for the debug screen. Unlisted, they were unanalysed:
            // measured, :androidApp's report said «2 number of kt files» while src held three.
            // detekt ignores a listed path that does not exist, so naming them costs nothing in
            // the modules that have neither.
            "src/debug/kotlin",
            "src/debugToolsIosMain/kotlin",
        )
    }
}
