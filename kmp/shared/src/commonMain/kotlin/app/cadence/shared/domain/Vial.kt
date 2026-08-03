package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * §03's `vials` — «Аптечка».
 *
 * There is no `status` here and no `remaining`, deliberately. §03's third
 * correction replaces the prototype's two disconnected vial datasets with one
 * table and derives the rest: `remaining = total_doses − count(dose_events
 * .vial_id)`, and status (sealed / active / low / expiring) is computed on
 * read. Adding either field back is the project's «nothing derived is stored»
 * rule being broken — see [remainingDoses] and [vialStatus].
 */
data class Vial(
    val id: VialId,
    val patientId: UserId,
    val compoundId: CompoundId,
    val concentrationLabel: String,
    val totalDoses: Int,
    val openedAt: LocalDate?,
    val expiresOn: LocalDate,
    val lot: String?,
    val locationRu: String?,
    val disposedAt: LocalDate?,
    val labelPhotoPath: String?,
)
