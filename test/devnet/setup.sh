#!/usr/bin/env bash
# One-time local setup: creates the (gitignored) data directories and
# seeds them with the sample configs, mirroring the top-level `just
# setup-backend` convention for the single-instance mainnet dev stack.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

mkdir -p data/chain data/flnd-lsp data/flnd-client data/flnd-payer

[ -f data/chain/lokid.conf ] || cp lokid.conf.sample data/chain/lokid.conf
for role in lsp client payer; do
  [ -f "data/flnd-${role}/flnd.conf" ] || cp flnd.conf.sample "data/flnd-${role}/flnd.conf"
done

# LSP-role-only: flspd attaches as an HTLC interceptor via flnd's
# SubscribeHtlcInterceptor RPC. Without this flag, HTLCs only route through
# the interceptor stream while one happens to be attached; with it, every
# forward always waits for flspd's decision, matching how a real
# LSP-facing flnd node needs to behave. client/payer don't need it.
# Must land BEFORE the [protocol] section header, or flnd's config parser
# treats it as an (unknown) protocol.* option instead of a top-level one.
grep -q '^requireinterceptor=' data/flnd-lsp/flnd.conf || \
  sed -i '/^\[protocol\]/i requireinterceptor=true' data/flnd-lsp/flnd.conf

# Payer-role-only: strip option-scid-alias/zero-conf, which flnd.conf.sample
# otherwise enables identically on all three roles. These are feature bits
# negotiated per peer *connection*, not per channel -- with both sides
# advertising option-scid-alias, flnd's funding manager (funding/manager.go,
# NegotiatedAliasFeature/DeleteAliasEdge/addToGraph) treats even an ordinary
# (non-zero-conf, not explicitly private) payer<->lsp channel as an
# scid-alias channel, and its delete-then-re-add-to-graph handling for that
# case leaves the edge missing from the graph DB -- so the channel_update
# each side generates for it fails with "edge not found" and never
# propagates to the other peer, which is exactly what breaks payment
# pathfinding through that channel (see BACKLOG.md T4-1's gossip-propagation
# finding). The LSP<->client leg is unaffected: it needs zero-conf/alias and
# still gets it, since that negotiation is independent, per the lsp<->client
# connection. Root-caused against flokiorg/flnd's discovery/gossiper.go +
# funding/manager.go; the payer role simply doesn't need these features at
# all, so disabling them here is a devnet-topology fix, not a flnd patch.
sed -i \
  -e '/^protocol\.option-scid-alias=true/d' \
  -e '/^protocol\.zero-conf=true/d' \
  data/flnd-payer/flnd.conf

echo "Devnet data dirs ready under test/devnet/data/ (gitignored)."
echo "Next: docker compose up -d"
