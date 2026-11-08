import Foundation

struct SavedClip: Identifiable, Codable, Equatable {
    let id: String
    var title: String
    var filename: String
    var sizeBytes: Int64
    var createdAt: Date
    var artworkFilename: String? = nil
    /// Non-nil only for a part produced by splitting one video into clips —
    /// shared across every part from the same job, so the Split tab can group
    /// them back into one card instead of showing N unrelated videos.
    var groupID: String? = nil
    /// The original video's title, without the "(Part N)" suffix — stored
    /// once at creation rather than parsed back out of `title` later.
    var groupTitle: String? = nil
    var partIndex: Int? = nil

    private static let audioExtensions: Set<String> = ["mp3", "m4a", "wav", "aac", "flac", "aiff", "caf"]

    var localURL: URL { LibraryStore.clipsDirectory.appendingPathComponent(filename) }
    var artworkURL: URL? {
        if let artworkFilename {
            return LibraryStore.clipsDirectory.appendingPathComponent(artworkFilename)
        }
        let autoName = "\(id).jpg"
        let fileURL = LibraryStore.clipsDirectory.appendingPathComponent(autoName)
        if FileManager.default.fileExists(atPath: fileURL.path) {
            return fileURL
        }
        if let gid = groupID {
            let groupURL = LibraryStore.clipsDirectory.appendingPathComponent("\(gid).jpg")
            if FileManager.default.fileExists(atPath: groupURL.path) {
                return groupURL
            }
        }
        return nil
    }
    var fileExtension: String { (filename as NSString).pathExtension.lowercased() }
    var isAudio: Bool { Self.audioExtensions.contains(fileExtension) }
    var isSplitPart: Bool { groupID != nil }
}

/// Keeps every downloaded clip on disk in the app's own sandbox, independent of
/// whether it also gets exported to Photos. This is what backs the Videos sheet —
/// without it, turning Auto Save off would have nowhere to keep a finished download.
@MainActor
final class LibraryStore: ObservableObject {
    static let shared = LibraryStore()

    @Published private(set) var clips: [SavedClip]
    @Published var autoSave: Bool {
        didSet { UserDefaults.standard.set(autoSave, forKey: Self.autoSaveKey) }
    }

    private static let autoSaveKey = "autoSaveToPhotos"
    private static let indexFilename = "clips.json"

    nonisolated static let clipsDirectory: URL = {
        let dir = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("Clips", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }()

    private init() {
        autoSave = (UserDefaults.standard.object(forKey: Self.autoSaveKey) as? Bool) ?? true
        clips = Self.loadIndex()
    }

    func add(_ clip: SavedClip) {
        clips.insert(clip, at: 0)
        persist()
    }

    func remove(_ clip: SavedClip) {
        try? FileManager.default.removeItem(at: clip.localURL)
        if let artworkURL = clip.artworkURL {
            try? FileManager.default.removeItem(at: artworkURL)
        }
        clips.removeAll { $0.id == clip.id }
        persist()
        // Otherwise the mini player would keep "playing" a file that's gone.
        if PlayerModel.shared.currentClip?.id == clip.id {
            PlayerModel.shared.stop()
        }
    }

    func removeAll() {
        for clip in clips {
            try? FileManager.default.removeItem(at: clip.localURL)
            if let artworkURL = clip.artworkURL {
                try? FileManager.default.removeItem(at: artworkURL)
            }
        }
        clips.removeAll()
        persist()
        PlayerModel.shared.stop()
    }

    private func persist() {
        let url = Self.clipsDirectory.appendingPathComponent(Self.indexFilename)
        guard let data = try? JSONEncoder().encode(clips) else { return }
        try? data.write(to: url)
    }

    private static func loadIndex() -> [SavedClip] {
        let url = clipsDirectory.appendingPathComponent(indexFilename)
        guard let data = try? Data(contentsOf: url),
              let clips = try? JSONDecoder().decode([SavedClip].self, from: data)
        else { return [] }
        return clips
    }
}
