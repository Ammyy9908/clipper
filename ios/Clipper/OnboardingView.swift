import SwiftUI

/// Comprehensive onboarding flow highlighting all supported media source platforms
/// and storage/save destinations with custom brand logos and fluid interactions.
struct OnboardingView: View {
    let onFinish: () -> Void
    @State private var page = 0

    var body: some View {
        ZStack {
            Theme.background.ignoresSafeArea()

            VStack(spacing: 0) {
                // Top Bar with Skip
                HStack {
                    HStack(spacing: 6) {
                        Circle()
                            .fill(Theme.accent)
                            .frame(width: 8, height: 8)
                        Text("clipPlay")
                            .font(.system(size: 15, weight: .bold, design: .rounded))
                            .foregroundStyle(Theme.ink)
                    }

                    Spacer()

                    if page < 3 {
                        Button("Skip") {
                            UIImpactFeedbackGenerator(style: .light).impactOccurred()
                            onFinish()
                        }
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Theme.inkSecondary)
                        .padding(.horizontal, 12)
                        .padding(.vertical, 6)
                        .background(
                            Capsule()
                                .fill(Theme.surface)
                                .overlay(Capsule().stroke(Theme.hairline, lineWidth: 1))
                        )
                    }
                }
                .padding(.horizontal, 24)
                .padding(.top, 16)
                .frame(height: 44)

                // Paged Onboarding Content
                TabView(selection: $page) {
                    PlatformsSourcePage()
                        .tag(0)

                    SaveDestinationsPage()
                        .tag(1)

                    PlayerAndSplitPage()
                        .tag(2)

                    QuickSharePage()
                        .tag(3)
                }
                .tabViewStyle(.page(indexDisplayMode: .never))
                .animation(.spring(response: 0.45, dampingFraction: 0.85), value: page)

                // Bottom Controls
                VStack(spacing: 16) {
                    pageIndicator

                    Button {
                        UIImpactFeedbackGenerator(style: .medium).impactOccurred()
                        if page == 3 {
                            onFinish()
                        } else {
                            withAnimation(.spring(response: 0.4, dampingFraction: 0.8)) {
                                page += 1
                            }
                        }
                    } label: {
                        HStack(spacing: 8) {
                            Text(page == 3 ? "Get Started" : "Continue")
                                .font(.system(size: 16, weight: .bold, design: .rounded))
                            Image(systemName: page == 3 ? "sparkles" : "arrow.right")
                                .font(.system(size: 14, weight: .bold))
                        }
                        .foregroundStyle(.white)
                        .frame(maxWidth: .infinity)
                        .frame(height: 54)
                        .background(
                            RoundedRectangle(cornerRadius: Theme.controlRadius, style: .continuous)
                                .fill(Theme.accent)
                                .shadow(color: Theme.accent.opacity(0.35), radius: 12, y: 6)
                        )
                    }
                    .padding(.horizontal, 24)
                }
                .padding(.bottom, 24)
            }
        }
    }

    private var pageIndicator: some View {
        HStack(spacing: 6) {
            ForEach(0..<4, id: \.self) { index in
                Capsule()
                    .fill(index == page ? Theme.accent : Theme.hairline)
                    .frame(width: index == page ? 24 : 7, height: 7)
                    .animation(.spring(response: 0.35, dampingFraction: 0.75), value: page)
            }
        }
    }
}

// MARK: - Page 1: Supported Platforms (Where to Save From)

private struct PlatformsSourcePage: View {
    private let platforms: [PlatformItem] = [
        PlatformItem(name: "YouTube", category: "Videos & 4K", type: .youtube),
        PlatformItem(name: "Instagram", category: "Reels & Posts", type: .instagram),
        PlatformItem(name: "TikTok", category: "No Watermark", type: .tiktok),
        PlatformItem(name: "X / Twitter", category: "HD Clips", type: .twitter),
        PlatformItem(name: "Facebook", category: "Reels & Watch", type: .facebook),
        PlatformItem(name: "Pinterest", category: "Video Pins", type: .pinterest),
        PlatformItem(name: "SoundCloud", category: "High Res Audio", type: .soundcloud),
        PlatformItem(name: "Reddit", category: "Clips & Audio", type: .reddit),
        PlatformItem(name: "Twitch", category: "VODs & Clips", type: .twitch),
        PlatformItem(name: "Vimeo", category: "HD Streams", type: .vimeo),
        PlatformItem(name: "1,700+ Sites", category: "Universal Link", type: .universal)
    ]

