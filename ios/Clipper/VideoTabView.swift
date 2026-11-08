import SwiftUI
import AVKit
import AVFoundation

/// The Videos tab: every downloaded whole video. Music lives in its own tab,
/// and a split video's parts live grouped together in the Split tab instead
/// of showing up here as N unrelated rows.
struct VideoTabView: View {
    @ObservedObject var library: LibraryStore
    @ObservedObject private var player = PlayerModel.shared
    @State private var showingSettings = false

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
        .sheet(isPresented: $showingSettings) {
            SettingsView()
        }
    }

    private func play(_ clip: SavedClip) {
        // A video is its own one-item queue — playing through the same
        // shared player slot music uses, so starting it stops whatever song
        // was playing.
        player.play(clips: [clip], startIndex: 0, initialMode: .video)
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
            Button {
                showingSettings = true
            } label: {
                Image(systemName: "gearshape.fill")
                    .font(.system(size: 14, weight: .bold))
                    .foregroundStyle(Theme.inkSecondary)
                    .frame(width: 34, height: 34)
                    .background(Circle().fill(Theme.surface))
                    .overlay(Circle().strokeBorder(Theme.hairline, lineWidth: 1))
            }
            .accessibilityLabel("Settings")
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
                Button {
                    Task { await CloudStorageManager.shared.upload(clip: clip) }
                } label: {
                    Label("Save to Cloud (\(CloudStorageManager.shared.preferredDestination.shortName))", systemImage: "icloud.and.arrow.up")
                }
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
        let asset = AVURLAsset(url: url)
        let generator = AVAssetImageGenerator(asset: asset)
        generator.appliesPreferredTrackTransform = true
        generator.requestedTimeToleranceBefore = CMTime(seconds: 2, preferredTimescale: 600)
        generator.requestedTimeToleranceAfter = CMTime(seconds: 2, preferredTimescale: 600)
        let time = CMTime(seconds: 0.1, preferredTimescale: 600)
        if let (cgImage, _) = try? await generator.image(at: time) {
            image = UIImage(cgImage: cgImage)
        } else if let (cgImage, _) = try? await generator.image(at: .zero) {
            image = UIImage(cgImage: cgImage)
        }
    }
}

/// Unified full-screen player with YouTube Music-style Song / Video switcher.
/// For videos, users can toggle between full-screen Song mode (album art & lyrics style)
/// and Video mode (inline live video playback) seamlessly without interrupting playback.
struct NowPlayingSheet: View {
    @ObservedObject private var model = PlayerModel.shared
    /// Which way the outgoing/incoming artwork+title slide on track change
    @State private var removalEdge: Edge = .leading
    @State private var insertionEdge: Edge = .trailing

    var body: some View {
        ZStack {
            backdrop

            if let clip = model.currentClip {
                VStack(spacing: 0) {
                    header(clip: clip)

                    Spacer(minLength: 8)

                    centerVisual(clip: clip)
                        .id(clip.id)
                        .transition(
                            .asymmetric(
                                insertion: .move(edge: insertionEdge).combined(with: .opacity),
                                removal: .move(edge: removalEdge).combined(with: .opacity)
                            )
                        )

                    Spacer(minLength: 8)

                    trackInfo(clip: clip)

                    controlsSection

                    Spacer(minLength: 12)
                }
            }
        }
        .gesture(
            DragGesture(minimumDistance: 30)
                .onEnded { value in
                    let h = value.translation.width
                    let v = value.translation.height
                    if abs(v) > abs(h) {
                        if v > 0 { model.isExpanded = false }
                        return
                    }
                    if let clip = model.currentClip, !clip.isAudio {
                        UIImpactFeedbackGenerator(style: .medium).impactOccurred()
                        withAnimation(.spring(response: 0.35, dampingFraction: 0.8)) {
                            if h < 0 {
                                model.displayMode = (model.displayMode == .audio) ? .video : .audio
                            } else {
                                model.displayMode = (model.displayMode == .video) ? .audio : .video
                            }
                        }
                    } else {
                        skip(h > 0 ? -1 : 1)
                    }
                }
        )
    }

    // MARK: - Header with Audio / Video Switcher

    private func header(clip: SavedClip) -> some View {
        HStack(spacing: 0) {
            Button { model.isExpanded = false } label: {
                Image(systemName: "chevron.down")
                    .font(.system(size: 16, weight: .bold))
                    .foregroundStyle(.white)
                    .frame(width: 36, height: 36)
                    .background(Circle().fill(.white.opacity(0.18)))
            }

            Spacer()

            if !clip.isAudio {
                modeSwitcher
            } else {
                Text("Now Playing")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(.white.opacity(0.85))
            }

            Spacer()

            Menu {
                Button {
                    Task { await CloudStorageManager.shared.upload(clip: clip) }
                } label: {
                    Label("Save to Cloud (\(CloudStorageManager.shared.preferredDestination.shortName))", systemImage: "icloud.and.arrow.up")
                }
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
            } label: {
                Image(systemName: "ellipsis")
                    .font(.system(size: 16, weight: .bold))
                    .foregroundStyle(.white)
                    .frame(width: 36, height: 36)
                    .background(Circle().fill(.white.opacity(0.18)))
            }
        }
        .padding(.horizontal, 20)
        .padding(.top, 20)
    }

