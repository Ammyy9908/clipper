import SwiftUI
import AVKit
import AVFoundation

/// The Videos tab: every downloaded whole video. Music lives in its own tab,
/// and a split video's parts live grouped together in the Split tab instead
/// of showing up here as N unrelated rows.
struct VideoTabView: View {
    @ObservedObject var library: LibraryStore
    @ObservedObject private var player = PlayerModel.shared

    private var videos: [SavedClip] { library.clips.filter { !$0.isAudio && !$0.isSplitPart } }

    var body: some View {
        VStack(spacing: 0) {
            header
            autoSaveRow
            if videos.isEmpty {
                Spacer()
                emptyState
                Spacer()
            } else {
                ScrollView {
                    LazyVStack(spacing: 4) {
                        ForEach(videos) { clip in
                            ClipRow(
                                clip: clip,
                                isPlaying: player.currentClip?.id == clip.id,
                                onPlay: { play(clip) },
                                onDelete: { library.remove(clip) }
                            )
                        }
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 4)
                    .padding(.bottom, player.currentClip != nil && !player.isExpanded ? 84 : 20)
                }
            }
        }
    }

    private func play(_ clip: SavedClip) {
        // A video is its own one-item queue — playing through the same
        // shared player slot music uses, so starting it stops whatever song
        // was playing.
        player.play(clips: [clip], startIndex: 0)
        player.isExpanded = true
    }

    private var header: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text("Your Videos")
                    .font(.system(size: 34, weight: .heavy, design: .rounded))
                    .foregroundStyle(Theme.ink)
                Text(videos.isEmpty ? "Nothing here yet" : "\(videos.count) video\(videos.count == 1 ? "" : "s")")
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
            }
            Spacer()
        }
        .padding(.horizontal, 20)
        .padding(.top, 24)
        .padding(.bottom, 16)
    }

    private var autoSaveRow: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text("Auto-save to Photos")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Theme.ink)
                Text("Every video also lands in your photo library")
                    .font(.system(size: 12))
                    .foregroundStyle(Theme.inkSecondary)
            }
            Spacer()
            Toggle("", isOn: $library.autoSave)
                .labelsHidden()
                .tint(Theme.accent)
        }
        .padding(16)
        .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                .strokeBorder(Theme.hairline, lineWidth: 1)
        )
        .padding(.horizontal, 20)
        .padding(.bottom, 16)
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: "square.stack")
                .font(.system(size: 30))
                .foregroundStyle(Theme.inkTertiary)
            Text("No videos yet")
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Theme.ink)
            Text("Videos you save will show up here")
                .font(.system(size: 13))
                .foregroundStyle(Theme.inkSecondary)
        }
    }
}

/// Flat, borderless list row — matches SongRow's Spotify/YT Music styling so
/// Videos and Music read as the same app instead of two different ones.
struct ClipRow: View {
    let clip: SavedClip
    var isPlaying: Bool = false
    let onPlay: () -> Void
    let onDelete: () -> Void

    @State private var confirmingDelete = false

    var body: some View {
        HStack(spacing: 14) {
            Button(action: onPlay) {
                HStack(spacing: 14) {
                    ZStack {
                        Thumbnail(url: clip.localURL, isAudio: clip.isAudio, artworkURL: clip.artworkURL)
                        if isPlaying {
                            Color.black.opacity(0.35)
                            Image(systemName: "waveform")
                                .font(.system(size: 18, weight: .bold))
                                .foregroundStyle(.white)
                        }
                    }
                    .frame(width: 56, height: 56)
                    .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))

                    HStack(spacing: 6) {
                        Text(clip.title)
                            .font(.system(size: 16, weight: isPlaying ? .bold : .semibold))
                            .foregroundStyle(isPlaying ? Theme.accent : Theme.ink)
                            .lineLimit(1)
                            .truncationMode(.middle)
                        if clip.isAudio {
                            Text(clip.fileExtension.uppercased())
                                .font(.system(size: 10, weight: .bold))
                                .foregroundStyle(Theme.accent)
                                .padding(.horizontal, 6)
                                .padding(.vertical, 2)
                                .background(Capsule().fill(Theme.accent.opacity(0.12)))
                                .fixedSize()
                        }
                    }
                }
            }
            .buttonStyle(.plain)