    var body: some View {
        VStack(spacing: 20) {
            Spacer(minLength: 8)

            VStack(spacing: 8) {
                Text("SAVE FROM ANY PLATFORM")
                    .font(.system(size: 12, weight: .bold, design: .rounded))
                    .foregroundStyle(Theme.accent)
                    .tracking(1.2)

                Text("All Your Media,\nOne Universal App")
                    .font(.system(size: 28, weight: .heavy, design: .rounded))
                    .foregroundStyle(Theme.ink)
                    .multilineTextAlignment(.center)
                    .lineSpacing(2)

                Text("Paste links from any platform to extract high-quality video or crystal-clear audio in seconds.")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 28)
            }

            // Interactive Platform Badges Grid
            ScrollView(showsIndicators: false) {
                LazyVGrid(columns: [GridItem(.flexible(), spacing: 10), GridItem(.flexible(), spacing: 10)], spacing: 10) {
                    ForEach(platforms) { p in
                        HStack(spacing: 12) {
                            PlatformLogoView(type: p.type, size: 36)

                            VStack(alignment: .leading, spacing: 2) {
                                Text(p.name)
                                    .font(.system(size: 14, weight: .bold, design: .rounded))
                                    .foregroundStyle(Theme.ink)
                                    .lineLimit(1)

                                Text(p.category)
                                    .font(.system(size: 11, weight: .medium))
                                    .foregroundStyle(Theme.inkSecondary)
                                    .lineLimit(1)
                            }
                            Spacer(minLength: 0)
                        }
                        .padding(.horizontal, 12)
                        .padding(.vertical, 10)
                        .background(
                            RoundedRectangle(cornerRadius: 16, style: .continuous)
                                .fill(Theme.surface)
                                .overlay(
                                    RoundedRectangle(cornerRadius: 16, style: .continuous)
                                        .stroke(Theme.hairline, lineWidth: 1)
                                )
                        )
                    }
                }
                .padding(.horizontal, 20)
                .padding(.vertical, 4)
            }
            .frame(maxHeight: 330)

            Spacer(minLength: 8)
        }
    }
}

// MARK: - Page 2: Save Destinations (Where to Save To)

private struct SaveDestinationsPage: View {
    private let destinations: [DestinationItem] = [
        DestinationItem(
            title: "Photos Library",
            subtitle: "1-Tap Auto-Save directly to your iPhone Camera Roll",
            icon: "photo.stack.fill",
            color: Color.pink,
            badge: "Direct"
        ),
        DestinationItem(
            title: "Apple Files & iCloud",
            subtitle: "Save to local device storage or sync with iCloud Drive",
            icon: "folder.fill.badge.gearshape",
            color: Color.blue,
            badge: "Offline"
        ),
        DestinationItem(
            title: "Google Drive",
            subtitle: "Backup videos & audio to Google Drive folders",
            icon: "externaldrive.fill.badge.icloud",
            color: Color(red: 0.09, green: 0.51, blue: 0.96),
            badge: "Cloud"
        ),
        DestinationItem(
            title: "Dropbox",
            subtitle: "Seamless cloud export and cross-device sync",
            icon: "shippingbox.fill",
            color: Color(red: 0.0, green: 0.38, blue: 0.95),
            badge: "Cloud"
        ),
        DestinationItem(
            title: "Music & Playlists",
            subtitle: "Save audio tracks into your built-in music player",
            icon: "music.note.list",
            color: Theme.accent,
            badge: "Player"
        )
    ]

