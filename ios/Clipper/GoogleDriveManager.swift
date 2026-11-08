import Foundation
import GoogleSignIn
import UIKit

/// Thin wrapper around GIDSignIn's static shared instance, mirroring
/// DropboxManager's shape — auth state lives in the SDK itself (keychain),
/// this just exposes it to SwiftUI. Drive's REST API (list a folder,
/// download a file) doesn't need Google's full API client library — a
/// Bearer token plus URLSession is enough, and keeps this the same shape as
/// the Dropbox integration.
@MainActor
final class GoogleDriveManager: ObservableObject {
    static let shared = GoogleDriveManager()

    @Published private(set) var isLinked: Bool

    @Published private(set) var userEmail: String?

    private static let scopes = [
        "https://www.googleapis.com/auth/drive.file",
        "https://www.googleapis.com/auth/drive.readonly"
    ]

    private init() {
        let current = GIDSignIn.sharedInstance.currentUser
        isLinked = current != nil
        userEmail = current?.profile?.email
        // Silently restores a session saved from a previous launch, if any.
        GIDSignIn.sharedInstance.restorePreviousSignIn { [weak self] user, _ in
            Task { @MainActor in
                self?.isLinked = user != nil
                self?.userEmail = user?.profile?.email
            }
        }
    }

    func authorize() {
        guard let presenter = Self.rootViewController() else { return }
        GIDSignIn.sharedInstance.signIn(
            withPresenting: presenter,
            hint: nil,
            additionalScopes: Self.scopes
        ) { [weak self] result, error in
            guard error == nil, let user = result?.user else { return }
            Task { @MainActor in
                self?.isLinked = true
                self?.userEmail = user.profile?.email
            }
        }
    }

    /// Called from ClipperApp's onOpenURL — returns whether this URL was the
    /// Google redirect so the caller knows whether to check other handlers.
    @discardableResult
    func handleRedirect(_ url: URL) -> Bool {
        GIDSignIn.sharedInstance.handle(url)
    }

    func unlink() {
        GIDSignIn.sharedInstance.signOut()
        isLinked = false
        userEmail = nil
    }

    struct Entry: Identifiable {
        let id: String
        let name: String
        let isFolder: Bool
    }

    /// Drive addresses folders by ID, not path — "root" is the special ID
    /// for the top of My Drive.
    static let rootFolderID = "root"

    func list(folderID: String) async throws -> [Entry] {
        guard let token = currentAccessToken() else { throw DriveImportError.notLinked }

        var components = URLComponents(string: "https://www.googleapis.com/drive/v3/files")!
        components.queryItems = [
            URLQueryItem(name: "q", value: "'\(folderID)' in parents and trashed = false"),
            URLQueryItem(name: "fields", value: "files(id,name,mimeType)"),
            URLQueryItem(name: "pageSize", value: "1000"),
            URLQueryItem(name: "orderBy", value: "folder,name"),
        ]
        var request = URLRequest(url: components.url!)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw DriveImportError.api("Drive request failed")
        }
        let decoded = try JSONDecoder().decode(FileListResponse.self, from: data)
        return decoded.files.map {
            Entry(id: $0.id, name: $0.name, isFolder: $0.mimeType == "application/vnd.google-apps.folder")
        }
    }

    func download(fileID: String, to destination: URL) async throws {
        guard let token = currentAccessToken() else { throw DriveImportError.notLinked }

        var request = URLRequest(url: URL(string: "https://www.googleapis.com/drive/v3/files/\(fileID)?alt=media")!)
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw DriveImportError.api("Download failed")
        }
        try data.write(to: destination)
    }

    /// Uploads a local audio or video file directly to the user's Google Drive.
    @discardableResult
    func upload(fileURL: URL, name: String? = nil) async throws -> String {
        guard let token = currentAccessToken() else { throw DriveImportError.notLinked }

        let uploadName = name ?? fileURL.lastPathComponent
        let ext = fileURL.pathExtension.lowercased()
        let mimeType: String
        switch ext {
        case "mp4": mimeType = "video/mp4"
        case "m4a": mimeType = "audio/mp4"
        case "mp3": mimeType = "audio/mpeg"
        case "mov": mimeType = "video/quicktime"
        case "webm": mimeType = "video/webm"
        case "wav": mimeType = "audio/wav"
        default: mimeType = "application/octet-stream"
        }

        let fileData = try Data(contentsOf: fileURL)
        let boundary = "Boundary-\(UUID().uuidString)"

        var request = URLRequest(url: URL(string: "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart")!)
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("multipart/related; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")

        var body = Data()
        // Metadata part
        body.append("--\(boundary)\r\n".data(using: .utf8)!)
        body.append("Content-Type: application/json; charset=UTF-8\r\n\r\n".data(using: .utf8)!)
        let metadataJSON = "{\"name\": \"\(uploadName)\", \"mimeType\": \"\(mimeType)\"}\r\n"
        body.append(metadataJSON.data(using: .utf8)!)

        // Media content part
        body.append("--\(boundary)\r\n".data(using: .utf8)!)
        body.append("Content-Type: \(mimeType)\r\n\r\n".data(using: .utf8)!)
        body.append(fileData)
        body.append("\r\n--\(boundary)--\r\n".data(using: .utf8)!)

        request.httpBody = body

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            let errorText = String(data: data, encoding: .utf8) ?? "Upload failed"
            throw DriveImportError.api(errorText)
        }

        struct UploadResponse: Decodable {
            let id: String
        }
        let decoded = try JSONDecoder().decode(UploadResponse.self, from: data)
        return decoded.id
    }

    private func currentAccessToken() -> String? {
        GIDSignIn.sharedInstance.currentUser?.accessToken.tokenString
    }

    private struct FileListResponse: Decodable {
        let files: [DriveFile]
    }

    private struct DriveFile: Decodable {
        let id: String
        let name: String
        let mimeType: String
    }

    private static func rootViewController() -> UIViewController? {
        UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .flatMap { $0.windows }
            .first { $0.isKeyWindow }?
            .rootViewController
    }
}

enum DriveImportError: LocalizedError {
    case notLinked
    case api(String)

    var errorDescription: String? {
        switch self {
        case .notLinked: return "Not connected to Google Drive."
        case .api(let message): return message
        }
    }
}
