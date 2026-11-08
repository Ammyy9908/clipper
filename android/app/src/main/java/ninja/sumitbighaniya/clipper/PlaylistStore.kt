package ninja.sumitbighaniya.clipper

import android.content.Context
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
import java.util.UUID

@Serializable
data class Playlist(
    val id: String,
    val name: String,
    val clipIDs: List<String> = emptyList(),
    val createdAt: Long = System.currentTimeMillis()
)

class PlaylistStore private constructor(private val context: Context) {

    private val json = Json { ignoreUnknownKeys = true; prettyPrint = true }

    private val _playlists = MutableStateFlow<List<Playlist>>(emptyList())
    val playlists: StateFlow<List<Playlist>> = _playlists.asStateFlow()

    private val storageDir: File by lazy {
        File(context.filesDir, "Clips").apply { if (!exists()) mkdirs() }
    }

    init {
        loadPlaylists()
    }

    fun create(name: String): Playlist {
        val playlist = Playlist(id = UUID.randomUUID().toString(), name = name, clipIDs = emptyList())
        val current = _playlists.value.toMutableList()
        current.add(0, playlist)
        _playlists.value = current
        persist()
        return playlist
    }

    fun rename(playlistId: String, newName: String) {
        val current = _playlists.value.map {
            if (it.id == playlistId) it.copy(name = newName) else it
        }
        _playlists.value = current
        persist()
    }

    fun delete(playlistId: String) {
        val current = _playlists.value.filter { it.id != playlistId }
        _playlists.value = current
        persist()
    }

    fun addClip(playlistId: String, clipId: String) {
        val current = _playlists.value.map { pl ->
            if (pl.id == playlistId && !pl.clipIDs.contains(clipId)) {
                pl.copy(clipIDs = pl.clipIDs + clipId)
            } else pl
        }
        _playlists.value = current
        persist()
    }

    fun removeClip(playlistId: String, clipId: String) {
        val current = _playlists.value.map { pl ->
            if (pl.id == playlistId) {
                pl.copy(clipIDs = pl.clipIDs.filter { it != clipId })
            } else pl
        }
        _playlists.value = current
        persist()
    }

    fun getClips(playlist: Playlist, libraryClips: List<SavedClip>): List<SavedClip> {
        val byId = libraryClips.associateBy { it.id }
        return playlist.clipIDs.mapNotNull { byId[it] }
    }

    private fun persist() {
        CoroutineScope(Dispatchers.IO).launch {
            try {
                val indexFile = File(storageDir, INDEX_FILENAME)
                val text = json.encodeToString(_playlists.value)
                indexFile.writeText(text)
            } catch (_: Exception) {}
        }
    }

    private fun loadPlaylists() {
        val indexFile = File(storageDir, INDEX_FILENAME)
        if (!indexFile.exists()) return
        try {
            val text = indexFile.readText()
            val list = json.decodeFromString<List<Playlist>>(text)
            _playlists.value = list
        } catch (_: Exception) {}
    }

    companion object {
        private const val INDEX_FILENAME = "playlists.json"

        @Volatile
        private var INSTANCE: PlaylistStore? = null

        fun getInstance(context: Context): PlaylistStore {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: PlaylistStore(context.applicationContext).also { INSTANCE = it }
            }
        }
    }
}
