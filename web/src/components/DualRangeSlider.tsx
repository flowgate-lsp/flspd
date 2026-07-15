import React from 'react';

interface DualRangeSliderProps {
  min: number;
  max: number;
  step?: number;
  valueMin: number;
  valueMax: number;
  onChangeMin: (v: number) => void;
  onChangeMax: (v: number) => void;
  formatValue?: (v: number) => string;
}

// Two overlapping native <input type="range"> elements is the standard,
// dependency-free way to build a dual-thumb slider — no slider library is
// in package.json, and pulling one in for a single field isn't worth it.
// Both inputs share the same track visually; each is only interactive
// over its own half via pointer-events, so the thumbs don't fight for
// clicks in the middle of the range.
export const DualRangeSlider: React.FC<DualRangeSliderProps> = ({
  min, max, step = 1, valueMin, valueMax, onChangeMin, onChangeMax, formatValue,
}) => {
  const fmt = formatValue ?? ((v: number) => v.toLocaleString());
  const clampedMin = Math.min(valueMin, valueMax);
  const clampedMax = Math.max(valueMin, valueMax);
  const pctMin = max > min ? ((clampedMin - min) / (max - min)) * 100 : 0;
  const pctMax = max > min ? ((clampedMax - min) / (max - min)) * 100 : 100;

  return (
    <div className="space-y-4">
      <div className="flex justify-between text-[11px] font-mono text-secondary font-bold">
        <span>{fmt(clampedMin)}</span>
        <span>{fmt(clampedMax)}</span>
      </div>
      <div className="relative h-8 flex items-center">
        <div className="absolute w-full h-1.5 bg-white/5 rounded-full" />
        <div
          className="absolute h-1.5 bg-primary rounded-full"
          style={{ left: `${pctMin}%`, right: `${100 - pctMax}%` }}
        />
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={clampedMin}
          onChange={(e) => onChangeMin(Number(e.target.value))}
          className="range-thumb absolute w-full appearance-none bg-transparent pointer-events-none"
          style={{ zIndex: clampedMin > max - (max - min) / 2 ? 5 : 3 }}
        />
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={clampedMax}
          onChange={(e) => onChangeMax(Number(e.target.value))}
          className="range-thumb absolute w-full appearance-none bg-transparent pointer-events-none"
          style={{ zIndex: 4 }}
        />
      </div>
    </div>
  );
};
