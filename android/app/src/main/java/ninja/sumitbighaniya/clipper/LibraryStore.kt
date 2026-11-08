package ninja.sumitbighaniya.clipper

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import java.io.File

@Serializable
data class SavedClip(
    val id: String,
    val title: String,
    val filename: String,
    val sizeBytes: Long,
    val createdAt: Long, // timestamp ms
    val artworkFilename: String? = null,
    val groupID: String? = null,
    val groupTitle: String? = null,
    val partIndex: Int? = null
) {
    val fileExtension: String get() = filename.substringAfterLast('.', "").lowercase()
    val isAudio: Boolean get() = fileExtension in AUDIO_EXTENSIONS
    val isSplitPart: Boolean get() = groupID != null

    companion object {
        val AUDIO_EXTENSIONS = setOf("mp3", "m4a", "wav", "aac", "flac", "ogg", "opus")
    }
}

class LibraryStore private constructor(private val context: Context) {

    private val json = Json { ignoreUnknownKeys = true; prettyPrint = true }
    private val prefs: SharedPreferences = context.getSharedPreferences("clipper_library", Context.MODE_PRIVATE)

    private val _clips = MutableStateFlow<List<SavedClip>>(emptyList())
    val clips: StateFlow<List<SavedClip>> = _clips.asStateFlow()

    private val _autoSave = MutableStateFlow(prefs.getBoolean(KEY_AUTO_SAVE, true))
    val autoSave: StateFlow<Boolean> = _autoSave.asStateFlow()

    val clipsDirectory: File by lazy {
        File(context.filesDir, "Clips").apply { if (!exists()) mkdirs() }
    }

    init {
        loadClips()
    }

    fun setAutoSave(enabled: Boolean) {
        _autoSave.value = enabled
        prefs.edit().putBoolean(KEY_AUTO_SAVE, enabled).apply()
    }

    fun getFile(clip: SavedClip): File = File(clipsDirectory, clip.filename)

    fun getArtworkFile(clip: SavedClip): File? {
        clip.artworkFilename?.let {
            val f = File(clipsDirectory, it)
            if (f.exists()) return f
        }
        val direct = File(clipsDirectory, "${clip.id}.jpg")
        if (direct.exists()) return direct

        clip.groupID?.let { gid ->
            val groupArt = File(clipsDirectory, "$gid.jpg")
            if (groupArt.exists()) return groupArt
        }
        return null
    }

    fun add(clip: SavedClip) {
        val current = _clips.value.toMutableList()
        current.removeAll { it.id == clip.id }
        current.add(0, clip)
        _clips.value = current
        persist()
    }

    fun remove(clip: SavedClip) {
        try {
            getFile(clip).delete()
            getArtworkFile(clip)?.delete()
        } catch (_: Exception) {}

        val current = _clips.value.toMutableList()
        current.removeAll { it.id == clip.id }
        _clips.value = current
        persist()
    }

    private fun persist() {
        CoroutineScope(Dispatchers.IO).launch {
            try {
                val indexFile = File(clipsDirectory, INDEX_FILENAME)
                val text = json.encodeToString(_clips.value)
                indexFile.writeText(text)
            } catch (_: Exception) {}
        }
    }

    private fun loadClips() {
        val indexFile = File(clipsDirectory, INDEX_FILENAME)
        if (!indexFile.exists()) return
        try {
            val text = indexFile.readText()
            val list = json.decodeFromString<List<SavedClip>>(text)
            _clips.value = list
        } catch (_: Exception) {}
    }

    companion object {
        private const val KEY_AUTO_SAVE = "auto_save_photos"
        private const val INDEX_FILENAME = "clips.json"

        @Volatile
        private var INSTANCE: LibraryStore? = null

        fun getInstance(context: Context): LibraryStore {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: LibraryStore(context.applicationContext).also { INSTANCE = it }
            }
        }
    }
}
