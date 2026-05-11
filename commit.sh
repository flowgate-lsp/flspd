#!/bin/bash
set -e

# 1. Nostr relay reconnection fix
git add notifications/transport_nostr.go
git commit -m "$(cat <<'EOF'
fix: recreate relay object on reconnect instead of reusing dead connection

go-nostr Relay is one-time-use; connectionContext is permanently cancelled
after close(). Calling relay.Connect() on a dead relay never recovers.
Replace with nostr.RelayConnect() to get a fresh object on each reconnect.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"

# 2. LSPS5 notification improvements
git add lsps/handler_lsps5.go notifications/notification_service.go notifications/transport_https.go
git commit -m "$(cat <<'EOF'
feat: add Info-level logs for LSPS5 webhook registration and notification delivery

App previously only had Trace/Warn/Error. Add Info logs on webhook registration,
notification dispatch success, and delivery to make operational monitoring viable.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"

# 3. Expiry watcher milestone redesign
git add notifications/expiry_watcher.go
git commit -m "$(cat <<'EOF'
feat: redesign expiry_soon to fire at 144/72/24-block milestones with deduplication

Replace the single-threshold polling loop that spammed notifications every 10 min
with a milestone-based system. Each HTLC gets at most one notification per
milestone (warning → urgent → critical), tracked by (pubkey, paymentHash, threshold)
key. Stale entries are evicted when HTLCs settle or expire.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"

# 4. JIT interceptor: COMPLETED notification + log fix
git add interceptor/intercept_handler.go
git commit -m "$(cat <<'EOF'
fix: send COMPLETED notification and fix misleading log in JIT intercept path

JIT channel-open success path never fired order_state_changed COMPLETED.
Also fix log that claimed "notification sent" when notifyResult != nil actually
means the notification failed or the peer did not come online.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"

# 5. Dependency upgrades
git add go.mod go.sum
git commit -m "$(cat <<'EOF'
chore: upgrade public Go dependencies to latest compatible versions

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"

# 6. LND v21 regression fix - compile + runtime JIT correctness
git add lnd/interceptor.go lnd/client.go lsps/server.go cmd/flspd/run.go VERSION
git commit -m "$(cat <<'EOF'
fix: resolve flnd v0.1.21-beta HTLC interceptor regression for JIT channels

flnd v21 removed OutgoingRequestedChanId from ForwardHtlcInterceptResponse.
Simply dropping the field breaks lokihub JIT payments because phantom SCIDs
(used in LSPS2 route hints) are unknown to LND's switch, causing
FailUnknownNextPeer on HTLC resume.

Fix uses XAddLocalChanAliases (new v21 RPC) to register the phantom SCID
as a local alias for the real JIT channel SCID immediately before resuming.
This propagates via linkAliasUpdater -> switch.UpdateLinkAliases -> baseIndex,
allowing getLinkByMapping to find the channel via the alias path (which has
no IsUnadvertised check, so private JIT channels are routable).

Critically, phantom SCIDs must be in LND's alias block-height range
[16,000,000, 16,250,000) for XAddLocalChanAliases to accept them and for
IsAlias() to route through the correct switch path. Fix generateRandomScid()
to always produce SCIDs in this range; lokihub's ParseSCID is range-agnostic.

Also rename constants: MaxFlcFundingAmount -> MaxFlokicoinFundingAmount,
MaxFlcFundingAmountWumbo -> MaxFlokicoinFundingAmountWumbo.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
