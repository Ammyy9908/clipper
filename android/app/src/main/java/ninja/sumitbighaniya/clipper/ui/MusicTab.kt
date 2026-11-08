package ninja.sumitbighaniya.clipper.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.PlaylistAdd
import androidx.compose.material.icons.automirrored.rounded.QueueMusic
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

@Composable
fun MusicTab(
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current
    val libraryStore = remember { LibraryStore.getInstance(context) }
    val playlistStore = remember { PlaylistStore.getInstance(context) }
    val playerModel = remember { PlayerModel.getInstance(context) }

    val clips by libraryStore.clips.collectAsState()
    val playlists by playlistStore.playlists.collectAsState()

    val songs = remember(clips) { clips.filter { it.isAudio } }
    val recentSongs = remember(songs) { songs.take(6) }

    var showingNewPlaylistDialog by remember { mutableStateOf(false) }
    var newPlaylistName by remember { mutableStateOf("") }
    var selectedPlaylist by remember { mutableStateOf<Playlist?>(null) }
    var addingClipToPlaylist by remember { mutableStateOf<SavedClip?>(null) }

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
                .padding(bottom = 12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column {
                Text(
                    text = "Your Audio",
                    fontSize = 30.sp,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onBackground,
                    letterSpacing = (-0.5).sp
                )
                Text(
                    text = if (songs.isEmpty()) "Nothing saved yet" else "${songs.size} audio track${if (songs.size == 1) "" else "s"}",
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }

            Surface(
                onClick = { showingNewPlaylistDialog = true },
                shape = CircleShape,
                color = ClipperTheme.Accent.copy(alpha = 0.12f),
                border = BorderStroke(1.dp, ClipperTheme.Accent.copy(alpha = 0.3f))
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 7.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    Icon(
                        imageVector = Icons.Rounded.Add,
                        contentDescription = "New Playlist",
                        tint = ClipperTheme.Accent,
                        modifier = Modifier.size(18.dp)
                    )
                    Text(
                        text = "Playlist",
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = ClipperTheme.Accent
                    )
                }
            }
        }

        if (songs.isEmpty() && playlists.isEmpty()) {
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
                            imageVector = Icons.Rounded.LibraryMusic,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(32.dp)
                        )
                    }
                    Spacer(modifier = Modifier.height(14.dp))
                    Text(
                        text = "No audio yet",
                        fontSize = 17.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "Save links as MP3 to build your audio collection",
                        fontSize = 13.sp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        } else {
            LazyColumn(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(20.dp),
                contentPadding = PaddingValues(bottom = 100.dp)
            ) {
                // Playlists shelf
                if (playlists.isNotEmpty()) {
                    item {
                        Column {
                            Text(
                                text = "Playlists",
                                fontSize = 18.sp,
                                fontWeight = FontWeight.Bold,
                                color = MaterialTheme.colorScheme.onBackground
                            )
                            Spacer(modifier = Modifier.height(10.dp))
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .horizontalScroll(rememberScrollState()),
                                horizontalArrangement = Arrangement.spacedBy(12.dp)
                            ) {
                                playlists.forEach { pl ->
                                    val count = playlistStore.getClips(pl, clips).size
                                    Surface(
                                        onClick = { selectedPlaylist = pl },
                                        shape = ClipperTheme.CardShape,
                                        color = MaterialTheme.colorScheme.surface,
                                        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
                                        modifier = Modifier.width(130.dp)
                                    ) {
                                        Column(
                                            modifier = Modifier.padding(12.dp),
                                            horizontalAlignment = Alignment.Start
                                        ) {
                                            Box(
                                                modifier = Modifier
                                                    .size(44.dp)
                                                    .clip(RoundedCornerShape(10.dp))
                                                    .background(ClipperTheme.Accent.copy(alpha = 0.12f)),
                                                contentAlignment = Alignment.Center
                                            ) {
                                                Icon(
                                                    imageVector = Icons.AutoMirrored.Rounded.QueueMusic,
                                                    contentDescription = null,
                                                    tint = ClipperTheme.Accent,
                                                    modifier = Modifier.size(24.dp)
                                                )
                                            }
                                            Spacer(modifier = Modifier.height(10.dp))
                                            Text(
                                                text = pl.name,
                                                fontSize = 13.sp,
                                                fontWeight = FontWeight.SemiBold,
                                                color = MaterialTheme.colorScheme.onSurface,
                                                maxLines = 1,
                                                overflow = TextOverflow.Ellipsis
                                            )
                                            Text(
                                                text = "$count tracks",
                                                fontSize = 11.sp,
                                                color = MaterialTheme.colorScheme.onSurfaceVariant
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                // All Audio header & list
                item {
                    Text(
                        text = "All Audio",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onBackground
                    )
                }

                itemsIndexed(songs, key = { _, item -> item.id }) { index, song ->
                    val isCurrent = playerModel.currentClip?.id == song.id
                    SongItemRow(
                        clip = song,
                        artworkFile = libraryStore.getArtworkFile(song),
                        isPlaying = isCurrent && playerModel.isPlaying.collectAsState().value,
                        onPlay = {
                            playerModel.play(songs, index, PlaybackDisplayMode.AUDIO)
                            playerModel.setExpanded(true)
                        },
                        onAddToPlaylist = { addingClipToPlaylist = song },
                        onDelete = {
                            if (playerModel.currentClip?.id == song.id) {
                                playerModel.stop()
                            }
                            libraryStore.remove(song)
                        }
                    )
                }
            }
        }
    }

    // New Playlist Dialog
    if (showingNewPlaylistDialog) {
        AlertDialog(
            onDismissRequest = {
                showingNewPlaylistDialog = false
                newPlaylistName = ""
            },
            title = { Text("New Playlist", fontWeight = FontWeight.Bold) },
            text = {
                OutlinedTextField(
                    value = newPlaylistName,
                    onValueChange = { newPlaylistName = it },
                    placeholder = { Text("Playlist name") },
                    singleLine = true,
                    shape = ClipperTheme.ControlShape,
                    modifier = Modifier.fillMaxWidth()
                )
            },
            confirmButton = {
                Button(
                    onClick = {
                        val trimmed = newPlaylistName.trim()
                        if (trimmed.isNotEmpty()) {
                            playlistStore.create(trimmed)
                        }
                        showingNewPlaylistDialog = false
                        newPlaylistName = ""
                    },
                    shape = ClipperTheme.ControlShape,
                    colors = ButtonDefaults.buttonColors(containerColor = ClipperTheme.Accent)
                ) {
                    Text("Create")
                }
            },
            dismissButton = {
                TextButton(onClick = {
                    showingNewPlaylistDialog = false
                    newPlaylistName = ""
                }) {
                    Text("Cancel")
                }
            }
        )
    }

    // Playlist detail sheet
    selectedPlaylist?.let { pl ->
        PlaylistDetailDialog(
            playlist = pl,
            playlistStore = playlistStore,
            libraryStore = libraryStore,
            playerModel = playerModel,
            onDismiss = { selectedPlaylist = null }
        )
    }

    // Add to Playlist picker
    addingClipToPlaylist?.let { clip ->
        AddToPlaylistDialog(
            clip = clip,
            playlists = playlists,
            onSelect = { pl ->
                playlistStore.addClip(pl.id, clip.id)
                addingClipToPlaylist = null
            },
            onCreateAndAdd = { name ->
                val newPl = playlistStore.create(name)
                playlistStore.addClip(newPl.id, clip.id)
                addingClipToPlaylist = null
            },
            onDismiss = { addingClipToPlaylist = null }
        )
    }
}

@Composable
private fun SongItemRow(
    clip: SavedClip,
    artworkFile: File?,
    isPlaying: Boolean,
    onPlay: () -> Unit,
    onAddToPlaylist: () -> Unit,
    onDelete: () -> Unit
) {
    var menuExpanded by remember { mutableStateOf(false) }

    Surface(
        shape = ClipperTheme.CardShape,
        color = MaterialTheme.colorScheme.surface,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onPlay() }
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(14.dp)
        ) {
            // Album Artwork
            Box(
                modifier = Modifier
                    .size(52.dp)
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
                        imageVector = Icons.Rounded.MusicNote,
                        contentDescription = null,
                        tint = ClipperTheme.Accent,
                        modifier = Modifier.size(24.dp)
                    )
                }

                if (isPlaying) {
                    Box(
                        modifier = Modifier
                            .fillMaxSize()
                            .background(Color.Black.copy(alpha = 0.4f)),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = Icons.Rounded.GraphicEq,
                            contentDescription = null,
                            tint = Color.White,
                            modifier = Modifier.size(20.dp)
                        )
                    }
                }
            }

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = clip.title,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = if (isPlaying) ClipperTheme.Accent else MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Spacer(modifier = Modifier.height(2.dp))
                Text(
                    text = "Audio track",
                    fontSize = 12.sp,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }

            Box {
                IconButton(onClick = { menuExpanded = true }) {
                    Icon(
                        imageVector = Icons.Rounded.MoreVert,
                        contentDescription = "Options",
                        tint = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                DropdownMenu(
                    expanded = menuExpanded,
                    onDismissRequest = { menuExpanded = false }
                ) {
                    DropdownMenuItem(
                        text = { Text("Play Audio") },
                        leadingIcon = { Icon(Icons.Rounded.PlayArrow, contentDescription = null) },
                        onClick = {
                            menuExpanded = false
                            onPlay()
                        }
                    )
                    DropdownMenuItem(
                        text = { Text("Add to Playlist") },
                        leadingIcon = { Icon(Icons.AutoMirrored.Rounded.PlaylistAdd, contentDescription = null) },
                        onClick = {
                            menuExpanded = false
                            onAddToPlaylist()
                        }
                    )
                    DropdownMenuItem(
                        text = { Text("Delete", color = ClipperTheme.Danger) },
                        leadingIcon = { Icon(Icons.Rounded.Delete, contentDescription = null, tint = ClipperTheme.Danger) },
                        onClick = {
                            menuExpanded = false
                            onDelete()
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun PlaylistDetailDialog(
    playlist: Playlist,
    playlistStore: PlaylistStore,
    libraryStore: LibraryStore,
    playerModel: PlayerModel,
    onDismiss: () -> Unit
) {
    val clips by libraryStore.clips.collectAsState()
    val songs = remember(clips, playlist) { playlistStore.getClips(playlist, clips) }

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
                            text = playlist.name,
                            fontSize = 20.sp,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurface,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Text(
                            text = "${songs.size} tracks",
                            fontSize = 12.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(onClick = onDismiss) {
                        Icon(Icons.Rounded.Close, contentDescription = "Close")
                    }
                }

                Spacer(modifier = Modifier.height(14.dp))

                if (songs.isNotEmpty()) {
                    Button(
                        onClick = {
                            playerModel.play(songs, 0, PlaybackDisplayMode.AUDIO)
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
                        Text("Play All", fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    }
                    Spacer(modifier = Modifier.height(14.dp))
                }

                if (songs.isEmpty()) {
                    Box(modifier = Modifier.weight(1f), contentAlignment = Alignment.Center) {
                        Text(
                            text = "No audio in this playlist yet",
                            fontSize = 14.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                } else {
                    LazyColumn(
                        modifier = Modifier.weight(1f),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        itemsIndexed(songs, key = { _, item -> item.id }) { idx, song ->
                            Surface(
                                shape = RoundedCornerShape(12.dp),
                                color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.4f),
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable {
                                        playerModel.play(songs, idx, PlaybackDisplayMode.AUDIO)
                                        playerModel.setExpanded(true)
                                        onDismiss()
                                    }
                            ) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(12.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                                ) {
                                    Text(
                                        text = song.title,
                                        fontSize = 13.sp,
                                        fontWeight = FontWeight.Medium,
                                        color = MaterialTheme.colorScheme.onSurface,
                                        modifier = Modifier.weight(1f),
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                    IconButton(
                                        onClick = { playlistStore.removeClip(playlist.id, song.id) },
                                        modifier = Modifier.size(24.dp)
                                    ) {
                                        Icon(
                                            Icons.Rounded.RemoveCircleOutline,
                                            contentDescription = "Remove",
                                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                            modifier = Modifier.size(18.dp)
                                        )
                                    }
                                }
                            }
                        }
                    }
                }

                Spacer(modifier = Modifier.height(10.dp))

                TextButton(
                    onClick = {
                        playlistStore.delete(playlist.id)
                        onDismiss()
                    },
                    modifier = Modifier.align(Alignment.CenterHorizontally)
                ) {
                    Text("Delete Playlist", color = ClipperTheme.Danger, fontSize = 13.sp)
                }
            }
        }
    }
}

@Composable
private fun AddToPlaylistDialog(
    clip: SavedClip,
    playlists: List<Playlist>,
    onSelect: (Playlist) -> Unit,
    onCreateAndAdd: (String) -> Unit,
    onDismiss: () -> Unit
) {
    var newName by remember { mutableStateOf("") }
    var isCreating by remember { mutableStateOf(false) }

    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false)
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth(0.9f)
                .wrapContentHeight(),
            shape = ClipperTheme.CardShape,
            color = MaterialTheme.colorScheme.surface,
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline)
        ) {
            Column(modifier = Modifier.padding(20.dp)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = "Add to Playlist",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    IconButton(onClick = onDismiss) {
                        Icon(Icons.Rounded.Close, contentDescription = "Close")
                    }
                }

                Spacer(modifier = Modifier.height(12.dp))

                if (isCreating) {
                    OutlinedTextField(
                        value = newName,
                        onValueChange = { newName = it },
                        placeholder = { Text("Playlist name") },
                        singleLine = true,
                        shape = ClipperTheme.ControlShape,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    Button(
                        onClick = {
                            if (newName.isNotBlank()) onCreateAndAdd(newName.trim())
                        },
                        shape = ClipperTheme.ControlShape,
                        colors = ButtonDefaults.buttonColors(containerColor = ClipperTheme.Accent),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Text("Create & Add")
                    }
                } else {
                    LazyColumn(
                        modifier = Modifier.heightIn(max = 240.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        items(playlists) { pl ->
                            Surface(
                                shape = RoundedCornerShape(10.dp),
                                color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable { onSelect(pl) }
                            ) {
                                Text(
                                    text = pl.name,
                                    fontSize = 14.sp,
                                    fontWeight = FontWeight.Medium,
                                    color = MaterialTheme.colorScheme.onSurface,
                                    modifier = Modifier.padding(14.dp)
                                )
                            }
                        }
                    }

                    Spacer(modifier = Modifier.height(12.dp))

                    OutlinedButton(
                        onClick = { isCreating = true },
                        shape = ClipperTheme.ControlShape,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Icon(Icons.Rounded.Add, contentDescription = null, modifier = Modifier.size(18.dp))
                        Spacer(modifier = Modifier.width(6.dp))
                        Text("New Playlist")
                    }
                }
            }
        }
    }
}
