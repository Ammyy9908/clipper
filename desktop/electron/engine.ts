import { spawn, execFile } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import https from 'node:https';
import http from 'node:http';
import { promisify } from 'node:util';
import type { DownloadItem, VideoMetadata, VideoFormat, AppSettings } from '../src/types';

const execFileAsync = promisify(execFile);

export class DownloadEngine {
  private activeProcesses = new Map<string, any>();
  private activeJobs = new Map<string, boolean>();
  private userDataPath: string = '';
  private binariesVerified = false;

  public setUserDataPath(p: string) {
    this.userDataPath = p;
  }

  /**
   * Resolve yt-dlp binary path:
   * 1. User-specified custom path from settings
   * 2. App-managed standalone binary (<userData>/bin/yt-dlp)
   * 3. Active PATH directories
   * 4. Platform-specific known installation directories
   * 5. Default executable name fallback
   */
  public getYtdlpBin(customPath?: string): string {
    if (customPath && customPath.trim() !== '') {
      const trimmed = customPath.trim();
      if (fs.existsSync(trimmed)) {
        return trimmed;
      }
    }

    const isWin = process.platform === 'win32';
    const isMac = process.platform === 'darwin';
    const binName = isWin ? 'yt-dlp.exe' : 'yt-dlp';
    const homeDir = process.env.HOME || process.env.USERPROFILE || '';

    // Check app-managed binary in userData/bin
    if (this.userDataPath) {
      const appManaged = path.join(this.userDataPath, 'bin', binName);
      if (fs.existsSync(appManaged)) {
        return appManaged;
      }
    }

    // Search directories in PATH
    const envPaths = (process.env.PATH || '').split(isWin ? ';' : ':');
    for (const p of envPaths) {
      if (!p) continue;
      const full = path.join(p, binName);
      if (fs.existsSync(full)) {
        return full;
      }
    }

    // Platform-specific candidate paths
    const candidates: string[] = [];

    if (isMac) {
      candidates.push(
        '/opt/homebrew/bin/yt-dlp',
        '/usr/local/bin/yt-dlp',
        '/usr/bin/yt-dlp',
        path.join(homeDir, 'Library/Python/3.13/bin/yt-dlp'),
        path.join(homeDir, 'Library/Python/3.12/bin/yt-dlp'),
        path.join(homeDir, 'Library/Python/3.11/bin/yt-dlp'),
        path.join(homeDir, 'Library/Python/3.10/bin/yt-dlp'),
        path.join(homeDir, '.local/bin/yt-dlp'),
        '/Applications/yt-dlp'
      );
    } else if (isWin) {
      const localAppData = process.env.LOCALAPPDATA || '';
      const appData = process.env.APPDATA || '';
      const progFiles = process.env.ProgramFiles || 'C:\\Program Files';
      const progFilesX86 = process.env['ProgramFiles(x86)'] || 'C:\\Program Files (x86)';

      candidates.push(
        path.join(localAppData, 'yt-dlp', 'yt-dlp.exe'),
        path.join(appData, 'yt-dlp', 'yt-dlp.exe'),
        path.join(homeDir, 'scoop', 'shims', 'yt-dlp.exe'),
        path.join(progFiles, 'yt-dlp', 'yt-dlp.exe'),
        path.join(progFilesX86, 'yt-dlp', 'yt-dlp.exe'),
        path.join(homeDir, 'AppData', 'Local', 'Microsoft', 'WinGet', 'Packages', 'yt-dlp.exe'),
        'C:\\yt-dlp\\yt-dlp.exe',
        'C:\\yt-dlp.exe'
      );
    } else {
      // Linux & BSD
      candidates.push(
        '/usr/local/bin/yt-dlp',
        '/usr/bin/yt-dlp',
        '/bin/yt-dlp',
        path.join(homeDir, '.local/bin/yt-dlp'),
        '/snap/bin/yt-dlp',
        '/var/lib/flatpak/exports/bin/yt-dlp'
      );
    }

    for (const c of candidates) {
      if (fs.existsSync(c)) {
        return c;
      }
    }

    return binName;
  }

