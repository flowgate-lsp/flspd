import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUnit, DisplayUnit } from '../contexts/UnitContext';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const LOKI_PER_FLC = 100000;
export const MLOKI_PER_FLC = 100000000;

/**
 * Core formatting logic that handles both FLC and LOKI
 */
export function formatAmount(
  value: number | string | undefined | null, 
  unit: DisplayUnit, 
  sourceUnit: 'LOKI' | 'MLOKI' = 'LOKI'
): string {
  if (value === undefined || value === null) return 'No activity';
  const n = typeof value === 'string' ? parseInt(value, 10) : value;
  if (isNaN(n)) return 'No activity';
  if (n === 0) return unit === 'FLC' ? '0.00 FLC' : '0 LOKI';

  if (unit === 'FLC') {
    const divisor = sourceUnit === 'MLOKI' ? MLOKI_PER_FLC : LOKI_PER_FLC;
    const flc = n / divisor;
    return new Intl.NumberFormat(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: sourceUnit === 'MLOKI' ? 8 : 6
    }).format(flc) + ' FLC';
  } else {
    // Loki display
    const lokiValue = sourceUnit === 'MLOKI' ? n / 1000 : n;
    return new Intl.NumberFormat().format(lokiValue) + ' LOKI';
  }
}

/**
 * Hook for components to easily format amounts using the active unit preference.
 */
export function useFormatAmount() {
  const { unit } = useUnit();
  
  return {
    format: (value: number | string | undefined | null, sourceUnit: 'LOKI' | 'MLOKI' = 'LOKI') => 
      formatAmount(value, unit, sourceUnit),
    unit
  };
}

/**
 * Formats a value in Loki with thousand separators. (e.g. 12,500 LOKI)
 * Maintained for backward compatibility.
 */
export function formatLoki(loki: number | string | undefined | null): string {
  return formatAmount(loki, 'LOKI');
}

/**
 * Formats a value in FLC (e.g. 1.25 FLC)
 * Maintained for backward compatibility.
 */
export function formatFLC(loki: number | string | undefined | null): string {
  return formatAmount(loki, 'FLC');
}

/**
 * Smart formatter: uses FLC or LOKI based on global setting.
 * In the new design, "smart" simply means respecting the user's preference.
 */
export function formatSmart(loki: number | string | undefined | null): string {
  // Since we can't use hooks in a plain function, we'll try to get the unit from localStorage
  // as a fallback for legacy non-component calls.
  const saved = typeof window !== 'undefined' ? localStorage.getItem('flspd_unit') : 'FLC';
  const unit = (saved as DisplayUnit) || 'FLC';
  
  return formatAmount(loki, unit);
}