    var body: some View {
        VStack(spacing: 18) {
            Spacer(minLength: 8)

            VStack(spacing: 8) {
                Text("SAVE & EXPORT DESTINATIONS")
                    .font(.system(size: 12, weight: .bold, design: .rounded))
                    .foregroundStyle(Theme.accent)
                    .tracking(1.2)

                Text("Save Anywhere You Want")
                    .font(.system(size: 28, weight: .heavy, design: .rounded))
                    .foregroundStyle(Theme.ink)
                    .multilineTextAlignment(.center)

                Text("Export directly to Photos, Apple Files, Google Drive, Dropbox, or your offline Music library.")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 28)
            }

            VStack(spacing: 10) {
                ForEach(destinations) { d in
                    HStack(spacing: 14) {
                        ZStack {
                            RoundedRectangle(cornerRadius: 12, style: .continuous)
                                .fill(d.color.opacity(0.14))
                                .frame(width: 44, height: 44)
                            Image(systemName: d.icon)
                                .font(.system(size: 20, weight: .semibold))
                                .foregroundStyle(d.color)
                        }

                        VStack(alignment: .leading, spacing: 3) {
                            HStack {
                                Text(d.title)
                                    .font(.system(size: 15, weight: .bold, design: .rounded))
                                    .foregroundStyle(Theme.ink)

                                Spacer()

                                Text(d.badge)
                                    .font(.system(size: 10, weight: .bold, design: .rounded))
                                    .foregroundStyle(d.color)
                                    .padding(.horizontal, 8)
                                    .padding(.vertical, 3)
                                    .background(
                                        Capsule()
                                            .fill(d.color.opacity(0.12))
                                    )
                            }

                            Text(d.subtitle)
                                .font(.system(size: 12, weight: .medium))
                                .foregroundStyle(Theme.inkSecondary)
                                .lineLimit(1)
                        }
                    }
                    .padding(.horizontal, 14)
                    .padding(.vertical, 10)
                    .background(
                        RoundedRectangle(cornerRadius: 18, style: .continuous)
                            .fill(Theme.surface)
                            .overlay(
                                RoundedRectangle(cornerRadius: 18, style: .continuous)
                                    .stroke(Theme.hairline, lineWidth: 1)
                            )
                    )
                }
            }
            .padding(.horizontal, 20)

            Spacer(minLength: 8)
        }
    }
}

// MARK: - Page 3: Offline Player & Video Reframe

private struct PlayerAndSplitPage: View {
    var body: some View {
        VStack(spacing: 22) {
            Spacer(minLength: 8)

            VStack(spacing: 8) {
                Text("POWERFUL MEDIA TOOLS")
                    .font(.system(size: 12, weight: .bold, design: .rounded))
                    .foregroundStyle(Theme.accent)
                    .tracking(1.2)

                Text("Built-in Player &\nSmart Splitter")
                    .font(.system(size: 28, weight: .heavy, design: .rounded))
                    .foregroundStyle(Theme.ink)
                    .multilineTextAlignment(.center)

                Text("Listen to music with lock-screen controls, or crop videos into vertical 9:16 Shorts & Reels.")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 28)
            }

            VStack(spacing: 12) {
                // Player Feature Card
                HStack(spacing: 16) {
                    ZStack {
                        Circle()
                            .fill(Theme.accent.opacity(0.15))
                            .frame(width: 50, height: 50)
                        Image(systemName: "waveform")
                            .font(.system(size: 24, weight: .bold))
                            .foregroundStyle(Theme.accent)
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text("High-Res Audio Player")
                            .font(.system(size: 16, weight: .bold, design: .rounded))
                            .foregroundStyle(Theme.ink)
                        Text("Background playback, custom playlists, artwork extraction, and lock-screen widget.")
                            .font(.system(size: 13, weight: .medium))
                            .foregroundStyle(Theme.inkSecondary)
                    }
                    Spacer()
                }
                .padding(16)
                .background(
                    RoundedRectangle(cornerRadius: 20, style: .continuous)
                        .fill(Theme.surface)
                        .overlay(
                            RoundedRectangle(cornerRadius: 20, style: .continuous)
                                .stroke(Theme.hairline, lineWidth: 1)
                        )
                )

                // Split Feature Card
                HStack(spacing: 16) {
                    ZStack {
                        Circle()
                            .fill(Color.purple.opacity(0.15))
                            .frame(width: 50, height: 50)
                        Image(systemName: "crop.rotate")
                            .font(.system(size: 22, weight: .bold))
                            .foregroundStyle(Color.purple)
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text("Auto 9:16 Reframe")
                            .font(.system(size: 16, weight: .bold, design: .rounded))
                            .foregroundStyle(Theme.ink)
                        Text("Split long podcasts and clips into vertical videos ready for Instagram & TikTok.")
                            .font(.system(size: 13, weight: .medium))
                            .foregroundStyle(Theme.inkSecondary)
                    }
                    Spacer()
                }
                .padding(16)
                .background(
                    RoundedRectangle(cornerRadius: 20, style: .continuous)
                        .fill(Theme.surface)
                        .overlay(
                            RoundedRectangle(cornerRadius: 20, style: .continuous)
                                .stroke(Theme.hairline, lineWidth: 1)
                        )
                )
            }
            .padding(.horizontal, 20)

            Spacer(minLength: 8)
        }
    }
}

