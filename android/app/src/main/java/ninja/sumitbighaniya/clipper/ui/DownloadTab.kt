package ninja.sumitbighaniya.clipper.ui

import android.content.Context
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import coil.compose.AsyncImage
import ninja.sumitbighaniya.clipper.*

@Composable
fun DownloadTab(
    vm: DownloadViewModel,
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current
    val phase by vm.phase.collectAsState()
    val selectedVariant by vm.selected.collectAsState()
    val splitSeconds by vm.splitSeconds.collectAsState()
    val downloadQueue = remember { DownloadQueue.getInstance(context) }
    val activeJobs by downloadQueue.jobs.collectAsState()

    var inputUrl by remember { mutableStateOf("") }
    var showingJobsSheet by remember { mutableStateOf(false) }
    val keyboardController = LocalSoftwareKeyboardController.current

    val activeCount = activeJobs.count { it.status !is JobStatus.Done && it.status !is JobStatus.Failed }

    val subtitle = when (val p = phase) {
        is Phase.Idle -> "Paste a video link from Instagram, TikTok, or YouTube"
        is Phase.Resolving -> "Reading the link…"
        is Phase.Ready -> "Ready to save"
        is Phase.Working -> "Preparing your download…"
        is Phase.Saving -> if (p.isAudio) "Saving audio…" else "Saving video…"
        is Phase.Done -> "Saved to library"
        is Phase.Failed -> "Something went wrong"
    }

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
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "clipPlay",
                    fontSize = 32.sp,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onBackground,
                    letterSpacing = (-0.5).sp
                )
                Text(
                    text = subtitle,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }

            if (activeJobs.isNotEmpty()) {
                Surface(
                    onClick = { showingJobsSheet = true },
                    shape = CircleShape,
                    color = MaterialTheme.colorScheme.surface,
                    border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
                    modifier = Modifier.padding(start = 8.dp)
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp)
                    ) {
                        if (activeCount > 0) {
                            CircularProgressIndicator(
                                color = ClipperTheme.Accent,
                                strokeWidth = 2.dp,
                                modifier = Modifier.size(12.dp)
                            )
                            Text(
                                text = "$activeCount",
                                fontSize = 13.sp,
                                fontWeight = FontWeight.Bold,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        } else {
                            Icon(
                                imageVector = Icons.Rounded.Check,
                                contentDescription = null,
                                tint = ClipperTheme.Accent,
                                modifier = Modifier.size(16.dp)
                            )
                            Text(
                                text = "${activeJobs.size}",
                                fontSize = 13.sp,
                                fontWeight = FontWeight.Bold,
                                color = MaterialTheme.colorScheme.onSurface
                            )
                        }
                    }
                }
            }
        }

        Spacer(modifier = Modifier.weight(1f))

        // Center card
        Card(
            modifier = Modifier
                .fillMaxWidth()
                .animateContentSize(),
            shape = ClipperTheme.CardShape,
            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
            elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
        ) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(20.dp),
                contentAlignment = Alignment.Center
            ) {
                when (val p = phase) {
                    is Phase.Idle -> {
                        IdleContent(
                            inputUrl = inputUrl,
                            onUrlChange = { inputUrl = it },
                            onPaste = {
                                val clipUrl = ClipboardHelper.getClipboardUrl(context)
                                if (!clipUrl.isNullOrBlank()) {
                                    inputUrl = clipUrl
                                    keyboardController?.hide()
                                    vm.resolve(clipUrl)
                                }
                            },
                            onSubmit = { url ->
                                val extracted = ClipboardHelper.extractUrl(url) ?: url.trim()
                                if (extracted.isNotBlank()) {
                                    keyboardController?.hide()
                                    vm.resolve(extracted)
                                }
                            }
                        )
                    }
                    is Phase.Resolving -> {
                        ResolvingContent()
                    }
                    is Phase.Ready -> {
                        ReadyContent(
                            info = p.info,
                            selected = selectedVariant,
                            splitSeconds = splitSeconds,
                            onSelectVariant = vm::select,
                            onSelectSplit = vm::setSplitSeconds,
                            onSave = {
                                val variant = selectedVariant ?: p.info.variants.firstOrNull()
                                if (variant != null) {
                                    downloadQueue.enqueue(p.info, variant, splitSeconds)
                                    vm.reset()
                                    inputUrl = ""
                                }
                            },
                            onCancel = {
                                vm.reset()
                                inputUrl = ""
                            }
                        )
                    }
                    is Phase.Working -> {
                        ResolvingContent()
                    }
                    is Phase.Saving -> {
                        ResolvingContent()
                    }
                    is Phase.Done -> {
                        DoneContent(onReset = { vm.reset(); inputUrl = "" })
                    }
                    is Phase.Failed -> {
                        FailedContent(message = p.message, onReset = { vm.reset() })
                    }
                }
            }
        }

        Spacer(modifier = Modifier.weight(1f))
    }

    if (showingJobsSheet) {
        DownloadJobsDialog(
            queue = downloadQueue,
            onDismiss = { showingJobsSheet = false }
        )
    }
}

