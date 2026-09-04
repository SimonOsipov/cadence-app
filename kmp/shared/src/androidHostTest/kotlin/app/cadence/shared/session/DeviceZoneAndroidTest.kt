package app.cadence.shared.session

import java.util.TimeZone
import kotlin.test.AfterTest
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * The default nothing in `commonTest` exercises: every test there passes a zone, and the app passes
 * none. Here because the device's zone can be substituted only on a JVM host.
 *
 * A shape assertion — «the answer is some name the server knows» — was measured not to hold this:
 * `"UTC"` is a name the server knows, so an app that always reported UTC passed it. What is pinned
 * instead is that the answer follows the device.
 */
class DeviceZoneAndroidTest {
    private val theRunnersOwn: TimeZone = TimeZone.getDefault()

    @AfterTest
    fun putTheRunnersZoneBack() {
        TimeZone.setDefault(theRunnersOwn)
    }

    @Test
    fun theZoneFollowsTheDevice() {
        TimeZone.setDefault(TimeZone.getTimeZone("Asia/Tbilisi"))

        assertEquals("Asia/Tbilisi", deviceZone())
    }

    // Two, because one is satisfied by any constant that happens to match it.
    @Test
    fun aDeviceMovedIsReportedMoved() {
        TimeZone.setDefault(TimeZone.getTimeZone("Europe/Moscow"))

        assertEquals("Europe/Moscow", deviceZone())
    }
}
