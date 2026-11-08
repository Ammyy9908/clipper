package ninja.sumitbighaniya.clipper.ui

import android.view.ViewGroup
import android.widget.FrameLayout
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import coil.compose.AsyncImage
import ninja.sumitbighaniya.clipper.*
import java.io.File

@OptIn(UnstableApi::class)
@Composable
fun NowPlayingSheet(
    playerModel: PlayerModel,
    onDismiss: () -> Unit
) {
    val context = LocalContext.current
    val libraryStore = remember { LibraryStore.getInstance(context) }
    val currentClip = playerModel.currentClip ?: run {
        onDismiss()
        return
    }

    val isPlaying by playerModel.isPlaying.collectAsState()
    val currentTimeMs by playerModel.currentTimeMs.collectAsState()
    val durationMs by playerModel.durationMs.collectAsState()
    val isShuffling by playerModel.isShuffling.collectAsState()
    val repeatMode by playerModel.repeatMode.collectAsState()
    val displayMode by playerModel.displayMode.collectAsState()

    val artworkFile = remember(currentClip) { libraryStore.getArtworkFile(currentClip) }
    var sliderValue by remember { mutableStateOf<Float?>(null) }

    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false, decorFitsSystemWindows = false)
    ) {
        Surface(
            modifier = Modifier.fillMaxSize(),
            color = MaterialTheme.colorScheme.background
        ) {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .statusBarsPadding()
                    .navigationBarsPadding()
                    .padding(horizontal = 24.dp, vertical = 16.dp),
                horizontalAlignment = Alignment.CenterHorizontally
            ) {
                // Top bar
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(bottom = 12.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    IconButton(onClick = onDismiss) {
                        Icon(
                            imageVector = Icons.Rounded.KeyboardArrowDown,
                            contentDescription = "Collapse",
                            tint = MaterialTheme.colorScheme.onBackground,
                            modifier = Modifier.size(30.dp)
                        )
                    }

                    // Song / Video switcher pill (Only show if video clip)
                    if (!currentClip.isAudio) {
                        Surface(
                            shape = CircleShape,
                            color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
                            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.5f))
                        ) {
                            Row(modifier = Modifier.padding(3.dp)) {
                                Surface(
                                    onClick = { playerModel.setDisplayMode(PlaybackDisplayMode.AUDIO) },
                                    shape = CircleShape,
                                    color = if (displayMode == PlaybackDisplayMode.AUDIO) ClipperTheme.Accent else Color.Transparent
                                ) {
                                    Text(
                                        text = "Audio",
                                        fontSize = 12.sp,
                                        fontWeight = FontWeight.Bold,
                                        color = if (displayMode == PlaybackDisplayMode.AUDIO) Color.White else MaterialTheme.colorScheme.onSurface,
                                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 6.dp)
                                    )
                                }
                                Surface(
                                    onClick = { playerModel.setDisplayMode(PlaybackDisplayMode.VIDEO) },
                                    shape = CircleShape,
                                    color = if (displayMode == PlaybackDisplayMode.VIDEO) ClipperTheme.Accent else Color.Transparent
                                ) {
                                    Text(
                                        text = "Video",
                                        fontSize = 12.sp,
                                        fontWeight = FontWeight.Bold,
                                        color = if (displayMode == PlaybackDisplayMode.VIDEO) Color.White else MaterialTheme.colorScheme.onSurface,
                                        modifier = Modifier.padding(horizontal = 14.dp, vertical = 6.dp)
                                    )
                                }
                            }
                        }
                    } else {
                        Text(
                            text = "Now Playing",
                            fontSize = 15.sp,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onBackground
                        )
                    }

                    IconButton(onClick = { playerModel.stop(); onDismiss() }) {
                        Icon(
                            imageVector = Icons.Rounded.Close,
                            contentDescription = "Close",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }

                Spacer(modifier = Modifier.height(12.dp))

                // Canvas area (Video Player or Audio Artwork) with swipe-to-switch
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .weight(1f)
                        .clip(RoundedCornerShape(20.dp))
                        .background(Color.Black)
                        .pointerInput(currentClip.id) {
                            var dragAmount = 0f
                            detectDragGestures(
                                onDragStart = { dragAmount = 0f },
                                onDragEnd = {
                                    if (!currentClip.isAudio) {
                                        if (dragAmount < -60f) {
                                            playerModel.setDisplayMode(PlaybackDisplayMode.VIDEO)
                                        } else if (dragAmount > 60f) {
                                            playerModel.setDisplayMode(PlaybackDisplayMode.AUDIO)
                                        }
                                    }
                                },
                                onDrag = { change, drag ->
                                    change.consume()
                                    dragAmount += drag.x
                                }
                            )
                        },
                    contentAlignment = Alignment.Center
                ) {
                    if (displayMode == PlaybackDisplayMode.VIDEO && !currentClip.isAudio) {
                        AndroidView(
                            factory = { ctx ->
                                PlayerView(ctx).apply {
                                    player = playerModel.exoPlayer
                                    useController = false
                                    resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
                                    layoutParams = FrameLayout.LayoutParams(
                                        ViewGroup.LayoutParams.MATCH_PARENT,
                                        ViewGroup.LayoutParams.MATCH_PARENT
                                    )
                                }
                            },
                            update = { playerView ->
                                if (playerView.player != playerModel.exoPlayer) {
                                    playerView.player = playerModel.exoPlayer
                                }
                            },
                            modifier = Modifier.fillMaxSize()
                        )
                    } else {
                        if (artworkFile != null && artworkFile.exists()) {
                            AsyncImage(
                                model = artworkFile,
                                contentDescription = null,
                                contentScale = ContentScale.Crop,
                                modifier = Modifier.fillMaxSize()
                            )
                        } else {
                            Box(
                                modifier = Modifier
                                    .fillMaxSize()
                                    .background(MaterialTheme.colorScheme.surfaceVariant),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = Icons.Rounded.MusicNote,
                                    contentDescription = null,
                                    tint = ClipperTheme.Accent,
                                    modifier = Modifier.size(80.dp)
                                )
                            }
                        }
                    }
                }

                Spacer(modifier = Modifier.height(24.dp))

                // Metadata
                Column(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalAlignment = Alignment.Start
                ) {
                    Text(
                        text = currentClip.title,
                        fontSize = 20.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onBackground,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = if (currentClip.isAudio) "Audio file" else "Video file",
                        fontSize = 14.sp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }

                Spacer(modifier = Modifier.height(16.dp))

                // Scrubber Slider
                val currentPos = sliderValue ?: (currentTimeMs.toFloat())
                val maxDuration = durationMs.toFloat().coerceAtLeast(1f)

                Slider(
                    value = (currentPos / maxDuration).coerceIn(0f, 1f),
                    onValueChange = { frac ->
                        playerModel.isScrubbing = true
                        sliderValue = frac * maxDuration
                    },
                    onValueChangeFinished = {
                        sliderValue?.let { targetMs ->
                            playerModel.seekTo(targetMs.toLong())
                        }
                        sliderValue = null
                        playerModel.isScrubbing = false
                    },
                    colors = SliderDefaults.colors(
                        thumbColor = ClipperTheme.Accent,
                        activeTrackColor = ClipperTheme.Accent,
                        inactiveTrackColor = MaterialTheme.colorScheme.outline
                    ),
                    modifier = Modifier.fillMaxWidth()
                )

                // Time labels
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        text = formatTimeMs(sliderValue?.toLong() ?: currentTimeMs),
                        fontSize = 12.sp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = formatTimeMs(durationMs),
                        fontSize = 12.sp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }

                Spacer(modifier = Modifier.height(20.dp))

                // Playback Controls
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceEvenly,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    // Shuffle
                    IconButton(onClick = { playerModel.toggleShuffle() }) {
                        Icon(
                            imageVector = Icons.Rounded.Shuffle,
                            contentDescription = "Shuffle",
                            tint = if (isShuffling) ClipperTheme.Accent else MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }

                    // Previous
                    IconButton(
                        onClick = { playerModel.previous() },
                        enabled = playerModel.hasPrevious
                    ) {
                        Icon(
                            imageVector = Icons.Rounded.SkipPrevious,
                            contentDescription = "Previous",
                            tint = if (playerModel.hasPrevious) MaterialTheme.colorScheme.onBackground else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.3f),
                            modifier = Modifier.size(36.dp)
                        )
                    }

                    // Play / Pause (Big Coral Button)
                    Surface(
                        onClick = { playerModel.togglePlayPause() },
                        shape = CircleShape,
                        color = ClipperTheme.Accent,
                        modifier = Modifier.size(64.dp),
                        shadowElevation = 4.dp
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                imageVector = if (isPlaying) Icons.Rounded.Pause else Icons.Rounded.PlayArrow,
                                contentDescription = if (isPlaying) "Pause" else "Play",
                                tint = Color.White,
                                modifier = Modifier.size(36.dp)
                            )
                        }
                    }

                    // Next
                    IconButton(
                        onClick = { playerModel.next() },
                        enabled = playerModel.hasNext
                    ) {
                        Icon(
                            imageVector = Icons.Rounded.SkipNext,
                            contentDescription = "Next",
                            tint = if (playerModel.hasNext) MaterialTheme.colorScheme.onBackground else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.3f),
                            modifier = Modifier.size(36.dp)
                        )
                    }

                    // Repeat
                    IconButton(onClick = { playerModel.toggleRepeat() }) {
                        val (icon, tint) = when (repeatMode) {
                            RepeatMode.OFF -> Icons.Rounded.Repeat to MaterialTheme.colorScheme.onSurfaceVariant
                            RepeatMode.ALL -> Icons.Rounded.Repeat to ClipperTheme.Accent
                            RepeatMode.ONE -> Icons.Rounded.RepeatOne to ClipperTheme.Accent
                        }
                        Icon(imageVector = icon, contentDescription = "Repeat", tint = tint)
                    }
                }

                Spacer(modifier = Modifier.height(16.dp))
            }
        }
    }
}

private fun formatTimeMs(ms: Long): String {
    val totalSeconds = (ms / 1000).coerceAtLeast(0)
    val minutes = totalSeconds / 60
    val seconds = totalSeconds % 60
    return String.format("%d:%02d", minutes, seconds)
}
