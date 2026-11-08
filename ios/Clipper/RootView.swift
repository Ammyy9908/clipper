import SwiftUI

/// Splash always shows briefly on launch, regardless of run count. Onboarding
/// only shows once, gated by a persisted flag — after that, straight to the
/// app on every subsequent launch.
struct RootView: View {
    @AppStorage("hasCompletedOnboarding") private var hasCompletedOnboarding = false
    @State private var showSplash = true

    var body: some View {
        Group {
            if showSplash {
                SplashScreen()
                    .transition(.opacity)
            } else if !hasCompletedOnboarding {
                OnboardingView {
                    withAnimation(.easeInOut) { hasCompletedOnboarding = true }
                }
                .transition(.opacity)
            } else {
                ContentView()
                    .transition(.opacity)
            }
        }
        .task {
            try? await Task.sleep(nanoseconds: 1_100_000_000)
            withAnimation(.easeInOut(duration: 0.35)) { showSplash = false }
        }
    }
}
