import { app, BrowserWindow, ipcMain, dialog, shell, Tray, Menu, nativeImage, globalShortcut, Notification } from 'electron';
import path from 'node:path';
import https from 'node:https';
import http from 'node:http';
import { ClipboardMonitor } from './clipboard';
import { DownloadEngine } from './engine';
import { StorageManager } from './store';
import type { DownloadItem, AppSettings, UpdateInfo } from '../src/types';

const UPDATE_ENDPOINTS = [
  'https://downloads.clipplay.video/builds/downloads/version.json',
  'https://pub-1f8445496fb9472ca5596c85047c043c.r2.dev/builds/downloads/version.json',
  'https://clipplay.video/version.json',
];

function isNewerVersion(remoteVer: string, currentVer: string): boolean {
  if (!remoteVer || !currentVer) return false;
  const clean = (v: string) => v.replace(/^v/i, '').trim();
  const rParts = clean(remoteVer).split('.').map((n) => parseInt(n, 10) || 0);
  const cParts = clean(currentVer).split('.').map((n) => parseInt(n, 10) || 0);
  const maxLen = Math.max(rParts.length, cParts.length);
  for (let i = 0; i < maxLen; i++) {
    const r = rParts[i] || 0;
    const c = cParts[i] || 0;
    if (r > c) return true;
    if (r < c) return false;
  }
  return false;
}

async function fetchUpdateManifest(): Promise<any> {
  for (const endpoint of UPDATE_ENDPOINTS) {
    try {
      const data = await new Promise<string>((resolve, reject) => {
        const parsedUrl = new URL(endpoint);
        const client = parsedUrl.protocol === 'https:' ? https : http;
        const req = client.get(
          endpoint,
          {
            headers: {
              'User-Agent': `ClipPlay-Desktop/${app.getVersion()} (${process.platform}; ${process.arch})`,
              'Cache-Control': 'no-cache',
            },
            timeout: 6000,
          },
          (res) => {
            if (res.statusCode && res.statusCode >= 200 && res.statusCode < 300) {
              let body = '';
              res.on('data', (chunk) => (body += chunk));
              res.on('end', () => resolve(body));
            } else {
              reject(new Error(`HTTP ${res.statusCode}`));
            }
          }
        );
        req.on('error', reject);
        req.on('timeout', () => {
          req.destroy();
          reject(new Error('Update check timeout'));
        });
      });
      return JSON.parse(data);
    } catch (e) {
      console.warn(`[UpdateChecker] Failed check at ${endpoint}:`, e);
    }
  }
  throw new Error('Unable to connect to update server');
}

async function checkAppUpdates(): Promise<UpdateInfo> {
  const currentVersion = app.getVersion();
  try {
    const manifest = await fetchUpdateManifest();
    const hasUpdate = isNewerVersion(manifest.version, currentVersion);

    let downloadUrl = manifest.downloads?.win_x64;
    if (process.platform === 'darwin') {
      downloadUrl =
        process.arch === 'arm64'
          ? manifest.downloads?.mac_arm64 || manifest.downloads?.mac_dmg
          : manifest.downloads?.mac_x64 || manifest.downloads?.mac_dmg;
    } else if (process.platform === 'linux') {
      downloadUrl = manifest.downloads?.linux_x64;
    }

    return {
      hasUpdate,
      currentVersion,
      latestVersion: manifest.version,
      releaseDate: manifest.releaseDate,
      notes: manifest.notes,
      mandatory: !!manifest.mandatory,
      downloadUrl: downloadUrl || 'https://clipplay.video/desktop',
    };
  } catch (err: any) {
    return {
      hasUpdate: false,
      currentVersion,
      error: err.message || 'Unable to check for updates',
    };
  }
}


