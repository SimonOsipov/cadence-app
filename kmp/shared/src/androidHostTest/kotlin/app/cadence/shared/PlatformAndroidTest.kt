package app.cadence.shared

import kotlin.test.Test
import kotlin.test.assertTrue

class PlatformAndroidTest {
    @Test
    fun currentPlatformIsAndroid() {
        val name = currentPlatform().name
        assertTrue(name.startsWith("Android"), "expected an Android platform, got \"$name\"")
    }
}
