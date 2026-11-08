# Clipper iOS

Watches the clipboard for a video link, resolves it against the clipper
backend, polls the job to completion, then saves the result to Photos.

## Running it

1. Open `Clipper.xcodeproj` in Xcode 16 or later.
2. Select the `Clipper` target → Signing & Capabilities → pick your team.
   A free Apple ID works for running on your own device.
3. Pick a destination and Run.

`Config.baseURL` in `API.swift` is set to `http://192.168.1.2:8080`.

- **Simulator** — can use `http://localhost:8080`, since it shares the Mac's
  network stack.
- **Real device** — needs the Mac's LAN IP on the same wifi. Check it with
  `ipconfig getifaddr en0` and update if your router reassigns.

The backend must be running (API, worker, and the compose stack), and
`S3_ENDPOINT` in the backend's `.env` must also be the LAN IP — otherwise the
job completes but the phone can't fetch the finished file from MinIO.

## Files

| File | Role |
|---|---|
| `ClipperApp.swift` | App entry point |
| `ContentView.swift` | The whole UI — one screen, six states |
| `DownloadModel.swift` | State machine: resolve → job → poll → save |
| `API.swift` | Backend client, wire types, error mapping |
| `ClipboardWatcher.swift` | Banner-free link detection |
| `PhotoSaver.swift` | Streamed download + Photos write |
| `Info.plist` | Photos usage string and the ATS exception |

## Two decisions worth knowing

**Clipboard reads use `detectPatterns`, not `.string`.** Reading the pasteboard
directly shows the "pasted from Safari" banner every time. `detectPatterns`
only reports that a URL-shaped thing exists without reading it; the actual read
happens inside the button tap, which iOS treats as consent and shows nothing.

**Photos access is `.addOnly`.** The app never reads the library, so requesting
full access would be an unnecessary prompt and an App Review flag.

## Known gaps

- **No real progress during the server phase.** The backend has no progress
  channel, so the UI shows an indeterminate spinner for however long the fetch
  takes. Backend fix: parse `yt-dlp --newline` in the worker, write percent to
  Redis, expose it on the job endpoint.
- **Foreground only.** Leaving the app pauses polling. Background completion
  needs a background `URLSession` or a push when the job finishes.
- **`NSAllowsArbitraryLoads` is on** for plain-HTTP LAN access. Remove it once
  the backend is behind HTTPS — App Review will ask about it otherwise.
- **No auth.** Matches the backend, which also has none.
- **Single clip only.** Takes `clips.first`; a split job returns many.
