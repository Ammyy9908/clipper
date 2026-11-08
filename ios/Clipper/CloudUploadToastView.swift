import SwiftUI

struct CloudUploadToastView: View {
    @ObservedObject var cloudManager = CloudStorageManager.shared

    var body: some View {
        if let message = cloudManager.toastMessage {
            HStack(spacing: 10) {
                if cloudManager.isUploading {
                    ProgressView()
                        .controlSize(.small)
                        .tint(.white)
                } else {
                    Image(systemName: cloudManager.toastType.icon)
                        .font(.system(size: 15, weight: .bold))
                        .foregroundStyle(cloudManager.toastType.color)
                }

                Text(message)
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(.white)
                    .lineLimit(2)
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 10)
            .background(
                Capsule()
                    .fill(Color(white: 0.12))
                    .shadow(color: Color.black.opacity(0.4), radius: 12, y: 6)
            )
            .overlay(
                Capsule().strokeBorder(Theme.hairline, lineWidth: 1)
            )
            .padding(.horizontal, 20)
            .padding(.top, 54)
            .transition(.move(edge: .top).combined(with: .opacity))
        }
    }
}
