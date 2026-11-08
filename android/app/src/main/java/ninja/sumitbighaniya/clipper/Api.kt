package ninja.sumitbighaniya.clipper

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

object Config {
    // Live backend API VM: 92.4.70.213:8080 (or 10.0.2.2:8080 for Android emulator)
    const val BASE_URL = "http://92.4.70.213:8080"
    const val POLL_INTERVAL_MS = 2_000L
    const val POLL_TIMEOUT_MS = 15 * 60 * 1000L
}

// MARK: requests

@Serializable
private data class ResolveRequest(val url: String)

@Serializable
private data class CreateJobRequest(
    val url: String,
    @SerialName("segment_seconds") val segmentSeconds: Int = 0,
    val format: String? = null,
)

// MARK: responses

@Serializable
data class ResolveResponse(
    val title: String,
    val uploader: String? = null,
    val platform: String,
    @SerialName("duration_seconds") val durationSeconds: Double = 0.0,
    val thumbnail: String? = null,
    val variants: List<Variant> = emptyList(),
)

@Serializable
data class Variant(
    val selector: String,
    val label: String,
    val height: Int = 0,
    val ext: String = "mp4",
    @SerialName("approx_bytes") val approxBytes: Long? = null,
    val muxed: Boolean = false,
    val audio: Boolean = false,
)

@Serializable
data class JobRef(val id: String, val status: String)

@Serializable
data class Job(
    val id: String,
    val title: String? = null,
    val status: String,
    val error: String? = null,
    @SerialName("error_code") val errorCode: String? = null,
    val clips: List<Clip> = emptyList(),
) {
    val isTerminal get() = status == "completed" || status == "failed"
}

@Serializable
data class Clip(
    val index: Int,
    val url: String,
    @SerialName("size_bytes") val sizeBytes: Long = 0,
)

@Serializable
private data class ErrorBody(val error: String? = null, val code: String? = null)

class ApiException(override val message: String) : Exception(message) {
    companion object {
        // Mirror the backend's error taxonomy so the UI can say something true
        // instead of "error".
        fun from(code: String?, message: String?) = ApiException(
            when (code) {
                "unavailable" -> "This video is private, removed, or unavailable in your region."
                "blocked" -> "Couldn't reach this video right now. Try again in a minute."
                "extractor_stale" -> "This platform isn't working right now."
                else -> message ?: "Something went wrong."
            }
        )
    }
}

object Api {
    // explicitNulls = false omits null fields rather than sending "format": null.
    // The backend rejects unknown fields, so keeping the body minimal is safer.
    private val json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
    }
    private val jsonType = "application/json".toMediaType()

    private val client = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .build()

    suspend fun resolve(url: String): ResolveResponse =
        post("/v1/resolve", json.encodeToString(ResolveRequest(url)))

    suspend fun createJob(url: String, format: String?, segmentSeconds: Int = 0): JobRef =
        post("/v1/jobs", json.encodeToString(CreateJobRequest(url, segmentSeconds, format)))

    suspend fun job(id: String): Job = withContext(Dispatchers.IO) {
        val req = Request.Builder().url("${Config.BASE_URL}/v1/jobs/$id").build()
        execute<Job>(req)
    }

    /**
     * Polls until the job reaches a terminal state. The backend has no push
     * channel, so this is an interval poll with a hard ceiling — without the
     * ceiling a wedged job spins forever on the user's battery.
     */
    suspend fun waitForCompletion(id: String, onUpdate: (Job) -> Unit): Job {
        val deadline = System.currentTimeMillis() + Config.POLL_TIMEOUT_MS
        while (System.currentTimeMillis() < deadline) {
            val job = job(id)
            onUpdate(job)
            if (job.isTerminal) {
                if (job.status == "failed") throw ApiException.from(job.errorCode, job.error)
                return job
            }
            delay(Config.POLL_INTERVAL_MS)
        }
        throw ApiException("This is taking longer than expected. Check back later.")
    }

    private suspend inline fun <reified T> post(path: String, body: String): T =
        withContext(Dispatchers.IO) {
            val req = Request.Builder()
                .url("${Config.BASE_URL}$path")
                .post(body.toRequestBody(jsonType))
                .build()
            execute<T>(req)
        }

    private inline fun <reified T> execute(req: Request): T {
        client.newCall(req).execute().use { res ->
            val text = res.body?.string().orEmpty()
            if (!res.isSuccessful) {
                val err = runCatching { json.decodeFromString<ErrorBody>(text) }.getOrNull()
                throw ApiException.from(err?.code, err?.error)
            }
            return json.decodeFromString<T>(text)
        }
    }
}