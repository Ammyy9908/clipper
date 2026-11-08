package ninja.sumitbighaniya.clipper

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import ninja.sumitbighaniya.clipper.ui.*

class MainActivity : ComponentActivity() {

    private val downloadViewModel: DownloadViewModel by viewModels()
    private val incomingUrl = mutableStateOf<String?>(null)
    private val selectedTab = mutableStateOf(AppTab.DOWNLOAD)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        handleIncomingIntent(intent)

        setContent {
            ClipperAppTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background
                ) {
                    val currentTab by selectedTab
                    val playerModel = remember { PlayerModel.getInstance(applicationContext) }
                    val isPlayerExpanded by playerModel.isExpanded.collectAsStateWithLifecycle()
                    val currentClip = playerModel.currentClip

                    // Handle incoming shared URL from other apps
                    val sharedUrl by incomingUrl
                    LaunchedEffect(sharedUrl) {
                        sharedUrl?.let { url ->
                            selectedTab.value = AppTab.DOWNLOAD
                            downloadViewModel.resolve(url)
                            incomingUrl.value = null
                        }
                    }

                    Box(modifier = Modifier.fillMaxSize()) {
                        Scaffold(
                            bottomBar = {
                                Column(modifier = Modifier.fillMaxWidth()) {
                                    // Mini Player floats above bottom bar when media is active
                                    if (currentClip != null && !isPlayerExpanded) {
                                        MiniPlayerBar(playerModel = playerModel)
                                    }
                                    ClipperBottomBar(
                                        currentTab = currentTab,
                                        onSelectTab = { selectedTab.value = it }
                                    )
                                }
                            },
                            containerColor = MaterialTheme.colorScheme.background
                        ) { innerPadding ->
                            Box(
                                modifier = Modifier
                                    .fillMaxSize()
                                    .padding(bottom = innerPadding.calculateBottomPadding())
                            ) {
                                when (currentTab) {
                                    AppTab.DOWNLOAD -> DownloadTab(vm = downloadViewModel)
                                    AppTab.VIDEOS -> VideosTab()
                                    AppTab.SPLIT -> SplitTab()
                                    AppTab.MUSIC -> MusicTab()
                                }
                            }
                        }

                        // Fullscreen Now Playing Sheet
                        if (isPlayerExpanded) {
                            NowPlayingSheet(
                                playerModel = playerModel,
                                onDismiss = { playerModel.setExpanded(false) }
                            )
                        }
                    }
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleIncomingIntent(intent)
    }

    private fun handleIncomingIntent(intent: Intent?) {
        if (intent == null) return
        if (intent.action == Intent.ACTION_SEND && intent.type == "text/plain") {
            val text = intent.getStringExtra(Intent.EXTRA_TEXT)
            val extracted = ClipboardHelper.extractUrl(text)
            if (!extracted.isNullOrBlank()) {
                incomingUrl.value = extracted
            }
        }
    }
}
