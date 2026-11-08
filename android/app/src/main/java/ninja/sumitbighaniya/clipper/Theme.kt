package ninja.sumitbighaniya.clipper

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

object ClipperTheme {
    // Brand Accent Color: Coral-Red
    val Accent = Color(0xFFFF4A38)

    // Backgrounds
    val LightBackground = Color(0xFFF9F6F2) // Warm off-white
    val DarkBackground = Color(0xFF121110)  // Warm near-black

    // Surfaces (Cards)
    val LightSurface = Color(0xFFFFFFFF)
    val DarkSurface = Color(0xFF21201E)     // Warm charcoal card

    // Typography Colors
    val LightInk = Color(0xFF1C1A17)        // Near-black
    val DarkInk = Color(0xFFF2F0ED)         // Near-white

    val LightInkSecondary = Color(0xFF857D73) // Warm gray
    val DarkInkSecondary = Color(0xFFADA69C)

    val LightInkTertiary = Color(0xFFB8B0A6)
    val DarkInkTertiary = Color(0xFF757069)

    // Hairlines & Borders
    val LightHairline = Color(0x14000000)   // 8% black
    val DarkHairline = Color(0x1AFFFFFF)    // 10% white

    // Danger / Errors
    val Danger = Color(0xFFE53935)
    val LightDanger = Color(0xFFD13329)
    val DarkDanger = Color(0xFFF0544A)

    // Shapes
    val CardRadius = 20.dp
    val ControlRadius = 14.dp

    val CardShape = RoundedCornerShape(CardRadius)
    val ControlShape = RoundedCornerShape(ControlRadius)
}

@Composable
fun ClipperAppTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colorScheme = if (darkTheme) {
        darkColorScheme(
            primary = ClipperTheme.Accent,
            onPrimary = Color.White,
            primaryContainer = ClipperTheme.Accent.copy(alpha = 0.2f),
            onPrimaryContainer = ClipperTheme.Accent,
            background = ClipperTheme.DarkBackground,
            onBackground = ClipperTheme.DarkInk,
            surface = ClipperTheme.DarkSurface,
            onSurface = ClipperTheme.DarkInk,
            surfaceVariant = ClipperTheme.DarkSurface,
            onSurfaceVariant = ClipperTheme.DarkInkSecondary,
            outline = ClipperTheme.DarkHairline,
            error = ClipperTheme.DarkDanger,
            onError = Color.White,
        )
    } else {
        lightColorScheme(
            primary = ClipperTheme.Accent,
            onPrimary = Color.White,
            primaryContainer = ClipperTheme.Accent.copy(alpha = 0.15f),
            onPrimaryContainer = ClipperTheme.Accent,
            background = ClipperTheme.LightBackground,
            onBackground = ClipperTheme.LightInk,
            surface = ClipperTheme.LightSurface,
            onSurface = ClipperTheme.LightInk,
            surfaceVariant = ClipperTheme.LightSurface,
            onSurfaceVariant = ClipperTheme.LightInkSecondary,
            outline = ClipperTheme.LightHairline,
            error = ClipperTheme.LightDanger,
            onError = Color.White,
        )
    }

    MaterialTheme(
        colorScheme = colorScheme,
        shapes = Shapes(
            small = RoundedCornerShape(ClipperTheme.ControlRadius),
            medium = RoundedCornerShape(ClipperTheme.CardRadius),
            large = RoundedCornerShape(ClipperTheme.CardRadius)
        ),
        content = content
    )
}