@Composable
private fun IdleContent(
    inputUrl: String,
    onUrlChange: (String) -> Unit,
    onPaste: () -> Unit,
    onSubmit: (String) -> Unit
) {
    val context = LocalContext.current
    val clipboardUrl = remember { ClipboardHelper.getClipboardUrl(context) }

    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        // Icon badge
        Box(
            modifier = Modifier
                .size(56.dp)
                .clip(CircleShape)
                .background(ClipperTheme.Accent.copy(alpha = 0.12f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = Icons.Rounded.Link,
                contentDescription = null,
                tint = ClipperTheme.Accent,
                modifier = Modifier.size(28.dp)
            )
        }

        Spacer(modifier = Modifier.height(16.dp))

        Text(
            text = "Enter or Paste Link",
            fontSize = 18.sp,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        Spacer(modifier = Modifier.height(6.dp))

        Text(
            text = "Paste any video URL from YouTube, Instagram, TikTok, etc.",
            fontSize = 13.sp,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center
        )

        Spacer(modifier = Modifier.height(20.dp))

        // URL Input with direct Paste & Submit
        OutlinedTextField(
            value = inputUrl,
            onValueChange = onUrlChange,
            placeholder = { Text("https://...", fontSize = 14.sp) },
            singleLine = true,
            shape = ClipperTheme.ControlShape,
            colors = OutlinedTextFieldDefaults.colors(
                focusedBorderColor = ClipperTheme.Accent,
                unfocusedBorderColor = MaterialTheme.colorScheme.outline
            ),
            trailingIcon = {
                if (inputUrl.isNotEmpty()) {
                    IconButton(onClick = { onUrlChange("") }) {
                        Icon(Icons.Rounded.Close, contentDescription = "Clear", modifier = Modifier.size(18.dp))
                    }
                } else {
                    IconButton(onClick = onPaste) {
                        Icon(Icons.Rounded.ContentPaste, contentDescription = "Paste", tint = ClipperTheme.Accent, modifier = Modifier.size(20.dp))
                    }
                }
            },
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Go),
            keyboardActions = KeyboardActions(onGo = { onSubmit(inputUrl) }),
            modifier = Modifier.fillMaxWidth()
        )

        Spacer(modifier = Modifier.height(16.dp))

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            // Instant Paste & Fetch Button
            OutlinedButton(
                onClick = onPaste,
                modifier = Modifier
                    .weight(1f)
                    .height(48.dp),
                shape = ClipperTheme.ControlShape,
                border = BorderStroke(1.dp, ClipperTheme.Accent)
            ) {
                Icon(
                    Icons.Rounded.ContentPaste,
                    contentDescription = null,
                    tint = ClipperTheme.Accent,
                    modifier = Modifier.size(18.dp)
                )
                Spacer(modifier = Modifier.width(6.dp))
                Text(
                    text = "Paste Link",
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = ClipperTheme.Accent
                )
            }

            // Fetch Button
            Button(
                onClick = { onSubmit(inputUrl) },
                enabled = inputUrl.isNotBlank(),
                modifier = Modifier
                    .weight(1f)
                    .height(48.dp),
                shape = ClipperTheme.ControlShape,
                colors = ButtonDefaults.buttonColors(
                    containerColor = ClipperTheme.Accent,
                    contentColor = Color.White
                )
            ) {
                Text(
                    text = "Fetch Video",
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold
                )
            }
        }
    }
}

