import Foundation
import AVFoundation
import MediaPlayer
import UIKit

/// Owns the single AVPlayer for the app's one "now playing" slot — video or
/// audio, never both at once, exactly like a real media app. Living outside
/// any view is what lets playback survive a sheet being dismissed: minimizing
/// keeps it going and surfaces a mini player, instead of tearing down with
/// the view the way a locally-owned AVPlayer would.
@MainActor
final class PlayerModel: ObservableObject {
    static let shared = PlayerModel()

    enum RepeatMode {
        case off, all, one
    }

    enum PlaybackDisplayMode: String, CaseIterable {
        case audio = "Audio"
        case video = "Video"
    }

    @Published private(set) var clips: [SavedClip] = []
    @Published private(set) var index: Int = 0
    @Published var isPlaying = false
    @Published var currentTime: Double = 0
    @Published var duration: Double = 0
    @Published var artwork: UIImage?
    @Published var artworkPalette: ArtworkPalette = .default
    @Published var isExpanded = false
    @Published private(set) var isShuffling = false
    @Published var repeatMode: RepeatMode = .off
    @Published var displayMode: PlaybackDisplayMode = .audio {
        didSet {
            if displayMode == .audio && artwork == nil {
                Task {
                    await loadArtwork()
                    updateNowPlayingInfo()
                }
            }
        }
    }

    /// Not published — the periodic time observer reads this to avoid
    /// fighting a Slider drag the user has in progress.
    var isScrubbing = false

    private var player: AVPlayer?
    private var timeObserver: Any?
    private var rateObservation: NSKeyValueObservation?
    private var itemEndObserver: NSObjectProtocol?

    /// Indices already visited this shuffle session (most recent last) and
    /// the remaining shuffled draw order — kept separate from `index` so
    /// "previous" under shuffle retraces actual history instead of just
    /// picking another random track.
    private var shuffleHistory: [Int] = []
    private var shuffleUpcoming: [Int] = []

    /// Exposed so the video full-screen view can hand this same player to
    /// `VideoPlayer(player:)` — its native transport controls act on it
    /// directly, which is why isPlaying is mirrored via KVO below rather than
    /// only updated from this class's own methods.
    var avPlayer: AVPlayer? { player }

    var currentClip: SavedClip? { clips.indices.contains(index) ? clips[index] : nil }

    var hasPrevious: Bool {
        if isShuffling { return shuffleHistory.count > 1 }
        if repeatMode == .all { return clips.count > 1 }
        return index > 0
    }

    var hasNext: Bool {
        if isShuffling { return !shuffleUpcoming.isEmpty || (repeatMode == .all && clips.count > 1) }
        if repeatMode == .all { return clips.count > 1 }
        return index < clips.count - 1
    }

    private init() {
        // Default session category (soloAmbient) mutes on backgrounding and
        // respects the silent switch — wrong for something the user explicitly
        // pressed play on. .playback keeps it going behind the lock screen and
        // in other apps, paired with the "audio" UIBackgroundModes entry.
        do {
            let session = AVAudioSession.sharedInstance()
            try session.setCategory(.playback, mode: .default)
            Task.detached(priority: .userInitiated) {
                try? session.setActive(true)
            }
        } catch {
            // Non-fatal: playback just won't survive backgrounding this run.
        }
        configureRemoteCommands()
    }

    func play(clips: [SavedClip], startIndex: Int, initialMode: PlaybackDisplayMode? = nil) {
        guard clips.indices.contains(startIndex) else { return }
        let clip = clips[startIndex]
        if clip.isAudio {
            displayMode = .audio
        } else if let initialMode {
            displayMode = initialMode
        }

        if player != nil, self.clips.map(\.id) == clips.map(\.id), index == startIndex {
            if !isPlaying { player?.play(); isPlaying = true }
            if displayMode == .audio && artwork == nil {
                Task {
                    await loadArtwork()
                    updateNowPlayingInfo()
                }
            }
            return
        }
        self.clips = clips
        index = startIndex
        if isShuffling { reshuffle(keeping: startIndex) }
        load(autoplay: true)
    }

    func toggleShuffle() {
        isShuffling.toggle()
        if isShuffling {
            reshuffle(keeping: index)
        } else {
            shuffleHistory = []
            shuffleUpcoming = []
        }
        updateNowPlayingInfo()
    }

    func cycleRepeatMode() {
        switch repeatMode {
        case .off: repeatMode = .all
        case .all: repeatMode = .one
        case .one: repeatMode = .off
        }
        updateNowPlayingInfo()
    }

    private func reshuffle(keeping current: Int) {
        shuffleHistory = [current]
        shuffleUpcoming = clips.indices.filter { $0 != current }.shuffled()
    }

    func togglePlay() {
        guard let player else { return }
        if isPlaying {
            player.pause()
        } else {
            if let item = player.currentItem, item.currentTime() >= item.duration {
                player.seek(to: .zero)
            }
            player.play()
        }
        isPlaying.toggle()
        updateNowPlayingInfo()
    }

    func skip(by delta: Int, autoplay: Bool? = nil) {
        guard let target = resolveSkipTarget(delta) else { return }
        index = target
        load(autoplay: autoplay ?? isPlaying)
    }

