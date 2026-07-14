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

echo "Devnet data dirs ready under test/devnet/data/ (gitignored)."
echo "Next: docker compose up -d"