    private var modeSwitcher: some View {
        HStack(spacing: 0) {
            ForEach(PlayerModel.PlaybackDisplayMode.allCases, id: \.self) { mode in
                let isSelected = model.displayMode == mode
                Button {
                    withAnimation(.spring(response: 0.35, dampingFraction: 0.8)) {
                        model.displayMode = mode
                    }
                } label: {
                    HStack(spacing: 5) {
                        Image(systemName: mode == .audio ? "music.note" : "video.fill")
                            .font(.system(size: 11, weight: .bold))
                        Text(mode.rawValue)
                            .font(.system(size: 13, weight: .bold))
                    }
                    .foregroundStyle(isSelected ? .white : .white.opacity(0.75))
                    .padding(.horizontal, 14)
                    .padding(.vertical, 6)
                    .background(
                        Capsule().fill(isSelected ? model.artworkPalette.primaryAccent : Color.clear)
                    )
                    .shadow(color: isSelected ? model.artworkPalette.primaryAccent.opacity(0.45) : .clear, radius: 8, y: 2)
                }
            }
        }
        .padding(3)
        .background(Capsule().fill(Color.white.opacity(0.18)))
    }

    // MARK: - Center Visual (Artwork vs Video)

    @ViewBuilder
    private func centerVisual(clip: SavedClip) -> some View {
        Group {
            if model.displayMode == .video && !clip.isAudio {
                videoSurface
                    .transition(.asymmetric(
                        insertion: .opacity.combined(with: .scale(scale: 0.95)),
                        removal: .opacity.combined(with: .scale(scale: 0.95))
                    ))
            } else {
                artworkTile
                    .transition(.asymmetric(
                        insertion: .opacity.combined(with: .scale(scale: 0.95)),
                        removal: .opacity.combined(with: .scale(scale: 0.95))
                    ))
            }
        }
        .animation(.spring(response: 0.35, dampingFraction: 0.8), value: model.displayMode)
        .contentShape(Rectangle())
        .gesture(
            DragGesture(minimumDistance: 20)
                .onEnded { value in
                    let h = value.translation.width
                    let v = value.translation.height
                    if abs(h) > abs(v) && abs(h) > 25 {
                        if !clip.isAudio {
                            UIImpactFeedbackGenerator(style: .medium).impactOccurred()
                            withAnimation(.spring(response: 0.35, dampingFraction: 0.8)) {
                                if h < 0 {
                                    model.displayMode = (model.displayMode == .audio) ? .video : .audio
                                } else {
                                    model.displayMode = (model.displayMode == .video) ? .audio : .video
                                }
                            }
                        } else {
                            skip(h > 0 ? -1 : 1)
                        }
                    } else if abs(v) > abs(h) && v > 50 {
                        model.isExpanded = false
                    }
                }
        )
    }

