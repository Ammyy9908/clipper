import SwiftUI
import UIKit

/// Extracted dynamic color palette based on album art / video thumbnail
struct ArtworkPalette: Equatable {
    let primaryAccent: Color
    let dominantColor: Color
    let gradientTop: Color
    let gradientMid: Color
    let gradientBottom: Color
    let uiAccent: UIColor

    static let `default` = ArtworkPalette(
        primaryAccent: Theme.accent,
        dominantColor: Color(red: 0.12, green: 0.12, blue: 0.16),
        gradientTop: Color(red: 0.14, green: 0.13, blue: 0.18),
        gradientMid: Color(red: 0.08, green: 0.08, blue: 0.11),
        gradientBottom: Color(red: 0.04, green: 0.04, blue: 0.06),
        uiAccent: UIColor(red: 1.0, green: 0.29, blue: 0.22, alpha: 1.0)
    )
}

enum ArtworkColorExtractor {
    /// Extracts a high quality dynamic palette from a given UIImage asynchronously
    static func extractPalette(from image: UIImage) async -> ArtworkPalette {
        await Task.detached(priority: .userInitiated) {
            return extractPaletteSync(from: image)
        }.value
    }

    private struct ColorBucket {
        var totalR: CGFloat = 0
        var totalG: CGFloat = 0
        var totalB: CGFloat = 0
        var count: Int = 0

        var avgR: CGFloat { count > 0 ? totalR / CGFloat(count) : 0 }
        var avgG: CGFloat { count > 0 ? totalG / CGFloat(count) : 0 }
        var avgB: CGFloat { count > 0 ? totalB / CGFloat(count) : 0 }
    }