            Spacer()

            Menu {
                if !clip.isAudio {
                    Button {
                        Task { try? await PhotoSaver.addToPhotos(fileURL: clip.localURL) }
                    } label: {
                        Label("Save to Photos", systemImage: "square.and.arrow.down")
                    }
                }
                ShareLink(item: clip.localURL) {
                    Label("Share", systemImage: "square.and.arrow.up")
                }
                Button(role: .destructive) {
                    confirmingDelete = true
                } label: {
                    Label("Delete", systemImage: "trash")
                }
            } label: {
                Image(systemName: "ellipsis")
                    .font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(Theme.inkSecondary)
                    .frame(width: 32, height: 32)
            }
        }
        .padding(.vertical, 6)
        .confirmationDialog(clip.isAudio ? "Delete this MP3?" : "Delete this video?", isPresented: $confirmingDelete, titleVisibility: .visible) {
            Button("Delete", role: .destructive) { onDelete() }
        }
    }
}

struct Thumbnail: View {
    let url: URL
    let isAudio: Bool
    let artworkURL: URL?
    @State private var image: UIImage?

    var body: some View {
        Group {
            if let image {
                Image(uiImage: image).resizable().aspectRatio(contentMode: .fill)
            } else if isAudio {
                Theme.background.overlay(Image(systemName: "music.note").foregroundStyle(Theme.inkTertiary))
            } else {
                Theme.background
            }
        }
        .frame(width: 56, height: 56)
        .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        .task { await generate() }
    }

    private func generate() async {
        guard image == nil else { return }
        if let artworkURL, let data = try? Data(contentsOf: artworkURL), let art = UIImage(data: data) {
            image = art
            return
        }
        guard !isAudio else { return }
        let generator = AVAssetImageGenerator(asset: AVURLAsset(url: url))
        generator.appliesPreferredTrackTransform = true
        guard let cgImage = try? await generator.image(at: .zero).image else { return }
        image = UIImage(cgImage: cgImage)
    }
}

/// Full-screen video, mirroring AudioPlayerSheet's relationship to the shared
/// player: the chevron minimizes rather than stops, so video behaves exactly
/// like audio — keep playing, drop to a mini player, tap back in.
struct VideoNowPlayingSheet: View {
    @ObservedObject private var model = PlayerModel.shared

    var body: some View {
        ZStack(alignment: .topLeading) {
            Color.black.ignoresSafeArea()
            if let avPlayer = model.avPlayer {
                VideoPlayer(player: avPlayer)
                    .ignoresSafeArea()
            }

            Button { model.isExpanded = false } label: {
                Image(systemName: "chevron.down")
                    .font(.system(size: 15, weight: .bold))
                    .foregroundStyle(.white)
                    .frame(width: 32, height: 32)
                    .background(Circle().fill(.black.opacity(0.5)))
            }
            .padding(.top, 16)
            .padding(.leading, 16)
        }
        .gesture(
            DragGesture(minimumDistance: 30)
                .onEnded { value in
                    guard value.translation.height > abs(value.translation.width) else { return }
                    model.isExpanded = false
                }
        )
    }
}

/// The full-screen now-playing UI. Reads PlayerModel.shared rather than
/// owning an AVPlayer itself — the down-chevron just collapses this sheet
/// (`isExpanded = false`), it never stops playback the way dismissing an
/// owned player would.
struct AudioPlayerSheet: View {
    @ObservedObject private var model = PlayerModel.shared
    /// Which way the outgoing/incoming artwork+title slide — set right
    /// before a skip so the transition reads as "sliding to" the new track
    /// rather than an instant swap.
    @State private var removalEdge: Edge = .leading
    @State private var insertionEdge: Edge = .trailing

