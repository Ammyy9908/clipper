import React from 'react';
import { Sparkles, ArrowRight, X } from 'lucide-react';

interface ClipboardToastProps {
  url: string | null;
  onFetch: (url: string) => void;
  onDismiss: () => void;
}

export const ClipboardToast: React.FC<ClipboardToastProps> = ({
  url,
  onFetch,
  onDismiss,
}) => {
  if (!url) return null;

  return (
    <div className="fixed bottom-5 right-5 z-50 animate-in slide-in-from-bottom-5 fade-in duration-300">
      <div className="p-3.5 rounded-2xl shadow-2xl border border-[rgba(241,241,238,0.15)] flex items-center gap-3 max-w-md bg-[#171716] backdrop-blur-2xl">
        <div className="w-8 h-8 rounded-xl bg-[#ff4a38]/15 border border-[#ff4a38]/30 flex items-center justify-center text-[#ff4a38] flex-shrink-0">
          <Sparkles className="w-4 h-4" />
        </div>

        <div className="flex-1 min-w-0">
          <span className="text-[10px] font-bold uppercase tracking-wider text-[#ff4a38] block">
            Media Link in Clipboard
          </span>
          <p className="text-xs text-[#f1f1ee] truncate font-medium mt-0.5">
            {url}
          </p>
        </div>

        <div className="flex items-center gap-1.5 flex-shrink-0">
          <button
            onClick={() => onFetch(url)}
            className="btn-primary px-3.5 py-1.5 text-xs font-semibold flex items-center gap-1"
          >
            <span>Fetch</span>
            <ArrowRight className="w-3 h-3" />
          </button>
          <button
            onClick={onDismiss}
            className="p-1.5 text-[#8d8d86] hover:text-[#f1f1ee] hover:bg-white/10 rounded-lg transition-colors"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  );
};
export default ClipboardToast;
