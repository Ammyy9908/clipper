import Foundation
import SwiftUI

enum CloudDestination: String, CaseIterable, Identifiable, Codable {
    case googleDrive = "googleDrive"
    case dropbox = "dropbox"
    case ask = "ask"

    var id: String { rawValue }

    var title: String {
        switch self {
        case .googleDrive: return "Google Drive"
        case .dropbox: return "Dropbox"
        case .ask: return "Always Ask"
        }
    }

    var shortName: String {
        switch self {
        case .googleDrive: return "Google Drive"
        case .dropbox: return "Dropbox"
        case .ask: return "Cloud"
        }
    }

    var icon: String {
        switch self {
        case .googleDrive: return "externaldrive.fill.badge.icloud"
        case .dropbox: return "shippingbox.fill"
        case .ask: return "arrow.up.and.down.and.sparkles"
        }
    }

    var brandColor: Color {
        switch self {
        case .googleDrive: return Color(red: 0.09, green: 0.51, blue: 0.96)
        case .dropbox: return Color(red: 0.0, green: 0.38, blue: 0.95)
        case .ask: return Theme.accent
        }
    }
}

enum ToastType {
    case success
    case error
    case info

    var icon: String {
        switch self {
        case .success: return "checkmark.circle.fill"
        case .error: return "exclamationmark.circle.fill"
        case .info: return "arrow.up.circle.fill"
        }
    }

    var color: Color {
        switch self {
        case .success: return Color.green
        case .error: return Theme.danger
        case .info: return Theme.accent
        }
    }
}

@MainActor
final class CloudStorageManager: ObservableObject {
    static let shared = CloudStorageManager()

    @AppStorage("preferredCloudDestination_v1") var preferredDestinationRaw: String = CloudDestination.googleDrive.rawValue
    @AppStorage("autoUploadToCloud_v1") var autoUploadToCloud: Bool = false

    var preferredDestination: CloudDestination {
        get { CloudDestination(rawValue: preferredDestinationRaw) ?? .googleDrive }
        set { preferredDestinationRaw = newValue.rawValue }
    }

    @Published var isUploading: Bool = false
    @Published var currentUploadingTitle: String? = nil
    @Published var toastMessage: String? = nil
    @Published var toastType: ToastType = .info

    private var toastTimerTask: Task<Void, Never>?

    private init() {}

    func showToast(_ message: String, type: ToastType = .info) {
        toastTimerTask?.cancel()
        withAnimation(.spring(response: 0.35, dampingFraction: 0.8)) {
            self.toastMessage = message
            self.toastType = type
        }
        toastTimerTask = Task {
            try? await Task.sleep(nanoseconds: 3_500_000_000)
            withAnimation(.easeInOut(duration: 0.25)) {
                if self.toastMessage == message {
                    self.toastMessage = nil
                }
            }
        }
    }

    /// Upload a saved clip to the chosen or preferred cloud service.
    func upload(clip: SavedClip, target: CloudDestination? = nil) async {
        let dest = target ?? preferredDestination

        switch dest {
        case .googleDrive:
            await uploadToGoogleDrive(fileURL: clip.localURL, title: clip.title)
        case .dropbox:
            await uploadToDropbox(fileURL: clip.localURL, title: clip.title)
        case .ask:
            // Fallback to whichever is linked or default to Google Drive
            if GoogleDriveManager.shared.isLinked {
                await uploadToGoogleDrive(fileURL: clip.localURL, title: clip.title)
            } else if DropboxManager.shared.isLinked {
                await uploadToDropbox(fileURL: clip.localURL, title: clip.title)
            } else {
                showToast("Connect Google Drive or Dropbox in Settings first", type: .info)
                GoogleDriveManager.shared.authorize()
            }
        }
    }

    /// Upload a local file URL directly.
    func upload(fileURL: URL, name: String, target: CloudDestination? = nil) async {
        let dest = target ?? preferredDestination

        switch dest {
        case .googleDrive:
            await uploadToGoogleDrive(fileURL: fileURL, title: name)
        case .dropbox:
            await uploadToDropbox(fileURL: fileURL, title: name)
        case .ask:
            if GoogleDriveManager.shared.isLinked {
                await uploadToGoogleDrive(fileURL: fileURL, title: name)
            } else if DropboxManager.shared.isLinked {
                await uploadToDropbox(fileURL: fileURL, title: name)
            } else {
                showToast("Connect Google Drive or Dropbox in Settings first", type: .info)
                GoogleDriveManager.shared.authorize()
            }
        }
    }

    private func uploadToGoogleDrive(fileURL: URL, title: String) async {
        guard GoogleDriveManager.shared.isLinked else {
            showToast("Connecting to Google Drive…", type: .info)
            GoogleDriveManager.shared.authorize()
            return
        }

        isUploading = true
        currentUploadingTitle = title
        showToast("Uploading \"\(title)\" to Google Drive…", type: .info)

        do {
            try await GoogleDriveManager.shared.upload(fileURL: fileURL, name: nil)
            isUploading = false
            currentUploadingTitle = nil
            showToast("Saved to Google Drive!", type: .success)
        } catch {
            isUploading = false
            currentUploadingTitle = nil
            showToast("Drive upload failed: \(error.localizedDescription)", type: .error)
        }
    }

    private func uploadToDropbox(fileURL: URL, title: String) async {
        guard DropboxManager.shared.isLinked else {
            showToast("Connecting to Dropbox…", type: .info)
            DropboxManager.shared.authorize()
            return
        }

        isUploading = true
        currentUploadingTitle = title
        showToast("Uploading \"\(title)\" to Dropbox…", type: .info)

        do {
            try await DropboxManager.shared.upload(fileURL: fileURL, name: nil)
            isUploading = false
            currentUploadingTitle = nil
            showToast("Saved to Dropbox (/clipPlay)!", type: .success)
        } catch {
            isUploading = false
            currentUploadingTitle = nil
            showToast("Dropbox upload failed: \(error.localizedDescription)", type: .error)
        }
    }
}
