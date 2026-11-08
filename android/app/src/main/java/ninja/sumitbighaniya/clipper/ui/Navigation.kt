package ninja.sumitbighaniya.clipper.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.CallSplit
import androidx.compose.material.icons.rounded.ArrowCircleDown
import androidx.compose.material.icons.rounded.LibraryMusic
import androidx.compose.material.icons.rounded.Videocam
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import ninja.sumitbighaniya.clipper.ClipperTheme

enum class AppTab(val title: String, val icon: ImageVector) {
    DOWNLOAD("Download", Icons.Rounded.ArrowCircleDown),
    VIDEOS("Videos", Icons.Rounded.Videocam),
    SPLIT("Split", Icons.AutoMirrored.Rounded.CallSplit),
    MUSIC("Music", Icons.Rounded.LibraryMusic)
}

@Composable
fun ClipperBottomBar(
    currentTab: AppTab,
    onSelectTab: (AppTab) -> Unit,
    modifier: Modifier = Modifier
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .background(MaterialTheme.colorScheme.surface)
            .border(width = 1.dp, color = MaterialTheme.colorScheme.outline, shape = RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp))
            .clip(RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp))
            .padding(horizontal = 16.dp, vertical = 8.dp)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceAround,
            verticalAlignment = Alignment.CenterVertically
        ) {
            AppTab.entries.forEach { tab ->
                val isSelected = currentTab == tab
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    modifier = Modifier
                        .clip(RoundedCornerShape(12.dp))
                        .clickable { onSelectTab(tab) }
                        .padding(horizontal = 12.dp, vertical = 6.dp)
                ) {
                    Icon(
                        imageVector = tab.icon,
                        contentDescription = tab.title,
                        tint = if (isSelected) ClipperTheme.Accent else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                        modifier = Modifier.size(24.dp)
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = tab.title,
                        fontSize = 11.sp,
                        fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
                        color = if (isSelected) ClipperTheme.Accent else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                    )
                }
            }
        }
    }
}
