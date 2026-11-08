import SwiftUI

struct PlaylistDetailView: View {
    let playlist: Playlist
    @ObservedObject var library: LibraryStore
    @ObservedObject private var playlists = PlaylistStore.shared
    @ObservedObject private var player = PlayerModel.shared
    @Environment(\.dismiss) private var dismiss

    @State private var confirmingDelete = false
    @State private var renaming = false
    @State private var renameText = ""

    /// Re-read from the live store rather than closing over the value passed
    /// in — a rename made from this same screen should show up immediately.
    private var current: Playlist {
        playlists.playlists.first { $0.id == playlist.id } ?? playlist
    }

    private var songs: [SavedClip] { playlists.clips(in: current, library: library) }

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()
            VStack(spacing: 0) {
                header
                if songs.isEmpty {
                    Spacer()
                    emptyState
                    Spacer()
                } else {
                    ScrollView {
                        VStack(spacing: 10) {
                            ForEach(songs) { clip in
                                SongRow(
                                    clip: clip,
                                    isCurrent: player.currentClip?.id == clip.id,
                                    isPlaying: player.isPlaying,
                                    onPlay: { play(clip) },
                                    onToggle: { player.togglePlay() }
                                ) {
                                    Button {
                                        Task { await CloudStorageManager.shared.upload(clip: clip) }
                                    } label: {
                                        Label("Save to Cloud (\(CloudStorageManager.shared.preferredDestination.shortName))", systemImage: "icloud.and.arrow.up")
                                    }
                                    Button(role: .destructive) {
                                        playlists.remove(clip, from: current)
                                    } label: {
                                        Label("Remove from Playlist", systemImage: "minus.circle")
                                    }
                                    ShareLink(item: clip.localURL) {
                                        Label("Share", systemImage: "square.and.arrow.up")
                                    }
                                }
                            }
                        }
                        .padding(.horizontal, 20)
                        .padding(.top, 4)
                        .padding(.bottom, 20)
                    }
                }
                if let nowPlaying = player.currentClip, !player.isExpanded {
                    MiniPlayerBar(model: player, clip: nowPlaying)
                        .padding(.horizontal, 20)
                        .padding(.bottom, 12)
                }
            }
        }
        .alert("Rename Playlist", isPresented: $renaming) {
            TextField("Name", text: $renameText)
            Button("Cancel", role: .cancel) {}
            Button("Save") {
                let name = renameText.trimmingCharacters(in: .whitespacesAndNewlines)
                if !name.isEmpty { playlists.rename(current, to: name) }
            }
        }
        .confirmationDialog("Delete this playlist?", isPresented: $confirmingDelete, titleVisibility: .visible) {
            Button("Delete", role: .destructive) {
                playlists.delete(current)
                dismiss()
            }
        }
    }

    private var header: some View {
        HStack {
            Button { dismiss() } label: {
                Image(systemName: "chevron.left")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.ink)
                    .frame(width: 36, height: 36)
                    .background(Circle().fill(Theme.surface))
                    .overlay(Circle().strokeBorder(Theme.hairline, lineWidth: 1))
            }
            Spacer()
            Text(current.name)
                .font(.system(size: 20, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.ink)
                .lineLimit(1)
            Spacer()
            Menu {
                Button {
                    renameText = current.name
                    renaming = true
                } label: {
                    Label("Rename", systemImage: "pencil")
                }
                Button(role: .destructive) {
                    confirmingDelete = true
                } label: {
                    Label("Delete Playlist", systemImage: "trash")
                }
            } label: {
                Image(systemName: "ellipsis")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.ink)
                    .frame(width: 36, height: 36)
                    .background(Circle().fill(Theme.surface))
                    .overlay(Circle().strokeBorder(Theme.hairline, lineWidth: 1))
            }
        }
        .padding(.horizontal, 20)
        .padding(.top, 20)
        .padding(.bottom, 12)
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: "music.note.list")
                .font(.system(size: 30))
                .foregroundStyle(Theme.inkTertiary)
            Text("No audio yet")
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Theme.ink)
            Text("Add audio tracks from the Music tab's \"...\" menu")
                .font(.system(size: 13))
                .foregroundStyle(Theme.inkSecondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 40)
        }
    }

    private func play(_ clip: SavedClip) {
        guard let idx = songs.firstIndex(where: { $0.id == clip.id }) else { return }
        player.play(clips: songs, startIndex: idx)
        player.isExpanded = true
    }
}
