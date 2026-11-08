package ninja.sumitbighaniya.clipper

import android.content.ClipboardManager
import android.content.Context
import androidx.core.content.getSystemService

/**
 * Android restricts clipboard reads to the foreground app since Android 10, and
 * shows a "pasted from" toast on Android 12+. So the read happens once, on
 * resume, and the result is held rather than re-read on every recomposition —
 * re-reading would fire the toast repeatedly.
 */
class ClipboardWatcher(private val context: Context) {

    fun currentLink(): String? {
        val cm = context.getSystemService<ClipboardManager>() ?: return null
        if (!cm.hasPrimaryClip()) return null

        val clip = cm.primaryClip ?: return null
        if (clip.itemCount == 0) return null

        val text = clip.getItemAt(0).coerceToText(context).toString().trim()
        return text.takeIf { it.startsWith("http://") || it.startsWith("https://") }
    }
}
