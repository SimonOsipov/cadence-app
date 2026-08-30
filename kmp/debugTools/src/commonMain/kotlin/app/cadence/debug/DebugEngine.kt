package app.cadence.debug

import io.ktor.client.engine.HttpClientEngine

/**
 * The platform's HTTP engine, chosen here rather than resolved at runtime.
 *
 * Kotlin/Native has no service loader, so an engine that is only a dependency is an engine the
 * framework does not link — the failure lands on client construction and reads as a broken
 * module rather than a missing dependency.
 */
expect fun debugEngine(): HttpClientEngine
