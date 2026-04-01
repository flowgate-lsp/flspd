import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatLoki(loki: number | string | undefined | null): string {
  const n = typeof loki === 'string' ? parseInt(loki, 10) : (loki ?? 0);
  const safeN = isNaN(n as number) ? 0 : n;
  return new Intl.NumberFormat().format(safeN as number) + ' loki';
}
