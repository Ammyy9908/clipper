import SwiftUI

@main
struct ClipperApp: App {
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            RootView()
                .onOpenURL { url in
                    guard !DropboxManager.shared.handleRedirect(url) else { return }
                    guard !GoogleDriveManager.shared.handleRedirect(url) else { return }
                    IncomingLink.shared.handle(url)
                }
                .onChange(of: scenePhase) { _, newPhase in
                    if newPhase == .active {
                        DownloadQueue.shared.rehydrate()
                    }
                }
        }
    }
}
