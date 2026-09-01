package app.cadence.shared.auth

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

private const val TOKEN = "e75b4d4f54a86c915c0afdfc5db3b5cb6eea78ba43c1ccf6bd24c5cb"

class InvitationLinkTest {
    @Test
    fun anInvitationCarriesItsToken() {
        assertEquals(TOKEN, invitationToken("cadence://accept?token_hash=$TOKEN"))
    }

    // Everything else the system may hand the app. A parser that answers a token for any of
    // these sends a stranger's string to /verify on launch.
    @Test
    fun nothingElseCarriesOne() {
        val notInvitations =
            listOf(
                "cadence://accept",
                "cadence://accept?token_hash=",
                "cadence://recover?token_hash=$TOKEN",
                "cadence://accept/../recover?token_hash=$TOKEN",
                "https://cadence.app/accept?token_hash=$TOKEN",
                "cadence://accept?other=$TOKEN",
                "",
            )

        for (link in notInvitations) {
            assertNull(invitationToken(link), "«$link» was read as an invitation")
        }
    }
}