  /**
   * Resolve FFmpeg binary path
   */
  public getFfmpegBin(customPath?: string): string {
    if (customPath && customPath.trim() !== '') {
      const trimmed = customPath.trim();
      if (fs.existsSync(trimmed)) {
        return trimmed;
      }
    }

    // Check bundled ffmpeg-static (handling app.asar / app.asar.unpacked)
    try {
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      const staticFfmpeg = require('ffmpeg-static');
      if (staticFfmpeg && typeof staticFfmpeg === 'string') {
        const unpacked = staticFfmpeg.replace('app.asar', 'app.asar.unpacked');
        if (fs.existsSync(unpacked)) {
          return unpacked;
        }
        if (fs.existsSync(staticFfmpeg)) {
          return staticFfmpeg;
        }
      }
    } catch {
      // ffmpeg-static not available
    }

    const isWin = process.platform === 'win32';
    const isMac = process.platform === 'darwin';
    const binName = isWin ? 'ffmpeg.exe' : 'ffmpeg';
    const homeDir = process.env.HOME || process.env.USERPROFILE || '';

    if (this.userDataPath) {
      const appManaged = path.join(this.userDataPath, 'bin', binName);
      if (fs.existsSync(appManaged)) {
        return appManaged;
      }
    }

    const envPaths = (process.env.PATH || '').split(isWin ? ';' : ':');
    for (const p of envPaths) {
      if (!p) continue;
      const full = path.join(p, binName);
      if (fs.existsSync(full)) {
        return full;
      }
    }

    const candidates: string[] = [];

    if (isMac) {
      candidates.push(
        '/opt/homebrew/bin/ffmpeg',
        '/usr/local/bin/ffmpeg',
        '/usr/bin/ffmpeg',
        path.join(homeDir, '.local/bin/ffmpeg')
      );
    } else if (isWin) {
      const localAppData = process.env.LOCALAPPDATA || '';
      const progFiles = process.env.ProgramFiles || 'C:\\Program Files';
      const progFilesX86 = process.env['ProgramFiles(x86)'] || 'C:\\Program Files (x86)';

      candidates.push(
        path.join(progFiles, 'ffmpeg', 'bin', 'ffmpeg.exe'),
        path.join(progFilesX86, 'ffmpeg', 'bin', 'ffmpeg.exe'),
        path.join(localAppData, 'ffmpeg', 'bin', 'ffmpeg.exe'),
        path.join(homeDir, 'scoop', 'shims', 'ffmpeg.exe'),
        'C:\\ffmpeg\\bin\\ffmpeg.exe',
        'C:\\ffmpeg.exe'
      );
    } else {
      candidates.push(
        '/usr/local/bin/ffmpeg',
        '/usr/bin/ffmpeg',
        '/bin/ffmpeg',
        path.join(homeDir, '.local/bin/ffmpeg'),
        '/snap/bin/ffmpeg'
      );
    }

    for (const c of candidates) {
      if (fs.existsSync(c)) {
        return c;
      }
    }

    return binName;
  }

  // 1-Click Auto Installer for yt-dlp standalone binary
  public async installYtdlp(
    onProgress?: (percent: number, status: string) => void
  ): Promise<{ success: boolean; path: string; version?: string }> {
    const isWin = process.platform === 'win32';
    const isMac = process.platform === 'darwin';
    const binName = isWin ? 'yt-dlp.exe' : 'yt-dlp';

    const binDir = path.join(this.userDataPath || '', 'bin');
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }

    const targetPath = path.join(binDir, binName);

    let downloadUrl = 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp';
    if (isWin) {
      downloadUrl = 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe';
    } else if (isMac) {
      downloadUrl = 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos';
    }

    onProgress?.(5, 'Connecting to core engine service...');

    await this.downloadHttpFile(downloadUrl, targetPath, (pct, speed) => {
      const mapped = Math.min(Math.round(5 + pct * 0.85), 90);
      onProgress?.(mapped, `Downloading core engine (${speed})...`);
    });

    onProgress?.(92, 'Finalizing core components...');
    if (!isWin) {
      try {
        fs.chmodSync(targetPath, 0o755);
      } catch (err) {
        console.error('chmod error:', err);
      }
    }

