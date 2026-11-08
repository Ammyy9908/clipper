import UIKit

/// Detects a copied link without reading the pasteboard.
///
/// Reading UIPasteboard.general.string shows the system "pasted from Safari"
/// banner every single time — on an app whose whole flow is paste-driven that
/// banner fires constantly and users hate it. detectPatterns only reports that
/// a URL-shaped thing exists; the actual read happens after the user taps,
/// which counts as consent and shows no banner.
@MainActor
final class ClipboardWatcher: ObservableObject {
    @Published private(set) var hasLink = false

    private var lastChangeCount = -1

    func check() async {
            let count = UIPasteboard.general.changeCount
            guard count != lastChangeCount else { return }
            lastChangeCount = count

            let patterns: Set<UIPasteboard.DetectionPattern> = await withCheckedContinuation { continuation in
                UIPasteboard.general.detectPatterns(for: [.probableWebURL]) { result in
                    continuation.resume(returning: (try? result.get()) ?? [])
                }
            }
            hasLink = patterns.contains(.probableWebURL)
        }

    /// Only called from a user-initiated tap. This is the read that needs consent.
    func take() -> String? {
        defer { hasLink = false }
        guard let s = UIPasteboard.general.string?.trimmingCharacters(in: .whitespacesAndNewlines),
              let url = URL(string: s), url.scheme?.hasPrefix("http") == true
        else { return nil }
        return s
    }
}
