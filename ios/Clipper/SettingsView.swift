import SwiftUI

struct SettingsView: View {
    @Environment(\.dismiss) private var dismiss
    @ObservedObject private var userSession = UserSession.shared
    @ObservedObject private var googleDrive = GoogleDriveManager.shared
    @ObservedObject private var dropbox = DropboxManager.shared
    @ObservedObject private var cloudManager = CloudStorageManager.shared
    @ObservedObject private var library = LibraryStore.shared

    @State private var showingOnboarding = false
    @State private var confirmingClearStorage = false

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 20) {
                    accountQuotaSection
                    cloudDestinationSection
                    connectedAccountsSection
                    autoUploadSection
                    localStorageSection
                    appTourSection
                    aboutSection
                }
                .padding(.horizontal, 20)
                .padding(.top, 16)
                .padding(.bottom, 32)
            }
            .background(Theme.background.ignoresSafeArea())
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Done") { dismiss() }
                        .font(.system(size: 16, weight: .bold))
                        .foregroundStyle(Theme.accent)
                }
            }
            .sheet(isPresented: $showingOnboarding) {
                OnboardingView { showingOnboarding = false }
            }
            .confirmationDialog("Clear All Local Downloads?", isPresented: $confirmingClearStorage, titleVisibility: .visible) {
                Button("Delete All Local Media", role: .destructive) {
                    library.removeAll()
                    cloudManager.showToast("Local media library cleared", type: .info)
                }
            } message: {
                Text("This removes downloaded files from your device storage. Media uploaded to Cloud or Photos remains safe.")
            }
        }
    }

    // MARK: - Account & Daily Quota

    private var accountQuotaSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionHeader("ACCOUNT & DAILY QUOTA", icon: "bolt.badge.clock.fill")

            VStack(spacing: 16) {
                if userSession.isSignedIn {
                    // Authenticated State (20 downloads/day)
                    HStack(spacing: 14) {
                        ZStack {
                            Circle()
                                .fill(Theme.accent.opacity(0.12))
                                .frame(width: 44, height: 44)
                            Image(systemName: "person.crop.circle.badge.checkmark")
                                .font(.system(size: 20))
                                .foregroundStyle(Theme.accent)
                        }

                        VStack(alignment: .leading, spacing: 3) {
                            Text(userSession.userName ?? userSession.userEmail ?? "Google Account")
                                .font(.system(size: 16, weight: .bold))
                                .foregroundStyle(Theme.ink)
                            Text(userSession.userEmail ?? "Signed In")
                                .font(.system(size: 12))
                                .foregroundStyle(Theme.inkSecondary)
                        }

                        Spacer()

                        Button {
                            userSession.signOut()
                        } label: {
                            Text("Sign Out")
                                .font(.system(size: 13, weight: .bold))
                                .foregroundStyle(Theme.danger)
                                .padding(.horizontal, 14)
                                .padding(.vertical, 7)
                                .background(Capsule().fill(Theme.danger.opacity(0.12)))
                        }
                    }

                    Divider().background(Theme.hairline)

                    // Quota Usage Bar
                    let limit = userSession.quota?.limit ?? 20
                    let remaining = userSession.quota?.remaining ?? limit
                    let used = max(0, limit - remaining)
                    let progress = limit > 0 ? Double(used) / Double(limit) : 0.0

                    VStack(spacing: 8) {
                        HStack {
                            Text("Daily Downloads")
                                .font(.system(size: 13, weight: .semibold))
                                .foregroundStyle(Theme.ink)
                            Spacer()
                            Text("\(used) of \(limit) used today")
                                .font(.system(size: 12, weight: .bold, design: .rounded))
                                .foregroundStyle(Theme.accent)
                        }

                        GeometryReader { geo in
                            ZStack(alignment: .leading) {
                                Capsule().fill(Theme.hairline)
                                    .frame(height: 8)
                                Capsule().fill(Theme.accent)
                                    .frame(width: geo.size.width * CGFloat(min(1.0, max(0.0, progress))), height: 8)
                            }
                        }
                        .frame(height: 8)

                        HStack {
                            Text("\(remaining) remaining")
                                .font(.system(size: 11, weight: .medium))
                                .foregroundStyle(Theme.inkTertiary)
                            Spacer()
                            Text("Resets at 00:00 UTC")
                                .font(.system(size: 11, weight: .medium))
                                .foregroundStyle(Theme.inkTertiary)
                        }
                    }
                } else {
                    // Anonymous State (3 downloads/day)
                    let limit = userSession.quota?.limit ?? 3
                    let remaining = userSession.quota?.remaining ?? limit
                    let used = max(0, limit - remaining)
                    let progress = limit > 0 ? Double(used) / Double(limit) : 0.0

                    VStack(alignment: .leading, spacing: 12) {
                        HStack {
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Free Tier")
                                    .font(.system(size: 16, weight: .bold))
                                    .foregroundStyle(Theme.ink)
                                Text("\(remaining) of \(limit) free downloads left today")
                                    .font(.system(size: 12, weight: .medium))
                                    .foregroundStyle(Theme.inkSecondary)
                            }
                            Spacer()
                            Text("3 / day")
                                .font(.system(size: 13, weight: .bold, design: .rounded))
                                .foregroundStyle(Theme.accent)
                                .padding(.horizontal, 10)
                                .padding(.vertical, 4)
                                .background(Capsule().fill(Theme.accent.opacity(0.12)))
                        }

                        GeometryReader { geo in
                            ZStack(alignment: .leading) {
                                Capsule().fill(Theme.hairline)
                                    .frame(height: 8)
                                Capsule().fill(used >= limit ? Theme.danger : Theme.accent)
                                    .frame(width: geo.size.width * CGFloat(min(1.0, max(0.0, progress))), height: 8)
                            }
                        }
                        .frame(height: 8)

                        Button {
                            userSession.signInWithGoogle()
                        } label: {
                            HStack(spacing: 8) {
                                if userSession.isSigningIn {
                                    ProgressView().controlSize(.small).tint(.white)
                                } else {
                                    Image(systemName: "person.crop.circle.badge.plus")
                                        .font(.system(size: 15, weight: .bold))
                                }
                                Text("Sign In with Google to get 20/day")
                                    .font(.system(size: 14, weight: .bold))
                            }
                            .foregroundStyle(.white)
                            .frame(maxWidth: .infinity)
                            .frame(height: 44)
                            .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(Theme.accent))
                        }
                        .buttonStyle(.plain)
                        .disabled(userSession.isSigningIn)

                        if let err = userSession.authError {
                            Text(err)
                                .font(.system(size: 11))
                                .foregroundStyle(Theme.danger)
                        }
                    }
                }
            }
            .padding(18)
            .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
        }
    }

    // MARK: - Preferred Cloud Destination

    private var cloudDestinationSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionHeader("DEFAULT CLOUD DESTINATION", icon: "icloud.fill")

            VStack(spacing: 12) {
                Text("Choose where 1-tap \"Save to Cloud\" sends your videos and music.")
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(Theme.inkSecondary)
                    .frame(maxWidth: .infinity, alignment: .leading)

                HStack(spacing: 10) {
                    destinationOption(
                        dest: .googleDrive,
                        title: "Google Drive",
                        icon: "externaldrive.fill.badge.icloud",
                        color: Color(red: 0.09, green: 0.51, blue: 0.96),
                        isLinked: googleDrive.isLinked
                    )

                    destinationOption(
                        dest: .dropbox,
                        title: "Dropbox",
                        icon: "shippingbox.fill",
                        color: Color(red: 0.0, green: 0.38, blue: 0.95),
                        isLinked: dropbox.isLinked
                    )
                }
            }
            .padding(18)
            .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
        }
    }

    private func destinationOption(
        dest: CloudDestination,
        title: String,
        icon: String,
        color: Color,
        isLinked: Bool
    ) -> some View {
        let isSelected = cloudManager.preferredDestination == dest

        return Button {
            withAnimation(.easeInOut(duration: 0.2)) {
                cloudManager.preferredDestination = dest
            }
        } label: {
            VStack(spacing: 10) {
                HStack {
                    Image(systemName: icon)
                        .font(.system(size: 20))
                        .foregroundStyle(color)
                    Spacer()
                    if isSelected {
                        Image(systemName: "checkmark.circle.fill")
                            .font(.system(size: 18))
                            .foregroundStyle(Theme.accent)
                    } else {
                        Circle()
                            .strokeBorder(Theme.hairline, lineWidth: 1.5)
                            .frame(width: 18, height: 18)
                    }
                }

                VStack(alignment: .leading, spacing: 2) {
                    Text(title)
                        .font(.system(size: 15, weight: .bold))
                        .foregroundStyle(Theme.ink)
                        .frame(maxWidth: .infinity, alignment: .leading)

                    HStack(spacing: 4) {
                        Circle()
                            .fill(isLinked ? Color.green : Theme.inkTertiary)
                            .frame(width: 6, height: 6)
                        Text(isLinked ? "Connected" : "Not Linked")
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(isLinked ? Color.green : Theme.inkTertiary)
                    }
                }
            }
            .padding(14)
            .background(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .fill(isSelected ? Theme.accent.opacity(0.08) : Theme.background)
            )
            .overlay(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .strokeBorder(isSelected ? Theme.accent : Theme.hairline, lineWidth: isSelected ? 1.5 : 1)
            )
        }
        .buttonStyle(.plain)
    }

    // MARK: - Connected Accounts

    private var connectedAccountsSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionHeader("CONNECTED ACCOUNTS", icon: "person.crop.circle.badge.checkmark")

            VStack(spacing: 0) {
                // Google Drive Row
                HStack(spacing: 14) {
                    ZStack {
                        Circle()
                            .fill(Color(red: 0.09, green: 0.51, blue: 0.96).opacity(0.12))
                            .frame(width: 42, height: 42)
                        Image(systemName: "externaldrive.fill.badge.icloud")
                            .font(.system(size: 18))
                            .foregroundStyle(Color(red: 0.09, green: 0.51, blue: 0.96))
                    }

                    VStack(alignment: .leading, spacing: 2) {
                        Text("Google Drive")
                            .font(.system(size: 16, weight: .semibold))
                            .foregroundStyle(Theme.ink)
                        Text(googleDrive.isLinked ? (googleDrive.userEmail ?? "Connected") : "Not connected")
                            .font(.system(size: 12))
                            .foregroundStyle(googleDrive.isLinked ? Theme.inkSecondary : Theme.inkTertiary)
                    }

                    Spacer()

                    Button {
                        if googleDrive.isLinked {
                            googleDrive.unlink()
                        } else {
                            googleDrive.authorize()
                        }
                    } label: {
                        Text(googleDrive.isLinked ? "Disconnect" : "Connect")
                            .font(.system(size: 13, weight: .bold))
                            .foregroundStyle(googleDrive.isLinked ? Theme.danger : .white)
                            .padding(.horizontal, 14)
                            .padding(.vertical, 7)
                            .background(
                                Capsule().fill(googleDrive.isLinked ? Theme.danger.opacity(0.12) : Color(red: 0.09, green: 0.51, blue: 0.96))
                            )
                    }
                }
                .padding(16)

                Divider().background(Theme.hairline)

                // Dropbox Row
                HStack(spacing: 14) {
                    ZStack {
                        Circle()
                            .fill(Color(red: 0.0, green: 0.38, blue: 0.95).opacity(0.12))
                            .frame(width: 42, height: 42)
                        Image(systemName: "shippingbox.fill")
                            .font(.system(size: 18))
                            .foregroundStyle(Color(red: 0.0, green: 0.38, blue: 0.95))
                    }

                    VStack(alignment: .leading, spacing: 2) {
                        Text("Dropbox")
                            .font(.system(size: 16, weight: .semibold))
                            .foregroundStyle(Theme.ink)
                        Text(dropbox.isLinked ? "Connected (/clipPlay)" : "Not connected")
                            .font(.system(size: 12))
                            .foregroundStyle(dropbox.isLinked ? Theme.inkSecondary : Theme.inkTertiary)
                    }

                    Spacer()

                    Button {
                        if dropbox.isLinked {
                            dropbox.unlink()
                        } else {
                            dropbox.authorize()
                        }
                    } label: {
                        Text(dropbox.isLinked ? "Disconnect" : "Connect")
                            .font(.system(size: 13, weight: .bold))
                            .foregroundStyle(dropbox.isLinked ? Theme.danger : .white)
                            .padding(.horizontal, 14)
                            .padding(.vertical, 7)
                            .background(
                                Capsule().fill(dropbox.isLinked ? Theme.danger.opacity(0.12) : Color(red: 0.0, green: 0.38, blue: 0.95))
                            )
                    }
                }
                .padding(16)
            }
            .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
        }
    }

    // MARK: - Auto-Upload & Auto-Save

    private var autoUploadSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionHeader("AUTOMATION & SYNC", icon: "arrow.triangle.2.circlepath")

            VStack(spacing: 0) {
                // Auto-Upload to Cloud Toggle
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Auto-Upload to Cloud")
                            .font(.system(size: 15, weight: .semibold))
                            .foregroundStyle(Theme.ink)
                        Text("Upload every finished download to your preferred cloud")
                            .font(.system(size: 12))
                            .foregroundStyle(Theme.inkSecondary)
                    }
                    Spacer()
                    Toggle("", isOn: $cloudManager.autoUploadToCloud)
                        .labelsHidden()
                        .tint(Theme.accent)
                }
                .padding(16)

                Divider().background(Theme.hairline)

                // Auto-Save to Photos Toggle
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Auto-Save Videos to Photos")
                            .font(.system(size: 15, weight: .semibold))
                            .foregroundStyle(Theme.ink)
                        Text("Save downloaded videos directly to iPhone Camera Roll")
                            .font(.system(size: 12))
                            .foregroundStyle(Theme.inkSecondary)
                    }
                    Spacer()
                    Toggle("", isOn: $library.autoSave)
                        .labelsHidden()
                        .tint(Theme.accent)
                }
                .padding(16)
            }
            .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
        }
    }

    // MARK: - Local Storage

    private var localStorageSection: some View {
        let totalBytes = library.clips.reduce(Int64(0)) { $0 + $1.sizeBytes }
        let formattedSize = ByteCountFormatter.string(fromByteCount: totalBytes, countStyle: .file)

        return VStack(alignment: .leading, spacing: 10) {
            sectionHeader("LOCAL STORAGE", icon: "internaldrive.fill")

            VStack(spacing: 14) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Downloaded Media")
                            .font(.system(size: 15, weight: .semibold))
                            .foregroundStyle(Theme.ink)
                        Text("\(library.clips.count) saved clip\(library.clips.count == 1 ? "" : "s")")
                            .font(.system(size: 12))
                            .foregroundStyle(Theme.inkSecondary)
                    }
                    Spacer()
                    Text(formattedSize)
                        .font(.system(size: 15, weight: .bold, design: .rounded))
                        .foregroundStyle(Theme.accent)
                }

                if !library.clips.isEmpty {
                    Button {
                        confirmingClearStorage = true
                    } label: {
                        HStack {
                            Image(systemName: "trash")
                            Text("Clear All Local Downloads")
                        }
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Theme.danger)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 10)
                        .background(RoundedRectangle(cornerRadius: 10, style: .continuous).fill(Theme.danger.opacity(0.1)))
                    }
                }
            }
            .padding(16)
            .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                    .strokeBorder(Theme.hairline, lineWidth: 1)
            )
        }
    }

    // MARK: - App Tour & Supported Platforms

    private var appTourSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            sectionHeader("APP EXPERIENCE", icon: "sparkles")

            Button {
                showingOnboarding = true
            } label: {
                HStack(spacing: 14) {
                    ZStack {
                        Circle()
                            .fill(Theme.accent.opacity(0.12))
                            .frame(width: 42, height: 42)
                        Image(systemName: "sparkles")
                            .font(.system(size: 18))
                            .foregroundStyle(Theme.accent)
                    }

                    VStack(alignment: .leading, spacing: 2) {
                        Text("Intro Tour & Supported Platforms")
                            .font(.system(size: 15, weight: .semibold))
                            .foregroundStyle(Theme.ink)
                        Text("View platform logos, save destinations & workflows")
                            .font(.system(size: 12))
                            .foregroundStyle(Theme.inkSecondary)
                    }

                    Spacer()

                    Image(systemName: "chevron.right")
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(Theme.inkTertiary)
                }
                .padding(16)
                .background(RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous).fill(Theme.surface))
                .overlay(
                    RoundedRectangle(cornerRadius: Theme.cardRadius, style: .continuous)
                        .strokeBorder(Theme.hairline, lineWidth: 1)
                )
            }
            .buttonStyle(.plain)
        }
    }

    // MARK: - About

    private var aboutSection: some View {
        VStack(spacing: 6) {
            Text("clipPlay v1.0")
                .font(.system(size: 13, weight: .semibold, design: .rounded))
                .foregroundStyle(Theme.inkSecondary)
            Text("Universal Media Downloader, Cloud Sync & Player")
                .font(.system(size: 11))
                .foregroundStyle(Theme.inkTertiary)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, 12)
    }

    private func sectionHeader(_ title: String, icon: String) -> some View {
        HStack(spacing: 6) {
            Image(systemName: icon)
                .font(.system(size: 11, weight: .bold))
                .foregroundStyle(Theme.accent)
            Text(title)
                .font(.system(size: 11, weight: .bold, design: .rounded))
                .foregroundStyle(Theme.inkSecondary)
                .tracking(1.0)
        }
        .padding(.leading, 4)
    }
}
