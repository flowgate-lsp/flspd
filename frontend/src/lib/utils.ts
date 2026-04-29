import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUnit, DisplayUnit } from '../contexts/UnitContext';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const LOKI_PER_FLC = 100000000;
export const MLOKI_PER_FLC = 100000000000;

/**
 * International number formatter for consistent UI display (with spacing)
 */
export const numberFormatter = new Intl.NumberFormat('en-US', {
  maximumFractionDigits: 8,
});

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
  
  // Normalize to Loki
  const lokiValue = sourceUnit === 'MLOKI' ? n / 1000 : n;
  
  // Decide target unit
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

/**
 * Formatter specifically for inputs (clean numeric string)
 */
export function formatInput(value: number | string | undefined | null): string {
  if (value === undefined || value === null) return '';
  const n = typeof value === 'string' ? parseInt(value, 10) : value;
  if (isNaN(n)) return '';
  return numberFormatter.format(n);
}

// ... rest of file (hooks and legacy aliases) ...
