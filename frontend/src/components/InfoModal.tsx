import React from 'react';
import { GlassCard } from '../components/GlassCard';

interface InfoModalProps {
  isOpen: boolean;
  onClose: () => void;
  label: string;
  explanation: string;
}

export const InfoModal: React.FC<InfoModalProps> = ({ isOpen, onClose, label, explanation }) => {
  React.useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handleEsc);
    return () => window.removeEventListener('keydown', handleEsc);
  }, [onClose]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[200] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <GlassCard className="w-full max-w-lg p-8 space-y-6">
        <h3 className="text-lg font-bold text-white uppercase italic tracking-tighter">{label}</h3>
        <p className="text-sm text-white/80 leading-relaxed font-mono">{explanation}</p>
        <div className="flex justify-end pt-4">
          <button onClick={onClose} className="px-6 py-2 bg-primary text-black text-xs font-bold rounded-lg hover:brightness-110 transition-all">OK</button>
        </div>
      </GlassCard>
    </div>
  );
};
