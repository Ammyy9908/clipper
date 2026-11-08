import UIKit
import UniformTypeIdentifiers

/// Hands whatever link was shared straight to the main app via the `clipper://`
/// scheme and closes itself. No UI of its own — picking "Clipper" in the share
/// sheet is the whole interaction.
///
/// This must never hang: if it does, the host app (Instagram, TikTok, ...) sits
/// there waiting on the extension forever, which reads as "the app is stuck."
/// Every path — no attachment, a type we don't handle, a loadItem that never
/// calls back, an open(_:) that never calls back — has to end in complete().
final class ShareViewController: UIViewController {

    private var didComplete = false

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .clear
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        handleShare()

        // Watchdog: whatever goes wrong above, don't leave the host app hanging.
        DispatchQueue.main.asyncAfter(deadline: .now() + 6) { [weak self] in
            self?.complete()
        }
    }

    private func handleShare() {
        guard
            let item = extensionContext?.inputItems.first as? NSExtensionItem,
            let attachments = item.attachments, !attachments.isEmpty
        else {
            complete()
            return
        }

        // Some apps attach more than one item (e.g. a thumbnail image alongside
        // the link) — find the one that's actually a URL or text, don't assume
        // it's first.
        if let urlAttachment = attachments.first(where: { $0.hasItemConformingToTypeIdentifier(UTType.url.identifier) }) {
            urlAttachment.loadItem(forTypeIdentifier: UTType.url.identifier) { [weak self] data, _ in
                self?.openHostApp(with: (data as? URL)?.absoluteString)
            }
        } else if let textAttachment = attachments.first(where: { $0.hasItemConformingToTypeIdentifier(UTType.plainText.identifier) }) {
            textAttachment.loadItem(forTypeIdentifier: UTType.plainText.identifier) { [weak self] data, _ in
                let text = (data as? String) ?? (data as? Data).flatMap { String(data: $0, encoding: .utf8) }
                self?.openHostApp(with: text)
            }
        } else {
            complete()
        }
    }

    private func openHostApp(with text: String?) {
        guard
            let text,
            let encoded = text.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed),
            let url = URL(string: "clipper://resolve?url=\(encoded)")
        else {
            complete()
            return
        }

        DispatchQueue.main.async { [weak self] in
            self?.extensionContext?.open(url) { [weak self] _ in
                self?.complete()
            }
        }
    }

    private func complete() {
        DispatchQueue.main.async { [weak self] in
            guard let self, !self.didComplete else { return }
            self.didComplete = true
            self.extensionContext?.completeRequest(returningItems: nil)
        }
    }
}
