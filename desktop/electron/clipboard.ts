import { clipboard } from 'electron';

export class ClipboardMonitor {
  private lastText = '';
  private timer: NodeJS.Timeout | null = null;
  private onDetectedCallback: ((url: string) => void) | null = null;
  private isEnabled = true;

  // Regex patterns for video platforms
  private patterns = [
    /https?:\/\/(?:www\.)?(?:youtube\.com\/(?:watch\?v=|shorts\/|embed\/)|youtu\.be\/)[a-zA-Z0-9_-]+/i,
    /https?:\/\/(?:www\.)?instagram\.com\/(?:p|reel|reels|tv)\/[a-zA-Z0-9_-]+/i,
    /https?:\/\/(?:www\.|vm\.|vt\.)?tiktok\.com\/[@a-zA-Z0-9_.\/]+/i,
    /https?:\/\/(?:www\.)?(?:twitter\.com|x\.com)\/[a-zA-Z0-9_]+\/status\/[0-9]+/i,
    /https?:\/\/(?:www\.)?reddit\.com\/r\/[a-zA-Z0-9_]+\/comments\/[a-zA-Z0-9_]+/i,
    /https?:\/\/(?:www\.)?pinterest\.(?:com|co\.[a-z]+)\/pin\/[0-9]+/i,
    /https?:\/\/(?:www\.)?facebook\.com\/(?:watch\/\?v=|reel\/|[a-zA-Z0-9.]+\/videos\/)[0-9]+/i,
  ];

  constructor(onDetected?: (url: string) => void) {
    if (onDetected) {
      this.onDetectedCallback = onDetected;
    }
  }

  public setCallback(callback: (url: string) => void) {
    this.onDetectedCallback = callback;
  }

  public setEnabled(enabled: boolean) {
    this.isEnabled = enabled;
  }

  public start(intervalMs = 1200) {
    if (this.timer) return;
    this.lastText = clipboard.readText().trim();

    this.timer = setInterval(() => {
      if (!this.isEnabled) return;

      try {
        const currentText = clipboard.readText().trim();
        if (!currentText || currentText === this.lastText) {
          return;
        }

        this.lastText = currentText;
        const matched = this.matchUrl(currentText);
        if (matched && this.onDetectedCallback) {
          this.onDetectedCallback(matched);
        }
      } catch (err) {
        // clipboard access error ignored
      }
    }, intervalMs);
  }

  public stop() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  public matchUrl(text: string): string | null {
    for (const pattern of this.patterns) {
      const match = text.match(pattern);
      if (match) {
        return match[0];
      }
    }
    return null;
  }
}
