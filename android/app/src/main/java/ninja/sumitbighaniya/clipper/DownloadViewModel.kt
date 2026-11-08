package ninja.sumitbighaniya.clipper

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed interface Phase {
    data object Idle : Phase
    data object Resolving : Phase
    data class Ready(val info: ResolvedInfo) : Phase
    data class Working(val stage: String) : Phase
    data class Saving(val progress: Float, val isAudio: Boolean) : Phase
    data class Done(val isAudio: Boolean, val isSplit: Boolean) : Phase
    data class Failed(val message: String) : Phase
}

data class ResolvedInfo(
    val url: String,
    val title: String,
    val uploader: String,
    val thumbnail: String?,
    val variants: List<Variant>,
)

class DownloadViewModel(app: Application) : AndroidViewModel(app) {

    private val _phase = MutableStateFlow<Phase>(Phase.Idle)
    val phase: StateFlow<Phase> = _phase.asStateFlow()

    private val _selected = MutableStateFlow<Variant?>(null)
    val selected: StateFlow<Variant?> = _selected.asStateFlow()

    private val _splitSeconds = MutableStateFlow<Int?>(null)
    val splitSeconds: StateFlow<Int?> = _splitSeconds.asStateFlow()

    fun select(v: Variant) {
        _selected.value = v
        if (v.audio) {
            _splitSeconds.value = null
        }
    }

    fun setSplitSeconds(seconds: Int?) {
        _splitSeconds.value = seconds
    }

    fun resolve(url: String) = viewModelScope.launch {
        _phase.value = Phase.Resolving
        runCatching { Api.resolve(url) }
            .onSuccess { r ->
                _selected.value = r.variants.firstOrNull()
                _splitSeconds.value = null
                _phase.value = Phase.Ready(
                    ResolvedInfo(url, r.title, r.uploader ?: r.platform, r.thumbnail, r.variants)
                )
            }
            .onFailure { _phase.value = Phase.Failed(it.message ?: "Couldn't read that link.") }
    }

    fun startDownload() {
        val info = (_phase.value as? Phase.Ready)?.info ?: return
        val variant = _selected.value ?: info.variants.firstOrNull() ?: return
        DownloadQueue.getInstance(getApplication()).enqueue(info, variant, _splitSeconds.value)
        reset()
    }

    fun reset() {
        _phase.value = Phase.Idle
        _selected.value = null
        _splitSeconds.value = null
    }
}
