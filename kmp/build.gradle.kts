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
        source.setFrom(
            "src/commonMain/kotlin",
            "src/androidMain/kotlin",
            "src/iosMain/kotlin",
            "src/main/kotlin",
            "src/commonTest/kotlin",
            "src/androidHostTest/kotlin",
            "src/iosTest/kotlin",
        )
    }
}
