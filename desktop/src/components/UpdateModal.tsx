import React from 'react';
import { Sparkles, Download, X, ArrowUpRight, Calendar, CheckCircle2 } from 'lucide-react';
import type { UpdateInfo } from '../types';

interface UpdateModalProps {
  isOpen: boolean;
  updateInfo: UpdateInfo | null;
  onClose: () => void;
}

export const UpdateModal: React.FC<UpdateModalProps> = ({
  isOpen,
  updateInfo,
  onClose,
}) => {
  if (!isOpen || !updateInfo || !updateInfo.hasUpdate) return null;

  const handleDownload = () => {
    if (updateInfo.downloadUrl) {
      if (window.electronAPI?.openExternal) {
        window.electronAPI.openExternal(updateInfo.downloadUrl);
      } else {
        window.open(updateInfo.downloadUrl, '_blank');
      }
    }
    if (!updateInfo.mandatory) {
      onClose();
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-md animate-in fade-in duration-200 select-none">
      <div className="w-full max-w-md rounded-3xl p-6 shadow-2xl border border-[rgba(241,241,238,0.12)] bg-[#171716] animate-in zoom-in-95 duration-200 flex flex-col gap-5 relative overflow-hidden">
        {/* Glow ambient background decoration */}
        <div className="absolute -top-16 -right-16 w-36 h-36 rounded-full bg-[#ff4a38]/15 blur-3xl pointer-events-none" />
        <div className="absolute -bottom-16 -left-16 w-36 h-36 rounded-full bg-[#ff4a38]/10 blur-3xl pointer-events-none" />

        {/* Header */}
        <div className="flex items-center justify-between pb-3 border-b border-[rgba(241,241,238,0.08)] relative z-10">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-2xl bg-gradient-to-tr from-[#ff4a38] to-[#ff7b63] p-0.5 shadow-lg shadow-[#ff4a38]/20 flex items-center justify-center text-white">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-bold text-[#f1f1ee]">New Update Available</h3>
                <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-[#ff4a38]/20 text-[#ff4a38] border border-[#ff4a38]/30">
                  v{updateInfo.latestVersion}
                </span>
              </div>
              <p className="text-[11px] text-[#8d8d86]">A fresh version of ClipPlay Desktop is ready</p>
            </div>
          </div>
          {!updateInfo.mandatory && (
            <button
              onClick={onClose}
              className="p-1.5 text-[#8d8d86] hover:text-[#f1f1ee] hover:bg-white/10 rounded-lg transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Version Compare Banner */}
        <div className="p-3.5 rounded-2xl bg-[#211f1e]/80 border border-[rgba(241,241,238,0.06)] flex items-center justify-between relative z-10">
          <div>
            <span className="text-[10px] uppercase font-bold tracking-wider text-[#8d8d86] block">
              Installed Version
            </span>
            <span className="text-xs font-mono font-semibold text-[#b1b1aa]">
              v{updateInfo.currentVersion}
            </span>
          </div>
          <div className="h-6 w-px bg-white/10" />
          <div>
            <span className="text-[10px] uppercase font-bold tracking-wider text-[#ff4a38] block">
              Latest Version
            </span>
            <span className="text-xs font-mono font-bold text-[#f1f1ee]">
              v{updateInfo.latestVersion}
            </span>
          </div>
          {updateInfo.releaseDate && (
            <>
              <div className="h-6 w-px bg-white/10" />
              <div className="flex items-center gap-1.5 text-[11px] text-[#8d8d86]">
                <Calendar className="w-3.5 h-3.5 text-[#8d8d86]" />
                <span>{updateInfo.releaseDate}</span>
              </div>
            </>
          )}
        </div>

        {/* Release Notes */}
        {updateInfo.notes && (
          <div className="space-y-1.5 relative z-10">
            <span className="text-xs font-semibold text-[#f1f1ee] block">
              What's New in This Release
            </span>
            <div className="p-3.5 rounded-2xl bg-[#111110] border border-[rgba(241,241,238,0.06)] text-xs text-[#b1b1aa] leading-relaxed max-h-36 overflow-y-auto font-sans">
              <div className="flex items-start gap-2">
                <CheckCircle2 className="w-4 h-4 text-emerald-400 mt-0.5 flex-shrink-0" />
                <p className="text-xs text-[#d1d1ca]">{updateInfo.notes}</p>
              </div>
            </div>
          </div>
        )}

        {/* Footer Actions */}
        <div className="pt-3 border-t border-[rgba(241,241,238,0.08)] flex items-center justify-between relative z-10">
          {!updateInfo.mandatory ? (
            <button
              onClick={onClose}
              className="px-4 py-2 rounded-xl text-xs font-semibold text-[#8d8d86] hover:text-[#f1f1ee] hover:bg-white/5 transition-colors"
            >
              Remind Me Later
            </button>
          ) : (
            <div className="text-[11px] text-amber-400 font-medium">
              * Required update to continue
            </div>
          )}

          <button
            onClick={handleDownload}
            className="btn-primary px-5 py-2.5 text-xs font-semibold flex items-center gap-2 shadow-lg shadow-[#ff4a38]/25 hover:shadow-[#ff4a38]/40 transition-all cursor-pointer"
          >
            <Download className="w-4 h-4" />
            <span>Download & Install</span>
            <ArrowUpRight className="w-3.5 h-3.5 opacity-70" />
          </button>
        </div>
      </div>
    </div>
  );
};
export default UpdateModal;