    private func resolveSkipTarget(_ delta: Int) -> Int? {
        guard delta != 0, !clips.isEmpty else { return nil }

        if isShuffling {
            if delta > 0 {
                if let next = shuffleUpcoming.popLast() {
                    shuffleHistory.append(next)
                    return next
                }
                guard repeatMode == .all, clips.count > 1 else { return nil }
                shuffleUpcoming = clips.indices.filter { $0 != index }.shuffled()
                guard let next = shuffleUpcoming.popLast() else { return nil }
                shuffleHistory.append(next)
                return next
            } else {
                guard shuffleHistory.count > 1 else { return nil }
                shuffleHistory.removeLast()
                return shuffleHistory.last
            }
        }

        let linear = index + delta
        if clips.indices.contains(linear) { return linear }
        guard repeatMode == .all, clips.count > 1 else { return nil }
        return delta > 0 ? 0 : clips.count - 1
    }

    func seek(to seconds: Double) {
        player?.seek(to: CMTime(seconds: seconds, preferredTimescale: 600))
        updateNowPlayingInfo()
    }

    /// Fully tears down playback — used when the mini player's close button is
    /// tapped, or when the clip currently playing gets deleted out from under it.
    func stop() {
        if let itemEndObserver {
            NotificationCenter.default.removeObserver(itemEndObserver)
            self.itemEndObserver = nil
        }
        if let timeObserver, let player {
            player.removeTimeObserver(timeObserver)
        }
        player?.pause()
        player = nil
        timeObserver = nil
        rateObservation = nil
        clips = []
        index = 0
        isPlaying = false
        currentTime = 0
        duration = 0
        artwork = nil
        artworkPalette = .default
        isExpanded = false
        MPNowPlayingInfoCenter.default().nowPlayingInfo = nil
    }

    private func load(autoplay: Bool) {
        guard let clip = currentClip else { return }
        if clip.isAudio {
            displayMode = .audio
        }
        currentTime = 0
        duration = 0
        artwork = nil
        artworkPalette = .default

        let item = AVPlayerItem(url: clip.localURL)

        if let itemEndObserver {
            NotificationCenter.default.removeObserver(itemEndObserver)
            self.itemEndObserver = nil
        }

        itemEndObserver = NotificationCenter.default.addObserver(
            forName: .AVPlayerItemDidPlayToEndTime,
            object: item,
            queue: .main
        ) { [weak self] _ in
            Task { @MainActor in
                guard let self else { return }
                if self.repeatMode == .one {
                    self.player?.seek(to: .zero)
                    self.player?.play()
                    return
                }
                if self.hasNext {
                    self.skip(by: 1, autoplay: true)
                } else {
                    self.isPlaying = false
                    self.player?.seek(to: .zero)
                    self.currentTime = 0
                    self.updateNowPlayingInfo()
                }
            }
        }

        if let player {
            player.replaceCurrentItem(with: item)
        } else {
            let p = AVPlayer(playerItem: item)
            player = p
            let interval = CMTime(seconds: 0.25, preferredTimescale: 600)
            timeObserver = p.addPeriodicTimeObserver(forInterval: interval, queue: .main) { [weak self] time in
                Task { @MainActor in
                    guard let self, !self.isScrubbing else { return }
                    self.currentTime = time.seconds
                }
            }
            // Video's full-screen player exposes native transport controls that
            // act on this AVPlayer directly, bypassing togglePlay() — this KVO
            // keeps isPlaying (mini player icon, Now Playing rate) truthful
            // regardless of which controls the user actually touched.
            rateObservation = p.observe(\.timeControlStatus, options: [.new]) { [weak self] player, _ in
                Task { @MainActor in
                    guard let self else { return }
                    self.isPlaying = player.timeControlStatus == .playing
                    self.updateNowPlayingInfo()
                }
            }
        }

        isPlaying = autoplay
        if autoplay { player?.play() }
        updateNowPlayingInfo()

        Task {
            await loadDuration()
            updateNowPlayingInfo()
        }
        Task {
            await loadArtwork()
            updateNowPlayingInfo()
        }
    }

    private func loadDuration() async {
        guard let asset = player?.currentItem?.asset else { return }
        if let seconds = try? await asset.load(.duration).seconds, seconds.isFinite {
            duration = seconds
        }
    }

    func loadArtwork() async {
        guard let clip = currentClip else { return }
        var resolvedImage: UIImage?
        if let url = clip.artworkURL, let data = try? Data(contentsOf: url), let img = UIImage(data: data) {
            resolvedImage = img
        } else if !clip.isAudio {
            let asset = AVURLAsset(url: clip.localURL)
            let generator = AVAssetImageGenerator(asset: asset)
            generator.appliesPreferredTrackTransform = true
            generator.requestedTimeToleranceBefore = CMTime(seconds: 2, preferredTimescale: 600)
            generator.requestedTimeToleranceAfter = CMTime(seconds: 2, preferredTimescale: 600)
            let time = CMTime(seconds: 0.1, preferredTimescale: 600)
            if let (cgImage, _) = try? await generator.image(at: time) {
                resolvedImage = UIImage(cgImage: cgImage)
            } else if let (cgImage, _) = try? await generator.image(at: .zero) {
                resolvedImage = UIImage(cgImage: cgImage)
            }
        }

        self.artwork = resolvedImage
        if let resolvedImage {
            let palette = await ArtworkColorExtractor.extractPalette(from: resolvedImage)
            self.artworkPalette = palette
        } else {
            self.artworkPalette = .default
        }
    }

