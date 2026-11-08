import Foundation

struct Playlist: Identifiable, Codable, Equatable {
    let id: String
    var name: String
    var clipIDs: [String]
    var createdAt: Date
}

/// Playlists are just named, ordered lists of clip IDs pointing back into
/// LibraryStore — never a second copy of the file. That's what makes deletion
/// safe: dropping a download out of LibraryStore doesn't require reaching into
/// every playlist to scrub it out, `clips(in:library:)` just silently stops
/// resolving that ID.
@MainActor
final class PlaylistStore: ObservableObject {
    static let shared = PlaylistStore()

    @Published private(set) var playlists: [Playlist]

    private static let indexFilename = "playlists.json"

    private init() {
        playlists = Self.loadIndex()
    }

    @discardableResult
    func create(name: String) -> Playlist {
        let playlist = Playlist(id: UUID().uuidString, name: name, clipIDs: [], createdAt: Date())
        playlists.insert(playlist, at: 0)
        persist()
        return playlist
    }

    func rename(_ playlist: Playlist, to name: String) {
        guard let i = playlists.firstIndex(where: { $0.id == playlist.id }) else { return }
        playlists[i].name = name
        persist()
    }

    func delete(_ playlist: Playlist) {
        playlists.removeAll { $0.id == playlist.id }
        persist()
    }

    func add(_ clip: SavedClip, to playlist: Playlist) {
        guard let i = playlists.firstIndex(where: { $0.id == playlist.id }) else { return }
        guard !playlists[i].clipIDs.contains(clip.id) else { return }
        playlists[i].clipIDs.append(clip.id)
        persist()
    }

    func remove(_ clip: SavedClip, from playlist: Playlist) {
        guard let i = playlists.firstIndex(where: { $0.id == playlist.id }) else { return }
        playlists[i].clipIDs.removeAll { $0 == clip.id }
        persist()
    }

    /// Resolves this playlist's clip IDs against the live library, dropping
    /// any that no longer exist — the count you see always matches what will
    /// actually play.
    func clips(in playlist: Playlist, library: LibraryStore) -> [SavedClip] {
        let byID = Dictionary(uniqueKeysWithValues: library.clips.map { ($0.id, $0) })
        return playlist.clipIDs.compactMap { byID[$0] }
    }

    private func persist() {
        let url = LibraryStore.clipsDirectory.appendingPathComponent(Self.indexFilename)
        guard let data = try? JSONEncoder().encode(playlists) else { return }
        try? data.write(to: url)
    }

    private static func loadIndex() -> [Playlist] {
        let url = clipsDirectoryURL.appendingPathComponent(indexFilename)
        guard let data = try? Data(contentsOf: url),
              let playlists = try? JSONDecoder().decode([Playlist].self, from: data)
        else { return [] }
        return playlists
    }

    private static var clipsDirectoryURL: URL { LibraryStore.clipsDirectory }
}