    var body: some View {
        ZStack {
            backdrop

            if let clip = model.currentClip {
                VStack(spacing: 0) {
                    HStack {
                        Button { model.isExpanded = false } label: {
                            Image(systemName: "chevron.down")
                                .font(.system(size: 17, weight: .semibold))
                                .foregroundStyle(.white)
                                .frame(width: 36, height: 36)
                                .background(Circle().fill(.white.opacity(0.18)))
                        }
                        Spacer()
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 20)

                    Spacer()

                    VStack(spacing: 0) {
                        artworkTile

                        Text(clip.title)
                            .font(.system(size: 18, weight: .semibold))
                            .foregroundStyle(.white)
                            .multilineTextAlignment(.center)
                            .lineLimit(2)
                            .frame(maxWidth: .infinity)
                            .padding(.horizontal, 32)
                            .padding(.top, 28)
                    }
                    .id(clip.id)
                    .transition(
                        .asymmetric(
                            insertion: .move(edge: insertionEdge).combined(with: .opacity),
                            removal: .move(edge: removalEdge).combined(with: .opacity)
                        )
                    )

                    Spacer()

                    VStack(spacing: 6) {
                        Slider(
                            value: $model.currentTime,
                            in: 0...max(model.duration, 1),
                            onEditingChanged: { editing in
                                model.isScrubbing = editing
                                if !editing { model.seek(to: model.currentTime) }
                            }
                        )
                        .tint(.white)

                        HStack {
                            Text(timeString(model.currentTime))
                            Spacer()
                            Text(timeString(model.duration))
                        }
                        .font(.system(size: 12))
                        .foregroundStyle(.white.opacity(0.7))
                    }
                    .padding(.horizontal, 32)
                    .padding(.top, 24)

                    VolumeControl()
                        .padding(.horizontal, 32)
                        .padding(.top, 20)

                    HStack(spacing: 22) {
                        Button(action: model.toggleShuffle) {
                            Image(systemName: "shuffle")
                                .font(.system(size: 16, weight: .bold))
                                .foregroundStyle(model.isShuffling ? Theme.accent : .white.opacity(0.55))
                                .frame(width: 36, height: 36)
                        }

                        Button { skip(-1) } label: {
                            Image(systemName: "backward.fill")
                                .font(.system(size: 20, weight: .bold))
                                .foregroundStyle(.white.opacity(model.hasPrevious ? 1 : 0.35))
                                .frame(width: 44, height: 44)
                        }
                        .disabled(!model.hasPrevious)

                        Button(action: model.togglePlay) {
                            Image(systemName: model.isPlaying ? "pause.fill" : "play.fill")
                                .font(.system(size: 26, weight: .bold))
                                .foregroundStyle(.white)
                                .frame(width: 64, height: 64)
                                .background(Circle().fill(Theme.accent))
                        }

                        Button { skip(1) } label: {
                            Image(systemName: "forward.fill")
                                .font(.system(size: 20, weight: .bold))
                                .foregroundStyle(.white.opacity(model.hasNext ? 1 : 0.35))
                                .frame(width: 44, height: 44)
                        }
                        .disabled(!model.hasNext)

                        Button(action: model.cycleRepeatMode) {
                            Image(systemName: model.repeatMode == .one ? "repeat.1" : "repeat")
                                .font(.system(size: 16, weight: .bold))
                                .foregroundStyle(model.repeatMode == .off ? .white.opacity(0.55) : Theme.accent)
                                .frame(width: 36, height: 36)
                        }
                    }
                    .padding(.top, 28)
                    .padding(.bottom, 40)
                }
            }
        }
        // skip(by:) already no-ops past either end of the queue, so there's
        // nothing extra to guard here for "no next/previous".
        .gesture(
            DragGesture(minimumDistance: 30)
                .onEnded { value in
                    let h = value.translation.width
                    let v = value.translation.height
                    if abs(v) > abs(h) {
                        if v > 0 { model.isExpanded = false }
                        return
                    }
                    skip(h > 0 ? -1 : 1)
                }
        )
    }

    /// Sets which edges the outgoing/incoming track slide from based on
    /// direction, then performs the skip inside the same animation block —
    /// used by both the swipe gesture and the transport buttons, so either
    /// way of switching tracks gets the same slide instead of an instant cut.
    private func skip(_ delta: Int) {
        removalEdge = delta > 0 ? .leading : .trailing
        insertionEdge = delta > 0 ? .trailing : .leading
        withAnimation(.easeInOut(duration: 0.28)) {
            model.skip(by: delta)
        }
    }

