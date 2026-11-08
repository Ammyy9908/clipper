# Clipper Android — minimal client

Reads a copied video link, resolves it against the clipper backend, polls the
job to completion, then writes the result into the device gallery.

## Files

| File | Role |
|---|---|
| `MainActivity.kt` | Compose UI — one screen, seven states |
| `DownloadViewModel.kt` | State machine: resolve → job → poll → save |
| `Api.kt` | Backend client, wire types, error mapping |
| `ClipboardWatcher.kt` | Reads the copied link |
| `MediaStoreSaver.kt` | Streamed download into the gallery |

## Running it

1. Android Studio → **Open**, pick this folder. Not Import, not New Project.
2. There is no `gradle-wrapper.jar` in this zip (it is a binary). On first open
   Android Studio notices and offers to set up the wrapper — accept, or run
   `gradle wrapper` if you have Gradle installed. Either way it only happens
   once.
3. Let Gradle sync. First sync downloads the Android SDK bits and takes a few
   minutes.
4. Tools → Device Manager → create any API 29+ emulator, start it.
5. Run.

`Config.BASE_URL` in `Api.kt` defaults to `http://10.0.2.2:8080`, which is how
the emulator reaches your Mac's localhost. For a real device use your Mac's LAN
IP (`ipconfig getifaddr en0`) on the same wifi, or a
`cloudflared tunnel --url http://localhost:8080` URL from anywhere.

Make sure the clipper backend is actually running — API, worker, and the
compose stack — or every resolve returns a connection error.

## Two decisions worth knowing

**`IS_PENDING = 1` while writing.** The MediaStore row is created before any
bytes arrive, and stays hidden from the gallery until the write finishes. A
failed download rolls the row back rather than leaving a zero-byte ghost with
a broken thumbnail. This is the detail that separates a clean gallery from a
littered one.

**No storage permission.** On API 29+ the app owns what it inserts under
scoped storage, so writing to the gallery needs no runtime prompt. `minSdk` is
29 specifically to keep it that way — dropping lower means requesting
`WRITE_EXTERNAL_STORAGE` and handling a denial path.

## Known gaps

- **No real progress during the server phase.** The backend has no progress
  channel, so the UI shows an indeterminate spinner for as long as the fetch
  takes — minutes on a long podcast. Backend fix: parse `yt-dlp --newline` in
  the worker, write percent to Redis, expose it on the job endpoint.
- **Foreground only.** `viewModelScope` dies if the process is killed. Real
  downloads belong in a `WorkManager` job with a foreground notification, which
  is also what Android expects for long network work.
- **Share sheet is declared but not wired.** The intent filter is in the
  manifest; handle `ACTION_SEND` in `onNewIntent` to skip the clipboard
  entirely. That's the better UX of the two paths.
- **No auth.** Matches the backend, which also has none.
- **Single clip only.** Takes `clips.first()`; a split job returns many.
