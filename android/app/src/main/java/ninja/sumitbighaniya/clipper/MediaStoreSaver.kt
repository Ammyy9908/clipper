package ninja.sumitbighaniya.clipper

import android.content.ContentValues
import android.content.Context
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.File
import java.io.FileInputStream
import java.util.concurrent.TimeUnit

class MediaStoreSaver(private val context: Context) {

    private val client = OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .build()

    suspend fun saveAudio(file: File, title: String, artist: String? = null) = saveLocalFile(file, title, isAudio = true, artist = artist)

    suspend fun saveVideo(file: File, title: String) = saveLocalFile(file, title, isAudio = false)

    suspend fun saveLocalFile(
        file: File,
        displayName: String,
        isAudio: Boolean,
        artist: String? = null
    ) = withContext(Dispatchers.IO) {
        val resolver = context.contentResolver
        val ext = if (isAudio) "mp3" else "mp4"
        val mime = if (isAudio) "audio/mpeg" else "video/mp4"
        val relDir = if (isAudio) "${Environment.DIRECTORY_MUSIC}/Clipper" else "${Environment.DIRECTORY_MOVIES}/Clipper"

        val values = ContentValues().apply {
            if (isAudio) {
                put(MediaStore.Audio.Media.DISPLAY_NAME, sanitize(displayName, ext))
                put(MediaStore.Audio.Media.MIME_TYPE, mime)
                if (!artist.isNullOrBlank()) {
                    put(MediaStore.Audio.Media.ARTIST, artist)
                }
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    put(MediaStore.Audio.Media.RELATIVE_PATH, relDir)
                    put(MediaStore.Audio.Media.IS_PENDING, 1)
                }
            } else {
                put(MediaStore.Video.Media.DISPLAY_NAME, sanitize(displayName, ext))
                put(MediaStore.Video.Media.MIME_TYPE, mime)
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    put(MediaStore.Video.Media.RELATIVE_PATH, relDir)
                    put(MediaStore.Video.Media.IS_PENDING, 1)
                }
            }
        }

        val collection = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            if (isAudio) {
                MediaStore.Audio.Media.getContentUri(MediaStore.VOLUME_EXTERNAL_PRIMARY)
            } else {
                MediaStore.Video.Media.getContentUri(MediaStore.VOLUME_EXTERNAL_PRIMARY)
            }
        } else {
            if (isAudio) {
                MediaStore.Audio.Media.EXTERNAL_CONTENT_URI
            } else {
                MediaStore.Video.Media.EXTERNAL_CONTENT_URI
            }
        }

        val uri = resolver.insert(collection, values)
            ?: throw Exception("Couldn't create an entry in media storage.")

        try {
            resolver.openOutputStream(uri)?.use { out ->
                FileInputStream(file).use { input ->
                    input.copyTo(out)
                }
            } ?: throw Exception("Couldn't write to media storage.")

            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                val updateValues = ContentValues().apply {
                    if (isAudio) {
                        put(MediaStore.Audio.Media.IS_PENDING, 0)
                    } else {
                        put(MediaStore.Video.Media.IS_PENDING, 0)
                    }
                }
                resolver.update(uri, updateValues, null, null)
            }
        } catch (e: Exception) {
            resolver.delete(uri, null, null)
            throw e
        }
    }

    companion object {
        fun sanitize(name: String, ext: String): String {
            val cleaned = name.replace(Regex("[^A-Za-z0-9 _.-]"), "").trim().take(80)
            return if (cleaned.isEmpty()) "clipper_${System.currentTimeMillis()}.$ext" else "$cleaned.$ext"
        }
    }
}
