import Foundation
import Photos

enum SaveError: LocalizedError {
    case permissionDenied
    case downloadFailed
    case writeFailed

    var errorDescription: String? {
        switch self {
        case .permissionDenied: return "Allow photo access in Settings to save videos."
        case .downloadFailed:   return "Couldn't download the video."
        case .writeFailed:      return "Couldn't save to your photo library."
        }
    }
}

enum PhotoSaver {

    /// Downloads straight into the app's own Clips folder. This is the one and only
    /// network fetch for a clip — Photos export (when wanted) reads back from here
    /// instead of hitting the remote URL a second time.
    static func downloadToLibrary(from remote: URL, filename: String, progress: @escaping @Sendable (Double) -> Void) async throws -> URL {
        let dest = LibraryStore.clipsDirectory.appendingPathComponent(filename)
        try await download(remote, to: dest, progress: progress)
        return dest
    }

    /// Hands an already-local file to Photos. .addOnly is the right authorization
    /// level — we never read the user's library, so asking for full access would be
    /// an unnecessary permission prompt.
    static func addToPhotos(fileURL: URL) async throws {
        let status = await PHPhotoLibrary.requestAuthorization(for: .addOnly)
        guard status == .authorized || status == .limited else { throw SaveError.permissionDenied }

        do {
            try await PHPhotoLibrary.shared().performChanges {
                PHAssetCreationRequest.forAsset()
                    .addResource(with: .video, fileURL: fileURL, options: nil)
            }
        } catch {
            throw SaveError.writeFailed
        }
    }

    /// Saves the source thumbnail as local cover art for an audio-only clip —
    /// small enough to fetch in one shot, no need for the chunked writer below.
    static func downloadArtwork(from remote: URL, filename: String) async -> Bool {
        guard let (data, response) = try? await URLSession.shared.data(from: remote),
              let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode)
        else { return false }
        let dest = LibraryStore.clipsDirectory.appendingPathComponent(filename)
        return (try? data.write(to: dest)) != nil
    }

    private static func download(_ url: URL, to dest: URL, progress: @escaping @Sendable (Double) -> Void) async throws {
        let (bytes, response) = try await URLSession.shared.bytes(from: url)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw SaveError.downloadFailed
        }

        let total = http.expectedContentLength
        try? FileManager.default.removeItem(at: dest)
        FileManager.default.createFile(atPath: dest.path, contents: nil)

        guard let handle = try? FileHandle(forWritingTo: dest) else { throw SaveError.writeFailed }
        defer { try? handle.close() }

        // Buffer in 1MB chunks for high-throughput disk writing and lower CPU usage
        var buffer = Data()
        buffer.reserveCapacity(1024 * 1024)
        var written: Int64 = 0

        for try await byte in bytes {
            buffer.append(byte)
            if buffer.count >= 1024 * 1024 {
                try handle.write(contentsOf: buffer)
                written += Int64(buffer.count)
                buffer.removeAll(keepingCapacity: true)
                if total > 0 { progress(Double(written) / Double(total)) }
            }
        }
        if !buffer.isEmpty {
            try handle.write(contentsOf: buffer)
            written += Int64(buffer.count)
        }
        progress(1)
    }
}
