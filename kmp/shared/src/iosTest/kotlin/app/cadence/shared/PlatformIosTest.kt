package app.cadence.shared

import kotlin.test.Test
import kotlin.test.assertTrue

class PlatformIosTest {
    @Test
    fun currentPlatformIsIos() {
        val name = currentPlatform().name
        assertTrue(name.startsWith("iOS"), "expected an iOS platform, got \"$name\"")
    }
}
