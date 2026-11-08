import SwiftUI

/// Shown briefly on every launch, unconditionally — unlike onboarding, this
/// isn't gated by a "first run" flag.
struct SplashScreen: View {
    @State private var scale: CGFloat = 0.85
    @State private var opacity: Double = 0

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()
            VStack(spacing: 16) {
                ZStack {
                    RoundedRectangle(cornerRadius: 28, style: .continuous)
                        .fill(Theme.accent)
                        .frame(width: 96, height: 96)
                        .shadow(color: Theme.accent.opacity(0.35), radius: 24, y: 12)
                    Image(systemName: "play.circle.fill")
                        .font(.system(size: 44, weight: .bold))
                        .foregroundStyle(.white)
                }

                VStack(spacing: 4) {
                    Text("clipPlay")
                        .font(.system(size: 30, weight: .bold, design: .rounded))
                        .foregroundStyle(Theme.ink)
                    Text("Download. Organize. Play.")
                        .font(.system(size: 14, weight: .medium))
                        .foregroundStyle(Theme.inkSecondary)
                }
            }
            .scaleEffect(scale)
            .opacity(opacity)
        }
        .onAppear {
            withAnimation(.spring(response: 0.55, dampingFraction: 0.7)) {
                scale = 1
                opacity = 1
            }
        }
    }
}
