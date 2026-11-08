import React, { useState } from 'react';
import { Download, Music, Video, Clock, User, CheckCircle2, Sparkles, Volume2 } from 'lucide-react';
import type { VideoMetadata, EngineMode } from '../types';

interface VideoCardProps {
  metadata: VideoMetadata;
  engine?: EngineMode;
  onDownload: (formatId: string, audioOnly: boolean, title: string, thumbnail: string) => void;
  onClose?: () => void;
}

export const VideoCard: React.FC<VideoCardProps> = ({
  metadata,
  onDownload,
}) => {
  const [activeTab, setActiveTab] = useState<'video' | 'audio'>('video');
  const [selectedFormat, setSelectedFormat] = useState<string>('best');

  const formatDuration = (seconds: number) => {
    if (!seconds) return '00:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  };

  const videoFormats = metadata.formats.filter(f => f.hasVideo && f.height);
  const maxHeight = videoFormats.length > 0 ? Math.max(...videoFormats.map(f => f.height || 0)) : 1080;

  const handleDownload = () => {
    onDownload(
      selectedFormat,
      activeTab === 'audio',
      metadata.title,
      metadata.thumbnail
    );
  };

  return (
    <div className="w-full max-w-2xl mx-auto rounded-3xl p-5 shadow-2xl border border-[rgba(241,241,238,0.12)] bg-[#171716] animate-in fade-in zoom-in-95 duration-300">
      <div className="flex flex-col md:flex-row gap-4 items-start">
        {/* Thumbnail Preview */}
        <div className="relative w-full md:w-52 aspect-video rounded-2xl overflow-hidden bg-black/40 border border-[rgba(241,241,238,0.1)] flex-shrink-0 shadow-lg group">
          {metadata.thumbnail ? (
            <img
              src={metadata.thumbnail}
              alt={metadata.title}
              className="w-full h-full object-cover transition-transform duration-500 group-hover:scale-105"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center text-[#8d8d86]">
              <Video className="w-8 h-8 opacity-40" />
            </div>
          )}
          {metadata.duration > 0 && (
            <div className="absolute bottom-2 right-2 px-2 py-0.5 rounded-full bg-black/80 backdrop-blur-md text-[10px] font-mono font-medium text-white flex items-center gap-1 border border-white/10">
              <Clock className="w-2.5 h-2.5 text-[#ff4a38]" />
              <span>{formatDuration(metadata.duration)}</span>
            </div>
          )}
        </div>

        {/* Video Info */}
        <div className="flex-1 min-w-0 flex flex-col justify-between">
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <span className="px-2.5 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider bg-[#ff4a38]/10 text-[#ff4a38] border border-[#ff4a38]/20">
                {metadata.platform}
              </span>
              <span className="text-xs text-[#b1b1aa] flex items-center gap-1">
                <User className="w-3 h-3 text-[#8d8d86]" />
                <span className="truncate">{metadata.uploader}</span>
              </span>
            </div>

            <h3 className="text-sm font-medium text-[#f1f1ee] line-clamp-2 leading-snug" title={metadata.title}>
              {metadata.title}
            </h3>
          </div>

          {/* Video / Audio Tab selector */}
          <div className="mt-3 flex items-center gap-2">
            <div className="flex bg-[#211f1e] p-1 rounded-full border border-[rgba(241,241,238,0.06)]">
              <button
                onClick={() => {
                  setActiveTab('video');
                  setSelectedFormat('best');
                }}
                className={`flex items-center gap-1.5 px-3.5 py-1 rounded-full text-xs font-medium transition-all ${
                  activeTab === 'video'
                    ? 'bg-[#ff4a38] text-white shadow-sm'
                    : 'text-[#b1b1aa] hover:text-[#f1f1ee]'
                }`}
              >
                <Video className="w-3.5 h-3.5" />
                <span>Video MP4 ({maxHeight}p)</span>
              </button>
              <button
                onClick={() => {
                  setActiveTab('audio');
                  setSelectedFormat('bestaudio');
                }}
                className={`flex items-center gap-1.5 px-3.5 py-1 rounded-full text-xs font-medium transition-all ${
                  activeTab === 'audio'
                    ? 'bg-[#ff4a38] text-white shadow-sm'
                    : 'text-[#b1b1aa] hover:text-[#f1f1ee]'
                }`}
              >
                <Music className="w-3.5 h-3.5" />
                <span>Audio MP3 (320k)</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Format Options Grid */}
      <div className="mt-4 pt-4 border-t border-[rgba(241,241,238,0.08)]">
        <label className="text-[11px] font-semibold text-[#8d8d86] uppercase tracking-wider block mb-2">
          {activeTab === 'video' ? 'Select Video Quality' : 'Select Audio Format'}
        </label>

        {activeTab === 'video' ? (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2 max-h-48 overflow-y-auto pr-1">
            {/* Auto Best Quality */}
            <button
              onClick={() => setSelectedFormat('best')}
              className={`p-2.5 rounded-2xl text-left border transition-all ${
                selectedFormat === 'best'
                  ? 'bg-[#ff4a38]/15 border-[#ff4a38] text-[#f1f1ee] shadow-sm'
                  : 'bg-[#211f1e]/60 border-[rgba(241,241,238,0.06)] text-[#b1b1aa] hover:bg-[#211f1e]'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-[#f1f1ee] flex items-center gap-1">
                  <Sparkles className="w-3 h-3 text-[#ff4a38]" />
                  <span>Best Quality</span>
                </span>
                {selectedFormat === 'best' && <CheckCircle2 className="w-3.5 h-3.5 text-[#ff4a38]" />}
              </div>
              <span className="text-[10px] text-[#8d8d86] block mt-0.5">
                Auto {maxHeight >= 2160 ? '4K Ultra HD' : `${maxHeight}p`}
              </span>
            </button>

            {/* All Available Resolution Tiers */}
            {videoFormats.map((fmt) => {
              const h = fmt.height;
              const val = String(h);
              const label = `${h}p`;

              return (
                <button
                  key={fmt.id || h}
                  onClick={() => setSelectedFormat(val)}
                  className={`p-2.5 rounded-2xl text-left border transition-all ${
                    selectedFormat === val
                      ? 'bg-[#ff4a38]/15 border-[#ff4a38] text-[#f1f1ee] shadow-sm'
                      : 'bg-[#211f1e]/60 border-[rgba(241,241,238,0.06)] text-[#b1b1aa] hover:bg-[#211f1e]'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-[#f1f1ee]">{label}</span>
                    {selectedFormat === val && <CheckCircle2 className="w-3.5 h-3.5 text-[#ff4a38]" />}
                  </div>
                  <span className="text-[10px] text-[#8d8d86] block mt-0.5 truncate">
                    {fmt.note || (h && h >= 1080 ? 'Full HD' : 'Standard')}
                  </span>
                </button>
              );
            })}
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
            <button
              onClick={() => setSelectedFormat('bestaudio')}
              className={`p-3 rounded-2xl text-left border transition-all ${
                selectedFormat === 'bestaudio'
                  ? 'bg-[#ff4a38]/15 border-[#ff4a38] text-[#f1f1ee] shadow-sm'
                  : 'bg-[#211f1e]/60 border-[rgba(241,241,238,0.06)] text-[#b1b1aa] hover:bg-[#211f1e]'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-[#f1f1ee] flex items-center gap-1.5">
                  <Volume2 className="w-3.5 h-3.5 text-[#ff4a38]" />
                  <span>MP3 (320 kbps)</span>
                </span>
                {selectedFormat === 'bestaudio' && <CheckCircle2 className="w-3.5 h-3.5 text-[#ff4a38]" />}
              </div>
              <span className="text-[10px] text-[#8d8d86] block mt-1">High fidelity master audio</span>
            </button>

            <button
              onClick={() => setSelectedFormat('m4a')}
              className={`p-3 rounded-2xl text-left border transition-all ${
                selectedFormat === 'm4a'
                  ? 'bg-[#ff4a38]/15 border-[#ff4a38] text-[#f1f1ee] shadow-sm'
                  : 'bg-[#211f1e]/60 border-[rgba(241,241,238,0.06)] text-[#b1b1aa] hover:bg-[#211f1e]'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-[#f1f1ee] flex items-center gap-1.5">
                  <Music className="w-3.5 h-3.5 text-[#ff4a38]" />
                  <span>M4A / AAC</span>
                </span>
                {selectedFormat === 'm4a' && <CheckCircle2 className="w-3.5 h-3.5 text-[#ff4a38]" />}
              </div>
              <span className="text-[10px] text-[#8d8d86] block mt-1">Apple Music & iOS format</span>
            </button>

            <button
              onClick={() => setSelectedFormat('bestaudio')}
              className={`p-3 rounded-2xl text-left border transition-all ${
                selectedFormat === 'mp3_192'
                  ? 'bg-[#ff4a38]/15 border-[#ff4a38] text-[#f1f1ee] shadow-sm'
                  : 'bg-[#211f1e]/60 border-[rgba(241,241,238,0.06)] text-[#b1b1aa] hover:bg-[#211f1e]'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-[#f1f1ee]">MP3 (192 kbps)</span>
                {selectedFormat === 'mp3_192' && <CheckCircle2 className="w-3.5 h-3.5 text-[#ff4a38]" />}
              </div>
              <span className="text-[10px] text-[#8d8d86] block mt-1">Standard compact audio</span>
            </button>
          </div>
        )}

        {/* Download Action Button */}
        <div className="mt-4 flex items-center justify-between">
          <div className="text-[11px] text-[#8d8d86] flex items-center gap-1.5 font-medium">
            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
            <span className="text-[#b1b1aa]">Live Multiplexed Download</span>
          </div>

          <button
            onClick={handleDownload}
            className="btn-primary flex items-center gap-2 px-6 py-2.5 text-xs font-semibold"
          >
            <Download className="w-4 h-4" />
            <span>Download</span>
          </button>
        </div>
      </div>
    </div>
  );
};
export default VideoCard;
