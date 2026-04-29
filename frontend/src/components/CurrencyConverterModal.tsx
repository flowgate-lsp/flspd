import React, { useState } from 'react';
import { GlassCard } from '../components/GlassCard';
import { formatAmount, formatInput, DisplayUnit } from '../lib/utils';

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

  if (!isOpen) return null;

  const handleConfirm = () => {
    const val = parseInt(inputValue.replace(/,/g, ''), 10);
    if (!isNaN(val)) {
      onConfirm(val);
      onClose();
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <GlassCard className="w-full max-w-md space-y-6 p-6">
        <h3 className="text-sm font-bold text-white uppercase italic tracking-tighter">Edit {label}</h3>
        <p className="text-[10px] text-on-surface-variant font-mono">{explanation}</p>
        <input
          type="text"
          value={formatInput(inputValue)}
          onChange={(e) => setInputValue(e.target.value.replace(/,/g, ''))}
          className="w-full bg-black/40 border border-white/10 rounded-lg p-3 text-white font-mono"
        />
        <div className="flex gap-2">
          {['AUTO', 'FLC', 'LOKI'].map((u) => (
            <button
              key={u}
              onClick={() => setUnit(u as DisplayUnit)}
              className={`px-4 py-2 rounded-lg text-xs font-bold ${unit === u ? 'bg-primary text-black' : 'bg-white/5 text-white'}`}
            >
              {u}
            </button>
          ))}
        </div>
        <p className="text-[10px] text-on-surface-variant font-mono">
          Preview: {formatAmount(inputValue, unit, sourceUnit)}
        </p>
        <div className="flex justify-end gap-3">
          <button onClick={onClose} className="text-xs text-white/50 hover:text-white">Cancel</button>
          <button onClick={handleConfirm} className="px-4 py-2 bg-primary text-black text-xs font-bold rounded-lg">Confirm</button>
        </div>
      </GlassCard>
    </div>
  );
};
