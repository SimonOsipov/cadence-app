package app.cadence.shared

/**
 * Platform describes the host the shared code runs on.
 *
 * It exists so that the expect/actual seam is exercised from the first commit:
 * the real per-platform work — health import, secure storage, notification
 * scheduling — arrives behind seams shaped exactly like this one.
 */
interface Platform {
    /** name identifies the host, e.g. "Android 36" or "iOS 26.5". */
    val name: String
}

/** currentPlatform returns the host this code was compiled for. */
expect fun currentPlatform(): Platform
