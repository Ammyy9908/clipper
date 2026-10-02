import SwiftUI

/// One card per split video, not one row per part — VideoTabView already
/// excludes anything with a groupID, so this tab is the only place split
/// parts show up at all.
struct SplitTabView: View {
    @ObservedObject var library: LibraryStore
    @ObservedObject private var player = PlayerModel.shared
    @State private var openGroup: SplitGroup?

    private var groups: [SplitGroup] {
        let grouped = Dictionary(grouping: library.clips.filter { $0.groupID != nil }, by: { $0.groupID! })
        return grouped.compactMap { id, clips -> SplitGroup? in
            guard !clips.isEmpty else { return nil }
            let sorted = clips.sorted { ($0.partIndex ?? 0) < ($1.partIndex ?? 0) }
            return SplitGroup(
                id: id,
                title: sorted.first?.groupTitle ?? sorted.first?.title ?? "Split video",
                parts: sorted,
                createdAt: sorted.map(\.createdAt).min() ?? Date()
            )
        }
        .sorted { $0.createdAt > $1.createdAt }
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            if groups.isEmpty {
                Spacer()
                emptyState
                Spacer()
            } else {
                ScrollView {
                    VStack(spacing: 12) {
                        ForEach(groups) { group in
                            SplitGroupCard(group: group) { openGroup = group }
                        }
                    }
                    .padding(.horizontal, 20)
                    .padding(.top, 4)
                    .padding(.bottom, player.currentClip != nil && !player.isExpanded ? 84 : 20)
                }
            }
        }
        .sheet(item: $openGroup) { group in
            SplitGroupDetailView(group: group, library: library)
        }
    }

    private var header: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text("Split")
                    .font(.system(size: 28, weight: .bold, design: .rounded))
                    .foregroundStyle(Theme.ink)
                Text(groups.isEmpty ? "Nothing here yet" : "\(groups.count) video\(groups.count == 1 ? "" : "s") split")
                    .font(.system(size: 13))
                    .foregroundStyle(Theme.inkSecondary)
            }
            Spacer()
        }
        .padding(.horizontal, 20)
        .padding(.top, 20)
        .padding(.bottom, 12)
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: "square.split.2x1")
                .font(.system(size: 30))
                .foregroundStyle(Theme.inkTertiary)
            Text("No split videos yet")
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Theme.ink)
            Text("Turn on \"Split into clips\" when saving a video to see it here")
                .font(.system(size: 13))
                .foregroundStyle(Theme.inkSecondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 40)
        }
    }
}

struct SplitGroup: Identifiable {
    let id: String
    let title: String
    let parts: [SavedClip]
    let createdAt: Date
}

private struct SplitGroupCard: View {
    let group: SplitGroup
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            HStack(spacing: 14) {
                Thumbnail(url: group.parts[0].localURL, isAudio: false, artworkURL: nil)

                VStack(alignment: .leading, spacing: 4) {
                    Text(group.title)
                        .font(.system(size: 15, weight: .medium))
                        .foregroundStyle(Theme.ink)
                        .lineLimit(1)
                        .truncationMode(.middle)
                    Text("\(group.parts.count) parts")
                        .font(.system(size: 13))
                        .foregroundStyle(Theme.inkSecondary)
                }

                Spacer()

                Image(systemName: "chevron.right")
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(Theme.inkTertiary)
            }
            .padding(12)
            .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
        }
        .buttonStyle(.plain)
    }
}

/// Playing a part here queues Next/Previous through the other parts of the
/// same split — reuses PlayerModel's ordinary multi-clip queue, same as a
/// playlist does, since `parts` is just another [SavedClip] array.
private struct SplitGroupDetailView: View {
    let group: SplitGroup
    @ObservedObject var library: LibraryStore
    @ObservedObject private var player = PlayerModel.shared
    @Environment(\.dismiss) private var dismiss
    @State private var confirmingDeleteAll = false

    private var parts: [SavedClip] {
        library.clips
            .filter { $0.groupID == group.id }
            .sorted { ($0.partIndex ?? 0) < ($1.partIndex ?? 0) }
    }

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()
            VStack(spacing: 0) {
                header
                if parts.isEmpty {
                    Spacer()
                    Text("All parts deleted")
                        .font(.system(size: 14))
                        .foregroundStyle(Theme.inkSecondary)
                    Spacer()
                } else {
                    ScrollView {
                        VStack(spacing: 12) {
                            ForEach(parts) { clip in
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
                        .padding(.bottom, 20)
                    }
                }
            }
        }
        .confirmationDialog(
            "Delete all \(parts.count) parts?",
            isPresented: $confirmingDeleteAll,
            titleVisibility: .visible
        ) {
            Button("Delete All", role: .destructive) {
                for clip in parts { library.remove(clip) }
                dismiss()
            }
        }
    }

    private func play(_ clip: SavedClip) {
        guard let idx = parts.firstIndex(where: { $0.id == clip.id }) else { return }
        player.play(clips: parts, startIndex: idx)
        player.isExpanded = true
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
            Text(group.title)
                .font(.system(size: 18, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.ink)
                .lineLimit(1)
            Spacer()
            Menu {
                Button(role: .destructive) {
                    confirmingDeleteAll = true
                } label: {
                    Label("Delete All Parts", systemImage: "trash")
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
}
