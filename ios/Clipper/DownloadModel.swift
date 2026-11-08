import Foundation

/// Owns only the resolve → pick-a-format step, which is inherently
/// interactive and synchronous. Once "Save" is tapped, the actual fetch and
/// save move to DownloadQueue — this resets straight back to idle instead of
/// sitting on a "Downloading…" screen the user can't leave.
@MainActor
final class DownloadModel: ObservableObject {

    enum Phase: Equatable {
        case idle
        case resolving
        case ready(ResolvedInfo)
        case failed(String)
    }

    struct ResolvedInfo: Equatable {
        let url: String
        let title: String
        let uploader: String
        let thumbnail: String?
        let variants: [Variant]

        static func == (a: Self, b: Self) -> Bool { a.url == b.url }
    }

    @Published private(set) var phase: Phase = .idle
    @Published var selected: Variant?
    /// nil = whole video, one file. Non-nil = split into clips of this many
    /// seconds each — audio-only jobs never split, the backend rejects that
    /// combination outright.
    @Published var splitSeconds: Int?

    func resolve(_ url: String) async {
        phase = .resolving
        do {
            let r = try await API.shared.resolve(url: url)
            let info = ResolvedInfo(
                url: url,
                title: r.title,
                uploader: r.uploader ?? r.platform,
                thumbnail: r.thumbnail,
                variants: r.variants
            )
            selected = r.variants.first
            splitSeconds = nil
            phase = .ready(info)
        } catch {
            phase = .failed(error.localizedDescription)
        }
    }

    /// Enqueues the download and immediately resets back to idle — the job
    /// itself lives in DownloadQueue from here, polling and saving on its own.
    func startDownload() {
        guard case .ready(let info) = phase else { return }
        let isAudio = selected?.audio ?? false
        let segments = isAudio ? 0 : (splitSeconds ?? 0)

        DownloadQueue.shared.start(
            url: info.url,
            title: info.title,
            thumbnail: info.thumbnail,
            format: selected?.selector,
            isAudio: isAudio,
            segmentSeconds: segments
        )
        reset()
    }

    func reset() {
        phase = .idle
        selected = nil
        splitSeconds = nil
    }
}
