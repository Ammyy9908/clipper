# Chrome Web Store Listing — clipPlay

*Last updated: 2026-09-24*
*Version: 1.0.0*

---

## 1. Store Listing Metadata

| Field | Content | Limits / Requirements |
|---|---|---|
| **Name** | clipPlay - Video & MP3 Downloader | Max 45 characters (used: 33) |
| **Short Description** | Download YouTube videos in 1080p/4K or convert to MP3 with one click on clipPlay. | Max 132 characters (used: 82) |
| **Detailed Description** | *(See Section 2 below)* | Up to 16,000 characters |
| **Category** | Productivity / Multimedia | Best fit for core utility |
| **Primary Language** | English | |
| **Website URL** | https://www.clipplay.video | |
| **Support URL** | https://www.clipplay.video | |
| **Privacy Policy URL** | https://www.clipplay.video/privacy | Live & matching |

---

## 2. Graphic Assets & Screenshots

| Asset | Size | Path |
|---|---|---|
| **Store Icon** (Required) | 128x128 | `extension/icons/icon-128.png` |
| **Screenshot 1** (YouTube Injected Button) | 1280x800 | `extension/screenshots/screenshot-1-yt-button.png` |
| **Screenshot 2** (Format & MP3 Selector) | 1280x800 | `extension/screenshots/screenshot-2-formats.png` |
| **Screenshot 3** (Shorts & Clip Trimmer) | 1280x800 | `extension/screenshots/screenshot-3-shorts.png` |
| **Small Promo Tile** (Optional) | 440x280 | `extension/screenshots/promo-small-440x280.png` |
| **Marquee Promo Tile** (Optional) | 1400x560 | `extension/screenshots/promo-marquee-1400x560.png` |

---

## 3. Detailed Description (Copy-Paste for Dashboard)

```markdown
clipPlay adds a clean, fast "Download" button directly below YouTube videos and YouTube Shorts, allowing you to save videos in up to 1080p/4K or extract high-quality MP3 audio in seconds.

### ⚡ Key Features:
- **Direct YouTube Integration**: Injects a native-feeling "Download" button right into YouTube's action bar alongside Like and Share.
- **Multiple Video Qualities**: Save in 1080p Full HD, 720p HD, 480p, 360p, or 4K.
- **MP3 Audio Converter**: Extract crisp 320kbps MP3 audio with one click.
- **Video Splitting & Clipping**: Seamlessly trim or split videos into 15s, 30s, or 60s clips for Shorts, TikTok, and Instagram Reels.
- **Privacy by Design**: No tracking scripts, no advertisements, and files are automatically purged after 24 hours.

### 🚀 How to Use:
1. Open any video on YouTube (or YouTube Shorts).
2. Click the red "Download" button under the video player.
3. clipPlay opens with the video pre-resolved — choose your preferred format and save!
```

---

## 4. Permissions Justification (For Privacy Tab in Dashboard)

| Permission / Host | Why It's Needed (Plain English) |
|---|---|
| `activeTab` | Detects the current active video URL when the extension icon is clicked in the toolbar. |
| `tabs` | Opens clipPlay in a new tab with the target video URL parameter. |
| `contextMenus` | Adds a right-click "Download with clipPlay" option when right-clicking YouTube video links. |
| `https://*.youtube.com/*` | Injects the Download button directly into YouTube's action bar below the video player. |

---

## 5. Single Purpose Description

> "This extension provides a one-click button on YouTube video and Shorts pages to quickly open and resolve videos on clipPlay for downloading in multiple resolutions or converting to MP3 audio."

---

## 6. Data Privacy & Use Disclosure

- **Personally Identifiable Information**: None collected.
- **Health / Financial / Authentication Data**: None collected.
- **Browsing History**: Not collected or tracked.
- **Third-party scripts**: Zero third-party trackers or ad networks.
