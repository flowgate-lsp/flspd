import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUnit, type DisplayUnit } from '../contexts/UnitContext';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Corrected Conversion: 1 FLC = 100,000,000 Loki
export const LOKI_PER_FLC = 100000000;
export const MLOKI_PER_FLC = 100000000000;

/**
 * Core formatting logic that handles AUTO, FLC, and LOKI
 */
export function formatAmount(
  value: number | string | undefined | null, 
  preferredUnit: DisplayUnit, 
  sourceUnit: 'LOKI' | 'MLOKI' = 'LOKI'
): string {
  if (value === undefined || value === null) return 'No activity';
  const n = typeof value === 'string' ? parseInt(value, 10) : value;
  if (isNaN(n)) return 'No activity';
  if (n === 0) return preferredUnit === 'LOKI' ? '0 LOKI' : '0.00 FLC';

  // Normalize everything to Loki for logic
  const lokiValue = sourceUnit === 'MLOKI' ? n / 1000 : n;
  
  // Decide target unit
  let targetUnit: 'FLC' | 'LOKI' = 'FLC';
  if (preferredUnit === 'AUTO') {
    // Switch to FLC if >= 0.1 FLC (10,000,000 Loki) or adjust as desired. 
    // Using 1 FLC (100,000,000 Loki) as the threshold.
    targetUnit = Math.abs(lokiValue) >= LOKI_PER_FLC ? 'FLC' : 'LOKI';
  } else {
    targetUnit = preferredUnit as 'FLC' | 'LOKI';
  }

  if (targetUnit === 'FLC') {
    const flc = lokiValue / LOKI_PER_FLC;
    return new Intl.NumberFormat(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 8
    }).format(flc) + ' FLC';
  } else {
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
 */
export function formatLoki(loki: number | string | undefined | null): string {
  return formatAmount(loki, 'LOKI');
}

/**
 * Formats a value in FLC (e.g. 1.25 FLC)
 */
export function formatFLC(loki: number | string | undefined | null): string {
  return formatAmount(loki, 'FLC');
}

/**
 * Smart formatter: uses FLC or LOKI based on global setting.
 */
export function formatSmart(loki: number | string | undefined | null): string {
  const saved = typeof window !== 'undefined' ? localStorage.getItem('flspd_unit') : 'AUTO';
  const unit = (saved as DisplayUnit) || 'AUTO';
  
  return formatAmount(loki, unit);
}
