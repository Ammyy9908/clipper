import { app } from 'electron';
import fs from 'node:fs';
import path from 'node:path';
import type { AppSettings, DownloadItem } from '../src/types';

const getDefaultDownloadDir = () => path.join(app.getPath('downloads'), 'ClipPlay');

const defaultSettings: AppSettings = {
  downloadDir: getDefaultDownloadDir(),
  engine: 'local',
  autoClipboard: true,
  maxConcurrentDownloads: 3,
  theme: 'dark',
  notifyOnComplete: true,
  autoCheckUpdates: true,
  cloudApiUrl: 'https://api.clipplay.video',
};

export class StorageManager {
  private configPath: string;
  private historyPath: string;
  private settings: AppSettings;
  private history: DownloadItem[] = [];

  constructor() {
    const userData = app.getPath('userData');
    if (!fs.existsSync(userData)) {
      fs.mkdirSync(userData, { recursive: true });
    }

    this.configPath = path.join(userData, 'settings.json');
    this.historyPath = path.join(userData, 'history.json');

    this.settings = this.loadSettings();
    this.history = this.loadHistory();
  }

  private loadSettings(): AppSettings {
    try {
      if (fs.existsSync(this.configPath)) {
        const data = fs.readFileSync(this.configPath, 'utf-8');
        const parsed = JSON.parse(data);
        const s = { ...defaultSettings, ...parsed };
        // Upgrade legacy generic download path to ~/Downloads/ClipPlay
        if (!s.downloadDir || s.downloadDir === app.getPath('downloads')) {
          s.downloadDir = getDefaultDownloadDir();
        }
        return s;
      }
    } catch (err) {
      console.error('Failed to load settings:', err);
    }
    return { ...defaultSettings };
  }


  private loadHistory(): DownloadItem[] {
    try {
      if (fs.existsSync(this.historyPath)) {
        const data = fs.readFileSync(this.historyPath, 'utf-8');
        return JSON.parse(data);
      }
    } catch (err) {
      console.error('Failed to load history:', err);
    }
    return [];
  }

  public getSettings(): AppSettings {
    return this.settings;
  }

  public saveSettings(newSettings: Partial<AppSettings>): AppSettings {
    this.settings = { ...this.settings, ...newSettings };
    try {
      fs.writeFileSync(this.configPath, JSON.stringify(this.settings, null, 2), 'utf-8');
    } catch (err) {
      console.error('Failed to save settings:', err);
    }
    return this.settings;
  }

  public getHistory(): DownloadItem[] {
    return this.history;
  }

  public addOrUpdateHistory(item: DownloadItem) {
    const index = this.history.findIndex(h => h.id === item.id);
    if (index >= 0) {
      this.history[index] = item;
    } else {
      this.history.unshift(item);
    }
    // Cap history at 200 items
    if (this.history.length > 200) {
      this.history = this.history.slice(0, 200);
    }
    this.persistHistory();
  }

  public clearHistory() {
    this.history = [];
    this.persistHistory();
  }

  private persistHistory() {
    try {
      fs.writeFileSync(this.historyPath, JSON.stringify(this.history, null, 2), 'utf-8');
    } catch (err) {
      console.error('Failed to persist history:', err);
    }
  }
}
