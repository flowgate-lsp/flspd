import { describe, it, expect } from 'vitest';
import { computeOpeningFeeLoki } from './feeCalc';

describe('computeOpeningFeeLoki', () => {
  it('applies the proportional rate when it exceeds the floor', () => {
    // 1% of 10,000 loki = 100 loki.
    expect(computeOpeningFeeLoki(10_000, 1, 0)).toBe(100);
  });

  it('applies the floor when the proportional fee is smaller', () => {
    expect(computeOpeningFeeLoki(1_000, 0.01, 5)).toBe(5);
  });

  it('rounds the proportional fee up to the nearest loki, not down', () => {
    // Same capacity/rate shape as the Go-side F-5 regression fixture
    // (lsps/handler_lsps1_coverage_test.go): the exact proportional value
    // is 2.9995 loki, which must round UP to 3, not truncate to 2.
    expect(computeOpeningFeeLoki(29_995, 0.01, 0.1)).toBe(3);
  });

  it('returns 0 for a zero payment size below the floor', () => {
    expect(computeOpeningFeeLoki(0, 1, 0)).toBe(0);
  });

  it('returns non-finite-safe 0 for NaN input', () => {
    expect(computeOpeningFeeLoki(NaN, 1, 0)).toBe(0);
  });
});
