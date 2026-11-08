import Foundation

/// Receives a link handed off from the Share Extension via the `clipper://`
/// URL scheme. This path never touches the pasteboard, so none of
/// ClipboardWatcher's consent dance applies — picking "Clipper" in the share
/// sheet is itself the explicit user action iOS wants to see.
@MainActor
final class IncomingLink: ObservableObject {
    static let shared = IncomingLink()

    @Published var url: String?

    private init() {}

    func handle(_ openedURL: URL) {
        guard
            openedURL.scheme == "clipper",
            openedURL.host == "resolve",
            let components = URLComponents(url: openedURL, resolvingAgainstBaseURL: false),
            let link = components.queryItems?.first(where: { $0.name == "url" })?.value,
            !link.isEmpty
        else { return }
        url = link
    }
}
