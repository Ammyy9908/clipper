package ninja.sumitbighaniya.clipper

import android.content.ClipboardManager
import android.content.Context
import androidx.core.content.getSystemService

object ClipboardHelper {

    private val URL_REGEX = Regex("https?://[a-zA-Z0-9.-]+(?:/[^\\s]*)?")

    /**
     * Extracts the first valid http(s) URL from an arbitrary text snippet,
     * such as text shared from Instagram, TikTok, or YouTube apps.
     */
    fun extractUrl(text: String?): String? {
        if (text.isNullOrBlank()) return null
        val match = URL_REGEX.find(text)
        return match?.value?.trim()
    }

    /**
     * Reads the current primary clipboard content and returns an extracted URL if present.
     */
    fun getClipboardUrl(context: Context): String? {
        val cm = context.getSystemService<ClipboardManager>() ?: return null
        if (!cm.hasPrimaryClip()) return null
        val clip = cm.primaryClip ?: return null
        if (clip.itemCount == 0) return null

        val text = clip.getItemAt(0).coerceToText(context).toString()
        return extractUrl(text)
    }
}
