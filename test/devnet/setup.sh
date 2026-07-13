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

echo "Devnet data dirs ready under test/devnet/data/ (gitignored)."
echo "Next: docker compose up -d"
