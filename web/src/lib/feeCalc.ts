// Mirrors lsps.ComputeOpeningFee (lsps/server.go, F-5's unified fee
// function) and the rate_percent/min_loki -> permyriad/milliloki
// conversion in config/tiered.go's applyTieredConfig, for an instant
// client-side preview only — the authoritative computation always happens
// server-side (ComputeOpeningFee itself, or `flspd config explain fees`).
// If the rounding rule changes there, update this too.
export function computeOpeningFeeLoki(
  paymentLoki: number,
  ratePercent: number,
  minLoki: number
): number {
  if (!isFinite(paymentLoki) || !isFinite(ratePercent) || !isFinite(minLoki)) {
    return 0;
  }
  const paymentSizeMloki = paymentLoki * 1000;
  // rate_percent -> permyriad (x100) -> ppm (x100) = x10,000 overall.
  const proportional = Math.round(ratePercent * 10_000);
  const minFeeMloki = minLoki * 1000;

  const prod = paymentSizeMloki * proportional;
  const openingFeeMloki = Math.max(Math.ceil(prod / 1_000_000), minFeeMloki);
  return Math.floor(openingFeeMloki / 1000);
}