    // MARK: - Now Playing (lock screen / Control Center / Dynamic Island)

    private func configureRemoteCommands() {
        let center = MPRemoteCommandCenter.shared()

        center.playCommand.addTarget { [weak self] _ in
            guard let self, let player = self.player else { return .commandFailed }
            player.play()
            self.isPlaying = true
            self.updateNowPlayingInfo()
            return .success
        }
        center.pauseCommand.addTarget { [weak self] _ in
            guard let self, let player = self.player else { return .commandFailed }
            player.pause()
            self.isPlaying = false
            self.updateNowPlayingInfo()
            return .success
        }
        center.togglePlayPauseCommand.addTarget { [weak self] _ in
            self?.togglePlay()
            return .success
        }
        center.nextTrackCommand.addTarget { [weak self] _ in
            guard let self, self.hasNext else { return .commandFailed }
            self.skip(by: 1)
            return .success
        }
        center.previousTrackCommand.addTarget { [weak self] _ in
            guard let self, self.hasPrevious else { return .commandFailed }
            self.skip(by: -1)
            return .success
        }
        center.changePlaybackPositionCommand.addTarget { [weak self] event in
            guard let self, let event = event as? MPChangePlaybackPositionCommandEvent else { return .commandFailed }
            self.currentTime = event.positionTime
            self.seek(to: event.positionTime)
            return .success
        }

        // Disabled by default — without this, Control Center's shuffle/repeat
        // toggles are visible but silently do nothing.
        center.changeShuffleModeCommand.isEnabled = true
        center.changeShuffleModeCommand.addTarget { [weak self] event in
            guard let self, let event = event as? MPChangeShuffleModeCommandEvent else { return .commandFailed }
            let wantsShuffle = event.shuffleType != .off
            if wantsShuffle != self.isShuffling { self.toggleShuffle() }
            return .success
        }
        center.changeRepeatModeCommand.isEnabled = true
        center.changeRepeatModeCommand.addTarget { [weak self] event in
            guard let self, let event = event as? MPChangeRepeatModeCommandEvent else { return .commandFailed }
            switch event.repeatType {
            case .off: self.repeatMode = .off
            case .one: self.repeatMode = .one
            case .all: self.repeatMode = .all
            @unknown default: break
            }
            self.updateNowPlayingInfo()
            return .success
        }
    }

    /// Publishes the current track to MPNowPlayingInfoCenter — this is what
    /// actually makes it show up on the lock screen, Control Center, and the
    /// Dynamic Island; without it, background audio plays but stays invisible
    /// to the system.
    private func updateNowPlayingInfo() {
        guard let clip = currentClip else {
            MPNowPlayingInfoCenter.default().nowPlayingInfo = nil
            return
        }

        let (title, artist) = Self.splitTitle(clip.title)

        var info: [String: Any] = [
            MPMediaItemPropertyTitle: title,
            MPMediaItemPropertyPlaybackDuration: duration,
            MPNowPlayingInfoPropertyElapsedPlaybackTime: currentTime,
            MPNowPlayingInfoPropertyPlaybackRate: isPlaying ? 1.0 : 0.0,
        ]
        if let artist { info[MPMediaItemPropertyArtist] = artist }
        if let artwork {
            info[MPMediaItemPropertyArtwork] = MPMediaItemArtwork(boundsSize: artwork.size) { _ in artwork }
        }
        MPNowPlayingInfoCenter.default().nowPlayingInfo = info

        let center = MPRemoteCommandCenter.shared()
        center.nextTrackCommand.isEnabled = hasNext
        center.previousTrackCommand.isEnabled = hasPrevious
        center.changeShuffleModeCommand.currentShuffleType = isShuffling ? .items : .off
        switch repeatMode {
        case .off: center.changeRepeatModeCommand.currentRepeatType = .off
        case .all: center.changeRepeatModeCommand.currentRepeatType = .all
        case .one: center.changeRepeatModeCommand.currentRepeatType = .one
        }
    }

    /// Downloaded titles are often the raw "Artist - Song" string yt-dlp pulled
    /// from the source, crammed into one field — that's what forces the Lock
    /// Screen / Control Center title to marquee-scroll. Splitting on the first
    /// hyphen gives the system two short fields instead of one long one.
    private static func splitTitle(_ raw: String) -> (title: String, artist: String?) {
        guard let range = raw.range(of: " - ") else { return (raw, nil) }
        let artist = raw[..<range.lowerBound].trimmingCharacters(in: .whitespaces)
        let title = raw[range.upperBound...].trimmingCharacters(in: .whitespaces)
        guard !artist.isEmpty, !title.isEmpty else { return (raw, nil) }
        return (title, artist)
    }
}