    private static func extractPaletteSync(from image: UIImage) -> ArtworkPalette {
        let width = 40
        let height = 40
        let bytesPerPixel = 4
        let bytesPerRow = width * bytesPerPixel
        let bitsPerComponent = 8
        let colorSpace = CGColorSpaceCreateDeviceRGB()
        // Explicit Big-Endian RGBA format: Byte 0 = R, Byte 1 = G, Byte 2 = B, Byte 3 = A
        let bitmapInfo = CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue

        var rawData = [UInt8](repeating: 0, count: width * height * bytesPerPixel)

        let drewSuccessfully = rawData.withUnsafeMutableBytes { ptr -> Bool in
            guard let baseAddress = ptr.baseAddress,
                  let context = CGContext(
                    data: baseAddress,
                    width: width,
                    height: height,
                    bitsPerComponent: bitsPerComponent,
                    bytesPerRow: bytesPerRow,
                    space: colorSpace,
                    bitmapInfo: bitmapInfo
                  ) else {
                return false
            }

            UIGraphicsPushContext(context)
            context.translateBy(x: 0, y: CGFloat(height))
            context.scaleBy(x: 1.0, y: -1.0)
            image.draw(in: CGRect(x: 0, y: 0, width: width, height: height))
            UIGraphicsPopContext()
            return true
        }

        guard drewSuccessfully else { return .default }

        // 16x16x16 RGB color quantization histogram
        var buckets: [Int: ColorBucket] = [:]
        var totalValidPixels = 0

        for y in 0..<height {
            for x in 0..<width {
                let offset = y * bytesPerRow + x * bytesPerPixel
                let r = CGFloat(rawData[offset]) / 255.0
                let g = CGFloat(rawData[offset + 1]) / 255.0
                let b = CGFloat(rawData[offset + 2]) / 255.0
                let a = CGFloat(rawData[offset + 3]) / 255.0

                if a < 0.5 { continue }

                // Quantize to 4 bits per channel (16 bins each = 4096 bins total)
                let rBin = min(Int(r * 15.99), 15)
                let gBin = min(Int(g * 15.99), 15)
                let bBin = min(Int(b * 15.99), 15)
                let key = (rBin << 8) | (gBin << 4) | bBin

                var bucket = buckets[key] ?? ColorBucket()
                bucket.totalR += r
                bucket.totalG += g
                bucket.totalB += b
                bucket.count += 1
                buckets[key] = bucket

                totalValidPixels += 1
            }
        }

        if totalValidPixels == 0 { return .default }

        // Find candidate colors by scoring prominence + saturation + brightness
        var bestAccentR: CGFloat = 0
        var bestAccentG: CGFloat = 0
        var bestAccentB: CGFloat = 0
        var bestScore: CGFloat = -1

        var mostFrequentBucket: ColorBucket?
        var maxCount = 0

        for (_, bucket) in buckets {
            if bucket.count > maxCount {
                maxCount = bucket.count
                mostFrequentBucket = bucket
            }

            let r = bucket.avgR
            let g = bucket.avgG
            let b = bucket.avgB

            let maxC = max(r, max(g, b))
            let minC = min(r, min(g, b))
            let delta = maxC - minC
            let sat = maxC == 0 ? 0 : delta / maxC
            let bright = maxC

            // Ignore extreme pure blacks or pure blown-out whites for accent
            if bright < 0.15 || (bright > 0.95 && sat < 0.08) {
                continue
            }

            let frequency = CGFloat(bucket.count) / CGFloat(totalValidPixels)
            // Frequency weighting ensures dominant colors like yellow or pink win decisively over tiny details
            let frequencyWeight = pow(frequency, 0.45)
            let saturationBonus = 1.0 + pow(sat, 1.2) * 2.4
            let brightnessBonus = bright > 0.35 ? 1.0 : (bright / 0.35)

            let score = frequencyWeight * saturationBonus * brightnessBonus

            if score > bestScore {
                bestScore = score
                bestAccentR = r
                bestAccentG = g
                bestAccentB = b
            }
        }

        let domR = mostFrequentBucket?.avgR ?? bestAccentR
        let domG = mostFrequentBucket?.avgG ?? bestAccentG
        let domB = mostFrequentBucket?.avgB ?? bestAccentB

        // Check if monochrome
        let isMonochrome = (bestScore <= 0) || (max(domR, max(domG, domB)) - min(domR, min(domG, domB)) < 0.08 && max(bestAccentR, max(bestAccentG, bestAccentB)) - min(bestAccentR, min(bestAccentG, bestAccentB)) < 0.08)

        let primaryAccentUI: UIColor
        let primaryAccentColor: Color

        if isMonochrome {
            primaryAccentColor = Theme.accent
            primaryAccentUI = UIColor(red: 1.0, green: 0.29, blue: 0.22, alpha: 1.0)
        } else {
            // Tune the extracted accent to be luminous and high-contrast on dark UI
            var h: CGFloat = 0
            var s: CGFloat = 0
            var v: CGFloat = 0
            var alpha: CGFloat = 0
            let rawUI = UIColor(red: bestAccentR, green: bestAccentG, blue: bestAccentB, alpha: 1.0)
            rawUI.getHue(&h, saturation: &s, brightness: &v, alpha: &alpha)

            let finalV = max(v, 0.85) // High brightness for dark UI readability
            let finalS = min(max(s, 0.65), 0.95) // Vivid saturation
            primaryAccentUI = UIColor(hue: h, saturation: finalS, brightness: finalV, alpha: 1.0)
            primaryAccentColor = Color(primaryAccentUI)
        }

        let dominantColor = Color(red: domR, green: domG, blue: domB)

        // Backdrop gradients derived from the dominant/accent mood color
        let targetToneR = isMonochrome ? 0.12 : (bestAccentR * 0.7 + domR * 0.3)
        let targetToneG = isMonochrome ? 0.12 : (bestAccentG * 0.7 + domG * 0.3)
        let targetToneB = isMonochrome ? 0.16 : (bestAccentB * 0.7 + domB * 0.3)

        let gradientTop = Color(red: min(targetToneR * 0.9, 1.0), green: min(targetToneG * 0.9, 1.0), blue: min(targetToneB * 0.9, 1.0))
        let gradientMid = Color(red: targetToneR * 0.35, green: targetToneG * 0.35, blue: targetToneB * 0.35)
        let gradientBottom = Color(red: 0.04, green: 0.04, blue: 0.06)

        return ArtworkPalette(
            primaryAccent: primaryAccentColor,
            dominantColor: dominantColor,
            gradientTop: gradientTop,
            gradientMid: gradientMid,
            gradientBottom: gradientBottom,
            uiAccent: primaryAccentUI
        )
    }
}
