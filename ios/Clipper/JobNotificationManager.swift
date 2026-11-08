import Foundation
import UserNotifications

/// Fires a local system notification when a download job finishes or fails.
/// Works reliably while the app is foregrounded or briefly backgrounded —
/// this app has no background-fetch entitlement, so a job that completes
/// while fully suspended won't notify until the app is reopened and its
/// poll loop resumes.
@MainActor
final class JobNotificationManager: NSObject {
    static let shared = JobNotificationManager()

    private var didRequestAuthorization = false

    private override init() {
        super.init()
        UNUserNotificationCenter.current().delegate = self
    }

    /// Call right when the user actually starts a download, not on launch —
    /// asking for permission at the point it's relevant beats asking before
    /// they've done anything that would need it.
    func requestAuthorizationIfNeeded() {
        guard !didRequestAuthorization else { return }
        didRequestAuthorization = true
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { _, _ in }
    }

    func notifyDone(title: String, isAudio: Bool, isSplit: Bool = false) {
        let content = UNMutableNotificationContent()
        if isAudio {
            content.title = "MP3 saved"
        } else if isSplit {
            content.title = "Video split & saved"
        } else {
            content.title = "Video saved"
        }
        content.body = title
        content.sound = .default
        post(content)
    }

    func notifyFailed(title: String, message: String) {
        let content = UNMutableNotificationContent()
        content.title = "Download failed"
        content.body = "\(title): \(message)"
        content.sound = .default
        post(content)
    }

    private func post(_ content: UNMutableNotificationContent) {
        let request = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
        UNUserNotificationCenter.current().add(request)
    }
}

extension JobNotificationManager: UNUserNotificationCenterDelegate {
    /// Without this, a notification posted while the app is already in the
    /// foreground is silently swallowed — this is what makes it actually
    /// show as a banner even when you're looking at the app.
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        completionHandler([.banner, .sound, .list])
    }
}
