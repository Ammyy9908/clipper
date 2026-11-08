import React from 'react';
import { Play, X, Music, Video, Zap, HardDrive } from 'lucide-react';
import type { DownloadItem } from '../types';

interface DownloadQueueProps {
  items: DownloadItem[];
  onCancel: (id: string) => void;
}

export const DownloadQueue: React.FC<DownloadQueueProps> = ({
  items,
  onCancel,
}) => {
  const activeItems = items.filter(
    (i) => i.status === 'downloading' || i.status === 'processing' || i.status === 'queued'
  );

  if (activeItems.length === 0) {
    return null;
  }

  return (
    <div className="w-full max-w-2xl mx-auto flex flex-col gap-4 animate-in fade-in duration-200">
      <div className="flex items-center justify-between px-1">
        <span className="text-[11px] font-semibold text-[#8d8d86] uppercase tracking-wider">
          Active Downloads ({activeItems.length})
        </span>
      </div>

      <div className="flex flex-col gap-3">
        {activeItems.map((item) => (
          <div
            key={item.id}
            className="rounded-2xl p-4 border border-[rgba(241,241,238,0.12)] bg-[#171716] shadow-xl relative overflow-hidden"
          >
            <div className="flex items-center gap-3.5">
              {/* Thumbnail / Icon */}
              <div className="w-14 h-14 rounded-xl bg-black/40 overflow-hidden border border-[rgba(241,241,238,0.1)] flex-shrink-0 relative">
                {item.thumbnail ? (
                  <img src={item.thumbnail} alt="" className="w-full h-full object-cover" />
                ) : (
                  <div className="w-full h-full flex items-center justify-center text-[#8d8d86]">
                    {item.audioOnly ? <Music className="w-6 h-6" /> : <Video className="w-6 h-6" />}
                  </div>
                )}
                <div className="absolute inset-0 bg-black/20" />
              </div>

              {/* Info & Progress */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2">
                  <h4 className="text-xs font-semibold text-[#f1f1ee] truncate" title={item.title}>
                    {item.title}
                  </h4>
                  <button
                    onClick={() => onCancel(item.id)}
                    className="p-1 text-[#8d8d86] hover:text-[#ff4a38] hover:bg-[#ff4a38]/10 rounded-lg transition-colors"
                    title="Cancel download"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>

                <div className="flex items-center gap-2 mt-1.5 text-[11px] text-[#8d8d86]">
                  <span className="flex items-center gap-1 font-mono text-[#ff4a38] font-semibold">
                    {item.status === 'processing' ? 'Processing...' : `${item.progress}%`}
                  </span>
                  {item.speed && (
                    <>
                      <span>•</span>
                      <span className="text-[#b1b1aa]">{item.speed}</span>
                    </>
                  )}
                  {item.eta && (
                    <>
                      <span>•</span>
                      <span className="text-[#b1b1aa]">ETA {item.eta}</span>
                    </>
                  )}
                </div>

                {/* Progress Bar */}
                <div className="w-full h-1.5 bg-[#211f1e] rounded-full overflow-hidden mt-2 relative">
                  <div
                    className="h-full bg-gradient-to-r from-[#ff4a38] to-[#ff7a50] rounded-full transition-all duration-300"
                    style={{ width: `${item.progress}%` }}
                  />
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
export default DownloadQueue;
