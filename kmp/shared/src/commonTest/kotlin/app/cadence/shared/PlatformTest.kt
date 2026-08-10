package app.cadence.shared

import kotlin.test.Test
import kotlin.test.assertTrue

class PlatformTest {
    @Test
    fun currentPlatformReportsANonBlankName() {
        assertTrue(currentPlatform().name.isNotBlank(), "platform name must identify the host")
    }
}
