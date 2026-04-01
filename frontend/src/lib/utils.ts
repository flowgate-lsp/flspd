import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * Formats a value (in loki/sats) to a human readable FLC string.
 * Supports handling NaN, undefined, and zero values with a custom placeholder.
 */
export function formatLoki(loki: number | string | undefined | null): string {
  if (loki === undefined || loki === null) return 'No activity';
  
  const n = typeof loki === 'string' ? parseInt(loki, 10) : loki;
  
  if (isNaN(n) || n === 0) return 'No activity';

  // 100,000 loki = 1 FLC
  const flc = n / 100000;
  
  // If it's a very small amount, show it in loki, otherwise FLC
  if (flc < 0.01 && n > 0) {
    return new Intl.NumberFormat().format(n) + ' loki';
  }

  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 8
  }).format(flc) + ' FLC';
}
