import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

const LOKI_PER_FLC = 100000;

/**
 * Formats a value in Loki with thousand separators. (e.g. 12,500 loki)
 */
export function formatLoki(loki: number | string | undefined | null): string {
  if (loki === undefined || loki === null) return 'No activity';
  const n = typeof loki === 'string' ? parseInt(loki, 10) : loki;
  if (isNaN(n)) return 'No activity';
  if (n === 0) return '0 loki';
  
  return new Intl.NumberFormat().format(n) + ' loki';
}

/**
 * Formats a value in FLC (e.g. 1.25 FLC)
 */
export function formatFLC(loki: number | string | undefined | null): string {
  if (loki === undefined || loki === null) return 'No activity';
  const n = typeof loki === 'string' ? parseInt(loki, 10) : loki;
  if (isNaN(n) || n === 0) return '0.00 FLC';

  const flc = n / LOKI_PER_FLC;
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 8
  }).format(flc) + ' FLC';
}

/**
 * Smart formatter: uses FLC for values >= 100,000 Loki, and Loki for smaller values.
 * If fallbackToLoki is true, it will use Loki for anything less than 1.0 FLC.
 */
export function formatSmart(loki: number | string | undefined | null, threshold: number = LOKI_PER_FLC): string {
  if (loki === undefined || loki === null) return 'No activity';
  const n = typeof loki === 'string' ? parseInt(loki, 10) : loki;
  if (isNaN(n)) return 'No activity';
  if (n === 0) return '0 loki';

  if (Math.abs(n) < threshold) {
    return formatLoki(n);
  }

  return formatFLC(n);
}
