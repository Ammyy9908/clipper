import React, { useState } from 'react';
import { WifiOff, Folder, ArrowRight, RefreshCw, Film } from 'lucide-react';

interface OfflineEmptyStateProps {
  onSwitchToDownloads: () => void;
  onShowInFolder: () => void;
  savedCount: number;
}

export const OfflineEmptyState: React.FC<OfflineEmptyStateProps> = ({
  onSwitchToDownloads,
  onShowInFolder,
  savedCount,
}) => {
  const [isRetrying, setIsRetrying] = useState(false);

  const handleRetry = () => {
    setIsRetrying(true);
    setTimeout(() => {
      setIsRetrying(false);
    }, 1500);
  };

  return (
    <div className="w-full max-w-xl mx-auto my-8 p-8 rounded-3xl bg-[#171716] border border-[rgba(241,241,238,0.12)] shadow-2xl text-center flex flex-col items-center animate-in fade-in zoom-in-95 duration-200">
      {/* Offline Status Icon */}
      <div className="w-16 h-16 rounded-2xl bg-[#ff4a38]/15 border border-[#ff4a38]/30 flex items-center justify-center text-[#ff4a38] mb-5 shadow-lg shadow-[#ff4a38]/10 animate-pulse">
        <WifiOff className="w-8 h-8" />
      </div>

      <h3 className="text-xl font-semibold text-[#f1f1ee] tracking-tight">
        You're Currently Offline
      </h3>

      <p className="text-xs sm:text-sm text-[#b1b1aa] mt-2 max-w-md leading-relaxed">
        An internet connection is required to resolve links and download new media. All your downloaded clips remain fully accessible offline.
      </p>

      {/* Action Buttons */}
      <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
        <button
          onClick={onSwitchToDownloads}
          className="btn-primary px-5 py-2.5 text-xs font-semibold flex items-center gap-2"
        >
          <Film className="w-4 h-4" />
          <span>View Saved Downloads ({savedCount})</span>
          <ArrowRight className="w-3.5 h-3.5" />
        </button>

        <button
          onClick={onShowInFolder}
          className="glass-button px-4 py-2.5 rounded-full text-xs font-semibold text-white flex items-center gap-2"
        >
          <Folder className="w-4 h-4 text-[#ff4a38]" />
          <span>Open in Finder</span>
        </button>

        <button
          onClick={handleRetry}
          disabled={isRetrying}
          className="glass-button px-3.5 py-2.5 rounded-full text-xs font-medium text-[#b1b1aa] hover:text-white flex items-center gap-1.5"
          title="Check connection again"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isRetrying ? 'animate-spin text-[#ff4a38]' : ''}`} />
          <span>{isRetrying ? 'Checking...' : 'Check Connection'}</span>
        </button>
      </div>

      <div className="mt-6 pt-5 border-t border-[rgba(241,241,238,0.06)] w-full flex items-center justify-center gap-2 text-[11px] text-[#8d8d86]">
        <span className="w-2 h-2 rounded-full bg-amber-400" />
        <span>Offline library mode active • Local media player ready</span>
      </div>
    </div>
  );
};
export default OfflineEmptyState;
