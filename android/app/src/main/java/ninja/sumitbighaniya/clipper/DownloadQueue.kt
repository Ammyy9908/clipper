package ninja.sumitbighaniya.clipper

import android.content.Context
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.File
import java.io.FileOutputStream
import java.util.UUID

sealed interface JobStatus {
    data object Queued : JobStatus
    data class Working(val stage: String) : JobStatus
    data class Saving(val progress: Float) : JobStatus
    data object Done : JobStatus
    data class Failed(val message: String) : JobStatus
}

data class DownloadJob(
    val id: String,
    val title: String,
    val thumbnail: String?,
    val isAudio: Boolean,
    val isSplit: Boolean,
    val status: JobStatus = JobStatus.Queued
)

class DownloadQueue private constructor(private val context: Context) {

    private val libraryStore = LibraryStore.getInstance(context)
    private val mediaStoreSaver = MediaStoreSaver(context)
    private val httpClient = OkHttpClient()
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    private val _jobs = MutableStateFlow<List<DownloadJob>>(emptyList())
    val jobs: StateFlow<List<DownloadJob>> = _jobs.asStateFlow()

    fun enqueue(
        info: ResolvedInfo,
        variant: Variant,
        splitSeconds: Int?
    ) {
        val jobId = UUID.randomUUID().toString()
        val isAudio = variant.audio
        val isSplit = splitSeconds != null

        val job = DownloadJob(
            id = jobId,
            title = info.title,
            thumbnail = info.thumbnail,
            isAudio = isAudio,
            isSplit = isSplit,
            status = JobStatus.Queued
        )

        _jobs.value = _jobs.value + job
        processJob(job, info, variant, splitSeconds)
    }

    fun remove(jobId: String) {
        _jobs.value = _jobs.value.filter { it.id != jobId }
    }

    private fun updateStatus(jobId: String, status: JobStatus) {
        _jobs.value = _jobs.value.map {
            if (it.id == jobId) it.copy(status = status) else it
        }
    }

    private fun processJob(
        job: DownloadJob,
        info: ResolvedInfo,
        variant: Variant,
        splitSeconds: Int?
    ) {
        scope.launch {
            try {
                updateStatus(job.id, JobStatus.Working("Submitting job…"))
                val ref = Api.createJob(
                    url = info.url,
                    format = variant.selector,
                    segmentSeconds = splitSeconds ?: 0
                )

                updateStatus(job.id, JobStatus.Working("Processing…"))
                val finishedJob = Api.waitForCompletion(ref.id) { current ->
                    val stage = if (current.status == "processing") "Processing video…" else "Preparing clips…"
                    updateStatus(job.id, JobStatus.Working(stage))
                }

                val clips = finishedJob.clips.sortedBy { it.index }
                if (clips.isEmpty()) {
                    updateStatus(job.id, JobStatus.Failed("No output files received"))
                    return@launch
                }

                // Download artwork if available
                var artworkFilename: String? = null
                if (!info.thumbnail.isNullOrEmpty()) {
                    val artName = "${job.id}.jpg"
                    val artFile = File(libraryStore.clipsDirectory, artName)
                    if (downloadFile(info.thumbnail, artFile)) {
                        artworkFilename = artName
                    }
                }

                val totalClips = clips.size
                clips.forEachIndexed { idx, clip ->
                    val progressFrac = (idx + 1).toFloat() / totalClips.coerceAtLeast(1)
                    updateStatus(job.id, JobStatus.Saving(progressFrac))

                    val ext = if (job.isAudio) "mp3" else "mp4"
                    val clipId = if (totalClips > 1) "${job.id}_part_${clip.index + 1}" else job.id
                    val localFilename = "$clipId.$ext"
                    val localFile = File(libraryStore.clipsDirectory, localFilename)

                    val req = Request.Builder().url(clip.url).build()
                    httpClient.newCall(req).execute().use { res ->
                        if (!res.isSuccessful) throw Exception("Failed to download clip ${idx + 1}")
                        val body = res.body ?: throw Exception("Empty body for clip ${idx + 1}")
                        FileOutputStream(localFile).use { fos ->
                            body.byteStream().copyTo(fos)
                        }
                    }

                    val title = if (totalClips > 1) {
                        "${info.title} (Part ${clip.index + 1})"
                    } else {
                        info.title
                    }

                    val savedClip = SavedClip(
                        id = clipId,
                        title = title,
                        filename = localFilename,
                        sizeBytes = localFile.length(),
                        createdAt = System.currentTimeMillis(),
                        artworkFilename = artworkFilename,
                        groupID = if (totalClips > 1) job.id else null,
                        groupTitle = if (totalClips > 1) info.title else null,
                        partIndex = if (totalClips > 1) (clip.index + 1) else null
                    )

                    libraryStore.add(savedClip)

                    // Auto-save to gallery / media store if enabled
                    if (libraryStore.autoSave.value) {
                        try {
                            if (job.isAudio) {
                                mediaStoreSaver.saveAudio(localFile, title, info.uploader)
                            } else {
                                mediaStoreSaver.saveVideo(localFile, title)
                            }
                        } catch (_: Exception) {}
                    }
                }

                updateStatus(job.id, JobStatus.Done)

                // Auto-cleanup finished job after 8 seconds
                delay(8000)
                remove(job.id)

            } catch (e: Exception) {
                updateStatus(job.id, JobStatus.Failed(e.message ?: "Download failed"))
            }
        }
    }

    private fun downloadFile(url: String, dest: File): Boolean {
        return try {
            val req = Request.Builder().url(url).build()
            httpClient.newCall(req).execute().use { res ->
                if (!res.isSuccessful) return false
                val body = res.body ?: return false
                FileOutputStream(dest).use { fos ->
                    body.byteStream().copyTo(fos)
                }
                true
            }
        } catch (_: Exception) {
            false
        }
    }

    companion object {
        @Volatile
        private var INSTANCE: DownloadQueue? = null

        fun getInstance(context: Context): DownloadQueue {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: DownloadQueue(context.applicationContext).also { INSTANCE = it }
            }
        }
    }
}
