import Foundation
import SwiftyDropbox
import UIKit

/// Thin wrapper around SwiftyDropbox's static DropboxClientsManager, which
/// owns all real auth state itself (keychain-backed token storage, refresh,
/// etc.) — this class just exposes that as something SwiftUI can bind to,
/// and turns the two calls Import needs (list a folder, download a file)
/// into async functions instead of the SDK's raw completion-handler style.
@MainActor
final class DropboxManager: ObservableObject {
    static let shared = DropboxManager()

    static let appKey = "ojq9419olfjdqkn"

    @Published private(set) var isLinked: Bool

    private init() {
        DropboxClientsManager.setupWithAppKey(Self.appKey)
        isLinked = DropboxClientsManager.authorizedClient != nil
    }

    /// Prefers the Dropbox app itself (if installed) via PKCE, falling back
    /// to an in-app web view — SwiftyDropbox picks between those, this just
    /// declares the read-only scopes an importer actually needs.
    func authorize() {
        let scopeRequest = ScopeRequest(
            scopeType: .user,
            scopes: ["files.metadata.read", "files.content.read", "files.content.write"],
            includeGrantedScopes: false
        )
        DropboxClientsManager.authorizeFromControllerV2(
            UIApplication.shared,
            controller: nil,
            loadingStatusDelegate: nil,
            openURL: { url in UIApplication.shared.open(url) },
            scopeRequest: scopeRequest
        )
    }

    /// Called from ClipperApp's onOpenURL — returns whether this URL was the
    /// Dropbox OAuth redirect (`db-<APP_KEY>://...`) so the caller knows
    /// whether to also check other URL handlers (e.g. incoming share links).
    @discardableResult
    func handleRedirect(_ url: URL) -> Bool {
        DropboxClientsManager.handleRedirectURL(url, includeBackgroundClient: false) { [weak self] _ in
            Task { @MainActor in
                self?.isLinked = DropboxClientsManager.authorizedClient != nil
            }
        }
    }

    func unlink() {
        DropboxClientsManager.unlinkClients()
        isLinked = false
    }

    struct Entry: Identifiable {
        let id: String
        let name: String
        let path: String
        let isFolder: Bool
    }

    /// Dropbox's API root is "" (empty string), not "/" — passing "/" is a
    /// malformed-path error.
    static let rootPath = ""

    func list(path: String) async throws -> [Entry] {
        guard let client = DropboxClientsManager.authorizedClient else {
            throw DropboxImportError.notLinked
        }
        return try await withCheckedThrowingContinuation { continuation in
            client.files.listFolder(path: path).response { response, error in
                if let error {
                    continuation.resume(throwing: DropboxImportError.api("\(error)"))
                    return
                }
                let entries: [Entry] = (response?.entries ?? []).compactMap { meta in
                    guard let pathLower = meta.pathLower else { return nil }
                    if let folder = meta as? Files.FolderMetadata {
                        return Entry(id: folder.id, name: folder.name, path: pathLower, isFolder: true)
                    }
                    if let file = meta as? Files.FileMetadata {
                        return Entry(id: file.id, name: file.name, path: pathLower, isFolder: false)
                    }
                    return nil
                }
                continuation.resume(returning: entries)
            }
        }
    }

    func download(path: String, to destination: URL) async throws {
        guard let client = DropboxClientsManager.authorizedClient else {
            throw DropboxImportError.notLinked
        }
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            client.files.download(path: path, overwrite: true, destination: destination).response { result, error in
                if let error {
                    continuation.resume(throwing: DropboxImportError.api("\(error)"))
                } else if result != nil {
                    continuation.resume()
                } else {
                    continuation.resume(throwing: DropboxImportError.api("download failed"))
                }
            }
        }
    }

    /// Uploads a local file directly to the user's Dropbox (/clipPlay folder).
    @discardableResult
    func upload(fileURL: URL, name: String? = nil) async throws -> String {
        guard let client = DropboxClientsManager.authorizedClient else {
            throw DropboxImportError.notLinked
        }

        let uploadName = name ?? fileURL.lastPathComponent
        let targetPath = "/clipPlay/\(uploadName)"
        let data = try Data(contentsOf: fileURL)

        return try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<String, Error>) in
            client.files.upload(
                path: targetPath,
                mode: .overwrite,
                input: data
            ).response { response, error in
                if let error {
                    continuation.resume(throwing: DropboxImportError.api("\(error)"))
                } else if let metadata = response {
                    continuation.resume(returning: metadata.id)
                } else {
                    continuation.resume(throwing: DropboxImportError.api("Upload failed without response"))
                }
            }
        }
    }
}

enum DropboxImportError: LocalizedError {
    case notLinked
    case api(String)

    var errorDescription: String? {
        switch self {
        case .notLinked: return "Not connected to Dropbox."
        case .api(let message): return message
        }
    }
}
