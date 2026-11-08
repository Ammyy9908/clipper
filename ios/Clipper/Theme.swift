import SwiftUI
import UIKit

/// Shared design tokens — clean, editorial, one bold accent on a warm neutral
/// base; no gradients, no heavy shadows. Each token is a dynamic UIColor pair
/// so the whole app follows the system appearance automatically — no
/// per-screen `.preferredColorScheme` override, no asset catalog color sets.
enum Theme {
    static let background = adaptive(
        light: (0.976, 0.965, 0.949),   // warm off-white
        dark: (0.07, 0.067, 0.063)      // warm near-black
    )
    static let surface = adaptive(
        light: (1.0, 1.0, 1.0),
        dark: (0.13, 0.125, 0.118)       // warm charcoal card
    )
    static let ink = adaptive(
        light: (0.11, 0.10, 0.09),      // near-black
        dark: (0.95, 0.94, 0.93)        // near-white
    )
    static let inkSecondary = adaptive(
        light: (0.52, 0.49, 0.45),      // warm gray
        dark: (0.68, 0.65, 0.61)
    )
    static let inkTertiary = adaptive(
        light: (0.72, 0.69, 0.65),
        dark: (0.46, 0.44, 0.41)
    )
    static let accent = Color(red: 1.0, green: 0.29, blue: 0.22)   // coral-red, same in both
    static let hairline = adaptive(
        light: (0, 0, 0), lightOpacity: 0.08,
        dark: (1, 1, 1), darkOpacity: 0.10
    )
    static let danger = adaptive(
        light: (0.82, 0.2, 0.16),
        dark: (0.94, 0.33, 0.29)         // brighter so it still reads on a dark card
    )

    static let cardRadius: CGFloat = 20
    static let controlRadius: CGFloat = 14

    private static func adaptive(light: (Double, Double, Double), dark: (Double, Double, Double)) -> Color {
        Color(uiColor: UIColor { trait in
            let c = trait.userInterfaceStyle == .dark ? dark : light
            return UIColor(red: c.0, green: c.1, blue: c.2, alpha: 1)
        })
    }

    private static func adaptive(
        light: (Double, Double, Double), lightOpacity: Double,
        dark: (Double, Double, Double), darkOpacity: Double
    ) -> Color {
        Color(uiColor: UIColor { trait in
            let isDark = trait.userInterfaceStyle == .dark
            let c = isDark ? dark : light
            let o = isDark ? darkOpacity : lightOpacity
            return UIColor(red: c.0, green: c.1, blue: c.2, alpha: o)
        })
    }
}