// MARK: - Page 4: Instant Share & Clipboard

private struct QuickSharePage: View {
    var body: some View {
        VStack(spacing: 22) {
            Spacer(minLength: 8)

            ZStack {
                Circle()
                    .fill(Theme.accent.opacity(0.12))
                    .frame(width: 110, height: 110)

                Image(systemName: "square.and.arrow.up.circle.fill")
                    .font(.system(size: 60, weight: .semibold))
                    .foregroundStyle(Theme.accent)
            }

            VStack(spacing: 10) {
                Text("LIGHTNING FAST WORKFLOW")
                    .font(.system(size: 12, weight: .bold, design: .rounded))
                    .foregroundStyle(Theme.accent)
                    .tracking(1.2)

                Text("Copy. Paste. Done.")
                    .font(.system(size: 28, weight: .heavy, design: .rounded))
                    .foregroundStyle(Theme.ink)

                Text("Copy a link from any app — clipPlay detects it instantly on launch. You can also share directly using the iOS Share Sheet.")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, 32)
            }

            VStack(spacing: 10) {
                HStack(spacing: 12) {
                    Image(systemName: "doc.on.clipboard.fill")
                        .font(.system(size: 18, weight: .semibold))
                        .foregroundStyle(Theme.accent)
                    Text("Auto-detect copied links from clipboard")
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(Theme.ink)
                    Spacer()
                }
                .padding(14)
                .background(
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .fill(Theme.surface)
                        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(Theme.hairline, lineWidth: 1))
                )

                HStack(spacing: 12) {
                    Image(systemName: "sparkles")
                        .font(.system(size: 18, weight: .semibold))
                        .foregroundStyle(Color.orange)
                    Text("Share Sheet extension for 1-tap download")
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(Theme.ink)
                    Spacer()
                }
                .padding(14)
                .background(
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .fill(Theme.surface)
                        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(Theme.hairline, lineWidth: 1))
                )
            }
            .padding(.horizontal, 24)

            Spacer(minLength: 8)
        }
    }
}

// MARK: - Platform Logo Component

enum PlatformLogoType {
    case youtube, instagram, tiktok, twitter, facebook, pinterest
    case soundcloud, reddit, twitch, vimeo, universal
}

struct PlatformLogoView: View {
    let type: PlatformLogoType
    var size: CGFloat = 36