// Fix environment PATH for macOS/Linux GUI apps so Homebrew and user binaries are accessible
const homeDir = process.env.HOME || '';
const extraPaths = [
  '/opt/homebrew/bin',
  '/opt/homebrew/sbin',
  '/usr/local/bin',
  '/usr/bin',
  '/bin',
  '/usr/sbin',
  '/sbin',
  path.join(homeDir, 'Library/Python/3.12/bin'),
  path.join(homeDir, 'Library/Python/3.11/bin'),
  path.join(homeDir, 'Library/Python/3.10/bin'),
  path.join(homeDir, '.local/bin'),
];
process.env.PATH = Array.from(new Set([...extraPaths, ...(process.env.PATH || '').split(':')])).filter(Boolean).join(':');

// The built directory structure
process.env.DIST = path.join(__dirname, '../dist');
process.env.VITE_PUBLIC = app.isPackaged ? process.env.DIST : path.join(process.env.DIST, '../public');

let win: BrowserWindow | null = null;
let tray: Tray | null = null;
const preload = path.join(__dirname, 'preload.js');
const url = process.env.VITE_DEV_SERVER_URL;
const indexHtml = path.join(process.env.DIST, 'index.html');

const storage = new StorageManager();
const engine = new DownloadEngine();
const clipboardMonitor = new ClipboardMonitor();

app.whenReady().then(() => {
  engine.setUserDataPath(app.getPath('userData'));
});