@Composable
private fun ResolvingContent() {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        CircularProgressIndicator(
            color = ClipperTheme.Accent,
            strokeWidth = 3.dp,
            modifier = Modifier.size(40.dp)
        )
        Spacer(modifier = Modifier.height(16.dp))
        Text(
            text = "Fetching video details…",
            fontSize = 15.sp,
            fontWeight = FontWeight.Medium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun ReadyContent(
    info: ResolvedInfo,
    selected: Variant?,
    splitSeconds: Int?,
    onSelectVariant: (Variant) -> Unit,
    onSelectSplit: (Int?) -> Unit,
    onSave: () -> Unit,
    onCancel: () -> Unit
) {
    val isAudio = selected?.audio == true
    val splitOptions = listOf(15, 30, 60, 120)

    val saveTitle = when {
        isAudio -> "Save as MP3"
        splitSeconds != null -> "Save & Split"
        else -> "Save Video"
    }

    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        // Thumbnail
        if (!info.thumbnail.isNullOrEmpty()) {
            AsyncImage(
                model = info.thumbnail,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(180.dp)
                    .clip(RoundedCornerShape(14.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant)
            )
        } else {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(160.dp)
                    .clip(RoundedCornerShape(14.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Rounded.PlayArrow,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(48.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(14.dp))

        Text(
            text = info.title,
            fontSize = 16.sp,
            fontWeight = FontWeight.SemiBold,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center
        )

        Spacer(modifier = Modifier.height(4.dp))

        Text(
            text = info.uploader,
            fontSize = 13.sp,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Variant selector pills
        if (info.variants.isNotEmpty()) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                info.variants.forEach { v ->
                    val isSelected = selected == v
                    Surface(
                        onClick = { onSelectVariant(v) },
                        shape = CircleShape,
                        color = if (isSelected) ClipperTheme.Accent else MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
                        border = if (isSelected) null else BorderStroke(1.dp, MaterialTheme.colorScheme.outline)
                    ) {
                        Text(
                            text = v.label,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = if (isSelected) Color.White else MaterialTheme.colorScheme.onSurface,
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 7.dp)
                        )
                    }
                }
            }
        }

        // Split options (if video)
        if (!isAudio) {
            Spacer(modifier = Modifier.height(16.dp))
            Column(modifier = Modifier.fillMaxWidth()) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column {
                        Text(
                            text = "Split into clips",
                            fontSize = 14.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Text(
                            text = "Cuts the video into shorter parts",
                            fontSize = 12.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    Switch(
                        checked = splitSeconds != null,
                        onCheckedChange = { onSelectSplit(if (it) 30 else null) },
                        colors = SwitchDefaults.colors(
                            checkedThumbColor = Color.White,
                            checkedTrackColor = ClipperTheme.Accent
                        )
                    )
                }

                if (splitSeconds != null) {
                    Spacer(modifier = Modifier.height(10.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        splitOptions.forEach { opt ->
                            val isChosen = splitSeconds == opt
                            Surface(
                                onClick = { onSelectSplit(opt) },
                                shape = CircleShape,
                                color = if (isChosen) ClipperTheme.Accent else MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f),
                                border = if (isChosen) null else BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
                                modifier = Modifier.weight(1f)
                            ) {
                                Text(
                                    text = "${opt}s",
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = if (isChosen) Color.White else MaterialTheme.colorScheme.onSurface,
                                    textAlign = TextAlign.Center,
                                    modifier = Modifier.padding(vertical = 7.dp)
                                )
                            }
                        }
                    }
                }
            }
        }

        Spacer(modifier = Modifier.height(20.dp))

        Button(
            onClick = onSave,
            modifier = Modifier
                .fillMaxWidth()
                .height(48.dp),
            shape = ClipperTheme.ControlShape,
            colors = ButtonDefaults.buttonColors(
                containerColor = ClipperTheme.Accent,
                contentColor = Color.White
            )
        ) {
            Text(
                text = saveTitle,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold
            )
        }

        Spacer(modifier = Modifier.height(10.dp))

        TextButton(onClick = onCancel) {
            Text(
                text = "Cancel",
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun DoneContent(onReset: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            modifier = Modifier
                .size(56.dp)
                .clip(CircleShape)
                .background(ClipperTheme.Accent.copy(alpha = 0.12f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = Icons.Rounded.Check,
                contentDescription = null,
                tint = ClipperTheme.Accent,
                modifier = Modifier.size(32.dp)
            )
        }

        Spacer(modifier = Modifier.height(16.dp))

        Text(
            text = "Saved to Library",
            fontSize = 18.sp,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        Spacer(modifier = Modifier.height(6.dp))

        Text(
            text = "Your clip is downloaded and ready to play",
            fontSize = 14.sp,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        Spacer(modifier = Modifier.height(20.dp))

        Button(
            onClick = onReset,
            modifier = Modifier
                .fillMaxWidth()
                .height(48.dp),
            shape = ClipperTheme.ControlShape,
            colors = ButtonDefaults.buttonColors(
                containerColor = ClipperTheme.Accent,
                contentColor = Color.White
            )
        ) {
            Text("Done", fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        }
    }
}

@Composable
private fun FailedContent(message: String, onReset: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            modifier = Modifier
                .size(56.dp)
                .clip(CircleShape)
                .background(ClipperTheme.Danger.copy(alpha = 0.12f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = Icons.Rounded.ErrorOutline,
                contentDescription = null,
                tint = ClipperTheme.Danger,
                modifier = Modifier.size(32.dp)
            )
        }

        Spacer(modifier = Modifier.height(16.dp))

        Text(
            text = "Download Failed",
            fontSize = 18.sp,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        Spacer(modifier = Modifier.height(6.dp))

        Text(
            text = message,
            fontSize = 13.sp,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center
        )

        Spacer(modifier = Modifier.height(20.dp))

        Button(
            onClick = onReset,
            modifier = Modifier
                .fillMaxWidth()
                .height(48.dp),
            shape = ClipperTheme.ControlShape,
            colors = ButtonDefaults.buttonColors(
                containerColor = ClipperTheme.Accent,
                contentColor = Color.White
            )
        ) {
            Text("Try again", fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
        }
    }
}

@Composable
private fun DownloadJobsDialog(
    queue: DownloadQueue,
    onDismiss: () -> Unit
) {
    val jobs by queue.jobs.collectAsState()

    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false)
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth(0.92f)
                .fillMaxHeight(0.75f),
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
                    Text(
                        text = "Downloads",
                        fontSize = 20.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    IconButton(onClick = onDismiss) {
                        Icon(Icons.Rounded.Close, contentDescription = "Close")
                    }
                }

                Spacer(modifier = Modifier.height(12.dp))

                if (jobs.isEmpty()) {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        Text(
                            text = "Nothing downloading right now",
                            fontSize = 14.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                } else {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        verticalArrangement = Arrangement.spacedBy(10.dp)
                    ) {
                        items(jobs, key = { it.id }) { job ->
                            JobItemRow(job = job, onRemove = { queue.remove(job.id) })
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun JobItemRow(job: DownloadJob, onRemove: () -> Unit) {
    Surface(
        shape = RoundedCornerShape(14.dp),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.4f),
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.5f)),
        modifier = Modifier.fillMaxWidth()
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            if (!job.thumbnail.isNullOrEmpty()) {
                AsyncImage(
                    model = job.thumbnail,
                    contentDescription = null,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier
                        .size(48.dp)
                        .clip(RoundedCornerShape(10.dp))
                )
            } else {
                Box(
                    modifier = Modifier
                        .size(48.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(MaterialTheme.colorScheme.surfaceVariant),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = if (job.isAudio) Icons.Rounded.LibraryMusic else Icons.Rounded.Videocam,
                        contentDescription = null,
                        tint = ClipperTheme.Accent,
                        modifier = Modifier.size(24.dp)
                    )
                }
            }

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = job.title,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Spacer(modifier = Modifier.height(2.dp))
                val statusText = when (val s = job.status) {
                    is JobStatus.Queued -> "Queued…"
                    is JobStatus.Working -> s.stage
                    is JobStatus.Saving -> "Saving ${(s.progress * 100).toInt()}%"
                    is JobStatus.Done -> "Completed"
                    is JobStatus.Failed -> s.message
                }
                Text(
                    text = statusText,
                    fontSize = 12.sp,
                    color = if (job.status is JobStatus.Failed) ClipperTheme.Danger else MaterialTheme.colorScheme.onSurfaceVariant
                )
            }

            when (job.status) {
                is JobStatus.Done -> {
                    IconButton(onClick = onRemove) {
                        Icon(Icons.Rounded.CheckCircle, contentDescription = null, tint = ClipperTheme.Accent)
                    }
                }
                is JobStatus.Failed -> {
                    IconButton(onClick = onRemove) {
                        Icon(Icons.Rounded.Cancel, contentDescription = null, tint = ClipperTheme.Danger)
                    }
                }
                else -> {
                    CircularProgressIndicator(
                        color = ClipperTheme.Accent,
                        strokeWidth = 2.dp,
                        modifier = Modifier.size(20.dp)
                    )
                }
            }
        }
    }
}
