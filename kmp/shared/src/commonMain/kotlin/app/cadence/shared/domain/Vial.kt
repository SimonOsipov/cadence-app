package app.cadence.shared.domain

import kotlinx.datetime.LocalDate

/**
 * No `status` and no `remaining`, deliberately: §03's third correction derives both on read
 * (see [remainingDoses] and [vialStatus]). Adding either back breaks «nothing derived is stored».
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
