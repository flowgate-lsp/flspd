// Order status -> visual tone. Semantic, not decorative: only COMPLETED is
// a "good" outcome; FAILED is deliberately the same amber tone as CREATED
// rather than alarm-red, because a FAILED LSPS1 order today usually means
// its hold invoice auto-cancelled and the client's payment already failed
// back on its own (see lsps.Lsps1SettlementService.cancelHeldOrder) -- not
// that funds are stuck. EXPIRED is its own inert/neutral tone: it never
// got paid, so there's nothing to reconcile, unlike FAILED.
export type StatusTone = 'good' | 'active' | 'warn' | 'neutral' | 'error';

export const ORDER_STATUS_TONE: Record<string, StatusTone> = {
  COMPLETED: 'good',
  PAID: 'active',
  CREATED: 'warn',
  FAILED: 'warn',
  EXPIRED: 'neutral',
};

// FAILED needs payment_state too: a FAILED order whose payment_state never
// reached REFUNDED means it was captured and never given back -- rare
// today, but worth flagging distinctly rather than filing it under the
// same calm amber as a normal auto-cancelled hold invoice.
export function orderStatusTone(state: string, paymentState?: string): StatusTone {
  if (state === 'FAILED' && paymentState && paymentState !== 'REFUNDED') return 'error';
  return ORDER_STATUS_TONE[state] || 'neutral';
}

export const ORDER_STATUS_PILL_CLASS: Record<StatusTone, string> = {
  good: "text-secondary border-secondary/30 bg-secondary/5",
  active: "text-primary border-primary/30 bg-primary/5",
  warn: "text-tertiary border-tertiary/30 bg-tertiary/5",
  neutral: "text-on-surface-variant border-outline bg-white/[0.02]",
  error: "text-error border-error/30 bg-error/5",
};

export const ORDER_STATUS_DOT_CLASS: Record<StatusTone, string> = {
  good: "bg-secondary shadow-[0_0_8px_rgba(0,253,135,0.6)]",
  active: "bg-primary shadow-[0_0_8px_rgba(0,186,255,0.6)]",
  warn: "bg-tertiary shadow-[0_0_8px_rgba(255,186,0,0.6)]",
  neutral: "bg-on-surface-variant shadow-[0_0_6px_rgba(154,159,174,0.4)]",
  error: "bg-error shadow-[0_0_8px_rgba(255,59,48,0.6)]",
};

export const orderStatusPillClass = (state: string, paymentState?: string) =>
  ORDER_STATUS_PILL_CLASS[orderStatusTone(state, paymentState)];
export const orderStatusDotClass = (state: string, paymentState?: string) =>
  ORDER_STATUS_DOT_CLASS[orderStatusTone(state, paymentState)];
