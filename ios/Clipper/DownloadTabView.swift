import SwiftUI

/// The Download tab: paste a link, resolve it, pick a format, save.
/// Displays active in-flight downloads with live progress indicators, and provides
/// a streamlined way to download additional links concurrently.
struct DownloadTabView: View {
    @ObservedObject var clipboard: ClipboardWatcher
    @ObservedObject var model: DownloadModel
    @ObservedObject private var queue = DownloadQueue.shared
    @ObservedObject private var userSession = UserSession.shared
    @State private var showingJobs = false
    @State private var showingPlatforms = false
    @State private var showingSettings = false

    var body: some View {
        VStack(spacing: 0) {
            header
            Spacer()
            content
            Spacer()
        }
        .padding(.horizontal, 24)
        .padding(.top, 20)
        .padding(.bottom, 16)
        .animation(.spring(response: 0.35, dampingFraction: 0.8), value: model.phase)
        .sheet(isPresented: $showingJobs) {
            DownloadJobsSheet(queue: queue)
        }
        .sheet(isPresented: $showingPlatforms) {
            OnboardingView {
                showingPlatforms = false
            }
        }
        .sheet(isPresented: $showingSettings) {
            SettingsView()
        }
    }

    // MARK: - Header

    private var header: some View {
        HStack(alignment: .center) {
            VStack(alignment: .leading, spacing: 4) {
                Text("clipPlay")
                    .font(.system(size: 30, weight: .bold, design: .rounded))
                    .foregroundStyle(Theme.ink)
                Text(subtitle)
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
            }
            Spacer(minLength: 8)
            HStack(spacing: 8) {
                quotaBadge

                Button {
                    showingPlatforms = true
                } label: {
                    Image(systemName: "sparkles")
                        .font(.system(size: 14, weight: .bold))
                        .foregroundStyle(Theme.accent)
                        .frame(width: 36, height: 36)
                        .background(Circle().fill(Theme.surface))
                        .overlay(Circle().strokeBorder(Theme.hairline, lineWidth: 1))
                }
                .accessibilityLabel("App Guide & Supported Platforms")

                Button {
                    showingSettings = true
                } label: {
                    Image(systemName: "gearshape.fill")
                        .font(.system(size: 14, weight: .bold))
                        .foregroundStyle(Theme.inkSecondary)
                        .frame(width: 36, height: 36)
                        .background(Circle().fill(Theme.surface))
                        .overlay(Circle().strokeBorder(Theme.hairline, lineWidth: 1))
                }
                .accessibilityLabel("Settings")

                if !queue.jobs.isEmpty {
                    jobsBadge
                }
            }
            .fixedSize(horizontal: true, vertical: true)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.bottom, 8)
    }

    private var quotaBadge: some View {
        Button {
            showingSettings = true
        } label: {
            HStack(spacing: 5) {
                Image(systemName: userSession.isSignedIn ? "bolt.fill" : "bolt")
                    .font(.system(size: 11, weight: .bold))
                    .foregroundStyle(Theme.accent)
                let remaining = userSession.quota?.remaining ?? (userSession.isSignedIn ? 20 : 3)
                let limit = userSession.quota?.limit ?? (userSession.isSignedIn ? 20 : 3)
                Text("\(remaining)/\(limit)")
                    .font(.system(size: 12, weight: .bold, design: .rounded))
                    .foregroundStyle(Theme.ink)
            }
            .padding(.horizontal, 10)
            .frame(height: 36)
            .background(Capsule().fill(Theme.surface))
            .overlay(Capsule().strokeBorder(Theme.hairline, lineWidth: 1))
        }
        .fixedSize(horizontal: true, vertical: true)
        .accessibilityLabel("Daily Download Quota: \(userSession.quota?.remaining ?? 3) remaining")
    }

    private var activeJobCount: Int {
        queue.activeJobs.count
    }