    /// A heavily blurred, darkened fill of the same artwork shown sharp in
    /// the center — same trick Apple Music and Spotify use so the now-playing
    /// screen always feels tied to the track instead of the app's own theme.
    private var backdrop: some View {
        GeometryReader { geo in
            Group {
                if let artwork = model.artwork {
                    Image(uiImage: artwork)
                        .resizable()
                        .aspectRatio(contentMode: .fill)
                        .frame(width: geo.size.width, height: geo.size.height)
                        .clipped()
                        .blur(radius: 50)
                        .overlay(Color.black.opacity(0.35))
                } else {
                    Theme.background
                }
            }
            .frame(width: geo.size.width, height: geo.size.height)
        }
        .ignoresSafeArea()
    }

    private var artworkTile: some View {
        Group {
            if let artwork = model.artwork {
                Image(uiImage: artwork)
                    .resizable()
                    .aspectRatio(contentMode: .fill)
            } else {
                ZStack {
                    Color.white.opacity(0.12)
                    Image(systemName: "music.note")
                        .font(.system(size: 64))
                        .foregroundStyle(.white)
                }
            }
        }
        .frame(width: 220, height: 220)
        .clipShape(RoundedRectangle(cornerRadius: 28, style: .continuous))
        .shadow(color: .black.opacity(0.3), radius: 24, y: 12)
    }

    private func timeString(_ seconds: Double) -> String {
        guard seconds.isFinite, seconds >= 0 else { return "0:00" }
        let total = Int(seconds)
        return String(format: "%d:%02d", total / 60, total % 60)
    }
}

/// Compact bar shown wherever the library is visible while a track plays in
/// the background — tap it to reopen AudioPlayerSheet full-screen.
struct MiniPlayerBar: View {
    @ObservedObject var model: PlayerModel
    let clip: SavedClip
    /// Which way the outgoing/incoming artwork+title slide — same convention
    /// as AudioPlayerSheet's transition, set right before a skip.
    @State private var removalEdge: Edge = .leading
    @State private var insertionEdge: Edge = .trailing

    var body: some View {
        HStack(spacing: 12) {
            HStack(spacing: 12) {
                artwork
                    .frame(width: 36, height: 36)
                    .clipShape(RoundedRectangle(cornerRadius: 8, style: .continuous))

                Text(clip.title)
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.ink)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            .id(clip.id)
            .transition(
                .asymmetric(
                    insertion: .move(edge: insertionEdge).combined(with: .opacity),
                    removal: .move(edge: removalEdge).combined(with: .opacity)
                )
            )

            Spacer()

            Button { model.togglePlay() } label: {
                Image(systemName: model.isPlaying ? "pause.fill" : "play.fill")
                    .font(.system(size: 16, weight: .bold))
                    .foregroundStyle(Theme.ink)
                    .frame(width: 32, height: 32)
            }

            Button { model.stop() } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 13, weight: .bold))
                    .foregroundStyle(Theme.inkSecondary)
                    .frame(width: 28, height: 28)
            }
        }
        .padding(10)
        .background(RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous).fill(Theme.surface))
        // Clips the sliding artwork/title to the bar's own rounded bounds so
        // it doesn't visually spill past the edges mid-transition.
        .clipShape(RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous)
                .strokeBorder(Theme.hairline, lineWidth: 1)
        )
        .shadow(color: .black.opacity(0.08), radius: 12, y: 4)
        .contentShape(Rectangle())
        .onTapGesture { model.isExpanded = true }
        .gesture(
            DragGesture(minimumDistance: 30)
                .onEnded { value in
                    let h = value.translation.width
                    let v = value.translation.height
                    if abs(v) > abs(h) {
                        if v < 0 { model.isExpanded = true }
                        return
                    }
                    skip(h > 0 ? -1 : 1)
                }
        )
    }

    private func skip(_ delta: Int) {
        removalEdge = delta > 0 ? .leading : .trailing
        insertionEdge = delta > 0 ? .trailing : .leading
        withAnimation(.easeInOut(duration: 0.28)) {
            model.skip(by: delta)
        }
    }

    private var artwork: some View {
        Group {
            if let artwork = model.artwork {
                Image(uiImage: artwork).resizable().aspectRatio(contentMode: .fill)
            } else {
                Theme.background.overlay(
                    Image(systemName: "music.note")
                        .font(.system(size: 14))
                        .foregroundStyle(Theme.inkTertiary)
                )
            }
        }
    }
}
