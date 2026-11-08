import { contextBridge, ipcRenderer } from 'electron';
import type { AppSettings, DownloadItem, VideoMetadata, UpdateInfo } from '../src/types';

export const electronAPI = {
  // Window controls
  minimize: () => ipcRenderer.send('window:minimize'),
  maximize: () => ipcRenderer.send('window:maximize'),
  close: () => ipcRenderer.send('window:close'),

  // Media Resolution
  resolveUrl: (url: string, engine: 'cloud' | 'local'): Promise<VideoMetadata> => 
    ipcRenderer.invoke('media:resolve', { url, engine }),

  // Downloads
  startDownload: (params: {
    id?: string;
    url: string;
    formatId?: string;
    audioOnly?: boolean;
    title: string;
    thumbnail: string;
    engine: 'cloud' | 'local';
  }): Promise<string> => ipcRenderer.invoke('download:start', params),

  pauseDownload: (id: string): Promise<boolean> => 
    ipcRenderer.invoke('download:pause', id),

  resumeDownload: (id: string): Promise<boolean> => 
    ipcRenderer.invoke('download:resume', id),

  cancelDownload: (id: string): Promise<boolean> => 
    ipcRenderer.invoke('download:cancel', id),

  // File management
  showInFolder: (filePath: string): Promise<void> => 
    ipcRenderer.invoke('file:showInFolder', filePath),

  openFile: (filePath: string): Promise<void> => 
    ipcRenderer.invoke('file:openFile', filePath),

  selectDownloadDir: (): Promise<string | null> => 
    ipcRenderer.invoke('dialog:selectDirectory'),

  selectFile: (options?: { title?: string; filters?: { name: string; extensions: string[] }[] }): Promise<string | null> => 
    ipcRenderer.invoke('dialog:selectFile', options),

  // Settings & Storage
  getSettings: (): Promise<AppSettings> => 
    ipcRenderer.invoke('settings:get'),

  saveSettings: (settings: Partial<AppSettings>): Promise<AppSettings> => 
    ipcRenderer.invoke('settings:save', settings),

  getHistory: (): Promise<DownloadItem[]> => 
    ipcRenderer.invoke('history:get'),

  clearHistory: (): Promise<void> => 
    ipcRenderer.invoke('history:clear'),

  getSystemInfo: (): Promise<{
    platform: string;
    arch: string;
    hasLocalYtdlp: boolean;
    hasLocalFfmpeg: boolean;
    ytdlpPath?: string;
    ffmpegPath?: string;
    ytdlpVersion?: string;
    ffmpegVersion?: string;
    version: string;
  }> => ipcRenderer.invoke('system:info'),

  checkBinaries: (customSettings?: Partial<AppSettings>): Promise<{
    platform: string;
    arch: string;
    hasLocalYtdlp: boolean;
    hasLocalFfmpeg: boolean;
    ytdlpPath?: string;
    ffmpegPath?: string;
    ytdlpVersion?: string;
    ffmpegVersion?: string;
    version: string;
  }> => ipcRenderer.invoke('system:checkBinaries', customSettings),

  checkDependencies: (): Promise<{
    ready: boolean;
    hasYtdlp: boolean;
    hasFfmpeg: boolean;
  }> => ipcRenderer.invoke('system:checkDependencies'),

  setupEngine: (): Promise<{
    success: boolean;
    error?: string;
  }> => ipcRenderer.invoke('system:setupEngine'),

  // App Updates & External
  checkForUpdates: (): Promise<UpdateInfo> =>
    ipcRenderer.invoke('app:checkForUpdates'),

  openExternal: (url: string): Promise<boolean> =>
    ipcRenderer.invoke('app:openExternal', url),

  // Subscriptions & Events
  onSetupProgress: (callback: (data: { pct: number; msg: string }) => void) => {
    const subscription = (_event: any, data: { pct: number; msg: string }) => callback(data);
    ipcRenderer.on('setup:progress', subscription);
    return () => {
      ipcRenderer.removeListener('setup:progress', subscription);
    };
  },

  onInstallProgress: (callback: (data: { pct: number; msg: string }) => void) => {
    const subscription = (_event: any, data: { pct: number; msg: string }) => callback(data);
    ipcRenderer.on('installer:progress', subscription);
    return () => {
      ipcRenderer.removeListener('installer:progress', subscription);
    };
  },

  onDownloadProgress: (callback: (item: DownloadItem) => void) => {
    const subscription = (_event: any, item: DownloadItem) => callback(item);
    ipcRenderer.on('download:progress', subscription);
    return () => {
      ipcRenderer.removeListener('download:progress', subscription);
    };
  },

  onClipboardDetected: (callback: (url: string) => void) => {
    const subscription = (_event: any, url: string) => callback(url);
    ipcRenderer.on('clipboard:detected', subscription);
    return () => {
      ipcRenderer.removeListener('clipboard:detected', subscription);
    };
  },

  onUpdateAvailable: (callback: (info: UpdateInfo) => void) => {
    const subscription = (_event: any, info: UpdateInfo) => callback(info);
    ipcRenderer.on('app:updateAvailable', subscription);
    return () => {
      ipcRenderer.removeListener('app:updateAvailable', subscription);
    };
  },
};

contextBridge.exposeInMainWorld('electronAPI', electronAPI);

declare global {
  interface Window {
    electronAPI: typeof electronAPI;
  }
}

