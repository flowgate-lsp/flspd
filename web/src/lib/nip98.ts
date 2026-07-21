import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex } from '@noble/hashes/utils.js';

// Matches the backend's ComputePayloadHash (transport/auth/nostr.go):
// lowercase hex SHA-256 of the exact request body bytes.
//
// Deliberately not using window.crypto.subtle: SubtleCrypto is only
// available in secure contexts (HTTPS, or the literal hostname
// "localhost"), so it's undefined when the dev server is reached over
// plain HTTP through a custom hostname (e.g. via the Caddy dev proxy) --
// nostr-tools already depends on @noble/hashes for the same primitive with
// no such restriction, so reuse it instead of the browser API.
export function sha256Hex(body: string): string {
  return bytesToHex(sha256(new TextEncoder().encode(body)));
}
