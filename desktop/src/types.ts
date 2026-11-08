export type EngineMode = 'cloud' | 'local';

export interface VideoFormat {
  id: string;
  ext: string;
  height?: number;
  width?: number;
  fps?: number;
  filesize?: number;
  approxFilesize?: number;
  vcodec?: string;
  acodec?: string;
  note?: string;
  hasVideo: boolean;
  hasAudio: boolean;
  qualityLabel?: string;
}

export interface VideoMetadata {
  id: string;
  url: string;
  title: string;
  uploader?: string;
  duration: number; // in seconds
  thumbnail: string;
  platform: 'youtube' | 'instagram' | 'tiktok' | 'twitter' | 'other';
  formats: VideoFormat[];
  isPlaylist?: boolean;
  playlistCount?: number;
}

export type DownloadStatus = 'queued' | 'downloading' | 'processing' | 'completed' | 'failed' | 'paused';

export interface DownloadItem {
  id: string;
  url: string;
  title: string;
  thumbnail: string;
  platform: string;
  formatId: string;
  formatLabel: string;
  quality: string;
  audioOnly: boolean;
  engine: EngineMode;
  status: DownloadStatus;
  progress: number; // 0 - 100
  speed?: string; // e.g. "12.4 MB/s"
  eta?: string; // e.g. "00:15"
  downloadedBytes?: number;
  totalBytes?: number;
  outputPath?: string;
  error?: string;
  createdAt: number;
  completedAt?: number;
}

export interface AppSettings {
  downloadDir: string;
  engine: EngineMode;
  autoClipboard: boolean;
  maxConcurrentDownloads: number;
  theme: 'dark' | 'midnight' | 'cyber';
  notifyOnComplete: boolean;
  autoCheckUpdates?: boolean;
  cloudApiUrl: string;
  customYtdlpPath?: string;
  customFfmpegPath?: string;
  cookiesPath?: string;
  extraArgs?: string;
}

export interface UpdateInfo {
  hasUpdate: boolean;
  currentVersion: string;
  latestVersion?: string;
  releaseDate?: string;
  notes?: string;
  mandatory?: boolean;
  downloadUrl?: string;
  error?: string;
}

export interface SystemInfo {
  platform: string;
  arch: string;
  hasLocalYtdlp: boolean;
  hasLocalFfmpeg: boolean;
  ytdlpPath?: string;
  ffmpegPath?: string;
  ytdlpVersion?: string;
  ffmpegVersion?: string;
  version: string;
}


