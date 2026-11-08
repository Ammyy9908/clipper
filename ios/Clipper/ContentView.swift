import SwiftUI

struct ContentView: View {
    @StateObject private var clipboard = ClipboardWatcher()
    @StateObject private var downloadModel = DownloadModel()
    @ObservedObject private var library = LibraryStore.shared
    @ObservedObject private var incoming = IncomingLink.shared
    @ObservedObject private var player = PlayerModel.shared
    @Environment(\.scenePhase) private var scenePhase
    @State private var selectedTab: Tab = .download

    private enum Tab { case download, video, split, music }

    /// UITabBar's standard content height — TabView doesn't expose this, and
    /// .safeAreaInset(edge: .bottom) on a TabView doesn't reliably reserve
    /// space above the bar the way it does on a plain stack (the bar is
    /// UIKit-managed underneath, not a participant in that layout pass), so
    /// this floats the mini player with an explicit offset instead.
    private static let tabBarHeight: CGFloat = 49

    var body: some View {
        ZStack(alignment: .bottom) {
            TabView(selection: $selectedTab) {
                DownloadTabView(clipboard: clipboard, model: downloadModel)
                    .background(Theme.background.ignoresSafeArea())
                    .tabItem { Label("Download", systemImage: "arrow.down.circle.fill") }
                    .tag(Tab.download)

                VideoTabView(library: library)
                    .background(Theme.background.ignoresSafeArea())
                    .tabItem { Label("Videos", systemImage: "video.fill") }
                    .tag(Tab.video)

                SplitTabView(library: library)
                    .background(Theme.background.ignoresSafeArea())
                    .tabItem { Label("Split", systemImage: "square.split.2x1") }
                    .tag(Tab.split)

                MusicPlayerHomeView(library: library)
                    .background(Theme.background.ignoresSafeArea())
                    .tabItem { Label("Music", systemImage: "music.note") }
                    .tag(Tab.music)
            }
            .tint(Theme.accent)

            if let nowPlaying = player.currentClip, !player.isExpanded {
                MiniPlayerBar(model: player, clip: nowPlaying)
                    .padding(.horizontal, 20)
                    .padding(.bottom, Self.tabBarHeight + 8)
            }
        }
        .overlay(alignment: .top) {
            CloudUploadToastView()
                .frame(maxWidth: .infinity)
                .ignoresSafeArea(edges: .top)
        }
        .task { await clipboard.check() }
        .onChange(of: scenePhase) { _, new in
            if new == .active { Task { await clipboard.check() } }
        }
        .onChange(of: incoming.url) { _, new in
            guard let link = new else { return }
            incoming.url = nil
            selectedTab = .download
            Task { await downloadModel.resolve(link) }
        }
        .fullScreenCover(isPresented: $player.isExpanded) {
            NowPlayingSheet()
        }
    }
}
