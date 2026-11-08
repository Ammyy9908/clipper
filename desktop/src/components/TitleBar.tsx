import React from 'react';
import { Settings, Minus, X, WifiOff, Download, Film, Sparkles } from 'lucide-react';
import type { UpdateInfo } from '../types';

interface TitleBarProps {
  onOpenSettings: () => void;
  activeCount: number;
  isOnline: boolean;
  activeTab: 'downloader' | 'library';
  onTabChange: (tab: 'downloader' | 'library') => void;
  savedCount: number;
  updateAvailable?: UpdateInfo | null;
  onOpenUpdateModal?: () => void;
}

export const TitleBar: React.FC<TitleBarProps> = ({
  onOpenSettings,
  activeCount,
  isOnline,
  activeTab,
  onTabChange,
  savedCount,
  updateAvailable,
  onOpenUpdateModal,
}) => {
  const isMac = navigator.userAgent.includes('Mac');

  return (
    <header className="h-11 w-full bg-[#0f0f0e]/95 backdrop-blur-md border-b border-[rgba(241,241,238,0.08)] flex items-center justify-between px-3 drag-region select-none z-50">
      {/* Left side: Brand Logo & Navigation Tabs */}
      <div className="flex items-center gap-3">
        {isMac && <div className="w-16" />} {/* Spacing for macOS traffic lights */}
        <div className="flex items-center gap-2 mr-2">
          <img
            src="./mark.png"
            alt="clipPlay"
            className="w-5 h-5 rounded-[6px] shadow-sm object-cover"
          />
          <span className="text-[13px] font-semibold tracking-tight text-[#f1f1ee] flex items-center">
            clipPlay
          </span>
        </div>

        {/* Tab Switcher: Downloader vs Downloads */}
        <div className="no-drag flex items-center bg-[#211f1e] p-0.5 rounded-full border border-[rgba(241,241,238,0.06)]">
          <button
            onClick={() => onTabChange('downloader')}
            className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold transition-all ${
              activeTab === 'downloader'
                ? 'bg-[#ff4a38] text-white shadow-sm'
                : 'text-[#b1b1aa] hover:text-[#f1f1ee]'
            }`}
          >
            <Download className="w-3.5 h-3.5" />
            <span>Download</span>
          </button>

          <button
            onClick={() => onTabChange('library')}
            className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold transition-all ${
              activeTab === 'library'
                ? 'bg-[#ff4a38] text-white shadow-sm'
                : 'text-[#b1b1aa] hover:text-[#f1f1ee]'
            }`}
          >
            <Film className="w-3.5 h-3.5" />
            <span>Downloads</span>
            {savedCount > 0 && (
              <span
                className={`text-[10px] px-1.5 py-0.2 rounded-full font-mono font-bold ${
                  activeTab === 'library'
                    ? 'bg-white/20 text-white'
                    : 'bg-white/10 text-[#b1b1aa]'
                }`}
              >
                {savedCount}
              </span>
            )}
          </button>
        </div>
      </div>

      {/* Center / Status Indicator: Offline Mode */}
      <div className="no-drag flex items-center">
        {!isOnline && (
          <div
            className="flex items-center gap-1.5 px-3 py-0.5 rounded-full bg-amber-500/10 border border-amber-500/25 text-[11px] font-medium text-amber-300"
            title="No internet connection. Offline library is active."
          >
            <WifiOff className="w-3.5 h-3.5 text-amber-400 animate-pulse" />
            <span>Offline Mode</span>
          </div>
        )}
      </div>

      {/* Right: Active Count, Update Alert, Settings & Window Controls */}
      <div className="flex items-center gap-1 no-drag">
        {updateAvailable?.hasUpdate && (
          <button
            onClick={onOpenUpdateModal}
            className="flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-[#ff4a38]/20 border border-[#ff4a38]/40 text-[11px] font-semibold text-[#ff4a38] mr-1.5 hover:bg-[#ff4a38]/30 transition-all cursor-pointer shadow-sm animate-pulse"
            title={`Update v${updateAvailable.latestVersion} is available. Click to review & download.`}
          >
            <Sparkles className="w-3 h-3 text-[#ff4a38]" />
            <span>Update v{updateAvailable.latestVersion}</span>
          </button>
        )}

        {activeCount > 0 && (
          <div className="flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-[#ff4a38]/10 border border-[#ff4a38]/20 text-[11px] text-[#ff4a38] mr-2 animate-pulse">
            <span className="w-1.5 h-1.5 rounded-full bg-[#ff4a38]" />
            <span>{activeCount} downloading</span>
          </div>
        )}

        <button
          onClick={onOpenSettings}
          className="p-1.5 text-[#b1b1aa] hover:text-[#f1f1ee] hover:bg-white/5 rounded-md transition-colors"
          title="Settings"
        >
          <Settings className="w-3.5 h-3.5" />
        </button>

        {!isMac && (
          <div className="flex items-center ml-1 border-l border-white/5 pl-1">
            <button
              onClick={() => window.electronAPI?.minimize()}
              className="p-1.5 text-[#b1b1aa] hover:text-[#f1f1ee] hover:bg-white/5 rounded-md transition-colors"
            >
              <Minus className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => window.electronAPI?.close()}
              className="p-1.5 text-[#b1b1aa] hover:text-rose-400 hover:bg-rose-500/10 rounded-md transition-colors"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        )}
      </div>
    </header>
  );
};
export default TitleBar;

