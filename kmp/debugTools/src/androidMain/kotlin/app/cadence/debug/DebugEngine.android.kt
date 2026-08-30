package app.cadence.debug

import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.okhttp.OkHttp

actual fun debugEngine(): HttpClientEngine = OkHttp.create()
