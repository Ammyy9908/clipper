# ⚡ ClipPlay Desktop

Cross-platform native desktop application for **ClipPlay** — The Ultimate Video & Audio Downloader. Built with **Electron 31**, **React 18**, **TypeScript**, and **Vite**.

Supports **macOS** (Universal Apple Silicon & Intel), **Windows** (x64/ia32), and **Linux** (AppImage & deb).

---

## 🌟 Key Features

- **⚡ Dual Engine Architecture**:
  - **Cloud API Engine**: Instant remote conversion on ClipPlay cloud servers with zero local CPU overhead.
  - **Local Engine (yt-dlp + FFmpeg)**: Direct offline downloading up to 4K/8K 60FPS with unlimited bandwidth & zero cloud rate-limits.
- **📋 Smart Clipboard Daemon**:
  - Automatically detects copied video links (YouTube, Instagram Reels/Stories, TikTok, Twitter/X, Reddit) in the background and offers instant 1-click download.
- **📥 Download Queue & Speed HUD**:
  - Real-time download progress, transfer speed (MB/s), ETA timer, pause/resume, and native "Show in Finder / Explorer" actions.
- **🎛️ System Tray & Global Hotkeys**:
  - Minimize to system tray or menu bar.
  - Global toggle hotkey: `CommandOrControl + Shift + D`.
- **🎨 Glassmorphic Dark UI**:
  - Custom frameless obsidian window, vibrant gradient accents, and responsive layout.

---

## 🚀 Development Setup

```bash
# Navigate to the desktop directory
cd desktop

# Start Vite dev server with Electron live reload
npm run dev
```

---

## 📦 Packaging & Building

### 1. macOS (`.dmg`, `.zip` Universal Binary)
```bash
npm run build:mac
# Outputs: release/1.0.0/ClipPlay-1.0.0.dmg (Apple Silicon & Intel)
```

### 2. Windows (`.exe` NSIS Installer & Portable)
```bash
npm run build:win
# Outputs: release/1.0.0/ClipPlay Setup 1.0.0.exe
```

### 3. Linux (`.AppImage`, `.deb`)
```bash
npm run build:linux
# Outputs: release/1.0.0/ClipPlay-1.0.0.AppImage
```

---

## 📂 Project Structure

```
desktop/
├── electron/
│   ├── main.ts              # Main process (frameless window, system tray, IPC, global shortcuts)
│   ├── preload.ts           # Secure contextBridge API for React renderer
│   ├── clipboard.ts         # Clipboard URL detector daemon
│   ├── engine.ts            # Local yt-dlp/ffmpeg engine & Cloud API client
│   └── store.ts             # Persistent settings & download history manager
├── src/
│   ├── App.tsx              # Main UI controller
│   ├── main.tsx             # React DOM entry point
│   ├── index.css            # Glassmorphism dark design system
│   ├── types.ts             # TypeScript interfaces
│   └── components/
│       ├── TitleBar.tsx      # Frameless title bar with window controls & engine toggle
│       ├── LinkInput.tsx     # Glowing search input with paste button & platform badges
│       ├── VideoCard.tsx     # Media preview card with format pills (4K, 1080p, MP3)
│       ├── DownloadQueue.tsx # Download manager with speed (MB/s) & folder actions
│       ├── ClipboardToast.tsx# Floating quick-download toast for copied links
│       └── SettingsModal.tsx # Download folder, notifications, and engine configuration
├── electron-builder.yml     # Multi-platform distribution configuration
└── package.json
```
