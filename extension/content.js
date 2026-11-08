// clipPlay YouTube In-Page Integration (Direct Fast Redirect)

(function () {
  "use strict";

  const SITE_URL = "https://www.clipplay.video";

  /**
   * Checks if current page is a YouTube video or Shorts page.
   */
  function isYouTubeVideoPage() {
    const path = window.location.pathname;
    return path === "/watch" || path.startsWith("/shorts/");
  }

  /**
   * Extracts clean canonical YouTube URL.
   */
  function getCurrentVideoUrl() {
    const url = new URL(window.location.href);
    if (url.pathname === "/watch") {
      const v = url.searchParams.get("v");
      return v ? `https://www.youtube.com/watch?v=${v}` : window.location.href;
    }
    if (url.pathname.startsWith("/shorts/")) {
      const shortId = url.pathname.split("/")[2];
      return shortId ? `https://www.youtube.com/shorts/${shortId}` : window.location.href;
    }
    return window.location.href;
  }

  /**
   * Opens clipPlay with the video URL query parameter.
   */
  function redirectToClipPlay() {
    const videoUrl = getCurrentVideoUrl();
    const targetUrl = `${SITE_URL}/?url=${encodeURIComponent(videoUrl)}`;

    try {
      if (chrome.runtime?.id) {
        chrome.runtime.sendMessage({
          action: "OPEN_CLIPPLAY_SITE",
          url: videoUrl,
        });
      } else {
        window.open(targetUrl, "_blank");
      }
    } catch {
      window.open(targetUrl, "_blank");
    }
  }

  /**
   * Injects the Download button into YouTube's action bar.
   */
  function injectDownloadButton() {
    if (!isYouTubeVideoPage()) {
      removeExistingButton();
      return;
    }

    if (document.getElementById("clipplay-action-btn")) {
      return;
    }

    const targetContainers = [
      "#actions #top-level-buttons-computed",
      "#actions-inner #top-level-buttons-computed",
      "ytd-watch-metadata #actions #top-level-buttons-computed",
      "ytd-watch-metadata #actions",
      "#top-level-buttons-computed",
      "#menu #top-level-buttons-computed",
      "ytd-reel-player-overlay-renderer #actions",
    ];

    let container = null;
    for (const selector of targetContainers) {
      const el = document.querySelector(selector);
      if (el) {
        container = el;
        break;
      }
    }

    if (!container) return;

    const btn = document.createElement("button");
    btn.id = "clipplay-action-btn";
    btn.className = "clipplay-yt-btn";
    btn.setAttribute("type", "button");
    btn.setAttribute("title", "Download video or convert to MP3 on clipPlay");

    btn.innerHTML = `
      <svg viewBox="0 0 24 24">
        <path d="M12 15.5l-5-5h3V4h4v6.5h3l-5 5zm-7 3.5v2h14v-2H5z"/>
      </svg>
      <span>Download</span>
      <span class="cp-badge">clipPlay</span>
    `;

    btn.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      redirectToClipPlay();
    });

    container.appendChild(btn);
  }

  function removeExistingButton() {
    const existing = document.getElementById("clipplay-action-btn");
    if (existing) existing.remove();
  }

  // Watch for YouTube SPA navigation events
  window.addEventListener("yt-navigate-finish", () => {
    setTimeout(injectDownloadButton, 300);
    setTimeout(injectDownloadButton, 1000);
  });

  window.addEventListener("spfdone", () => {
    setTimeout(injectDownloadButton, 300);
  });

  // Observe DOM for dynamic action bar insertion
  const observer = new MutationObserver(() => {
    if (isYouTubeVideoPage() && !document.getElementById("clipplay-action-btn")) {
      injectDownloadButton();
    }
  });

  observer.observe(document.body, {
    childList: true,
    subtree: true,
  });

  // Initial attempt on load
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => {
      setTimeout(injectDownloadButton, 500);
    });
  } else {
    setTimeout(injectDownloadButton, 500);
  }
})();
