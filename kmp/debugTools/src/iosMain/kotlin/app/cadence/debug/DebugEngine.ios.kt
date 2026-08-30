package app.cadence.debug

import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.darwin.Darwin

actual fun debugEngine(): HttpClientEngine = Darwin.create()