    private var videoSurface: some View {
        ZStack {
            if let avPlayer = model.avPlayer {
                CustomVideoPlayerView(player: avPlayer)
                    .aspectRatio(16/9, contentMode: .fit)
                    .background(Color.black)
                    .clipShape(RoundedRectangle(cornerRadius: 20, style: .continuous))
                    .overlay(
                        RoundedRectangle(cornerRadius: 20, style: .continuous)
                            .strokeBorder(Color.white.opacity(0.12), lineWidth: 1)
                    )
                    .shadow(color: .black.opacity(0.4), radius: 24, y: 10)
                    .contentShape(Rectangle())
                    .onTapGesture {
                        model.togglePlay()
                    }
            } else {
                RoundedRectangle(cornerRadius: 20, style: .continuous)
                    .fill(Color.black.opacity(0.5))
                    .aspectRatio(16/9, contentMode: .fit)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.horizontal, 20)
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
        .frame(width: 240, height: 240)
        .clipShape(RoundedRectangle(cornerRadius: 28, style: .continuous))
        .shadow(color: model.artworkPalette.primaryAccent.opacity(model.isPlaying ? 0.35 : 0.15), radius: 24, y: 12)
        .scaleEffect(model.isPlaying ? 1.0 : 0.94)
        .animation(.spring(response: 0.35, dampingFraction: 0.75), value: model.isPlaying)
        .contentShape(Rectangle())
        .onTapGesture {
            model.togglePlay()
        }
        .task(id: model.currentClip?.id) {
            if model.artwork == nil {
                await model.loadArtwork()
            }
        }
    }

    // MARK: - Track Info

    private func trackInfo(clip: SavedClip) -> some View {
        VStack(spacing: 4) {
            Text(clip.title)
                .font(.system(size: 18, weight: .bold))
                .foregroundStyle(.white)
                .multilineTextAlignment(.center)
                .lineLimit(2)
                .frame(maxWidth: .infinity)
                .padding(.horizontal, 28)

            if let groupTitle = clip.groupTitle, !groupTitle.isEmpty {
                Text(groupTitle)
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(.white.opacity(0.7))
            }
        }
        .padding(.top, model.displayMode == .video ? 14 : 20)
    }

    // MARK: - Controls

    private var controlsSection: some View {
        VStack(spacing: 0) {
            // Scrubber
            VStack(spacing: 6) {
                Slider(
                    value: $model.currentTime,
                    in: 0...max(model.duration, 1),
                    onEditingChanged: { editing in
                        model.isScrubbing = editing
                        if !editing { model.seek(to: model.currentTime) }
                    }
                )
                .tint(model.artworkPalette.primaryAccent)

                HStack {
                    Text(timeString(model.currentTime))
                    Spacer()
                    Text(timeString(model.duration))
                }
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(.white.opacity(0.7))
            }
            .padding(.horizontal, 32)
            .padding(.top, 18)

            // Volume control
            VolumeControl(tint: model.artworkPalette.uiAccent)
                .padding(.horizontal, 32)
                .padding(.top, 16)

            // Transport buttons
            HStack(spacing: 22) {
                Button(action: model.toggleShuffle) {
                    Image(systemName: "shuffle")
                        .font(.system(size: 16, weight: .bold))
                        .foregroundStyle(model.isShuffling ? model.artworkPalette.primaryAccent : .white.opacity(0.55))
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
                        .background(Circle().fill(model.artworkPalette.primaryAccent))
                        .shadow(color: model.artworkPalette.primaryAccent.opacity(0.45), radius: 14, y: 6)
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
                        .foregroundStyle(model.repeatMode == .off ? .white.opacity(0.55) : model.artworkPalette.primaryAccent)
                        .frame(width: 36, height: 36)
                }
            }
            .padding(.top, 24)
            .padding(.bottom, 32)
        }
    }

    private func skip(_ delta: Int) {
        removalEdge = delta > 0 ? .leading : .trailing
        insertionEdge = delta > 0 ? .trailing : .leading
        withAnimation(.easeInOut(duration: 0.28)) {
            model.skip(by: delta)
        }
    }

    private var backdrop: some View {
        GeometryReader { geo in
            ZStack {
                // Deep dark canvas
                Color(red: 0.05, green: 0.05, blue: 0.07)

                // Smooth dynamic vertical ambient gradient matching cover art
                LinearGradient(
                    colors: [
                        model.artworkPalette.gradientTop.opacity(0.85),
                        model.artworkPalette.gradientMid.opacity(0.45),
                        model.artworkPalette.gradientBottom.opacity(0.95),
                        Color(red: 0.05, green: 0.05, blue: 0.07)
                    ],
                    startPoint: .top,
                    endPoint: .bottom
                )

                // Ambient glow bloom centered behind artwork
                Circle()
                    .fill(model.artworkPalette.primaryAccent.opacity(0.28))
                    .frame(width: geo.size.width * 0.95, height: geo.size.width * 0.95)
                    .blur(radius: 70)
                    .offset(y: -geo.size.height * 0.12)
            }
            .frame(width: geo.size.width, height: geo.size.height)
        }
        .animation(.easeInOut(duration: 0.35), value: model.artworkPalette)
        .ignoresSafeArea()
    }

    private func timeString(_ seconds: Double) -> String {
        guard seconds.isFinite, seconds >= 0 else { return "0:00" }
        let total = Int(seconds)
        return String(format: "%d:%02d", total / 60, total % 60)
    }
}

/// Pure video rendering surface using AVPlayerLayer — displays crisp video frames
/// without any unwanted iOS default playback controls or overlays.
struct CustomVideoPlayerView: UIViewRepresentable {
    let player: AVPlayer

    func makeUIView(context: Context) -> PlayerUIView {
        let view = PlayerUIView()
        view.playerLayer.player = player
        view.playerLayer.videoGravity = .resizeAspect
        view.backgroundColor = .black
        return view
    }

    func updateUIView(_ uiView: PlayerUIView, context: Context) {
        if uiView.playerLayer.player != player {
            uiView.playerLayer.player = player
        }
    }

    final class PlayerUIView: UIView {
        override static var layerClass: AnyClass {
            AVPlayerLayer.self
        }

        var playerLayer: AVPlayerLayer {
            layer as! AVPlayerLayer
        }
    }
}

// Aliases to preserve backward compatibility if referenced elsewhere
typealias VideoNowPlayingSheet = NowPlayingSheet
typealias AudioPlayerSheet = NowPlayingSheet

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
