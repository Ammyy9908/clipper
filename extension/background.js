// clipPlay Chrome Extension - Background Service Worker (Manifest V3)

const SITE_URL = "https://www.clipplay.video";

/**
 * Opens clipPlay with the given video URL query parameter.
 */
async function openClipPlayForUrl(videoUrl) {
  let targetUrl = SITE_URL;
  if (videoUrl && typeof videoUrl === "string" && videoUrl.startsWith("http")) {
    targetUrl = `${SITE_URL}/?url=${encodeURIComponent(videoUrl)}`;
  }
  await chrome.tabs.create({ url: targetUrl });
}

// When clicking the extension action icon in the Chrome toolbar
chrome.action.onClicked.addListener(async (tab) => {
  const currentUrl = tab?.url || "";
  if (currentUrl.includes("youtube.com/watch") || currentUrl.includes("youtube.com/shorts") || currentUrl.includes("tiktok.com") || currentUrl.includes("instagram.com")) {
    await openClipPlayForUrl(currentUrl);
  } else {
    await chrome.tabs.create({ url: SITE_URL });
  }
});

// Handle runtime messages from content script
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.action === "OPEN_CLIPPLAY_SITE") {
    openClipPlayForUrl(message.url);
    sendResponse({ success: true });
  }
  return true;
});

// Create context menu on installation
if (chrome.contextMenus) {
  chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.create({
      id: "clipplay_download_menu",
      title: "Download with clipPlay",
      contexts: ["link", "page", "video"],
      targetUrlPatterns: [
        "*://*.youtube.com/watch*",
        "*://*.youtube.com/shorts/*",
        "*://youtube.com/watch*",
        "*://youtube.com/shorts/*",
      ],
    });
  });

  chrome.contextMenus.onClicked.addListener(async (info, tab) => {
    if (info.menuItemId === "clipplay_download_menu") {
      const targetUrl = info.linkUrl || info.pageUrl || tab?.url;
      if (targetUrl) {
        await openClipPlayForUrl(targetUrl);
      }
    }
  });
}
