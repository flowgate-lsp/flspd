import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatSats(sats: number | string): string {
  const n = typeof sats === 'string' ? parseInt(sats, 10) : sats;
  return new Intl.NumberFormat().format(n) + ' sats';
}
