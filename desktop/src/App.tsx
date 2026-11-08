import React, { useEffect, useState } from 'react';
import { TitleBar } from './components/TitleBar';
import { LinkInput } from './components/LinkInput';
import { VideoCard } from './components/VideoCard';
import { DownloadQueue } from './components/DownloadQueue';
import { DownloadsLibrary } from './components/DownloadsLibrary';
import { OfflineEmptyState } from './components/OfflineEmptyState';
import { ClipboardToast } from './components/ClipboardToast';
import { SettingsModal } from './components/SettingsModal';
import { SetupModal } from './components/SetupModal';
import { UpdateModal } from './components/UpdateModal';
import { useOnlineStatus } from './hooks/useOnlineStatus';
import type { AppSettings, DownloadItem, VideoMetadata, UpdateInfo } from './types';

export const App: React.FC = () => {
  const [settings, setSettings] = useState<AppSettings>({
    downloadDir: '',
    engine: 'local',
    autoClipboard: true,
    maxConcurrentDownloads: 3,
    theme: 'dark',
    notifyOnComplete: true,
    autoCheckUpdates: true,
    cloudApiUrl: 'https://api.clipplay.video',
  });

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [resolvedMeta, setResolvedMeta] = useState<VideoMetadata | null>(null);
  const [downloads, setDownloads] = useState<DownloadItem[]>([]);
  const [clipboardUrl, setClipboardUrl] = useState<string | null>(null);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isSetupModalOpen, setIsSetupModalOpen] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [isUpdateModalOpen, setIsUpdateModalOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<'downloader' | 'library'>('downloader');

  const isOnline = useOnlineStatus();

  // Initialize Settings, History, Dependency Check & IPC Subscriptions
  useEffect(() => {
    if (!window.electronAPI) return;

    window.electronAPI.getSettings().then((s) => {
      setSettings(s);
    });

    window.electronAPI.getHistory().then((h) => {
      setDownloads(h || []);
    });

    // Check if initial first-time dependency setup is needed
    if (window.electronAPI.checkDependencies) {
      window.electronAPI.checkDependencies().then((status) => {
        if (!status.ready) {
          setIsSetupModalOpen(true);
        }
      });
    }

    const unsubProgress = window.electronAPI.onDownloadProgress((updatedItem) => {
      setDownloads((prev) => {
        const index = prev.findIndex((i) => i.id === updatedItem.id);
        if (index >= 0) {
          const clone = [...prev];
          clone[index] = updatedItem;
          return clone;
        }
        return [updatedItem, ...prev];
      });
    });

    const unsubClipboard = window.electronAPI.onClipboardDetected((detectedUrl) => {
      setClipboardUrl(detectedUrl);
    });

    const unsubUpdate = window.electronAPI.onUpdateAvailable?.((info) => {
      if (info && info.hasUpdate) {
        setUpdateInfo(info);
        setIsUpdateModalOpen(true);
      }
    });

    return () => {
      unsubProgress();
      unsubClipboard();
      if (unsubUpdate) unsubUpdate();
    };
  }, []);

  const handleResolve = async (url: string) => {
    if (!isOnline) {
      setError('You are currently offline. Please reconnect to download videos.');
      return;
    }

    setIsLoading(true);
    setError(null);
    setResolvedMeta(null);
    setClipboardUrl(null);

    try {
      if (window.electronAPI) {
        const meta = await window.electronAPI.resolveUrl(url, 'local');
        setResolvedMeta(meta);
      }
    } catch (err: any) {
      console.error('Resolve failed:', err);
      setError(err.message || 'Unable to fetch video. Please check the URL.');
    } finally {
      setIsLoading(false);
    }
  };

  const handleStartDownload = async (
    formatId: string,
    audioOnly: boolean,
    title: string,
    thumbnail: string
  ) => {
    if (!resolvedMeta || !window.electronAPI) return;

    const tempId = `dl_${Date.now()}_${Math.random().toString(36).substring(2, 7)}`;
    const optimisticItem: DownloadItem = {
      id: tempId,
      url: resolvedMeta.url,
      title,
      thumbnail,
      platform: 'video',
      formatId: formatId || 'best',
      formatLabel: audioOnly ? 'MP3 Audio' : 'Video',
      quality: audioOnly ? '320kbps' : 'Best Quality',
      audioOnly,
      engine: 'local',
      status: 'downloading',
      progress: 0,
      speed: 'Connecting...',
      createdAt: Date.now(),
    };

    // Optimistically insert into active downloads tray immediately (0ms delay)
    setDownloads((prev) => [optimisticItem, ...prev.filter((i) => i.id !== tempId)]);

    // Clear preview card immediately
    setResolvedMeta(null);

    try {
      await window.electronAPI.startDownload({
        id: tempId,
        url: optimisticItem.url,
        formatId,
        audioOnly,
        title,
        thumbnail,
        engine: 'local',
      });
    } catch (err: any) {
      setError(`Download error: ${err.message}`);
      setDownloads((prev) => prev.filter((i) => i.id !== tempId));
    }
  };

  const handleCancelDownload = (id: string) => {
    window.electronAPI?.cancelDownload(id);
  };

  const handleShowInFolder = (filePath: string) => {
    window.electronAPI?.showInFolder(filePath);
  };

  const handleOpenFile = (filePath: string) => {
    window.electronAPI?.openFile(filePath);
  };

  const handleClearHistory = () => {
    window.electronAPI?.clearHistory();
    setDownloads((prev) => prev.filter((i) => i.status === 'downloading' || i.status === 'queued'));
  };

  const handleUpdateSettings = (newSettings: Partial<AppSettings>) => {
    if (window.electronAPI) {
      window.electronAPI.saveSettings(newSettings).then((s) => {
        setSettings(s);
      });
    }
  };

  const activeCount = downloads.filter(
    (i) => i.status === 'downloading' || i.status === 'processing' || i.status === 'queued'
  ).length;

  const savedCount = downloads.filter(
    (i) => i.status === 'completed'
  ).length;

  return (
    <div className="flex flex-col h-screen bg-[#0f0f0e] text-[#f1f1ee] overflow-hidden select-none">
      {/* Custom Title Bar with Two Tabs & Status */}
      <TitleBar
        onOpenSettings={() => setIsSettingsOpen(true)}
        activeCount={activeCount}
        isOnline={isOnline}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        savedCount={savedCount}
        updateAvailable={updateInfo}
        onOpenUpdateModal={() => setIsUpdateModalOpen(true)}
      />

      {/* Main Scrollable View */}
      <main className="flex-1 overflow-y-auto px-4 py-6 flex flex-col items-center">
        {activeTab === 'library' ? (
          /* Downloads Library Tab (Fully available offline) */
          <DownloadsLibrary
            items={downloads}
            downloadDir={settings.downloadDir}
            onOpenFile={handleOpenFile}
            onShowInFolder={handleShowInFolder}
            onClearHistory={handleClearHistory}
            onSwitchToDownloader={() => setActiveTab('downloader')}
          />
        ) : (
          /* Downloader Tab */
          <div className="w-full max-w-2xl flex flex-col items-center animate-in fade-in duration-200">
            {!isOnline ? (
              /* Offline Empty State when disconnected */
              <OfflineEmptyState
                onSwitchToDownloads={() => setActiveTab('library')}
                onShowInFolder={() => handleShowInFolder(settings.downloadDir)}
                savedCount={savedCount}
              />
            ) : (
              /* Online Downloader View */
              <>
                {/* Hero Section */}
                <div className="w-full text-center mb-6">
                  <h1 className="text-[30px] font-semibold tracking-tight text-[#f1f1ee] leading-[1.1]">
                    <span className="block text-[#8d8d86]">Paste a link.</span>
                    <span className="block">Keep the video.</span>
                  </h1>
                  <p className="text-xs sm:text-sm text-[#b1b1aa] mt-2 max-w-md mx-auto leading-relaxed">
                    Save video or audio straight to your computer in up to 4K & MP3 with instant native processing.
                  </p>
                </div>

                {/* Input Bar */}
                <LinkInput onResolve={handleResolve} isLoading={isLoading} />

                {/* Error Alert */}
                {error && (
                  <div className="w-full mt-4 p-4 rounded-2xl bg-[#171716] border border-[#ff4a38]/30 text-[#ff4a38] text-xs flex items-start justify-between gap-3 animate-in fade-in shadow-xl">
                    <span className="leading-relaxed font-medium">{error}</span>
                    <button
                      onClick={() => setError(null)}
                      className="text-[#b1b1aa] hover:text-[#f1f1ee] font-semibold text-[11px] flex-shrink-0"
                    >
                      Dismiss
                    </button>
                  </div>
                )}

                {/* Resolved Video Card */}
                {resolvedMeta && (
                  <div className="w-full mt-6">
                    <VideoCard
                      metadata={resolvedMeta}
                      onDownload={handleStartDownload}
                      onClose={() => setResolvedMeta(null)}
                    />
                  </div>
                )}

                {/* Active Downloads Queue (In-progress jobs only) */}
                <div className="w-full mt-7 pb-10">
                  <DownloadQueue
                    items={downloads}
                    onCancel={handleCancelDownload}
                  />
                </div>
              </>
            )}
          </div>
        )}
      </main>

      {/* Auto-detected Clipboard Toast */}
      {isOnline && (
        <ClipboardToast
          url={clipboardUrl}
          onFetch={handleResolve}
          onDismiss={() => setClipboardUrl(null)}
        />
      )}

      {/* Settings Modal */}
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        settings={settings}
        onUpdateSettings={handleUpdateSettings}
        onShowUpdateModal={(info) => {
          setUpdateInfo(info);
          setIsUpdateModalOpen(true);
        }}
      />

      {/* First Launch Dependency Setup Modal */}
      <SetupModal
        isOpen={isSetupModalOpen}
        onComplete={() => setIsSetupModalOpen(false)}
      />

      {/* App Update Available Modal */}
      <UpdateModal
        isOpen={isUpdateModalOpen}
        updateInfo={updateInfo}
        onClose={() => setIsUpdateModalOpen(false)}
      />
    </div>
  );
};
export default App;