    var body: some View {
        ZStack {
            switch type {
            case .youtube:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(Color(red: 1.0, green: 0.0, blue: 0.0))
                Image(systemName: "play.fill")
                    .font(.system(size: size * 0.42, weight: .bold))
                    .foregroundStyle(.white)

            case .instagram:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(LinearGradient(
                        colors: [
                            Color(red: 0.51, green: 0.23, blue: 0.74),
                            Color(red: 0.88, green: 0.19, blue: 0.42),
                            Color(red: 0.99, green: 0.69, blue: 0.28)
                        ],
                        startPoint: .topTrailing, endPoint: .bottomLeading
                    ))
                Image(systemName: "camera.fill")
                    .font(.system(size: size * 0.46, weight: .semibold))
                    .foregroundStyle(.white)

            case .tiktok:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(Color.black)
                ZStack {
                    Image(systemName: "music.note")
                        .font(.system(size: size * 0.48, weight: .bold))
                        .foregroundStyle(Color(red: 0.0, green: 0.95, blue: 0.92))
                        .offset(x: -1, y: -1)
                    Image(systemName: "music.note")
                        .font(.system(size: size * 0.48, weight: .bold))
                        .foregroundStyle(Color(red: 1.0, green: 0.0, blue: 0.31))
                        .offset(x: 1, y: 1)
                    Image(systemName: "music.note")
                        .font(.system(size: size * 0.48, weight: .bold))
                        .foregroundStyle(.white)
                }

            case .twitter:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(Color(white: 0.12))
                Text("𝕏")
                    .font(.system(size: size * 0.52, weight: .black, design: .serif))
                    .foregroundStyle(.white)

            case .facebook:
                Circle()
                    .fill(Color(red: 0.09, green: 0.51, blue: 0.96))
                Text("f")
                    .font(.system(size: size * 0.62, weight: .heavy, design: .rounded))
                    .foregroundStyle(.white)
                    .offset(x: size * 0.04, y: size * 0.03)

            case .pinterest:
                Circle()
                    .fill(Color(red: 0.90, green: 0.08, blue: 0.16))
                Text("P")
                    .font(.system(size: size * 0.6, weight: .black, design: .serif))
                    .foregroundStyle(.white)
                    .italic()

            case .soundcloud:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(LinearGradient(
                        colors: [Color(red: 1.0, green: 0.45, blue: 0.0), Color(red: 1.0, green: 0.20, blue: 0.0)],
                        startPoint: .top, endPoint: .bottom
                    ))
                Image(systemName: "waveform")
                    .font(.system(size: size * 0.48, weight: .bold))
                    .foregroundStyle(.white)

            case .reddit:
                Circle()
                    .fill(Color(red: 1.0, green: 0.27, blue: 0.0))
                Image(systemName: "bubble.left.and.bubble.right.fill")
                    .font(.system(size: size * 0.44))
                    .foregroundStyle(.white)

            case .twitch:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(Color(red: 0.57, green: 0.27, blue: 1.0))
                Image(systemName: "tv.fill")
                    .font(.system(size: size * 0.46))
                    .foregroundStyle(.white)

            case .vimeo:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(Color(red: 0.10, green: 0.71, blue: 0.96))
                Text("v")
                    .font(.system(size: size * 0.6, weight: .black, design: .rounded))
                    .foregroundStyle(.white)
                    .italic()

            case .universal:
                RoundedRectangle(cornerRadius: size * 0.24, style: .continuous)
                    .fill(LinearGradient(
                        colors: [Theme.accent, Color.orange],
                        startPoint: .topLeading, endPoint: .bottomTrailing
                    ))
                Image(systemName: "globe.americas.fill")
                    .font(.system(size: size * 0.48, weight: .bold))
                    .foregroundStyle(.white)
            }
        }
        .frame(width: size, height: size)
    }
}

// MARK: - Models

private struct PlatformItem: Identifiable {
    let id = UUID()
    let name: String
    let category: String
    let type: PlatformLogoType
}

private struct DestinationItem: Identifiable {
    let id = UUID()
    let title: String
    let subtitle: String
    let icon: String
    let color: Color
    let badge: String
}

