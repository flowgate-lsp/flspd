# Client-Side Handover: Nostr Notification Layer (LokiHub)

This document provides technical details for implementing the client-side (wallet) portion of the LSPS5 over Nostr notification system.

## Library Recommendation
The wallet should use the **`github.com/flowgate-lsp/nostr-lsps5`** library (Go) or implement the same protocol logic. This library handles the LSPS5-specific serialization and encryption.

## Protocol Specification

### 1. Nostr Event Details
- **Kind**: `31055` (LSPS5 Notification).
- **Encryption**: Currently uses **NIP-04** (Encrypted Direct Messages logic) but uses the dedicated Kind instead of Kind 4.
- **Tags**:
    - `["p", <recipient-pubkey>]`: The wallet's pubkey.
    - `["t", "lsps5"]`: Standardization tag for LSPS5 events.
- **Content**: A JSON-RPC 2.0 Notification object, encrypted via NIP-04.

### 2. Subscription Flow
To receive notifications over Nostr, the wallet must register its Nostr Pubkey with the LSP via the `lsps5.set_webhook` method using the `nostr+lsps5` URI scheme.

**Example `lsps5.set_webhook` params:**
```json
{
  "app_name": "LokiHub",
  "webhook": "nostr+lsps5://<hex-pubkey>?relay=wss://relay.damus.io&relay=wss://relay.nostr.info",
  "transport": "nostr"
}
```

This tells the LSP exactly which pubkey to encrypt for and which relays to publish to. The LSP will maintain a persistent connection to these relays.

### 3. Parsing Notifications
When the wallet receives a Kind 31055 event:
1. Verify the event signature.
2. Verify the `p` tag matches the wallet's pubkey.
3. Decrypt the `content` using the LSP's pubkey and the wallet's private key (NIP-04).
4. Unmarshal the resulting JSON-RPC 2.0 object.

**Example Notification Body (Decrypted):**
```json
{
  "jsonrpc": "2.0",
  "method": "lsps5.payment_incoming",
  "params": {
    "payment_hash": "..."
  }
}
```

## Integration Tips
- **Relays**: Ensure the relays specified in the URI are reliable and that the wallet maintains a subscription to them.
- **Background Processing**: Ensure the Nostr subscription remains active in the background to receive real-time "payment incoming" notifications.

## LSP Identity Verification

**Problem**: How does the wallet know that a received Nostr event is genuinely from the LSP and not from a malicious actor?

**Solution**: The wallet must know the LSP's Nostr Pubkey *before* subscribing.

### Flow:
1. **Connect to LSP** (Lightning/LSPS).
2. **Call `lsps2.get_info`** (or similar) → Response includes `nostr_pubkey` field.
3. **Store the LSP's Nostr Pubkey** locally.
4. **Subscribe to relays** for Kind 31055 events.
5. **When an event is received**:
   - Check: `event.PubKey == stored_lsp_nostr_pubkey`?
     - **YES**: Trust the event, proceed to decrypt.
     - **NO**: **Discard the event** (could be spam/attack).

### Implementation Note:
```go
// Pseudocode
if event.PubKey != lspNostrPubkey {
    log.Warn("Received event from unknown pubkey, ignoring")
    return
}
notification, err := lsps5.ParseNotificationEvent(event, lspNostrPubkey)
```

This binds the Nostr identity to the already-trusted Lightning identity.