    private var jobsBadge: some View {
        Button { showingJobs = true } label: {
            HStack(spacing: 6) {
                if activeJobCount > 0 {
                    ProgressView().controlSize(.small).tint(Theme.accent)
                    Text("\(activeJobCount)")
                } else {
                    Image(systemName: "checkmark")
                        .font(.system(size: 11, weight: .bold))
                    Text("\(queue.jobs.count)")
                }
            }
            .font(.system(size: 12, weight: .bold, design: .rounded))
            .foregroundStyle(Theme.ink)
            .padding(.horizontal, 10)
            .frame(height: 36)
            .background(Capsule().fill(Theme.surface))
            .overlay(Capsule().strokeBorder(Theme.hairline, lineWidth: 1))
        }
        .fixedSize(horizontal: true, vertical: true)
    }

    private var subtitle: String {
        if activeJobCount > 0 && model.phase == .idle {
            return activeJobCount == 1 ? "Downloading 1 video…" : "Downloading \(activeJobCount) videos…"
        }
        switch model.phase {
        case .idle: return clipboard.hasLink ? "A link is ready to paste" : "Copy a video link to get started"
        case .resolving: return "Reading the link…"
        case .ready: return "Ready to save"
        case .failed: return "Something went wrong"
        }
    }

    // MARK: - Card

    private func card<Content: View>(@ViewBuilder content: () -> Content) -> some View {
        VStack(spacing: 18) { content() }
            .padding(24)
            .frame(maxWidth: .infinity)
            .background(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .fill(Theme.surface)
            )
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
    }

    private func iconBadge(_ systemName: String, tint: Color, background: Color) -> some View {
        ZStack {
            Circle().fill(background).frame(width: 60, height: 60)
            Image(systemName: systemName)
                .font(.system(size: 22, weight: .semibold))
                .foregroundStyle(tint)
        }
    }

