import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUnit, type DisplayUnit } from '../contexts/UnitContext';
export type { DisplayUnit };
export { useUnit };

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const LOKI_PER_FLC = 100000000;
export const MLOKI_PER_FLC = 100000000000;

export const numberFormatter = new Intl.NumberFormat('en-US', {
  maximumFractionDigits: 8,
});

export function formatAmount(
  value: number | string | undefined | null, 
  preferredUnit: DisplayUnit, 
  sourceUnit: 'LOKI' | 'MLOKI' = 'LOKI'
): string {
  if (value === undefined || value === null) return 'No activity';
  const n = typeof value === 'string' ? parseInt(value, 10) : value;
  if (isNaN(n)) return 'No activity';
  if (n === 0) return preferredUnit === 'LOKI' ? '0 LOKI' : '0.00 FLC';

  const lokiValue = sourceUnit === 'MLOKI' ? n / 1000 : n;
  
  let targetUnit: 'FLC' | 'LOKI' = 'FLC';
  if (preferredUnit === 'AUTO') {
    targetUnit = Math.abs(lokiValue) >= LOKI_PER_FLC ? 'FLC' : 'LOKI';
  } else {
    targetUnit = preferredUnit as 'FLC' | 'LOKI';
  }

  if (targetUnit === 'FLC') {
    const flc = lokiValue / LOKI_PER_FLC;
    return new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 8
    }).format(flc) + ' FLC';
  } else {
    return numberFormatter.format(lokiValue) + ' LOKI';
  }
}

export function useFormatAmount() {
  const { unit } = useUnit();
  
  return {
    format: (value: number | string | undefined | null, sourceUnit: 'LOKI' | 'MLOKI' = 'LOKI') => 
      formatAmount(value, unit, sourceUnit),
    unit
  };
}

export function formatInput(value: number | string | undefined | null): string {
  if (value === undefined || value === null) return '';
  const n = typeof value === 'string' ? parseInt(value, 10) : value;
  if (isNaN(n)) return '';
  return numberFormatter.format(n);
}

export function formatLoki(loki: number | string | undefined | null): string {
  return formatAmount(loki, 'LOKI');
}

export function formatFLC(loki: number | string | undefined | null): string {
  return formatAmount(loki, 'FLC');
}

export function formatSmart(loki: number | string | undefined | null): string {
  const saved = typeof window !== 'undefined' ? localStorage.getItem('flspd_unit') : 'AUTO';
  const unit = (saved as DisplayUnit) || 'AUTO';
  return formatAmount(loki, unit);
}
