package app.cadence.shared.repository

import app.cadence.shared.domain.DoseEventId
import app.cadence.shared.domain.InjectionSite
import app.cadence.shared.domain.ProtocolItemId

/**
 * §11: `POST /me/dose-events`.
 *
 * The one write this step carries, and it is here because a seam nothing can
 * write through cannot be shown to work: the test that proves a mock swap is
 * invisible to a screen has to change something and read it back.
 *
 * The full wizard payload — mood, side effects, photo, the journal entry §03
 * says the same action writes — arrives with the wizard in step 5 of the
 * block. This is deliberately the narrow version.
 */
interface DoseLogRepository {
    suspend fun logDose(
        itemId: ProtocolItemId,
        site: InjectionSite?,
    ): DoseEventId
}
