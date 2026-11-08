package ninja.sumitbighaniya.clipper

import android.content.Context
import android.net.Uri
import androidx.annotation.OptIn
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.io.File

enum class RepeatMode { OFF, ALL, ONE }
enum class PlaybackDisplayMode { AUDIO, VIDEO }

@OptIn(UnstableApi::class)
class PlayerModel private constructor(private val context: Context) {

    private val libraryStore = LibraryStore.getInstance(context)
    val exoPlayer: ExoPlayer by lazy {
        ExoPlayer.Builder(context).build().apply {
            addListener(playerListener)
        }
    }

    private val _clips = MutableStateFlow<List<SavedClip>>(emptyList())
    val clips: StateFlow<List<SavedClip>> = _clips.asStateFlow()

    private val _index = MutableStateFlow(0)
    val index: StateFlow<Int> = _index.asStateFlow()

    private val _isPlaying = MutableStateFlow(false)
    val isPlaying: StateFlow<Boolean> = _isPlaying.asStateFlow()

    private val _currentTimeMs = MutableStateFlow(0L)
    val currentTimeMs: StateFlow<Long> = _currentTimeMs.asStateFlow()

    private val _durationMs = MutableStateFlow(0L)
    val durationMs: StateFlow<Long> = _durationMs.asStateFlow()

    private val _isExpanded = MutableStateFlow(false)
    val isExpanded: StateFlow<Boolean> = _isExpanded.asStateFlow()

    private val _isShuffling = MutableStateFlow(false)
    val isShuffling: StateFlow<Boolean> = _isShuffling.asStateFlow()

    private val _repeatMode = MutableStateFlow(RepeatMode.OFF)
    val repeatMode: StateFlow<RepeatMode> = _repeatMode.asStateFlow()

    private val _displayMode = MutableStateFlow(PlaybackDisplayMode.AUDIO)
    val displayMode: StateFlow<PlaybackDisplayMode> = _displayMode.asStateFlow()

    var isScrubbing: Boolean = false

    val currentClip: SavedClip?
        get() {
            val list = _clips.value
            val idx = _index.value
            return if (idx in list.indices) list[idx] else null
        }

    val hasPrevious: Boolean
        get() = _index.value > 0 || _repeatMode.value == RepeatMode.ALL

    val hasNext: Boolean
        get() = _index.value < _clips.value.size - 1 || _repeatMode.value == RepeatMode.ALL

    private var progressJob: kotlinx.coroutines.Job? = null
    private val scope = CoroutineScope(Dispatchers.Main + SupervisorJob())

    private val playerListener = object : Player.Listener {
        override fun onIsPlayingChanged(isPlaying: Boolean) {
            _isPlaying.value = isPlaying
            if (isPlaying) startProgressPolling() else stopProgressPolling()
        }

        override fun onPlaybackStateChanged(playbackState: Int) {
            if (playbackState == Player.STATE_READY) {
                _durationMs.value = exoPlayer.duration.coerceAtLeast(0L)
            } else if (playbackState == Player.STATE_ENDED) {
                onTrackEnded()
            }
        }
    }

    fun setExpanded(expanded: Boolean) {
        _isExpanded.value = expanded
    }

    fun setDisplayMode(mode: PlaybackDisplayMode) {
        _displayMode.value = mode
    }

    fun play(clips: List<SavedClip>, startIndex: Int, initialMode: PlaybackDisplayMode? = null) {
        if (clips.isEmpty() || startIndex !in clips.indices) return

        val clip = clips[startIndex]
        _displayMode.value = if (clip.isAudio) {
            PlaybackDisplayMode.AUDIO
        } else {
            initialMode ?: PlaybackDisplayMode.VIDEO
        }

        // Check if already playing the exact clip
        if (_clips.value == clips && _index.value == startIndex && exoPlayer.playbackState != Player.STATE_IDLE) {
            if (!exoPlayer.isPlaying) exoPlayer.play()
            return
        }

        _clips.value = clips
        _index.value = startIndex
        loadAndPlayCurrent()
    }

    fun togglePlayPause() {
        if (exoPlayer.isPlaying) {
            exoPlayer.pause()
        } else {
            if (exoPlayer.playbackState == Player.STATE_IDLE || exoPlayer.playbackState == Player.STATE_ENDED) {
                loadAndPlayCurrent()
            } else {
                exoPlayer.play()
            }
        }
    }

    fun seekTo(positionMs: Long) {
        _currentTimeMs.value = positionMs
        exoPlayer.seekTo(positionMs)
    }

    fun next() {
        val list = _clips.value
        if (list.isEmpty()) return

        if (_isShuffling.value && list.size > 1) {
            var nextIdx = _index.value
            while (nextIdx == _index.value) {
                nextIdx = list.indices.random()
            }
            _index.value = nextIdx
            loadAndPlayCurrent()
            return
        }

        if (_index.value < list.size - 1) {
            _index.value += 1
            loadAndPlayCurrent()
        } else if (_repeatMode.value == RepeatMode.ALL) {
            _index.value = 0
            loadAndPlayCurrent()
        }
    }

    fun previous() {
        val list = _clips.value
        if (list.isEmpty()) return

        if (_currentTimeMs.value > 3000L) {
            seekTo(0L)
            return
        }

        if (_index.value > 0) {
            _index.value -= 1
            loadAndPlayCurrent()
        } else if (_repeatMode.value == RepeatMode.ALL) {
            _index.value = list.size - 1
            loadAndPlayCurrent()
        } else {
            seekTo(0L)
        }
    }

    fun toggleShuffle() {
        _isShuffling.value = !_isShuffling.value
    }

    fun toggleRepeat() {
        _repeatMode.value = when (_repeatMode.value) {
            RepeatMode.OFF -> RepeatMode.ALL
            RepeatMode.ALL -> RepeatMode.ONE
            RepeatMode.ONE -> RepeatMode.OFF
        }
    }

    fun stop() {
        exoPlayer.stop()
        _isPlaying.value = false
        _clips.value = emptyList()
        _isExpanded.value = false
        stopProgressPolling()
    }

    private fun loadAndPlayCurrent() {
        val clip = currentClip ?: return
        val file = libraryStore.getFile(clip)
        if (!file.exists()) {
            return
        }

        val mediaItem = MediaItem.fromUri(Uri.fromFile(file))
        exoPlayer.setMediaItem(mediaItem)
        exoPlayer.prepare()
        exoPlayer.play()
    }

    private fun onTrackEnded() {
        when (_repeatMode.value) {
            RepeatMode.ONE -> {
                seekTo(0L)
                exoPlayer.play()
            }
            RepeatMode.ALL -> next()
            RepeatMode.OFF -> {
                if (_index.value < _clips.value.size - 1) {
                    next()
                } else {
                    _isPlaying.value = false
                    stopProgressPolling()
                }
            }
        }
    }

    private fun startProgressPolling() {
        stopProgressPolling()
        progressJob = scope.launch {
            while (isActive) {
                if (!isScrubbing) {
                    _currentTimeMs.value = exoPlayer.currentPosition.coerceAtLeast(0L)
                    _durationMs.value = exoPlayer.duration.coerceAtLeast(0L)
                }
                delay(250)
            }
        }
    }

    private fun stopProgressPolling() {
        progressJob?.cancel()
        progressJob = null
    }

    companion object {
        @Volatile
        private var INSTANCE: PlayerModel? = null

        fun getInstance(context: Context): PlayerModel {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: PlayerModel(context.applicationContext).also { INSTANCE = it }
            }
        }
    }
}
