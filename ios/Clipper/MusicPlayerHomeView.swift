import SwiftUI
import UniformTypeIdentifiers

/// The Music tab: Spotify-style playlists shelf plus every downloaded mp3.
/// This view never touches videos or the resolve/download flow — those are
/// the other two tabs' job; all three share only LibraryStore (the files on
/// disk) and PlayerModel (the one now-playing engine).
struct MusicPlayerHomeView: View {
    @ObservedObject var library: LibraryStore
    @ObservedObject private var playlists = PlaylistStore.shared
    @ObservedObject private var player = PlayerModel.shared

    @State private var showingNewPlaylist = false
    @State private var newPlaylistName = ""
    @State private var openPlaylist: Playlist?
    @State private var addingToPlaylistClip: SavedClip?
    @State private var showingImporter = false
    @State private var showingDropboxBrowser = false
    @State private var showingGoogleDriveBrowser = false

    private var songs: [SavedClip] { library.clips.filter { $0.isAudio } }

    private var recentSongs: [SavedClip] {
        Array(songs.sorted { $0.createdAt > $1.createdAt }.prefix(6))
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            if songs.isEmpty {
                Spacer()
                emptyState
                Spacer()
            } else {
                ScrollView {
                    VStack(alignment: .leading, spacing: 28) {
                        quickPicksSection
                        playlistsSection
                        allSongsSection
                    }
                    .padding(.top, 4)
                    .padding(.bottom, player.currentClip != nil && !player.isExpanded ? 84 : 20)
                }
            }
        }
        .sheet(item: $openPlaylist) { playlist in
            PlaylistDetailView(playlist: playlist, library: library)
        }
        .sheet(item: $addingToPlaylistClip) { clip in
            AddToPlaylistSheet(clip: clip, library: library)
        }
        .alert("New Playlist", isPresented: $showingNewPlaylist) {
            TextField("Playlist name", text: $newPlaylistName)
            Button("Cancel", role: .cancel) { newPlaylistName = "" }
            Button("Create") {
                let name = newPlaylistName.trimmingCharacters(in: .whitespacesAndNewlines)
                if !name.isEmpty { playlists.create(name: name) }
                newPlaylistName = ""
            }
        }
        .fileImporter(
            isPresented: $showingImporter,
            allowedContentTypes: [.audio],
            allowsMultipleSelection: true
        ) { result in
            guard case .success(let urls) = result else { return }
            for url in urls { importFile(url) }
        }
        .sheet(isPresented: $showingDropboxBrowser) {
            DropboxBrowserView(library: library)
        }
        .sheet(isPresented: $showingGoogleDriveBrowser) {
            GoogleDriveBrowserView(library: library)
        }
    }

    private var importMenu: some View {
        Menu {
            Button { showingImporter = true } label: {
                Label("Files", systemImage: "folder")
            }
            Button { showingDropboxBrowser = true } label: {
                Label("Dropbox", systemImage: "shippingbox")
            }
            Button { showingGoogleDriveBrowser = true } label: {
                Label("Google Drive", systemImage: "tray.full")
            }
        } label: {
            Image(systemName: "plus")
                .font(.system(size: 18, weight: .bold))
                .foregroundStyle(Theme.ink)
                .frame(width: 36, height: 36)
                .background(Circle().fill(Theme.surface))
        }
    }

    /// Files from iCloud Drive, Dropbox, Google Drive, or OneDrive all arrive
    /// here the same way — the system Files picker unifies them into one flow,
    /// we don't touch any of those providers directly. Security-scoped access
    /// only needs to be held long enough to copy the bytes into our own
    /// sandbox; the SavedClip afterward is a fully independent local file.
    private func importFile(_ url: URL) {
        let accessed = url.startAccessingSecurityScopedResource()
        defer { if accessed { url.stopAccessingSecurityScopedResource() } }

        let id = UUID().uuidString
        let ext = url.pathExtension.isEmpty ? "mp3" : url.pathExtension.lowercased()
        let filename = "\(id).\(ext)"
        let dest = LibraryStore.clipsDirectory.appendingPathComponent(filename)

        guard (try? FileManager.default.copyItem(at: url, to: dest)) != nil else { return }

        let size = (try? FileManager.default.attributesOfItem(atPath: dest.path)[.size] as? Int64) ?? 0
        let title = url.deletingPathExtension().lastPathComponent

        library.add(SavedClip(
            id: id,
            title: title.isEmpty ? filename : title,
            filename: filename,
            sizeBytes: size,
            createdAt: Date()
        ))
    }

    private var header: some View {
        HStack(alignment: .top) {
            VStack(alignment: .leading, spacing: 2) {
                Text("Your Music")
                    .font(.system(size: 34, weight: .heavy, design: .rounded))
                    .foregroundStyle(Theme.ink)
                Text(songs.isEmpty ? "Nothing here yet" : "\(songs.count) song\(songs.count == 1 ? "" : "s")")
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
            }
            Spacer()
            importMenu
        }
        .padding(.horizontal, 20)
        .padding(.top, 24)
        .padding(.bottom, 16)
    }

    /// Spotify Home's "Quick Picks" grid — the fastest path back into
    /// whatever you saved most recently, no digging through the full list.
    private var quickPicksSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Quick Picks")
                .font(.system(size: 20, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.ink)
                .padding(.horizontal, 20)

            LazyVGrid(columns: [GridItem(.flexible(), spacing: 10), GridItem(.flexible(), spacing: 10)], spacing: 10) {
                ForEach(recentSongs) { clip in
                    let isCurrent = player.currentClip?.id == clip.id
                    QuickPickCard(
                        clip: clip,
                        isCurrent: isCurrent,
                        isPlaying: player.isPlaying,
                        onSelect: { play(clip) },
                        onToggle: { player.togglePlay() }
                    )
                }
            }
            .padding(.horizontal, 20)
        }
    }

    private var playlistsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Playlists")
                .font(.system(size: 20, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.ink)
                .padding(.horizontal, 20)

            ScrollView(.horizontal, showsIndicators: false) {
                HStack(alignment: .top, spacing: 14) {
                    Button {
                        showingNewPlaylist = true
                    } label: {
                        VStack(spacing: 8) {
                            ZStack {
                                RoundedRectangle(cornerRadius: 12, style: .continuous)
                                    .strokeBorder(Theme.hairline, style: StrokeStyle(lineWidth: 1.5, dash: [6]))
                                Image(systemName: "plus")
                                    .font(.system(size: 26, weight: .semibold))
                                    .foregroundStyle(Theme.inkTertiary)
                            }
                            .frame(width: 140, height: 140)
                            Text("New Playlist")
                                .font(.system(size: 13, weight: .semibold))
                                .foregroundStyle(Theme.inkSecondary)
                                .frame(width: 140)
                        }
                    }
                    .buttonStyle(.plain)

                    ForEach(playlists.playlists) { playlist in
                        let songsInPlaylist = playlists.clips(in: playlist, library: library)
                        let isCurrent = player.currentClip.map { c in songsInPlaylist.contains { $0.id == c.id } } ?? false
                        PlaylistCard(
                            playlist: playlist,
                            songs: songsInPlaylist,
                            isCurrent: isCurrent,
                            isPlaying: player.isPlaying,
                            onOpen: { openPlaylist = playlist },
                            onToggle: {
                                if isCurrent {
                                    player.togglePlay()
                                } else if !songsInPlaylist.isEmpty {
                                    player.play(clips: songsInPlaylist, startIndex: 0)
                                    player.isExpanded = true
                                }
                            }
                        )
                    }
                }
                .padding(.horizontal, 20)
            }
        }
    }

    private var allSongsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("All Songs")
                .font(.system(size: 20, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.ink)
                .padding(.horizontal, 20)

            LazyVStack(spacing: 4) {
                ForEach(songs) { clip in
                    SongRow(
                        clip: clip,
                        isCurrent: player.currentClip?.id == clip.id,
                        isPlaying: player.isPlaying,
                        onPlay: { play(clip) },
                        onToggle: { player.togglePlay() }
                    ) {
                        Button {
                            addingToPlaylistClip = clip
                        } label: {
                            Label("Add to Playlist", systemImage: "text.badge.plus")
                        }
                        ShareLink(item: clip.localURL) {
                            Label("Share", systemImage: "square.and.arrow.up")
                        }
                        Button(role: .destructive) {
                            library.remove(clip)
                        } label: {
                            Label("Delete", systemImage: "trash")
                        }
                    }
                }
            }
            .padding(.horizontal, 20)
        }
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: "music.note")
                .font(.system(size: 30))
                .foregroundStyle(Theme.inkTertiary)
            Text("No music yet")
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Theme.ink)
            Text("Save an MP3 from the downloader, or import from Files, Dropbox, or Drive")
                .font(.system(size: 13))
                .foregroundStyle(Theme.inkSecondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 40)

            Menu {
                Button { showingImporter = true } label: {
                    Label("Files", systemImage: "folder")
                }
                Button { showingDropboxBrowser = true } label: {
                    Label("Dropbox", systemImage: "shippingbox")
                }
                Button { showingGoogleDriveBrowser = true } label: {
                    Label("Google Drive", systemImage: "tray.full")
                }
            } label: {
                Text("Import Music")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(.white)
                    .padding(.horizontal, 20)
                    .padding(.vertical, 10)
                    .background(Capsule().fill(Theme.accent))
            }
            .padding(.top, 8)
        }
    }

    private func play(_ clip: SavedClip) {
        guard let idx = songs.firstIndex(where: { $0.id == clip.id }) else { return }
        player.play(clips: songs, startIndex: idx)
        player.isExpanded = true
    }
}

