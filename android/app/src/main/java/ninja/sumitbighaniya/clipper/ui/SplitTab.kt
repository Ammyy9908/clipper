package ninja.sumitbighaniya.clipper.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.CallSplit
import androidx.compose.material.icons.rounded.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import coil.compose.AsyncImage
import ninja.sumitbighaniya.clipper.*
import java.io.File

data class SplitGroupItem(
    val id: String,
    val title: String,
    val parts: List<SavedClip>,
    val createdAt: Long
)

@Composable
fun SplitTab(
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current
    val libraryStore = remember { LibraryStore.getInstance(context) }
    val playerModel = remember { PlayerModel.getInstance(context) }

    val clips by libraryStore.clips.collectAsState()

    val groups = remember(clips) {
        val splitClips = clips.filter { it.groupID != null }
        val grouped = splitClips.groupBy { it.groupID!! }
        grouped.map { (gid, partList) ->
            val sorted = partList.sortedBy { it.partIndex ?: 0 }
            SplitGroupItem(
                id = gid,
                title = sorted.firstOrNull()?.groupTitle ?: sorted.firstOrNull()?.title ?: "Split video",
                parts = sorted,
                createdAt = sorted.minOfOrNull { it.createdAt } ?: System.currentTimeMillis()
            )
        }.sortedByDescending { it.createdAt }
    }

    var selectedGroup by remember { mutableStateOf<SplitGroupItem?>(null) }

    Column(
        modifier = modifier
            .fillMaxSize()
            .statusBarsPadding()
            .padding(horizontal = 20.dp, vertical = 12.dp)
    ) {
        // Header
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(bottom = 16.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column {
                Text(
                    text = "Split Clips",
                    fontSize = 30.sp,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onBackground,
                    letterSpacing = (-0.5).sp
                )
                Text(
                    text = if (groups.isEmpty()) "Nothing here yet" else "${groups.size} split video${if (groups.size == 1) "" else "s"}",
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }

        if (groups.isEmpty()) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f),
                contentAlignment = Alignment.Center
            ) {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) {
                    Box(
                        modifier = Modifier
                            .size(64.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = Icons.AutoMirrored.Rounded.CallSplit,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(32.dp)
                        )
                    }
                    Spacer(modifier = Modifier.height(14.dp))
                    Text(
                        text = "No split videos yet",
                        fontSize = 17.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "Turn on \"Split into clips\" when saving a video to see parts here",
                        fontSize = 13.sp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        } else {
            LazyColumn(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(10.dp),
                contentPadding = PaddingValues(bottom = 100.dp)
            ) {
                items(groups, key = { it.id }) { group ->
                    SplitGroupCard(
                        group = group,
                        artworkFile = group.parts.firstOrNull()?.let { libraryStore.getArtworkFile(it) },
                        onTap = { selectedGroup = group }
                    )
                }
            }
        }
    }

    selectedGroup?.let { group ->
        SplitGroupDetailDialog(
            group = group,
            libraryStore = libraryStore,
            playerModel = playerModel,
            onDismiss = { selectedGroup = null }
        )
    }
}

@Composable
private fun SplitGroupCard(
    group: SplitGroupItem,
    artworkFile: File?,
    onTap: () -> Unit
) {
    Surface(
        shape = ClipperTheme.CardShape,
        color = MaterialTheme.colorScheme.surface,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onTap() }
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(width = 80.dp, height = 56.dp)
                    .clip(RoundedCornerShape(10.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                if (artworkFile != null && artworkFile.exists()) {
                    AsyncImage(
                        model = artworkFile,
                        contentDescription = null,
                        contentScale = ContentScale.Crop,
                        modifier = Modifier.fillMaxSize()
                    )
                } else {
                    Icon(
                        imageVector = Icons.AutoMirrored.Rounded.CallSplit,
                        contentDescription = null,
                        tint = ClipperTheme.Accent,
                        modifier = Modifier.size(26.dp)
                    )
                }
            }

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = group.title,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Spacer(modifier = Modifier.height(3.dp))
                Row(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Surface(
                        shape = CircleShape,
                        color = ClipperTheme.Accent.copy(alpha = 0.12f)
                    ) {
                        Text(
                            text = "${group.parts.size} parts",
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Bold,
                            color = ClipperTheme.Accent,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                        )
                    }
                }
            }

            Icon(
                imageVector = Icons.Rounded.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun SplitGroupDetailDialog(
    group: SplitGroupItem,
    libraryStore: LibraryStore,
    playerModel: PlayerModel,
    onDismiss: () -> Unit
) {
    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false)
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth(0.92f)
                .fillMaxHeight(0.8f),
            shape = ClipperTheme.CardShape,
            color = MaterialTheme.colorScheme.surface,
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline)
        ) {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(20.dp)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = group.title,
                            fontSize = 18.sp,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurface,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Text(
                            text = "${group.parts.size} clips in sequence",
                            fontSize = 12.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(onClick = onDismiss) {
                        Icon(Icons.Rounded.Close, contentDescription = "Close")
                    }
                }

                Spacer(modifier = Modifier.height(16.dp))

                // Play All Button
                Button(
                    onClick = {
                        playerModel.play(group.parts, 0, PlaybackDisplayMode.VIDEO)
                        playerModel.setExpanded(true)
                        onDismiss()
                    },
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(44.dp),
                    shape = ClipperTheme.ControlShape,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = ClipperTheme.Accent,
                        contentColor = Color.White
                    )
                ) {
                    Icon(Icons.Rounded.PlayArrow, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(modifier = Modifier.width(6.dp))
                    Text("Play All Parts", fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                }

                Spacer(modifier = Modifier.height(16.dp))

                LazyColumn(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    itemsIndexed(group.parts, key = { _, item -> item.id }) { index, part ->
                        Surface(
                            shape = RoundedCornerShape(12.dp),
                            color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.4f),
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable {
                                    playerModel.play(group.parts, index, PlaybackDisplayMode.VIDEO)
                                    playerModel.setExpanded(true)
                                    onDismiss()
                                }
                        ) {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(12.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(12.dp)
                            ) {
                                Surface(
                                    shape = CircleShape,
                                    color = ClipperTheme.Accent,
                                    modifier = Modifier.size(28.dp)
                                ) {
                                    Box(contentAlignment = Alignment.Center) {
                                        Text(
                                            text = "${part.partIndex ?: (index + 1)}",
                                            fontSize = 12.sp,
                                            fontWeight = FontWeight.Bold,
                                            color = Color.White
                                        )
                                    }
                                }

                                Text(
                                    text = part.title,
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    modifier = Modifier.weight(1f),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )

                                Icon(
                                    imageVector = Icons.Rounded.PlayArrow,
                                    contentDescription = "Play",
                                    tint = ClipperTheme.Accent,
                                    modifier = Modifier.size(20.dp)
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
