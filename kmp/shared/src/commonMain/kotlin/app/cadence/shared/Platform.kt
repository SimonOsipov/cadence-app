package app.cadence.shared

/**
 * Exists so the expect/actual seam is exercised from the first commit: the real
 * per-platform work — health import, secure storage, notification scheduling — arrives
 * behind seams shaped exactly like this one.
 */
interface Platform {
    /** e.g. "Android 36" or "iOS 26.5". */
    val name: String
}

expect fun currentPlatform(): Platform