    private func primaryButton(_ title: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(title)
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(.white)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 14)
                .background(RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous).fill(Theme.accent))
        }
    }

    // MARK: - Content

    @ViewBuilder
    private var content: some View {
        switch model.phase {
        case .idle:
            if !queue.activeJobs.isEmpty {
                activeDownloadsSection
            } else {
                idle
            }
        case .resolving:
            card {
                ProgressView().tint(Theme.accent)
                Text("Fetching video details")
                    .font(.system(size: 14))
                    .foregroundStyle(Theme.inkSecondary)
            }
        case .ready(let info):
            ready(info)
        case .failed(let message):
            card {
                iconBadge("exclamationmark", tint: Theme.danger, background: Theme.danger.opacity(0.1))
                Text(message)
                    .font(.system(size: 14))
                    .foregroundStyle(Theme.inkSecondary)
                    .multilineTextAlignment(.center)
                primaryButton("Try again") { model.reset() }
            }
        }
    }

    // MARK: - In-Progress Download Section

    private var activeDownloadsSection: some View {
        VStack(spacing: 16) {
            if let job = queue.activeJobs.first {
                ActiveJobCard(job: job, totalActive: queue.activeJobs.count) {
                    showingJobs = true
                }
            }

            compactPasteBar
        }
    }

    private var compactPasteBar: some View {
        Button {
            if clipboard.hasLink {
                guard let url = clipboard.take() else { return }
                Task { await model.resolve(url) }
            } else {
                showingPlatforms = true
            }
        } label: {
            HStack(spacing: 12) {
                Image(systemName: clipboard.hasLink ? "link.badge.plus" : "plus.circle.fill")
                    .font(.system(size: 16, weight: .bold))
                    .foregroundStyle(clipboard.hasLink ? Theme.accent : Theme.inkSecondary)

                Text(clipboard.hasLink ? "Paste & fetch copied link" : "Download another video link")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.ink)
                    .lineLimit(1)

                Spacer()

                Image(systemName: "chevron.right")
                    .font(.system(size: 12, weight: .bold))
                    .foregroundStyle(Theme.inkTertiary)
            }
            .padding(.horizontal, 18)
            .padding(.vertical, 14)
            .background(
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .fill(Theme.surface)
            )
            .overlay(
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
        }
    }

    // MARK: - Idle Screen

    private var idle: some View {
        card {
            iconBadge(
                clipboard.hasLink ? "link.badge.plus" : "link",
                tint: clipboard.hasLink ? Theme.accent : Theme.inkTertiary,
                background: Theme.background
            )
            VStack(spacing: 4) {
                Text(clipboard.hasLink ? "Link copied" : "No link yet")
                    .font(.system(size: 17, weight: .semibold))
                    .foregroundStyle(Theme.ink)
                Text(clipboard.hasLink ? "Tap below to fetch this video" : "Copy a video or audio link from any platform")
                    .font(.system(size: 14))
                    .foregroundStyle(Theme.inkSecondary)
                    .multilineTextAlignment(.center)
            }
            if clipboard.hasLink {
                primaryButton("Use copied link") {
                    guard let url = clipboard.take() else { return }
                    Task { await model.resolve(url) }
                }
            } else {
                Button {
                    showingPlatforms = true
                } label: {
                    HStack(spacing: 8) {
                        PlatformLogoView(type: .youtube, size: 24)
                        PlatformLogoView(type: .instagram, size: 24)
                        PlatformLogoView(type: .tiktok, size: 24)
                        PlatformLogoView(type: .twitter, size: 24)
                        PlatformLogoView(type: .soundcloud, size: 24)
                        Text("+1.7k")
                            .font(.system(size: 11, weight: .bold, design: .rounded))
                            .foregroundStyle(Theme.inkSecondary)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 3)
                            .background(Capsule().fill(Theme.background))
                    }
                    .padding(.top, 4)
                }
            }
        }
    }

    // MARK: - Ready Screen

    private func ready(_ info: DownloadModel.ResolvedInfo) -> some View {
        card {
            Group {
                if let t = info.thumbnail, let u = URL(string: t) {
                    AsyncImage(url: u) { $0.resizable().aspectRatio(contentMode: .fill) }
                        placeholder: { Theme.background }
                        .frame(height: 180)
                        .frame(maxWidth: .infinity)
                        .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
                } else {
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .fill(Theme.background)
                        .frame(height: 180)
                        .overlay(Image(systemName: "play.fill").foregroundStyle(Theme.inkTertiary))
                }
            }

            VStack(spacing: 4) {
                Text(info.title)
                    .font(.system(size: 16, weight: .semibold))
                    .foregroundStyle(Theme.ink)
                    .multilineTextAlignment(.center)
                    .lineLimit(2)
                Text(info.uploader)
                    .font(.system(size: 13))
                    .foregroundStyle(Theme.inkSecondary)
            }

            if info.variants.count > 1 {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 8) {
                        ForEach(info.variants, id: \.self) { v in
                            let isSelected = model.selected == v
                            Text(v.label)
                                .font(.system(size: 13, weight: .semibold))
                                .foregroundStyle(isSelected ? .white : Theme.ink)
                                .padding(.horizontal, 14)
                                .padding(.vertical, 8)
                                .background(
                                    Capsule().fill(isSelected ? Theme.accent : Theme.background)
                                )
                                .overlay(
                                    Capsule().strokeBorder(Theme.hairline, lineWidth: isSelected ? 0 : 1)
                                )
                                .onTapGesture { model.selected = v }
                        }
                    }
                    .padding(.horizontal, 24)
                }
                .frame(maxWidth: .infinity)
                .padding(.horizontal, -24)
            }

            if model.selected?.audio != true {
                splitControl
            }

            primaryButton(saveButtonTitle) {
                model.startDownload()
            }

            Button("Cancel") { model.reset() }
                .font(.system(size: 14, weight: .medium))
                .foregroundStyle(Theme.inkSecondary)
        }
    }

    private var saveButtonTitle: String {
        if model.selected?.audio == true { return "Save as MP3" }
        return model.splitSeconds != nil ? "Save & Split" : "Save video"
    }

    private static let splitOptions = [15, 30, 60, 120]

    private var splitControl: some View {
        VStack(alignment: .leading, spacing: 10) {
            Toggle(isOn: Binding(
                get: { model.splitSeconds != nil },
                set: { on in model.splitSeconds = on ? Self.splitOptions[1] : nil }
            )) {
                VStack(alignment: .leading, spacing: 2) {
                    Text("Split into clips")
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Theme.ink)
                    Text("Cuts the video into shorter parts")
                        .font(.system(size: 12))
                        .foregroundStyle(Theme.inkSecondary)
                }
            }
            .tint(Theme.accent)

            if let seconds = model.splitSeconds {
                HStack(spacing: 8) {
                    ForEach(Self.splitOptions, id: \.self) { option in
                        let isSelected = seconds == option
                        Text("\(option)s")
                            .font(.system(size: 13, weight: .semibold))
                            .foregroundStyle(isSelected ? .white : Theme.ink)
                            .padding(.horizontal, 14)
                            .padding(.vertical, 8)
                            .background(Capsule().fill(isSelected ? Theme.accent : Theme.background))
                            .overlay(Capsule().strokeBorder(Theme.hairline, lineWidth: isSelected ? 0 : 1))
                            .onTapGesture { model.splitSeconds = option }
                    }
                }
            }
        }
    }
}

