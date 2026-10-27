// NIP-LSP announce block (nodes.yaml `announce`, T8-2) <-> the dashboard
// form. The server validates everything (platform/config/announce.go);
// this only shapes the form and gives early, local hints.

export const TITLE_MAX = 40;
export const CONTENT_MAX = 280;

export interface AnnounceConfig {
  enabled: boolean;
  title?: string;
  content?: string;
  website?: string;
  image?: string;
  operator?: { pubkey: string; relay?: string };
  languages?: string[];
  addrs?: string[];
}

export interface AnnounceForm {
  enabled: boolean;
  title: string;
  content: string;
  website: string;
  image: string;
  operatorPubkey: string;
  operatorRelay: string;
  /** Comma-separated ISO-639-1 codes, as typed. */
  languages: string;
  /** One host:port per line, as typed. */
  addrs: string;
}

export function toAnnounceForm(a: AnnounceConfig | undefined | null): AnnounceForm {
  return {
    enabled: !!a?.enabled,
    title: a?.title ?? '',
    content: a?.content ?? '',
    website: a?.website ?? '',
    image: a?.image ?? '',
    operatorPubkey: a?.operator?.pubkey ?? '',
    operatorRelay: a?.operator?.relay ?? '',
    languages: (a?.languages ?? []).join(', '),
    addrs: (a?.addrs ?? []).join('\n'),
  };
}

/** Characters as the server counts them (Unicode code points). */
export function charCount(s: string): number {
  return Array.from(s).length;
}

/**
 * Builds the config block. Addresses are kept exactly as typed and in
 * order; only empty lines are dropped. Returns undefined for an empty,
 * disabled form so a node that never used the feature keeps no block.
 */
export function fromAnnounceForm(f: AnnounceForm): AnnounceConfig | undefined {
  const addrs = f.addrs.split('\n').filter((l) => l !== '');
  const languages = f.languages.split(',').map((l) => l.trim()).filter((l) => l !== '');
  const out: AnnounceConfig = { enabled: f.enabled };
  if (f.title) out.title = f.title;
  if (f.content) out.content = f.content;
  if (f.website.trim()) out.website = f.website.trim();
  if (f.image.trim()) out.image = f.image.trim();
  if (f.operatorPubkey.trim()) {
    out.operator = { pubkey: f.operatorPubkey.trim() };
    if (f.operatorRelay.trim()) out.operator.relay = f.operatorRelay.trim();
  }
  if (languages.length) out.languages = languages;
  if (addrs.length) out.addrs = addrs;
  if (!out.enabled && Object.keys(out).length === 1) return undefined;
  return out;
}

/** Local hints shown before saving; the server's answer is authoritative. */
export function announceHints(f: AnnounceForm): string[] {
  const hints: string[] = [];
  if (charCount(f.title) > TITLE_MAX) hints.push(`Title is over ${TITLE_MAX} characters.`);
  if (/[\r\n]/.test(f.title)) hints.push('Title must be one line.');
  if (charCount(f.content) > CONTENT_MAX) hints.push(`Description is over ${CONTENT_MAX} characters.`);
  for (const [label, url] of [['Website', f.website], ['Logo', f.image]] as const) {
    if (url.trim() && !url.trim().startsWith('https://')) hints.push(`${label} must be an https:// URL.`);
  }
  if (f.operatorRelay.trim() && !f.operatorPubkey.trim()) hints.push('An operator relay needs an operator pubkey.');
  if (f.operatorPubkey.trim() && !f.operatorRelay.trim()) hints.push('An operator pubkey needs a relay (wss://).');
  if (f.enabled && f.addrs.split('\n').every((l) => l === '')) {
    hints.push('Add at least one address: announcing needs one.');
  }
  return hints;
}

// --- Announcement status (Pause/Resume/Unlist), stored by the server ---

export type AnnounceStatus = 'open' | 'paused' | 'closed';
export type AnnounceAction = 'pause' | 'resume' | 'unlist';

export interface AnnounceState {
  node_pubkey: string;
  enabled: boolean;
  status: AnnounceStatus;
  updated_at: string | null;
  relays: string[];
}

/**
 * The actions the server accepts from each status (core.NextAnnounceStatus):
 * pause only while open, resume while paused or unlisted (re-listing it),
 * unlist while not already unlisted.
 */
export function availableActions(status: AnnounceStatus): AnnounceAction[] {
  switch (status) {
    case 'open':
      return ['pause', 'unlist'];
    case 'paused':
      return ['resume', 'unlist'];
    case 'closed':
      return ['resume'];
  }
}

// The status is what the announcement tells wallets. It changes only the
// announcement: flspd still answers LSPS1 create_order and LSPS2 buy.
export const STATUS_LABEL: Record<AnnounceStatus, string> = {
  open: 'Open: announced as taking orders',
  paused: 'Paused: announced as taking no new orders',
  closed: 'Unlisted: announced as stopped selling',
};

export const STATUS_NOTE =
  'Announcement only: flspd still accepts LSPS1 orders and LSPS2 buys from wallets that reach it.';

export const ACTION_LABEL: Record<AnnounceAction, string> = {
  pause: 'Pause',
  resume: 'Resume',
  unlist: 'Unlist',
};
