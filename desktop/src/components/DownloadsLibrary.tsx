import React, { useState } from 'react';
import {
  Folder,
  Play,
  Search,
  CheckCircle2,
  AlertCircle,
  Music,
  Video,
  ExternalLink,
  Trash2,
  Film,
  Sparkles,
} from 'lucide-react';
import type { DownloadItem } from '../types';

interface DownloadsLibraryProps {
  items: DownloadItem[];
  downloadDir: string;
  onOpenFile: (filePath: string) => void;
  onShowInFolder: (filePath: string) => void;
  onClearHistory: () => void;
  onSwitchToDownloader: () => void;
}

export const DownloadsLibrary: React.FC<DownloadsLibraryProps> = ({
  items,
  downloadDir,
  onOpenFile,
  onShowInFolder,
  onClearHistory,
  onSwitchToDownloader,
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [filterType, setFilterType] = useState<'all' | 'video' | 'audio'>('all');

  const completed = items.filter(
    (i) => i.status === 'completed' || i.status === 'failed'
  );

  const filtered = completed.filter((item) => {
    const matchesSearch =
      !searchQuery.trim() ||
      item.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (item.outputPath && item.outputPath.toLowerCase().includes(searchQuery.toLowerCase()));

    if (!matchesSearch) return false;

    if (filterType === 'video') return !item.audioOnly;
    if (filterType === 'audio') return !!item.audioOnly;
    return true;
  });

  const handleOpenFolder = () => {
    if (downloadDir) {
      onShowInFolder(downloadDir);
    }
  };

  return (
    <div className="w-full max-w-3xl mx-auto flex flex-col gap-5 animate-in fade-in duration-200">
      {/* Top Bar: Search, Filters, and Open Folder Action */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 pb-2 border-b border-[rgba(241,241,238,0.08)]">
        {/* Search Bar */}
        <div className="relative flex-1 max-w-md">
          <Search className="w-3.5 h-3.5 absolute left-3.5 top-1/2 -translate-y-1/2 text-[#8d8d86]" />
          <input
            type="text"
            placeholder="Search saved videos and audio..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="glass-input w-full pl-9 pr-3 py-2 rounded-full text-xs text-[#f1f1ee] placeholder:text-[#8d8d86]"
          />
        </div>

        {/* Filter Pills & Actions */}
        <div className="flex items-center gap-2 flex-wrap sm:flex-nowrap">
          {/* Format Filter */}
          <div className="flex bg-[#211f1e] p-1 rounded-full border border-[rgba(241,241,238,0.06)] text-[11px]">
            <button
              onClick={() => setFilterType('all')}
              className={`px-3 py-1 rounded-full font-medium transition-all ${
                filterType === 'all'
                  ? 'bg-[#ff4a38] text-white shadow-sm'
                  : 'text-[#b1b1aa] hover:text-[#f1f1ee]'
              }`}
            >
              All ({completed.length})
            </button>
            <button
              onClick={() => setFilterType('video')}
              className={`px-3 py-1 rounded-full font-medium transition-all ${
                filterType === 'video'
                  ? 'bg-[#ff4a38] text-white shadow-sm'
                  : 'text-[#b1b1aa] hover:text-[#f1f1ee]'
              }`}
            >
              Videos
            </button>
            <button
              onClick={() => setFilterType('audio')}
              className={`px-3 py-1 rounded-full font-medium transition-all ${
                filterType === 'audio'
                  ? 'bg-[#ff4a38] text-white shadow-sm'
                  : 'text-[#b1b1aa] hover:text-[#f1f1ee]'
              }`}
            >
              Audio
            </button>
          </div>

          {/* Open Folder in Finder */}
          <button
            onClick={handleOpenFolder}
            className="glass-button px-3.5 py-1.5 rounded-full text-xs font-semibold text-white flex items-center gap-1.5 shrink-0"
            title="Open ~/Downloads/ClipPlay in Finder"
          >
            <Folder className="w-3.5 h-3.5 text-[#ff4a38]" />
            <span className="hidden sm:inline">Open Folder</span>
          </button>
        </div>
      </div>

      {/* Content List */}
      {filtered.length === 0 ? (
        <div className="py-16 text-center text-[#8d8d86]">
          <div className="w-14 h-14 rounded-2xl bg-[#171716] border border-[rgba(241,241,238,0.08)] mx-auto flex items-center justify-center mb-3 text-[#8d8d86]">
            <Film className="w-7 h-7" />
          </div>
          <h3 className="text-sm font-semibold text-[#f1f1ee]">
            {searchQuery.trim() ? 'No matching downloads' : 'No downloads yet'}
          </h3>
          <p className="text-xs text-[#8d8d86] mt-1 max-w-sm mx-auto">
            {searchQuery.trim()
              ? `No items found matching "${searchQuery}". Try a different search.`
              : 'All your downloaded videos and audio tracks will be stored and playable offline right here.'}
          </p>

          {!searchQuery.trim() && (
            <button
              onClick={onSwitchToDownloader}
              className="mt-4 btn-primary px-5 py-2 text-xs font-semibold inline-flex items-center gap-1.5"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span>Download a video</span>
            </button>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-2.5">
          <div className="flex items-center justify-between px-1 text-[11px] text-[#8d8d86]">
            <span>Showing {filtered.length} saved files</span>
            {completed.length > 0 && (
              <button
                onClick={onClearHistory}
                className="text-[#8d8d86] hover:text-[#ff4a38] flex items-center gap-1 transition-colors"
              >
                <Trash2 className="w-3 h-3" />
                <span>Clear History</span>
              </button>
            )}
          </div>

          {filtered.map((item) => {
            const isCompleted = item.status === 'completed';
            const dateStr = item.createdAt
              ? new Date(item.createdAt).toLocaleDateString(undefined, {
                  month: 'short',
                  day: 'numeric',
                  hour: '2-digit',
                  minute: '2-digit',
                })
              : '';

            return (
              <div
                key={item.id}
                className="bg-[#171716] hover:bg-[#211f1e] rounded-2xl p-3.5 border border-[rgba(241,241,238,0.08)] flex items-center justify-between gap-3.5 transition-all group shadow-sm hover:border-[rgba(241,241,238,0.15)]"
              >
                {/* Left: Thumbnail & Info */}
                <div className="flex items-center gap-3.5 min-w-0 flex-1">
                  <div className="w-12 h-12 rounded-xl bg-black/40 overflow-hidden border border-[rgba(241,241,238,0.08)] flex-shrink-0 relative group-hover:scale-105 transition-transform">
                    {item.thumbnail ? (
                      <img
                        src={item.thumbnail}
                        alt=""
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center text-[#8d8d86]">
                        {item.audioOnly ? (
                          <Music className="w-5 h-5 text-[#ff4a38]" />
                        ) : (
                          <Video className="w-5 h-5 text-[#ff4a38]" />
                        )}
                      </div>
                    )}
                    {isCompleted && item.outputPath && (
                      <div
                        onClick={() => onOpenFile(item.outputPath!)}
                        className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 flex items-center justify-center cursor-pointer transition-opacity"
                        title="Play file"
                      >
                        <Play className="w-5 h-5 text-white fill-white" />
                      </div>
                    )}
                  </div>

                  <div className="min-w-0 flex-1">
                    <h4
                      className="text-xs font-semibold text-[#f1f1ee] truncate cursor-pointer hover:text-[#ff4a38] transition-colors"
                      title={item.title}
                      onClick={() => isCompleted && item.outputPath && onOpenFile(item.outputPath)}
                    >
                      {item.title}
                    </h4>

                    <div className="flex items-center gap-2 mt-1 text-[11px] text-[#8d8d86] flex-wrap">
                      <span className="flex items-center gap-1">
                        {isCompleted ? (
                          <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                        ) : (
                          <AlertCircle className="w-3 h-3 text-[#ff4a38]" />
                        )}
                        <span className={isCompleted ? 'text-emerald-400 font-medium' : 'text-[#ff4a38]'}>
                          {isCompleted ? (item.audioOnly ? 'MP3 Audio' : 'MP4 Video') : 'Failed'}
                        </span>
                      </span>

                      {item.quality && (
                        <>
                          <span>•</span>
                          <span className="text-[#b1b1aa]">{item.quality}</span>
                        </>
                      )}

                      {dateStr && (
                        <>
                          <span>•</span>
                          <span>{dateStr}</span>
                        </>
                      )}
                    </div>
                  </div>
                </div>

                {/* Right: Quick Action Buttons */}
                {isCompleted && item.outputPath && (
                  <div className="flex items-center gap-1.5 flex-shrink-0">
                    <button
                      onClick={() => onOpenFile(item.outputPath!)}
                      className="btn-primary px-3 py-1.5 text-xs font-semibold flex items-center gap-1 shadow-sm"
                      title="Play / Open with Default Player"
                    >
                      <Play className="w-3.5 h-3.5 fill-white" />
                      <span className="hidden sm:inline">Play</span>
                    </button>

                    <button
                      onClick={() => onShowInFolder(item.outputPath!)}
                      className="p-2 text-[#b1b1aa] hover:text-[#f1f1ee] hover:bg-[#2b2928] rounded-xl transition-colors border border-[rgba(241,241,238,0.06)]"
                      title="Reveal in Finder / Explorer"
                    >
                      <Folder className="w-4 h-4" />
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
export default DownloadsLibrary;