// MARK: - Active In-Progress Download Card

private struct ActiveJobCard: View {
    @ObservedObject var job: DownloadJob
    let totalActive: Int
    let onOpenList: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            // Header: Live Indicator & Total Count
            HStack {
                HStack(spacing: 6) {
                    Circle()
                        .fill(Theme.accent)
                        .frame(width: 8, height: 8)
                    Text("DOWNLOADING")
                        .font(.system(size: 11, weight: .bold, design: .rounded))
                        .foregroundStyle(Theme.accent)
                }

                Spacer()

                if totalActive > 1 {
                    Button(action: onOpenList) {
                        HStack(spacing: 4) {
                            Text("\(totalActive) active")
                            Image(systemName: "chevron.right")
                                .font(.system(size: 10, weight: .bold))
                        }
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundStyle(Theme.inkSecondary)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 4)
                        .background(Capsule().fill(Theme.background))
                    }
                }
            }

            // Thumbnail + Title + Format
            HStack(spacing: 12) {
                Group {
                    if let t = job.thumbnail, let u = URL(string: t) {
                        AsyncImage(url: u) { $0.resizable().aspectRatio(contentMode: .fill) }
                            placeholder: { placeholderIcon }
                    } else {
                        placeholderIcon
                    }
                }
                .frame(width: 54, height: 54)
                .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))

                VStack(alignment: .leading, spacing: 4) {
                    Text(job.title)
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(Theme.ink)
                        .lineLimit(2)
                        .truncationMode(.middle)

                    Text(job.statusText)
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(Theme.inkSecondary)
                        .lineLimit(1)
                }

                Spacer()
            }

            // Progress Bar & Percentage
            VStack(spacing: 6) {
                if let p = job.progress {
                    GeometryReader { geo in
                        ZStack(alignment: .leading) {
                            Capsule()
                                .fill(Theme.background)
                                .frame(height: 6)
                            Capsule()
                                .fill(LinearGradient(
                                    colors: [Theme.accent, Theme.accent.opacity(0.8)],
                                    startPoint: .leading,
                                    endPoint: .trailing
                                ))
                                .frame(width: max(geo.size.width * CGFloat(p), 6), height: 6)
                                .animation(.easeInOut(duration: 0.25), value: p)
                        }
                    }
                    .frame(height: 6)

                    HStack {
                        Text("\(Int(p * 100))%")
                            .font(.system(size: 12, weight: .bold, design: .rounded))
                            .foregroundStyle(Theme.accent)
                        Spacer()
                        Text(job.isAudio ? "Audio (MP3)" : (job.isSplit ? "Splitting clips" : "Video"))
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(Theme.inkTertiary)
                    }
                } else {
                    ProgressView()
                        .progressViewStyle(.linear)
                        .tint(Theme.accent)
                        .frame(height: 6)

                    HStack {
                        Text("Processing…")
                            .font(.system(size: 12, weight: .medium, design: .rounded))
                            .foregroundStyle(Theme.accent)
                        Spacer()
                    }
                }
            }
        }
        .padding(20)
        .background(
            RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                .fill(Theme.surface)
        )
        .overlay(
            RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                .strokeBorder(Theme.hairline, lineWidth: 1)
        )
        .onTapGesture {
            onOpenList()
        }
    }

    private var placeholderIcon: some View {
        Theme.background.overlay(
            Image(systemName: job.isAudio ? "music.note" : (job.isSplit ? "square.split.2x1" : "video.fill"))
                .font(.system(size: 22))
                .foregroundStyle(Theme.inkTertiary)
        )
    }
}

