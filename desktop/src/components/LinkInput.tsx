import React, { useState } from 'react';
import { ArrowRight, Clipboard, Loader2, Youtube, Instagram } from 'lucide-react';

interface LinkInputProps {
  onResolve: (url: string) => void;
  isLoading: boolean;
}

export const LinkInput: React.FC<LinkInputProps> = ({ onResolve, isLoading }) => {
  const [inputUrl, setInputUrl] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputUrl.trim() || isLoading) return;
    onResolve(inputUrl.trim());
  };

  const handlePaste = async () => {
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        setInputUrl(text.trim());
        onResolve(text.trim());
      }
    } catch (err) {
      console.error('Failed to read clipboard:', err);
    }
  };

  return (
    <div className="w-full max-w-2xl mx-auto flex flex-col items-center">
      <div className="w-full">
        <form
          onSubmit={handleSubmit}
          className="flex items-center gap-1.5 rounded-full border border-[rgba(241,241,238,0.12)] bg-[#171716] p-1.5 pl-5 shadow-2xl transition-all focus-within:border-[rgba(241,241,238,0.35)]"
        >
          <input
            type="text"
            value={inputUrl}
            onChange={(e) => setInputUrl(e.target.value)}
            placeholder="Paste a video link (YouTube, Instagram, TikTok, X)..."
            className="h-11 min-w-0 flex-1 bg-transparent text-sm text-[#f1f1ee] outline-none placeholder:text-[#8d8d86]"
            disabled={isLoading}
          />

          <div className="flex items-center gap-1.5 pr-1">
            <button
              type="button"
              onClick={handlePaste}
              title="Paste from clipboard"
              className="grid size-10 place-items-center rounded-full text-[#b1b1aa] hover:bg-[#211f1e] hover:text-[#f1f1ee] transition active:scale-95"
            >
              <Clipboard className="w-4 h-4" />
            </button>

            <button
              type="submit"
              disabled={!inputUrl.trim() || isLoading}
              className={`flex items-center gap-1.5 px-5 py-2.5 rounded-full text-xs font-semibold transition-all ${
                inputUrl.trim() && !isLoading
                  ? 'btn-primary'
                  : 'bg-[#211f1e] text-[#8d8d86] cursor-not-allowed opacity-60'
              }`}
            >
              {isLoading ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Reading...</span>
                </>
              ) : (
                <>
                  <span>Get video</span>
                  <ArrowRight className="w-3.5 h-3.5" />
                </>
              )}
            </button>
          </div>
        </form>
      </div>

      {/* Quick platform badges */}
      <div className="flex items-center gap-3 mt-3 text-[11px] text-[#8d8d86]">
        <span className="flex items-center gap-1 hover:text-[#f1f1ee] transition-colors">
          <Youtube className="w-3 h-3 text-[#ff4a38]" /> YouTube
        </span>
        <span>•</span>
        <span className="flex items-center gap-1 hover:text-[#f1f1ee] transition-colors">
          <Instagram className="w-3 h-3 text-[#ff4a38]" /> Reels & Stories
        </span>
        <span>•</span>
        <span className="hover:text-[#f1f1ee] transition-colors">TikTok</span>
        <span>•</span>
        <span className="hover:text-[#f1f1ee] transition-colors">Twitter / X</span>
        <span>•</span>
        <span className="hover:text-[#f1f1ee] transition-colors">MP3 Audio</span>
      </div>
    </div>
  );
};
export default LinkInput;
