import SwiftUI

/// Browses the user's Dropbox and imports audio files straight into the
/// library — same destination (LibraryStore.clipsDirectory) and SavedClip
/// shape as the Files-picker import path, just reached through Dropbox's own
/// API instead of the system document picker.
struct DropboxBrowserView: View {
    @ObservedObject var library: LibraryStore
    @ObservedObject private var dropbox = DropboxManager.shared
    @Environment(\.dismiss) private var dismiss

    @State private var path = DropboxManager.rootPath
    @State private var folderName = "Dropbox"
    @State private var stack: [(path: String, name: String)] = []
    @State private var entries: [DropboxManager.Entry] = []
    @State private var isLoading = false
    @State private var errorMessage: String?
    @State private var importingPath: String?

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()
            VStack(spacing: 0) {
                header
                if !dropbox.isLinked {
                    Spacer()
                    connectPrompt
                    Spacer()
                } else if isLoading {
                    Spacer()
                    ProgressView().tint(Theme.accent)
                    Spacer()
                } else if let errorMessage {
                    Spacer()
                    Text(errorMessage)
                        .font(.system(size: 14))
                        .foregroundStyle(Theme.inkSecondary)
                        .multilineTextAlignment(.center)
                        .padding(.horizontal, 32)
                    Spacer()
                } else if entries.isEmpty {
                    Spacer()
                    Text("This folder is empty")
                        .font(.system(size: 14))
                        .foregroundStyle(Theme.inkSecondary)
                    Spacer()
                } else {
                    ScrollView {
                        VStack(spacing: 10) {
                            ForEach(entries) { entry in
                                entryRow(entry)
                            }
                        }
                        .padding(.horizontal, 20)
                        .padding(.top, 4)
                        .padding(.bottom, 20)
                    }
                }
            }
        }
        .task { if dropbox.isLinked { await load() } }
        .onChange(of: dropbox.isLinked) { _, linked in
            if linked { Task { await load() } }
        }
    }

    private var header: some View {
        HStack {
            Button {
                if stack.isEmpty {
                    dismiss()
                } else {
                    let previous = stack.removeLast()
                    path = previous.path
                    folderName = previous.name
                    Task { await load() }
                }
            } label: {
                Image(systemName: stack.isEmpty ? "xmark" : "chevron.left")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(Theme.ink)
                    .frame(width: 36, height: 36)
                    .background(Circle().fill(Theme.surface))
                    .overlay(Circle().strokeBorder(Theme.hairline, lineWidth: 1))
            }
            Spacer()
            Text(folderName)
                .font(.system(size: 18, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.ink)
                .lineLimit(1)
            Spacer()
            Color.clear.frame(width: 36, height: 36)
        }
        .padding(.horizontal, 20)
        .padding(.top, 20)
        .padding(.bottom, 12)
    }

    private var connectPrompt: some View {
        VStack(spacing: 12) {
            Image(systemName: "shippingbox")
                .font(.system(size: 30))
                .foregroundStyle(Theme.inkTertiary)
            Text("Connect Dropbox")
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Theme.ink)
            Text("clipPlay can browse your Dropbox and import audio directly, read-only.")
                .font(.system(size: 13))
                .foregroundStyle(Theme.inkSecondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 40)
            Button {
                dropbox.authorize()
            } label: {
                Text("Connect")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(.white)
                    .padding(.horizontal, 24)
                    .padding(.vertical, 10)
                    .background(Capsule().fill(Theme.accent))
            }
            .padding(.top, 4)
        }
    }

    // Every row reaching here is already a folder or an audio file — the
    // non-audio noise gets filtered out in load(), same as the Files picker
    // does natively via allowedContentTypes.
    private func entryRow(_ entry: DropboxManager.Entry) -> some View {
        Button {
            if entry.isFolder {
                stack.append((path: path, name: folderName))
                path = entry.path
                folderName = entry.name
                Task { await load() }
            } else {
                Task { await importFile(entry) }
            }
        } label: {
            HStack(spacing: 14) {
                Image(systemName: entry.isFolder ? "folder.fill" : "music.note")
                    .font(.system(size: 18))
                    .foregroundStyle(entry.isFolder ? Theme.inkSecondary : Theme.accent)
                    .frame(width: 32)

                Text(entry.name)
                    .font(.system(size: 15, weight: .medium))
                    .foregroundStyle(Theme.ink)
                    .lineLimit(1)
                    .truncationMode(.middle)

                Spacer()

                if importingPath == entry.path {
                    ProgressView().tint(Theme.accent)
                } else if entry.isFolder {
                    Image(systemName: "chevron.right")
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(Theme.inkTertiary)
                } else {
                    Image(systemName: "square.and.arrow.down")
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(Theme.accent)
                }
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

    private func isAudioFile(_ name: String) -> Bool {
        let ext = (name as NSString).pathExtension.lowercased()
        return ["mp3", "m4a", "wav", "aac", "flac", "aiff", "caf"].contains(ext)
    }

    private func load() async {
        isLoading = true
        errorMessage = nil
        do {
            entries = try await dropbox.list(path: path)
                .filter { $0.isFolder || isAudioFile($0.name) }
                .sorted { a, b in
                    if a.isFolder != b.isFolder { return a.isFolder }
                    return a.name.localizedCaseInsensitiveCompare(b.name) == .orderedAscending
                }
        } catch {
            errorMessage = error.localizedDescription
        }
        isLoading = false
    }

    private func importFile(_ entry: DropboxManager.Entry) async {
        importingPath = entry.path
        defer { importingPath = nil }

        let id = UUID().uuidString
        let ext = (entry.name as NSString).pathExtension
        let filename = "\(id).\(ext.isEmpty ? "mp3" : ext.lowercased())"
        let dest = LibraryStore.clipsDirectory.appendingPathComponent(filename)

        do {
            try await dropbox.download(path: entry.path, to: dest)
        } catch {
            errorMessage = error.localizedDescription
            return
        }

        let size = (try? FileManager.default.attributesOfItem(atPath: dest.path)[.size] as? Int64) ?? 0
        let title = (entry.name as NSString).deletingPathExtension

        library.add(SavedClip(
            id: id,
            title: title.isEmpty ? filename : title,
            filename: filename,
            sizeBytes: size,
            createdAt: Date()
        ))
    }
}