/// Shared row for any song list (All Songs, a playlist) — the trailing menu
/// differs per context, so its content is a plain @ViewBuilder rather than a
/// fixed set of actions.
/// Flat, borderless list row — Spotify/YT Music style rather than this app's
/// usual bordered card, since a dense scrolling song list reads as "modern
/// music app" more when rows are just separated by whitespace, not boxed.
struct SongRow<Menu: View>: View {
    let clip: SavedClip
    /// Whether this row's song is the one loaded in the player right now —
    /// distinct from `isPlaying` (the actual play/pause state), which only
    /// matters visually once this row already is the current one.
    var isCurrent: Bool = false
    var isPlaying: Bool = false
    let onPlay: () -> Void
    var onToggle: () -> Void = {}
    @ViewBuilder var menuItems: () -> Menu

    var body: some View {
        HStack(spacing: 14) {
            ZStack {
                SongArtwork(url: clip.artworkURL)
                if isCurrent {
                    Color.black.opacity(0.35)
                    // A real button here, not just an indicator — tapping it
                    // toggles play/pause instead of restarting the track from
                    // the top the way re-triggering onPlay would.
                    Button(action: onToggle) {
                        Image(systemName: isPlaying ? "pause.fill" : "play.fill")
                            .font(.system(size: 15, weight: .bold))
                            .foregroundStyle(.white)
                            .frame(width: 30, height: 30)
                            .background(Circle().fill(Theme.accent))
                    }
                }
            }
            .frame(width: 56, height: 56)
            .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))