function createWindow() {
  win = new BrowserWindow({
    title: 'ClipPlay Desktop',
    width: 980,
    height: 700,
    resizable: false,
    maximizable: false,
    fullscreenable: false,
    frame: false,
    titleBarStyle: 'hiddenInset',
    backgroundColor: '#0f0f0e',
    vibrancy: 'under-window',
    visualEffectState: 'active',
    webPreferences: {
      preload,
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  });

  if (url) {
    win.loadURL(url);
  } else {
    win.loadFile(indexHtml);
  }

  // Handle external links
  win.webContents.setWindowOpenHandler((details) => {
    shell.openExternal(details.url);
    return { action: 'deny' };
  });

  win.on('closed', () => {
    win = null;
  });
}

function createTray() {
  const publicDir = process.env.VITE_PUBLIC || path.join(__dirname, '../public');
  const iconPath = path.join(publicDir, 'icon.png');
  // Fallback to empty image if icon not yet created
  const trayIcon = nativeImage.createFromPath(iconPath).resize({ width: 18, height: 18 });
  
  tray = new Tray(trayIcon);
  const contextMenu = Menu.buildFromTemplate([
    {
      label: 'Open ClipPlay',
      click: () => {
        if (win) {
          win.show();
          win.focus();
        } else {
          createWindow();
        }
      },
    },
    { type: 'separator' },
    {
      label: 'Auto Clipboard Detection',
      type: 'checkbox',
      checked: storage.getSettings().autoClipboard,
      click: (menuItem) => {
        storage.saveSettings({ autoClipboard: menuItem.checked });
        clipboardMonitor.setEnabled(menuItem.checked);
      },
    },
    { type: 'separator' },
    {
      label: 'Quit',
      click: () => {
        app.quit();
      },
    },
  ]);

  tray.setToolTip('ClipPlay Desktop - Video Downloader');
  tray.setContextMenu(contextMenu);
  tray.on('click', () => {
    if (win) {
      if (win.isVisible()) {
        win.hide();
      } else {
        win.show();
        win.focus();
      }
    }
  });
}

// App lifecycle
app.whenReady().then(async () => {
  // Guide macOS users to move app to /Applications if running from DMG/Downloads
  if (app.isPackaged && process.platform === 'darwin') {
    try {
      if (!app.isInApplicationsFolder()) {
        app.moveToApplicationsFolder({
          conflictHandler: (conflictType) => {
            if (conflictType === 'exists') {
              return (
                dialog.showMessageBoxSync({
                  type: 'question',
                  buttons: ['Cancel', 'Replace'],
                  defaultId: 1,
                  cancelId: 0,
                  message: 'An existing version of ClipPlay was found in your Applications folder. Do you want to replace it?',
                }) === 1
              );
            }
            return true;
          },
        });
      }
    } catch (err) {
      console.error('Applications folder check error:', err);
    }
  }

  createWindow();
  createTray();

  // Register Global Shortcut (Cmd/Ctrl + Shift + D) to toggle ClipPlay window
  globalShortcut.register('CommandOrControl+Shift+D', () => {
    if (win) {
      if (win.isMinimized()) win.restore();
      win.show();
      win.focus();
    }
  });

  // Start clipboard monitor
  const settings = storage.getSettings();
  clipboardMonitor.setEnabled(settings.autoClipboard);
  clipboardMonitor.setCallback((detectedUrl) => {
    if (win && !win.isDestroyed()) {
      win.webContents.send('clipboard:detected', detectedUrl);
    }
  });
  clipboardMonitor.start();

  // Automatic background update check on startup
  setTimeout(async () => {
    try {
      const currentSettings = storage.getSettings();
      if (currentSettings.autoCheckUpdates !== false) {
        const update = await checkAppUpdates();
        if (update.hasUpdate && win && !win.isDestroyed()) {
          win.webContents.send('app:updateAvailable', update);
        }
      }
    } catch (e) {
      console.warn('Startup update check error:', e);
    }
  }, 3500);

  app.on('activate', () => {
    const allWindows = BrowserWindow.getAllWindows();
    if (allWindows.length) {
      allWindows[0].focus();
    } else {
      createWindow();
    }
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('will-quit', () => {
  globalShortcut.unregisterAll();
  clipboardMonitor.stop();
});

// IPC Event Handlers
ipcMain.on('window:minimize', () => win?.minimize());
ipcMain.on('window:maximize', () => {
  if (win?.isMaximized()) {
    win.unmaximize();
  } else {
    win?.maximize();
  }
});
ipcMain.on('window:close', () => win?.close());

// Media Resolve IPC
ipcMain.handle('media:resolve', async (_event, { url, engine: engineMode }) => {
  const settings = storage.getSettings();
  return engine.resolve(url, engineMode || settings.engine, settings.cloudApiUrl, settings);
});

// Download IPC
ipcMain.handle('download:start', async (_event, params) => {
  const settings = storage.getSettings();
  const id = params.id || `dl_${Date.now()}_${Math.random().toString(36).substring(2, 7)}`;

  const item: DownloadItem = {
    id,
    url: params.url,
    title: params.title,
    thumbnail: params.thumbnail,
    platform: 'video',
    formatId: params.formatId || 'best',
    formatLabel: params.audioOnly ? 'MP3 Audio' : 'Video',
    quality: params.audioOnly ? '320kbps' : 'Best Quality',
    audioOnly: !!params.audioOnly,
    engine: params.engine || settings.engine,
    status: 'downloading',
    progress: 0,
    speed: 'Connecting...',
    createdAt: Date.now(),
  };

  storage.addOrUpdateHistory(item);

  // Immediately broadcast to renderer so it appears in download tray at 0ms
  if (win && !win.isDestroyed()) {
    win.webContents.send('download:progress', item);
  }

  // Run in background without blocking IPC response
  engine.startDownload(item, settings.downloadDir, settings.cloudApiUrl, settings, (updated) => {
    storage.addOrUpdateHistory(updated);
    if (win && !win.isDestroyed()) {
      win.webContents.send('download:progress', updated);
    }

    if (updated.status === 'completed' && settings.notifyOnComplete) {
      new Notification({
        title: 'Download Complete!',
        body: updated.title,
        silent: false,
      }).show();
    }
  }).catch((err) => {
    item.status = 'failed';
    item.error = err.message;
    storage.addOrUpdateHistory(item);
    if (win && !win.isDestroyed()) {
      win.webContents.send('download:progress', item);
    }
  });

  return id;
});

ipcMain.handle('download:cancel', async (_event, id: string) => {
  engine.cancel(id);
  return true;
});

// Native Dialogs & Shell
ipcMain.handle('dialog:selectDirectory', async () => {
  if (!win) return null;
  const result = await dialog.showOpenDialog(win, {
    properties: ['openDirectory', 'createDirectory'],
  });
  if (result.canceled || result.filePaths.length === 0) {
    return null;
  }
  const chosenDir = result.filePaths[0];
  storage.saveSettings({ downloadDir: chosenDir });
  return chosenDir;
});

ipcMain.handle('dialog:selectFile', async (_event, options: { title?: string; filters?: { name: string; extensions: string[] }[] } = {}) => {
  if (!win) return null;
  const result = await dialog.showOpenDialog(win, {
    title: options.title || 'Select Binary or File',
    properties: ['openFile'],
    filters: options.filters || [],
  });
  if (result.canceled || result.filePaths.length === 0) {
    return null;
  }
  return result.filePaths[0];
});

ipcMain.handle('file:showInFolder', async (_event, filePath: string) => {
  shell.showItemInFolder(filePath);
});

ipcMain.handle('file:openFile', async (_event, filePath: string) => {
  shell.openPath(filePath);
});

// Settings & System
ipcMain.handle('settings:get', async () => storage.getSettings());
ipcMain.handle('settings:save', async (_event, newSettings) => {
  const updated = storage.saveSettings(newSettings);
  if (typeof newSettings.autoClipboard === 'boolean') {
    clipboardMonitor.setEnabled(newSettings.autoClipboard);
  }
  return updated;
});

ipcMain.handle('history:get', async () => storage.getHistory());
ipcMain.handle('history:clear', async () => storage.clearHistory());

ipcMain.handle('system:info', async () => {
  const settings = storage.getSettings();
  const info = await engine.checkSystemBinaries(settings);
  return {
    platform: process.platform,
    arch: process.arch,
    hasLocalYtdlp: info.hasYtdlp,
    hasLocalFfmpeg: info.hasFfmpeg,
    ytdlpPath: info.ytdlpPath,
    ffmpegPath: info.ffmpegPath,
    ytdlpVersion: info.ytdlpVersion,
    ffmpegVersion: info.ffmpegVersion,
    version: app.getVersion(),
  };
});

ipcMain.handle('system:checkBinaries', async (_event, customSettings?: Partial<AppSettings>) => {
  const currentSettings = storage.getSettings();
  const merged = { ...currentSettings, ...customSettings };
  const info = await engine.checkSystemBinaries(merged);
  return {
    platform: process.platform,
    arch: process.arch,
    hasLocalYtdlp: info.hasYtdlp,
    hasLocalFfmpeg: info.hasFfmpeg,
    ytdlpPath: info.ytdlpPath,
    ffmpegPath: info.ffmpegPath,
    ytdlpVersion: info.ytdlpVersion,
    ffmpegVersion: info.ffmpegVersion,
    version: app.getVersion(),
  };
});

ipcMain.handle('system:checkDependencies', async () => {
  const settings = storage.getSettings();
  const info = await engine.checkSystemBinaries(settings);
  return {
    ready: info.hasYtdlp && info.hasFfmpeg,
    hasYtdlp: info.hasYtdlp,
    hasFfmpeg: info.hasFfmpeg,
  };
});

ipcMain.handle('system:setupEngine', async () => {
  try {
    const success = await engine.ensureDependencies((pct, msg) => {
      if (win && !win.isDestroyed()) {
        win.webContents.send('setup:progress', { pct, msg });
      }
    });
    return { success };
  } catch (err: any) {
    return {
      success: false,
      error: err.message || 'Failed to initialize engine components',
    };
  }
});

// App Auto-Update & External Links IPC
ipcMain.handle('app:checkForUpdates', async () => {
  return checkAppUpdates();
});

ipcMain.handle('app:openExternal', async (_event, targetUrl: string) => {
  if (targetUrl && (targetUrl.startsWith('https://') || targetUrl.startsWith('http://'))) {
    shell.openExternal(targetUrl);
    return true;
  }
  return false;
});