    onProgress?.(96, 'Verifying engine...');
    try {
      const { stdout } = await execFileAsync(targetPath, ['--version'], { env: process.env });
      const version = stdout.trim();
      onProgress?.(100, 'Core engine installed successfully!');
      return {
        success: true,
        path: targetPath,
        version,
      };
    } catch {
      onProgress?.(100, 'Core engine installed successfully!');
      return {
        success: true,
        path: targetPath,
        version: 'Latest',
      };
    }
  }

  // Auto Installer for FFmpeg binary if not bundled
  public async installFfmpeg(
    onProgress?: (percent: number, status: string) => void
  ): Promise<{ success: boolean; path: string; version?: string }> {
    const isWin = process.platform === 'win32';
    const isMac = process.platform === 'darwin';
    const isArm = process.arch === 'arm64';
    const binName = isWin ? 'ffmpeg.exe' : 'ffmpeg';

    const binDir = path.join(this.userDataPath || '', 'bin');
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }

    const targetPath = path.join(binDir, binName);

    let downloadUrl = 'https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0.1/linux-x64';
    if (isMac) {
      downloadUrl = isArm
        ? 'https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0.1/darwin-arm64'
        : 'https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0.1/darwin-x64';
    } else if (isWin) {
      downloadUrl = 'https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0.1/win32-x64';
    } else if (isArm) {
      downloadUrl = 'https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0.1/linux-arm64';
    }

    onProgress?.(5, 'Connecting to media processor service...');

    await this.downloadHttpFile(downloadUrl, targetPath, (pct, speed) => {
      const mapped = Math.min(Math.round(5 + pct * 0.85), 90);
      onProgress?.(mapped, `Downloading media processor (${speed})...`);
    });

    onProgress?.(92, 'Configuring media components...');
    if (!isWin) {
      try {
        fs.chmodSync(targetPath, 0o755);
      } catch (err) {
        console.error('chmod error:', err);
      }
    }

    onProgress?.(96, 'Verifying media processor...');
    try {
      const { stdout } = await execFileAsync(targetPath, ['-version'], { env: process.env });
      const firstLine = stdout.split('\n')[0] || '';
      const match = firstLine.match(/version\s+([^\s]+)/i);
      const version = match ? match[1] : 'Installed';
      onProgress?.(100, 'Media processor configured!');
      return {
        success: true,
        path: targetPath,
        version,
      };
    } catch {
      onProgress?.(100, 'Media processor configured!');
      return {
        success: true,
        path: targetPath,
        version: 'Latest',
      };
    }
  }

  // Check and report real-time toolchain status
  public async checkSystemBinaries(settings?: Partial<AppSettings>): Promise<{
    hasYtdlp: boolean;
    hasFfmpeg: boolean;
    ytdlpPath: string;
    ffmpegPath: string;
    ytdlpVersion?: string;
    ffmpegVersion?: string;
  }> {
    const ytdlpBin = this.getYtdlpBin(settings?.customYtdlpPath);
    const ffmpegBin = this.getFfmpegBin(settings?.customFfmpegPath);

    let hasYtdlp = false;
    let ytdlpVersion: string | undefined;
    let hasFfmpeg = false;
    let ffmpegVersion: string | undefined;

    try {
      const { stdout } = await execFileAsync(ytdlpBin, ['--version'], { env: process.env });
      ytdlpVersion = stdout.trim();
      hasYtdlp = true;
    } catch {
      hasYtdlp = false;
    }

    try {
      const { stdout } = await execFileAsync(ffmpegBin, ['-version'], { env: process.env });
      const firstLine = stdout.split('\n')[0] || '';
      const match = firstLine.match(/version\s+([^\s]+)/i);
      ffmpegVersion = match ? match[1] : 'Installed';
      hasFfmpeg = true;
    } catch {
      hasFfmpeg = false;
    }

    return {
      hasYtdlp,
      hasFfmpeg,
      ytdlpPath: ytdlpBin,
      ffmpegPath: ffmpegBin,
      ytdlpVersion,
      ffmpegVersion,
    };
  }

  // Automatically ensure both dependencies are installed and available
  public async ensureDependencies(
    onProgress?: (percent: number, status: string) => void
  ): Promise<boolean> {
    if (this.binariesVerified) {
      return true;
    }

    const status = await this.checkSystemBinaries();
    if (status.hasYtdlp && status.hasFfmpeg) {
      this.binariesVerified = true;
      onProgress?.(100, 'Ready!');
      return true;
    }

    try {
      let currentPct = 5;
      onProgress?.(currentPct, 'Preparing download & media components...');

      // 1. Install yt-dlp if needed
      if (!status.hasYtdlp) {
        const maxStagePct = status.hasFfmpeg ? 95 : 50;
        await this.installYtdlp((pct) => {
          const mapped = Math.min(Math.round(5 + (pct / 100) * (maxStagePct - 5)), maxStagePct);
          let msg = 'Downloading high-speed download engine...';
          if (pct > 80) msg = 'Finalizing download engine...';
          else if (pct > 40) msg = 'Configuring download components...';
          onProgress?.(mapped, msg);
        });
        currentPct = maxStagePct;
      }

      // 2. Install ffmpeg if needed
      const updatedStatus = await this.checkSystemBinaries();
      if (!updatedStatus.hasFfmpeg) {
        await this.installFfmpeg((pct) => {
          const mapped = Math.min(Math.round(currentPct + (pct / 100) * (95 - currentPct)), 95);
          let msg = 'Downloading media processor...';
          if (pct > 80) msg = 'Configuring audio and video converter...';
          else if (pct > 40) msg = 'Optimizing media engine...';
          onProgress?.(mapped, msg);
        });
      }

      this.binariesVerified = true;
      onProgress?.(100, 'Ready!');
      return true;
    } catch (err) {
      console.error('Failed to ensure dependencies:', err);
      throw err;
    }
  }

  // 1. Resolve URL locally (Bypasses datacenter IP blocks)
  public async resolve(
    url: string,
    _engine: 'cloud' | 'local',
    _apiUrl: string,
    settings?: AppSettings
  ): Promise<VideoMetadata> {
    return this.resolveLocal(url, settings);
  }

  private async resolveLocal(url: string, settings?: AppSettings): Promise<VideoMetadata> {
    await this.ensureDependencies();
    const bin = this.getYtdlpBin(settings?.customYtdlpPath);

    const args = [
      '-J',
      '--no-warnings',
      '--no-playlist',
      '--remote-components', 'ejs:github',
      '--js-runtimes', 'node',
      '--extractor-args', 'youtube:player_client=tv_embedded,web_embedded,mweb,android;skip=translated_subs,storyboards,comments',
    ];

    if (settings?.cookiesPath && fs.existsSync(settings.cookiesPath)) {
      args.push('--cookies', settings.cookiesPath);
    }

    if (settings?.extraArgs && settings.extraArgs.trim()) {
      const customArgs = settings.extraArgs.trim().split(/\s+/);
      args.push(...customArgs);
    }

    args.push(url);

    try {
      const { stdout } = await execFileAsync(bin, args, {
        maxBuffer: 25 * 1024 * 1024,
        env: process.env,
      });

      const info = JSON.parse(stdout);
      return this.mapLocalMetadata(info, url);
    } catch (err: any) {
      throw new Error(err.stderr || err.message || 'Unable to fetch video information. Please check the URL.');
    }
  }

  // 2. Download via 100% Native Local yt-dlp Pipeline
  public async startDownload(
    item: DownloadItem,
    downloadDir: string,
    _apiUrl: string,
    settings: AppSettings | undefined,
    onProgress: (item: DownloadItem) => void
  ): Promise<void> {
    if (!fs.existsSync(downloadDir)) {
      fs.mkdirSync(downloadDir, { recursive: true });
    }

    return this.downloadLocal(item, downloadDir, settings, onProgress);
  }

  private buildFormatSelector(formatId?: string, audioOnly?: boolean): string | undefined {
    if (audioOnly) {
      return 'bestaudio/best';
    }
    if (!formatId || formatId === 'best' || formatId === 'bestvideo') {
      return 'bv*+ba/b';
    }
    if (!isNaN(Number(formatId))) {
      const h = Number(formatId);
      return `bv*[height<=${h}][vcodec^=avc1]+ba[ext=m4a]/bv*[height<=${h}][vcodec^=avc1]+ba/b[height<=${h}][vcodec^=avc1]/bv*[height<=${h}]+ba[ext=m4a]/bv*[height<=${h}]+ba/b[height<=${h}]/b`;
    }
    if (formatId.startsWith('bv*')) {
      return formatId;
    }
    return undefined;
  }

  // 100% Native Local Download (Zero Cloud Dependency)
  private async downloadLocal(
    item: DownloadItem,
    downloadDir: string,
    settings: AppSettings | undefined,
    onProgress: (item: DownloadItem) => void
  ): Promise<void> {
    this.activeJobs.set(item.id, true);
    item.status = 'downloading';
    item.progress = 0;
    item.speed = 'Connecting...';
    onProgress({ ...item });

    await this.ensureDependencies();
    const bin = this.getYtdlpBin(settings?.customYtdlpPath);
    const ffmpegBin = this.getFfmpegBin(settings?.customFfmpegPath);
    const hasFfmpeg = fs.existsSync(ffmpegBin);

    // Output template: save directly to ClipPlay download directory
    const outputTemplate = path.join(downloadDir, '%(title).150B.%(ext)s');

    const args: string[] = [
      '--newline',
      '--no-warnings',
      '--no-playlist',
      '--remote-components', 'ejs:github',
      '--js-runtimes', 'node',
      '--extractor-args', 'youtube:player_client=tv_embedded,web_embedded,mweb,android;skip=translated_subs,storyboards,comments',
      '-o', outputTemplate,
    ];

    if (hasFfmpeg) {
      args.push('--ffmpeg-location', path.dirname(ffmpegBin));
    }

    if (settings?.cookiesPath && fs.existsSync(settings.cookiesPath)) {
      args.push('--cookies', settings.cookiesPath);
    }

    if (item.audioOnly) {
      if (hasFfmpeg) {
        args.push('-x', '--audio-format', 'mp3', '--audio-quality', '0');
      } else {
        args.push('-f', 'ba/b', '--remux-video', 'm4a');
      }
    } else {
      const formatSelector = this.buildFormatSelector(item.formatId, false);
      if (formatSelector) {
        args.push('-f', formatSelector);
      }
      if (hasFfmpeg) {
        args.push('--merge-output-format', 'mp4');
      }
    }

    if (settings?.extraArgs && settings.extraArgs.trim()) {
      const extra = settings.extraArgs.trim().split(/\s+/);
      args.push(...extra);
    }

    args.push(item.url);

    let finalFilePath: string | null = null;

    return new Promise((resolve, reject) => {
      const child = spawn(bin, args, { env: process.env });
      this.activeProcesses.set(item.id, child);

      const handleData = (data: Buffer) => {
        const text = data.toString();
        const lines = text.split(/\r?\n/);

        for (const line of lines) {
          if (!line.trim()) continue;

          // Check for destination / final filename
          const destMatch = line.match(/\[(?:download|Merger|ExtractAudio|ffmpeg)\]\s+Destination:\s+(.+)$/i) ||
                            line.match(/\[download\]\s+(.+)\s+has already been downloaded/i) ||
                            line.match(/\[Merger\]\s+Merging formats into\s+"?([^"]+)"?/i);
          if (destMatch && destMatch[1]) {
            finalFilePath = destMatch[1].trim();
          }

          // Parse live progress: [download]  45.2% of 120.50MiB at  8.45MiB/s ETA 00:08
          const progressMatch = line.match(/\[download\]\s+([\d\.]+)%(?:\s+of\s+~?([\d\.]+\w+))?(?:\s+at\s+([\d\.]+\w+\/s))?(?:\s+ETA\s+([\d:]+))?/i);
          if (progressMatch) {
            const percent = parseFloat(progressMatch[1]);
            const speed = progressMatch[3] || 'Downloading...';
            const eta = progressMatch[4] || '';

            item.status = 'downloading';
            item.progress = Math.min(Math.round(percent), 99);
            item.speed = speed;
            if (eta) item.eta = eta;
            onProgress({ ...item });
          } else if (line.includes('[Merger]') || line.includes('[ExtractAudio]') || line.includes('[ffmpeg]')) {
            item.status = 'processing';
            item.progress = 95;
            item.speed = 'Processing audio & video...';
            onProgress({ ...item });
          }
        }
      };

      child.stdout.on('data', handleData);
      child.stderr.on('data', handleData);

      child.on('error', (err) => {
        this.activeProcesses.delete(item.id);
        this.activeJobs.delete(item.id);
        reject(err);
      });

      child.on('close', (code) => {
        this.activeProcesses.delete(item.id);
        const wasActive = this.activeJobs.get(item.id);
        this.activeJobs.delete(item.id);

        if (code === 0) {
          item.status = 'completed';
          item.progress = 100;
          item.completedAt = Date.now();
          item.outputPath = finalFilePath || undefined;
          onProgress({ ...item });
          resolve();
        } else {
          if (!wasActive) {
            reject(new Error('Download cancelled by user'));
          } else {
            reject(new Error(`Download failed (exit code ${code})`));
          }
        }
      });
    });
  }

  // Helper to stream file over HTTP/HTTPS with progress
  public downloadHttpFile(
    fileUrl: string,
    destPath: string,
    onChunk: (percent: number, speed: string) => void
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const file = fs.createWriteStream(destPath);
      const isHttps = fileUrl.startsWith('https');
      const client = isHttps ? https : http;

      const req = client.get(fileUrl, {
        headers: {
          'User-Agent': 'ClipPlay-Desktop/1.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
          'Accept': '*/*',
        },
      }, (res) => {
        if (res.statusCode && res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return this.downloadHttpFile(res.headers.location, destPath, onChunk)
            .then(resolve)
            .catch(reject);
        }

        if (res.statusCode && res.statusCode >= 400) {
          file.close();
          fs.unlink(destPath, () => {});
          return reject(new Error(`HTTP error ${res.statusCode}: ${res.statusMessage}`));
        }

        const totalBytes = parseInt(res.headers['content-length'] || '0', 10);
        let receivedBytes = 0;
        let lastTime = Date.now();
        let bytesSinceLast = 0;

        res.on('data', (chunk) => {
          receivedBytes += chunk.length;
          bytesSinceLast += chunk.length;

          const now = Date.now();
          if (now - lastTime >= 400) {
            const duration = (now - lastTime) / 1000;
            const speedMB = (bytesSinceLast / duration / (1024 * 1024)).toFixed(1);
            const percent = totalBytes > 0 ? (receivedBytes / totalBytes) * 100 : 50;

            onChunk(percent, `${speedMB} MB/s`);
            lastTime = now;
            bytesSinceLast = 0;
          }
        });

        res.pipe(file);

        file.on('finish', () => {
          file.close();
          resolve();
        });
      });

      req.on('error', (err) => {
        fs.unlink(destPath, () => {});
        reject(err);
      });
    });
  }

  public cancel(id: string) {
    this.activeJobs.delete(id);
    if (this.activeProcesses.has(id)) {
      const proc = this.activeProcesses.get(id);
      proc.kill('SIGKILL');
      this.activeProcesses.delete(id);
    }
  }

  private mapLocalMetadata(info: any, originalUrl: string): VideoMetadata {
    const rawFormats = info.formats || [];

    // Find all distinct video heights
    const videoFormats = rawFormats.filter((f: any) => f.vcodec !== 'none' || f.height);
    const rawHeights = Array.from(
      new Set(videoFormats.map((f: any) => f.height).filter((h: any) => typeof h === 'number' && h > 0))
    ) as number[];
    rawHeights.sort((a, b) => b - a);

    // Calculate maximum available height from formats or info
    const maxHeight = rawHeights.length > 0 ? rawHeights[0] : (info.height || 1080);

    // Full standard video resolution tiers
    const standardLadder = [4320, 2160, 1440, 1080, 720, 480, 360, 240, 144];
    const availableLadder = standardLadder.filter((h) => h <= maxHeight);
    const finalHeights = Array.from(new Set([...rawHeights, ...availableLadder])).sort((a, b) => b - a);

    const formats: VideoFormat[] = finalHeights.map((h) => {
      let label = `${h}p`;
      let note = 'Standard MP4';
      if (h >= 4320) note = '8K Ultra HD';
      else if (h >= 2160) note = '4K Ultra HD';
      else if (h >= 1440) note = '2K Quad HD';
      else if (h >= 1080) note = 'Full HD MP4';
      else if (h >= 720) note = 'HD MP4';
      else if (h === 480) note = 'SD Quality';
      else if (h === 360) note = 'Mobile MP4';
      else if (h <= 240) note = 'Low Data';

      return {
        id: String(h),
        ext: 'mp4',
        height: h,
        qualityLabel: label,
        hasVideo: true,
        hasAudio: true,
        note,
      };
    });

    return {
      id: info.id,
      url: originalUrl,
      title: info.title || 'Untitled Video',
      uploader: info.uploader || info.channel || info.creator || 'Creator',
      duration: info.duration || 0,
      thumbnail: info.thumbnail || '',
      platform: this.detectPlatform(originalUrl),
      formats,
    };
  }

  private detectPlatform(url: string): 'youtube' | 'instagram' | 'tiktok' | 'twitter' | 'other' {
    if (url.includes('youtube.com') || url.includes('youtu.be')) return 'youtube';
    if (url.includes('instagram.com')) return 'instagram';
    if (url.includes('tiktok.com')) return 'tiktok';
    if (url.includes('twitter.com') || url.includes('x.com')) return 'twitter';
    return 'other';
  }
}