            Text(clip.title)
                .font(.system(size: 16, weight: isCurrent ? .bold : .semibold))
                .foregroundStyle(isCurrent ? Theme.accent : Theme.ink)
                .lineLimit(1)
                .truncationMode(.middle)

            Spacer()

            SwiftUI.Menu {
                menuItems()
            } label: {
                Image(systemName: "ellipsis")
                    .font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(Theme.inkSecondary)
                    .frame(width: 32, height: 32)
            }
        }
        .padding(.vertical, 6)
        .contentShape(Rectangle())
        .onTapGesture {
            if isCurrent { onToggle() } else { onPlay() }
        }
    }
}

/// Spotify Home's small horizontal "Quick Picks" tile — thumbnail-left,
/// title-right, inside a tinted pill.
private struct QuickPickCard: View {
    let clip: SavedClip
    /// Whether this specific song is the one loaded in the player right now
    /// — distinct from `isPlaying`, which is the player's play/pause state
    /// and only matters visually when this card is the current one.
    let isCurrent: Bool
    let isPlaying: Bool
    let onSelect: () -> Void
    let onToggle: () -> Void

    var body: some View {
        HStack(spacing: 0) {
            ZStack(alignment: .bottomTrailing) {
                SongArtwork(url: clip.artworkURL)
                    .frame(width: 56, height: 56)

                Button(action: primaryAction) {
                    Image(systemName: isCurrent && isPlaying ? "pause.fill" : "play.fill")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundStyle(.white)
                        .frame(width: 22, height: 22)
                        .background(Circle().fill(Theme.accent))
                        .overlay(Circle().strokeBorder(Theme.surface, lineWidth: 2))
                }
                .offset(x: 4, y: 4)
            }

            Text(clip.title)
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(isCurrent ? Theme.accent : Theme.ink)
                .lineLimit(2)
                .multilineTextAlignment(.leading)
                .padding(.horizontal, 10)

            Spacer(minLength: 0)
        }
        .frame(height: 56)
        .background(isCurrent ? Theme.accent.opacity(0.12) : Theme.surface)
        .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
        .shadow(color: .black.opacity(0.08), radius: 6, y: 3)
        .contentShape(Rectangle())
        .onTapGesture(perform: primaryAction)
    }

