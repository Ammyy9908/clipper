import SwiftUI
import MediaPlayer

/// Wraps the system volume slider — iOS doesn't give an app its own
/// independent volume separate from the device's, so exposing this (same as
/// the hardware buttons and Control Center control) is the standard way a
/// now-playing screen offers volume control, exactly what Apple Music and
/// Spotify show here too.
private struct SystemVolumeView: UIViewRepresentable {
    var tint: UIColor?
    var onChange: () -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(onChange: onChange)
    }

    func makeUIView(context: Context) -> MPVolumeView {
        let view = MPVolumeView(frame: .zero)
        view.showsRouteButton = false
        style(view)
        if let slider = view.subviews.compactMap({ $0 as? UISlider }).first {
            slider.addTarget(context.coordinator, action: #selector(Coordinator.valueChanged), for: .valueChanged)
        }
        return view
    }

    func updateUIView(_ uiView: MPVolumeView, context: Context) {
        style(uiView)
    }

    private func style(_ view: MPVolumeView) {
        guard let slider = view.subviews.compactMap({ $0 as? UISlider }).first else { return }
        let t = tint ?? UIColor(red: 1.0, green: 0.29, blue: 0.22, alpha: 1)
        slider.minimumTrackTintColor = t
        slider.thumbTintColor = t
    }

    final class Coordinator: NSObject {
        let onChange: () -> Void
        init(onChange: @escaping () -> Void) { self.onChange = onChange }
        @objc func valueChanged() { onChange() }
    }
}

/// Collapsed to a single speaker button by default — tapping it reveals the
/// system volume slider, which auto-hides a couple seconds after the last
/// change instead of permanently taking up space on the now-playing screen.
struct VolumeControl: View {
    var tint: UIColor? = nil
    @State private var expanded = false
    @State private var hideTask: Task<Void, Never>?

    var body: some View {
        HStack(spacing: 10) {
            Button {
                withAnimation(.easeInOut(duration: 0.2)) { expanded.toggle() }
                if expanded { scheduleHide() } else { hideTask?.cancel() }
            } label: {
                Image(systemName: "speaker.wave.2.fill")
                    .font(.system(size: 15))
                    .foregroundStyle(.white.opacity(0.85))
                    .frame(width: 32, height: 32)
            }

            if expanded {
                SystemVolumeView(tint: tint, onChange: scheduleHide)
                    .frame(height: 24)
                    .transition(.opacity.combined(with: .move(edge: .leading)))
            }
        }
    }

    private func scheduleHide() {
        hideTask?.cancel()
        hideTask = Task {
            try? await Task.sleep(nanoseconds: 2_000_000_000)
            guard !Task.isCancelled else { return }
            withAnimation(.easeInOut(duration: 0.2)) { expanded = false }
        }
    }
}
