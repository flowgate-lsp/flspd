import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { formatAmount, formatInput, type DisplayUnit } from '../lib/utils';
import { motion, AnimatePresence } from 'framer-motion';
import { X, ArrowRightLeft, Check, Info } from 'lucide-react';
import { cn } from '../lib/utils';

interface CurrencyConverterModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (value: number) => void;
  currentValue: number;
  sourceUnit: 'LOKI' | 'MLOKI';
  label: string;
  explanation: string;
}

export const CurrencyConverterModal: React.FC<CurrencyConverterModalProps> = ({
  isOpen,
  onClose,
  onConfirm,
  currentValue,
  sourceUnit,
  label,
  explanation,
}) => {
  const [inputValue, setInputValue] = useState(currentValue.toString());
  const [unit, setUnit] = useState<DisplayUnit>('AUTO');

  const handleConfirm = () => {
    const val = parseInt(inputValue.replace(/,/g, ''), 10);
    if (!isNaN(val)) {
      onConfirm(val);
      onClose();
    }
  };

  const parsedValue = parseInt(inputValue.replace(/,/g, ''), 10) || 0;

  return (
    <AnimatePresence>
      {isOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 md:p-6">
          <motion.div 
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="absolute inset-0 bg-black/80 backdrop-blur-md"
          />
          
          <GlassCard 
            initial={{ opacity: 0, scale: 0.9, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.9, y: 20 }}
            transition={{ type: "spring", damping: 25, stiffness: 300 }}
            className="w-full max-w-xl p-0 overflow-hidden shadow-[0_32px_64px_rgba(0,0,0,0.5)] border-outline bg-surface-variant/95"
          >
            {/* Modal Header */}
            <div className="p-6 md:p-8 bg-surface-container-high/40 border-b border-outline-variant flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-2xl bg-primary/10 flex items-center justify-center text-primary">
                        <ArrowRightLeft className="w-6 h-6" />
                    </div>
                    <div>
                        <h3 className="text-xl font-black text-white uppercase italic tracking-tighter leading-none">Adjust Parameter</h3>
                        <p className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest mt-1">{label}</p>
                    </div>
                </div>
                <button onClick={onClose} className="p-2 hover:bg-white/5 rounded-xl transition-colors text-on-surface-variant hover:text-white">
                    <X className="w-6 h-6" />
                </button>
            </div>

            <div className="p-6 md:p-8 space-y-8">
                {/* Info Box */}
                <div className="flex gap-4 p-4 bg-primary/5 rounded-2xl border border-primary/10">
                    <Info className="w-5 h-5 text-primary shrink-0" />
                    <p className="text-xs text-on-surface-variant leading-relaxed font-medium">{explanation}</p>
                </div>

                {/* Input Section */}
                <div className="space-y-4">
                    <div className="flex justify-between items-end px-1">
                        <label className="text-[10px] font-black text-on-surface-variant uppercase tracking-widest">Raw Amount ({sourceUnit})</label>
                        <span className="text-[10px] font-mono text-primary font-bold">Base Unit</span>
                    </div>
                    <div className="relative group">
                        <input
                          type="text"
                          autoFocus
                          value={formatInput(inputValue)}
                          onChange={(e) => setInputValue(e.target.value.replace(/,/g, ''))}
                          className="w-full bg-surface-container-low/80 border border-outline rounded-2xl p-6 text-2xl md:text-3xl text-white font-mono focus:outline-none focus:border-primary/40 focus:ring-4 focus:ring-primary/5 transition-all selection:bg-primary/30"
                        />
                        <div className="absolute right-6 top-1/2 -translate-y-1/2 flex gap-2">
                             <button 
                                onClick={() => setInputValue('0')}
                                className="px-3 py-1.5 bg-white/5 hover:bg-white/10 rounded-lg text-[10px] font-black text-on-surface-variant hover:text-white transition-all uppercase tracking-widest"
                            >
                                Clear
                            </button>
                        </div>
                    </div>
                </div>

                {/* Conversion Preview */}
                <div className="space-y-4">
                    <div className="flex items-center justify-between px-1">
                        <label className="text-[10px] font-black text-on-surface-variant uppercase tracking-widest">Financial Representation</label>
                        <div className="flex gap-1 p-1 bg-surface-container-low/60 rounded-xl border border-outline-variant">
                            {['AUTO', 'FLC', 'LOKI'].map((u) => (
                                <button
                                    key={u}
                                    onClick={() => setUnit(u as DisplayUnit)}
                                    className={cn(
                                        "px-4 py-1.5 rounded-lg text-[9px] font-black tracking-widest transition-all",
                                        unit === u ? "bg-primary text-black" : "text-on-surface-variant hover:text-white"
                                    )}
                                >
                                    {u}
                                </button>
                            ))}
                        </div>
                    </div>
                    
                    <div className="p-6 bg-gradient-to-br from-secondary/10 to-transparent border border-secondary/20 rounded-[2rem] flex flex-col items-center justify-center gap-2 relative overflow-hidden group/preview">
                        <div className="absolute -right-10 -bottom-10 w-32 h-32 bg-secondary/5 blur-3xl rounded-full" />
                        <span className="text-[10px] font-black text-secondary/60 uppercase tracking-[0.3em]">Estimated Value</span>
                        <span className="text-3xl md:text-4xl font-black text-white tracking-tighter text-center break-all px-4">
                            {formatAmount(parsedValue, unit, sourceUnit)}
                        </span>
                        <div className="flex items-center gap-2 mt-2">
                            <div className="w-1.5 h-1.5 rounded-full bg-secondary animate-pulse" />
                            <span className="text-[10px] font-mono text-on-surface-variant uppercase tracking-widest">Live Conversion Active</span>
                        </div>
                    </div>
                </div>

                {/* Actions */}
                <div className="flex flex-col md:flex-row gap-4 pt-4">
                  <button
                    onClick={onClose}
                    className="flex-1 px-8 py-4 rounded-2xl bg-surface-container-high border border-outline text-white text-xs font-black uppercase tracking-widest hover:bg-white/10 transition-all active:scale-95"
                  >
                    Cancel
                  </button>
                  <button 
                    onClick={handleConfirm} 
                    className="flex-[2] flex items-center justify-center gap-3 px-8 py-4 bg-primary text-black text-xs font-black uppercase tracking-widest rounded-2xl hover:shadow-[0_8px_32px_rgba(0,186,255,0.3)] hover:brightness-110 transition-all active:scale-95 group"
                  >
                    <Check className="w-5 h-5 transition-transform group-hover:scale-125" />
                    Apply Changes
                  </button>
                </div>
            </div>
          </GlassCard>
        </div>
      )}
    </AnimatePresence>
  );
};