    private func primaryAction() {
        if isCurrent { onToggle() } else { onSelect() }
    }
}

/// Playlist shelf tile — tapping the cover/title opens the playlist, the
/// small rounded button in the corner starts (or pauses) playback directly
/// without navigating, same split Spotify uses.
private struct PlaylistCard: View {
    let playlist: Playlist
    let songs: [SavedClip]
    let isCurrent: Bool
    let isPlaying: Bool
    let onOpen: () -> Void
    let onToggle: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            ZStack(alignment: .bottomTrailing) {
                PlaylistCoverView(songs: songs, size: 140)
                    .shadow(color: .black.opacity(0.18), radius: 10, y: 6)

                Button(action: onToggle) {
                    Image(systemName: isCurrent && isPlaying ? "pause.fill" : "play.fill")
                        .font(.system(size: 16, weight: .bold))
                        .foregroundStyle(.white)
                        .frame(width: 40, height: 40)
                        .background(Circle().fill(Theme.accent))
                        .shadow(color: .black.opacity(0.25), radius: 6, y: 3)
                }
                .offset(x: -6, y: -6)
            }

            Text(playlist.name)
                .font(.system(size: 14, weight: .bold))
                .foregroundStyle(isCurrent ? Theme.accent : Theme.ink)
                .lineLimit(1)
                .frame(width: 140, alignment: .leading)
            Text("\(songs.count) songs")
                .font(.system(size: 12))
                .foregroundStyle(Theme.inkSecondary)
        }
        .contentShape(Rectangle())
        .onTapGesture(perform: onOpen)
    }
}

struct SongArtwork: View {
    let url: URL?
    @State private var image: UIImage?

    var body: some View {
        Group {
            if let image {
                Image(uiImage: image).resizable().aspectRatio(contentMode: .fill)
            } else {
                Theme.background.overlay(Image(systemName: "music.note").foregroundStyle(Theme.inkTertiary))
            }
        }
        .task {
            guard image == nil, let url, let data = try? Data(contentsOf: url) else { return }
            image = UIImage(data: data)
        }
    }
}

/// Playlist "cover art" is composed from its own tracks rather than asking
/// the user to pick one — one full cover for 1 or 3 songs (no clean split),
/// a 50/50 side-by-side for exactly 2, and a 2x2 grid of the first 4 once
/// there are 4 or more. Mirrors how Spotify auto-generates playlist art.
struct PlaylistCoverView: View {
    let songs: [SavedClip]
    let size: CGFloat

    var body: some View {
        content
            .frame(width: size, height: size)
            .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
    }

