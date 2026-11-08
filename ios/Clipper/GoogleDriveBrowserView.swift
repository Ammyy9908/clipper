import SwiftUI

/// Browses the user's Google Drive and imports audio files straight into the
/// library — mirrors DropboxBrowserView's structure exactly, swapping path-
/// based navigation for Drive's folder-ID-based navigation.
struct GoogleDriveBrowserView: View {
    @ObservedObject var library: LibraryStore
    @ObservedObject private var drive = GoogleDriveManager.shared
    @Environment(\.dismiss) private var dismiss

    @State private var folderID = GoogleDriveManager.rootFolderID
    @State private var folderName = "Google Drive"
    @State private var stack: [(id: String, name: String)] = []
    @State private var entries: [GoogleDriveManager.Entry] = []
    @State private var isLoading = false
    @State private var errorMessage: String?
    @State private var importingID: String?

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()
            VStack(spacing: 0) {
                header
                if !drive.isLinked {
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
        .task { if drive.isLinked { await load() } }
        .onChange(of: drive.isLinked) { _, linked in
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
                    folderID = previous.id
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
            Image(systemName: "tray.full")
                .font(.system(size: 30))
                .foregroundStyle(Theme.inkTertiary)
            Text("Connect Google Drive")
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Theme.ink)
            Text("clipPlay can browse your Drive and import audio directly, read-only.")
                .font(.system(size: 13))
                .foregroundStyle(Theme.inkSecondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 40)
            Button {
                drive.authorize()
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
    private func entryRow(_ entry: GoogleDriveManager.Entry) -> some View {
        Button {
            if entry.isFolder {
                stack.append((id: folderID, name: folderName))
                folderID = entry.id
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

                if importingID == entry.id {
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
            entries = try await drive.list(folderID: folderID)
                .filter { $0.isFolder || isAudioFile($0.name) }
        } catch {
            errorMessage = error.localizedDescription
        }
        isLoading = false
    }

    private func importFile(_ entry: GoogleDriveManager.Entry) async {
        importingID = entry.id
        defer { importingID = nil }

        let id = UUID().uuidString
        let ext = (entry.name as NSString).pathExtension
        let filename = "\(id).\(ext.isEmpty ? "mp3" : ext.lowercased())"
        let dest = LibraryStore.clipsDirectory.appendingPathComponent(filename)

        do {
            try await drive.download(fileID: entry.id, to: dest)
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