// MARK: - Download Jobs Sheet

private struct DownloadJobsSheet: View {
    @ObservedObject var queue: DownloadQueue
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()
            VStack(spacing: 0) {
                header
                if queue.jobs.isEmpty {
                    Spacer()
                    Text("Nothing downloading")
                        .font(.system(size: 14))
                        .foregroundStyle(Theme.inkSecondary)
                    Spacer()
                } else {
                    ScrollView {
                        VStack(spacing: 10) {
                            ForEach(queue.jobs) { job in
                                DownloadJobRow(job: job) { queue.remove(job) }
                            }
                        }
                        .padding(.horizontal, 20)
                        .padding(.top, 4)
                        .padding(.bottom, 20)
                    }
                }
            }
        }
    }

    private var header: some View {
        HStack {
            Button { dismiss() } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 15, weight: .bold))
                    .foregroundStyle(Theme.ink)
                    .frame(width: 36, height: 36)
                    .background(Circle().fill(Theme.surface))
                    .overlay(Circle().strokeBorder(Theme.hairline, lineWidth: 1))
            }
            Spacer()
            Text("Downloads")
                .font(.system(size: 18, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.ink)
            Spacer()
            Color.clear.frame(width: 36, height: 36)
        }
        .padding(.horizontal, 20)
        .padding(.top, 20)
        .padding(.bottom, 12)
    }
}

private struct DownloadJobRow: View {
    @ObservedObject var job: DownloadJob
    let onDismiss: () -> Void

    var body: some View {
        HStack(spacing: 14) {
            thumbnail

            VStack(alignment: .leading, spacing: 4) {
                Text(job.title)
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.ink)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Text(job.statusText)
                    .font(.system(size: 12))
                    .foregroundStyle(statusColor)
            }

            Spacer()

            trailing
        }
        .padding(12)
        .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                .strokeBorder(Theme.hairline, lineWidth: 1)
        )
    }

    private var thumbnail: some View {
        Group {
            if let t = job.thumbnail, let u = URL(string: t) {
                AsyncImage(url: u) { $0.resizable().aspectRatio(contentMode: .fill) }
                    placeholder: { placeholderIcon }
            } else {
                placeholderIcon
            }
        }
        .frame(width: 44, height: 44)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
    }

    private var placeholderIcon: some View {
        Theme.background.overlay(
            Image(systemName: job.isAudio ? "music.note" : (job.isSplit ? "square.split.2x1" : "video.fill"))
                .foregroundStyle(Theme.inkTertiary)
        )
    }

    @ViewBuilder
    private var trailing: some View {
        switch job.status {
        case .done:
            Button(action: onDismiss) {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(Theme.accent)
            }
        case .failed:
            Button(action: onDismiss) {
                Image(systemName: "xmark.circle.fill")
                    .foregroundStyle(Theme.danger)
            }
        default:
            ProgressView().tint(Theme.accent)
        }
    }

    private var statusColor: Color {
        switch job.status {
        case .failed: return Theme.danger
        case .done: return Theme.accent
        default: return Theme.inkSecondary
        }
    }
}