    @ViewBuilder
    private var content: some View {
        switch songs.count {
        case 0:
            Theme.surface.overlay(
                Image(systemName: "music.note.list")
                    .font(.system(size: size * 0.3))
                    .foregroundStyle(Theme.accent)
            )
        case 2:
            HStack(spacing: 0) {
                tile(songs[0], width: size / 2, height: size)
                tile(songs[1], width: size / 2, height: size)
            }
        case let n where n >= 4:
            VStack(spacing: 0) {
                HStack(spacing: 0) {
                    tile(songs[0], width: size / 2, height: size / 2)
                    tile(songs[1], width: size / 2, height: size / 2)
                }
                HStack(spacing: 0) {
                    tile(songs[2], width: size / 2, height: size / 2)
                    tile(songs[3], width: size / 2, height: size / 2)
                }
            }
        default:
            // 1 or 3 songs — no clean split, so the top (first-added) track's
            // cover stands in for the whole playlist.
            tile(songs[0], width: size, height: size)
        }
    }

    private func tile(_ clip: SavedClip, width: CGFloat, height: CGFloat) -> some View {
        SongArtwork(url: clip.artworkURL)
            .frame(width: width, height: height)
            .clipped()
    }
}

/// Presented from a song's "..." menu — pick an existing playlist or spin up
/// a new one, either way the clip lands in it and this sheet closes.
struct AddToPlaylistSheet: View {
    let clip: SavedClip
    @ObservedObject var library: LibraryStore
    @ObservedObject private var playlists = PlaylistStore.shared
    @Environment(\.dismiss) private var dismiss
    @State private var showingNewPlaylist = false
    @State private var newName = ""

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()
            VStack(spacing: 0) {
                HStack {
                    Text("Add to Playlist")
                        .font(.system(size: 18, weight: .bold, design: .rounded))
                        .foregroundStyle(Theme.ink)
                    Spacer()
                    Button("Done") { dismiss() }
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(Theme.accent)
                }
                .padding(20)

                ScrollView {
                    VStack(spacing: 10) {
                        Button {
                            showingNewPlaylist = true
                        } label: {
                            HStack(spacing: 12) {
                                Image(systemName: "plus.circle.fill")
                                    .font(.system(size: 20))
                                    .foregroundStyle(Theme.accent)
                                Text("New Playlist")
                                    .font(.system(size: 15, weight: .semibold))
                                    .foregroundStyle(Theme.ink)
                                Spacer()
                            }
                            .padding(14)
                            .background(RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous).fill(Theme.surface))
                            .overlay(
                                RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous)
                                    .strokeBorder(Theme.hairline, lineWidth: 1)
                            )
                        }
                        .buttonStyle(.plain)

                        ForEach(playlists.playlists) { playlist in
                            let alreadyIn = playlist.clipIDs.contains(clip.id)
                            Button {
                                playlists.add(clip, to: playlist)
                                dismiss()
                            } label: {
                                HStack {
                                    Text(playlist.name)
                                        .font(.system(size: 15, weight: .medium))
                                        .foregroundStyle(Theme.ink)
                                    Spacer()
                                    Text("\(playlists.clips(in: playlist, library: library).count)")
                                        .font(.system(size: 13))
                                        .foregroundStyle(Theme.inkSecondary)
                                    if alreadyIn {
                                        Image(systemName: "checkmark.circle.fill")
                                            .foregroundStyle(Theme.accent)
                                    }
                                }
                                .padding(14)
                                .background(RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous).fill(Theme.surface))
                                .overlay(
                                    RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous)
                                        .strokeBorder(Theme.hairline, lineWidth: 1)
                                )
                            }
                            .buttonStyle(.plain)
                        }
                    }
                    .padding(.horizontal, 20)
                    .padding(.bottom, 20)
                }
            }
        }
        .alert("New Playlist", isPresented: $showingNewPlaylist) {
            TextField("Playlist name", text: $newName)
            Button("Cancel", role: .cancel) { newName = "" }
            Button("Create") {
                let name = newName.trimmingCharacters(in: .whitespacesAndNewlines)
                if !name.isEmpty {
                    let p = playlists.create(name: name)
                    playlists.add(clip, to: p)
                }
                newName = ""
                dismiss()
            }
        }
    }
}
