import React, { useState, useEffect } from 'react';
import { Sparkles, CheckCircle2, AlertCircle, RefreshCw } from 'lucide-react';

interface SetupModalProps {
  isOpen: boolean;
  onComplete: () => void;
}

export const SetupModal: React.FC<SetupModalProps> = ({ isOpen, onComplete }) => {
  const [progress, setProgress] = useState(0);
  const [statusMessage, setStatusMessage] = useState('Initializing media engine...');
  const [isDone, setIsDone] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isRetrying, setIsRetrying] = useState(false);

  const startSetup = async () => {
    if (!window.electronAPI?.setupEngine) {
      onComplete();
      return;
    }

    setError(null);
    setIsRetrying(true);
    setProgress(5);
    setStatusMessage('Preparing download & media components...');

    try {
      const res = await window.electronAPI.setupEngine();
      if (res.success) {
        setProgress(100);
        setStatusMessage('Ready!');
        setIsDone(true);
        setTimeout(() => {
          onComplete();
        }, 800);
      } else {
        setError(res.error || 'Setup could not be completed. Please check your internet connection and try again.');
      }
    } catch (err: any) {
      setError(err.message || 'Setup could not be completed. Please check your internet connection.');
    } finally {
      setIsRetrying(false);
    }
  };

  useEffect(() => {
    if (!isOpen) return;

    if (window.electronAPI?.onSetupProgress) {
      const unsub = window.electronAPI.onSetupProgress(({ pct, msg }) => {
        setProgress(pct);
        if (msg) {
          setStatusMessage(msg);
        }
      });

      startSetup();

      return () => {
        unsub();
      };
    } else {
      startSetup();
    }
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-lg animate-in fade-in duration-300">
      <div className="w-full max-w-md rounded-3xl p-7 shadow-2xl border border-[rgba(241,241,238,0.12)] bg-[#171716] animate-in zoom-in-95 duration-200 flex flex-col items-center text-center gap-5">
        {/* Brand Icon with ambient glow */}
        <div className="relative">
          <div className="absolute -inset-2 rounded-2xl bg-[#ff4a38]/20 blur-lg animate-pulse" />
          <div className="relative w-14 h-14 rounded-2xl bg-gradient-to-tr from-[#ff4a38] to-[#ff7a6b] flex items-center justify-center text-white shadow-lg">
            {isDone ? (
              <CheckCircle2 className="w-7 h-7 animate-in zoom-in-75 duration-200" />
            ) : error ? (
              <AlertCircle className="w-7 h-7 text-white" />
            ) : (
              <Sparkles className="w-7 h-7 animate-bounce" />
            )}
          </div>
        </div>

        {/* Title and Subtitle */}
        <div className="space-y-1.5 max-w-sm">
          <h2 className="text-lg font-semibold text-[#f1f1ee] tracking-tight">
            {isDone ? 'Engine Ready!' : error ? 'Setup Interrupted' : 'Setting up clipPlay'}
          </h2>
          <p className="text-xs text-[#b1b1aa] leading-relaxed">
            {isDone
              ? 'All video and audio components are configured and ready.'
              : error
              ? error
              : 'Preparing high-speed video and audio download components for your device. This only takes a moment.'}
          </p>
        </div>

        {/* Progress Bar & Status */}
        {!error && (
          <div className="w-full space-y-2 pt-2">
            <div className="flex items-center justify-between text-[11px]">
              <span className="text-[#8d8d86] font-medium truncate max-w-[240px]">
                {statusMessage}
              </span>
              <span className="text-[#f1f1ee] font-mono font-semibold">
                {Math.round(progress)}%
              </span>
            </div>

            <div className="w-full h-2.5 rounded-full bg-[#211f1e] overflow-hidden p-0.5 border border-[rgba(241,241,238,0.06)]">
              <div
                className={`h-full rounded-full transition-all duration-300 ease-out ${
                  isDone
                    ? 'bg-emerald-500'
                    : 'bg-gradient-to-r from-[#ff4a38] to-[#ff7a6b]'
                }`}
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        )}

        {/* Error Retry CTA */}
        {error && (
          <div className="pt-2 w-full">
            <button
              onClick={startSetup}
              disabled={isRetrying}
              className="btn-primary w-full py-2.5 text-xs font-semibold flex items-center justify-center gap-2"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isRetrying ? 'animate-spin' : ''}`} />
              <span>{isRetrying ? 'Retrying...' : 'Retry Setup'}</span>
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
export default SetupModal;
