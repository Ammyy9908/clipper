import React, { useEffect, useState } from 'react';
import {
  X,
  Folder,
  Bell,
  Clipboard,
  CheckCircle2,
  Settings as SettingsIcon,
  HardDrive,
  Info,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  RefreshCw,
  Sparkles,
} from 'lucide-react';
import type { AppSettings, SystemInfo, UpdateInfo } from '../types';

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  settings: AppSettings;
  onUpdateSettings: (newSettings: Partial<AppSettings>) => void;
  onShowUpdateModal?: (info: UpdateInfo) => void;
}

export const SettingsModal: React.FC<SettingsModalProps> = ({
  isOpen,
  onClose,
  settings,
  onUpdateSettings,
  onShowUpdateModal,
}) => {
  const [sysInfo, setSysInfo] = useState<SystemInfo | null>(null);
  const [showDetails, setShowDetails] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isCheckingUpdate, setIsCheckingUpdate] = useState(false);
  const [updateStatusText, setUpdateStatusText] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen && window.electronAPI?.getSystemInfo) {
      window.electronAPI.getSystemInfo().then(setSysInfo);
    }
    setUpdateStatusText(null);
  }, [isOpen]);


  if (!isOpen) return null;

  const handleSelectFolder = async () => {
    if (!window.electronAPI?.selectDownloadDir) return;
    const chosen = await window.electronAPI.selectDownloadDir();
    if (chosen) {
      onUpdateSettings({ downloadDir: chosen });
    }
  };

  const handleRevealBinary = () => {
    if (sysInfo?.ytdlpPath && window.electronAPI?.showInFolder) {
      window.electronAPI.showInFolder(sysInfo.ytdlpPath);
    }
  };

  const handleCheckUpdates = async () => {
    if (!window.electronAPI?.checkBinaries) return;
    setIsRefreshing(true);
    try {
      const info = await window.electronAPI.checkBinaries();
      setSysInfo(info);
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleCheckAppUpdate = async () => {
    if (!window.electronAPI?.checkForUpdates) return;
    setIsCheckingUpdate(true);
    setUpdateStatusText(null);
    try {
      const info = await window.electronAPI.checkForUpdates();
      if (info.hasUpdate) {
        setUpdateStatusText(`v${info.latestVersion} available!`);
        if (onShowUpdateModal) {
          onShowUpdateModal(info);
        }
      } else if (info.error) {
        setUpdateStatusText('Offline / Error');
      } else {
        setUpdateStatusText(`v${info.currentVersion} is latest`);
      }
    } catch {
      setUpdateStatusText('Check failed');
    } finally {
      setIsCheckingUpdate(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-md animate-in fade-in duration-200">
      <div className="w-full max-w-md rounded-3xl p-6 shadow-2xl border border-[rgba(241,241,238,0.12)] bg-[#171716] animate-in zoom-in-95 duration-200 flex flex-col gap-5">
        {/* Header */}
        <div className="flex items-center justify-between pb-3 border-b border-[rgba(241,241,238,0.08)]">
          <div className="flex items-center gap-2.5">
            <div className="w-7 h-7 rounded-xl bg-[#ff4a38]/10 border border-[#ff4a38]/20 flex items-center justify-center text-[#ff4a38]">
              <SettingsIcon className="w-4 h-4" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-[#f1f1ee]">Preferences</h3>
              <p className="text-[11px] text-[#8d8d86]">Customize your clipPlay experience</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-[#8d8d86] hover:text-[#f1f1ee] hover:bg-white/10 rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Content */}
        <div className="space-y-4">
          {/* Download Location */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-[#f1f1ee] flex items-center gap-1.5">
              <HardDrive className="w-3.5 h-3.5 text-[#ff4a38]" />
              <span>Download Location</span>
            </label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                readOnly
                value={settings.downloadDir || '~/Downloads/ClipPlay'}
                className="glass-input flex-1 px-3 py-2 rounded-xl text-xs text-[#b1b1aa] truncate font-mono text-[11px]"
              />
              <button
                onClick={handleSelectFolder}
                className="glass-button px-3.5 py-2 rounded-xl text-xs font-semibold text-white flex items-center gap-1.5 flex-shrink-0"
              >
                <Folder className="w-3.5 h-3.5 text-[#ff4a38]" />
                <span>Browse</span>
              </button>
            </div>
          </div>

          {/* Auto Clipboard */}
          <div className="flex items-center justify-between p-3.5 rounded-2xl bg-[#211f1e]/70 border border-[rgba(241,241,238,0.06)]">
            <div className="flex items-center gap-3">
              <Clipboard className="w-4 h-4 text-[#ff4a38]" />
              <div>
                <span className="text-xs font-semibold text-[#f1f1ee] block">
                  Auto-detect Copied Links
                </span>
                <span className="text-[10px] text-[#8d8d86]">
                  Prompt to download when a video link is copied
                </span>
              </div>
            </div>
            <input
              type="checkbox"
              checked={settings.autoClipboard}
              onChange={(e) => onUpdateSettings({ autoClipboard: e.target.checked })}
              className="w-4 h-4 rounded accent-[#ff4a38] cursor-pointer"
            />
          </div>

          {/* System Notifications */}
          <div className="flex items-center justify-between p-3.5 rounded-2xl bg-[#211f1e]/70 border border-[rgba(241,241,238,0.06)]">
            <div className="flex items-center gap-3">
              <Bell className="w-4 h-4 text-[#ff4a38]" />
              <div>
                <span className="text-xs font-semibold text-[#f1f1ee] block">
                  System Notifications
                </span>
                <span className="text-[10px] text-[#8d8d86]">
                  Notify when video or audio finishes saving
                </span>
              </div>
            </div>
            <input
              type="checkbox"
              checked={settings.notifyOnComplete}
              onChange={(e) => onUpdateSettings({ notifyOnComplete: e.target.checked })}
              className="w-4 h-4 rounded accent-[#ff4a38] cursor-pointer"
            />
          </div>

          {/* Auto Updates Toggle */}
          <div className="flex items-center justify-between p-3.5 rounded-2xl bg-[#211f1e]/70 border border-[rgba(241,241,238,0.06)]">
            <div className="flex items-center gap-3">
              <Sparkles className="w-4 h-4 text-[#ff4a38]" />
              <div>
                <span className="text-xs font-semibold text-[#f1f1ee] block">
                  Automatic Update Checks
                </span>
                <span className="text-[10px] text-[#8d8d86]">
                  Notify when a new version of ClipPlay is released
                </span>
              </div>
            </div>
            <input
              type="checkbox"
              checked={settings.autoCheckUpdates !== false}
              onChange={(e) => onUpdateSettings({ autoCheckUpdates: e.target.checked })}
              className="w-4 h-4 rounded accent-[#ff4a38] cursor-pointer"
            />
          </div>

          {/* System Status & App Version Info */}
          <div className="rounded-2xl bg-[#211f1e]/50 border border-[rgba(241,241,238,0.06)] overflow-hidden transition-all">
            <div className="p-3.5 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <Info className="w-4 h-4 text-[#8d8d86]" />
                <div>
                  <span className="text-xs font-semibold text-[#f1f1ee] block">
                    clipPlay Desktop
                  </span>
                  <span className="text-[10px] text-[#8d8d86]">
                    v{sysInfo?.version || '1.0.0'} • Native Local Processing
                  </span>
                </div>
              </div>
              
              <div className="flex items-center gap-2">
                {updateStatusText && (
                  <span className="text-[10px] font-medium text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full border border-emerald-500/20">
                    {updateStatusText}
                  </span>
                )}
                <button
                  onClick={handleCheckAppUpdate}
                  disabled={isCheckingUpdate}
                  className="px-2.5 py-1 rounded-lg bg-white/5 hover:bg-white/10 text-[11px] font-semibold text-[#f1f1ee] border border-white/10 flex items-center gap-1.5 transition-colors"
                >
                  <RefreshCw className={`w-3 h-3 ${isCheckingUpdate ? 'animate-spin text-[#ff4a38]' : 'text-[#8d8d86]'}`} />
                  <span>{isCheckingUpdate ? 'Checking...' : 'Check Updates'}</span>
                </button>
              </div>
            </div>

            {/* Expandable Technical Verification Details */}
            <div
              onClick={() => setShowDetails(!showDetails)}
              className="px-3.5 py-2 border-t border-[rgba(241,241,238,0.06)] flex items-center justify-between cursor-pointer hover:bg-white/5 transition-colors text-[11px] text-[#8d8d86]"
            >
              <span>Technical Engine Status</span>
              {showDetails ? (
                <ChevronUp className="w-3.5 h-3.5 text-[#8d8d86]" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5 text-[#8d8d86]" />
              )}
            </div>

            {showDetails && (
              <div className="px-3.5 pb-3.5 pt-2 border-t border-[rgba(241,241,238,0.06)] space-y-2 text-[11px] animate-in fade-in">
                <div className="flex items-center justify-between text-[#8d8d86]">
                  <span>Core Engine:</span>
                  <span className="text-[#f1f1ee] font-mono">
                    {sysInfo?.hasLocalYtdlp ? `Active (v${sysInfo.ytdlpVersion || 'Latest'})` : 'Auto-configuring...'}
                  </span>
                </div>

                <div className="flex items-center justify-between text-[#8d8d86]">
                  <span>Media Processor:</span>
                  <span className="text-[#f1f1ee] font-mono">
                    {sysInfo?.hasLocalFfmpeg ? 'Active (High-Speed)' : 'Built-in Converter'}
                  </span>
                </div>

                {sysInfo?.ytdlpPath && (
                  <div className="pt-2 flex items-center justify-between border-t border-[rgba(241,241,238,0.06)]">
                    <button
                      onClick={handleRevealBinary}
                      className="text-[10px] text-[#ff4a38] hover:underline flex items-center gap-1 font-medium"
                      title="Reveal installed engine file in Finder"
                    >
                      <ExternalLink className="w-3 h-3" />
                      <span>Reveal Components in Finder</span>
                    </button>

                    <button
                      onClick={handleCheckUpdates}
                      disabled={isRefreshing}
                      className="text-[10px] text-[#b1b1aa] hover:text-[#f1f1ee] flex items-center gap-1"
                    >
                      <RefreshCw className={`w-3 h-3 ${isRefreshing ? 'animate-spin text-[#ff4a38]' : ''}`} />
                      <span>{isRefreshing ? 'Checking...' : 'Re-verify'}</span>
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="pt-3 border-t border-[rgba(241,241,238,0.08)] flex justify-end">
          <button
            onClick={onClose}
            className="btn-primary px-6 py-2 text-xs font-semibold"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
};
export default SettingsModal;
